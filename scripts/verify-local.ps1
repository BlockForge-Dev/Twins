param(
    [int]$Port = 8084,
    [switch]$SkipTests
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

$CacheDir = Join-Path $Root ".cache"
$GoCache = Join-Path $CacheDir "go-build"
$GoTmp = Join-Path $CacheDir "go-tmp"
$CargoHome = Join-Path $CacheDir "cargo-home"
$CargoTarget = Join-Path $CacheDir "cargo-target"
$RunId = Get-Date -Format "yyyyMMdd-HHmmss-fff"
$ApiBinary = Join-Path $CacheDir "twins-api-local-$RunId.exe"
$DataPath = Join-Path $CacheDir "twins-api-local-$RunId.json"
$ApiLog = $null

$WalletAddress = "7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDH"
$FixturePath = "workers/solana-watcher/fixtures/inbound_usdc_transfer.json"
$WrongTokenFixturePath = "workers/solana-watcher/fixtures/wrong_token_transfer.json"

New-Item -ItemType Directory -Force -Path $CacheDir, $GoCache, $GoTmp, $CargoHome, $CargoTarget | Out-Null

$env:GOCACHE = $GoCache
$env:GOTMPDIR = $GoTmp
$env:TMP = $GoTmp
$env:TEMP = $GoTmp
$env:GOTELEMETRY = "off"
$env:CARGO_HOME = $CargoHome
$env:CARGO_TARGET_DIR = $CargoTarget

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Invoke-Native {
    param(
        [string]$FilePath,
        [string[]]$Arguments
    )

    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$FilePath exited with code $LASTEXITCODE"
    }
}

function Test-PortOpen {
    param([int]$CandidatePort)

    $client = [System.Net.Sockets.TcpClient]::new()
    try {
        $async = $client.BeginConnect("127.0.0.1", $CandidatePort, $null, $null)
        if (-not $async.AsyncWaitHandle.WaitOne(200)) {
            return $false
        }
        $client.EndConnect($async)
        return $true
    }
    catch {
        return $false
    }
    finally {
        $client.Close()
    }
}

function Find-FreePort {
    param([int]$StartPort)

    for ($candidate = $StartPort; $candidate -lt ($StartPort + 40); $candidate++) {
        if (-not (Test-PortOpen -CandidatePort $candidate)) {
            return $candidate
        }
    }

    throw "No free port found starting at $StartPort"
}

function Wait-ForHealth {
    param([string]$BaseUrl)

    for ($i = 0; $i -lt 40; $i++) {
        try {
            $health = Invoke-RestMethod -Method Get -Uri "$BaseUrl/healthz" -TimeoutSec 2
            if ($health.status -eq "ok") {
                return
            }
        }
        catch {
            Start-Sleep -Milliseconds 250
        }
    }

    if ($ApiLog -and (Test-Path $ApiLog)) {
        Write-Host ""
        Write-Host "API log:" -ForegroundColor Yellow
        Get-Content $ApiLog
    }

    throw "API did not become healthy at $BaseUrl"
}

function Wait-ForReady {
    param([string]$BaseUrl)

    for ($i = 0; $i -lt 40; $i++) {
        try {
            $ready = Invoke-RestMethod -Method Get -Uri "$BaseUrl/readyz" -TimeoutSec 2
            if ($ready.status -eq "ready") {
                return
            }
        }
        catch {
            Start-Sleep -Milliseconds 250
        }
    }

    throw "API did not become ready at $BaseUrl"
}

function Invoke-ExpectHttpStatus {
    param(
        [scriptblock]$Script,
        [int]$StatusCode,
        [string]$Description
    )

    try {
        & $Script | Out-Null
        throw "Expected $Description to return HTTP $StatusCode, but it succeeded"
    }
    catch {
        $response = $_.Exception.Response
        if ($null -eq $response) {
            throw "Expected $Description to return HTTP $StatusCode, but got non-HTTP error: $($_.Exception.Message)"
        }

        $actualStatus = [int]$response.StatusCode
        if ($actualStatus -ne $StatusCode) {
            throw "Expected $Description to return HTTP $StatusCode, got $actualStatus"
        }
    }
}

function Get-ListeningPid {
    param([int]$ListeningPort)

    $lines = netstat -ano | Select-String "LISTENING" | Select-String ":$ListeningPort"
    foreach ($line in $lines) {
        if ($line.Line -match "^\s*TCP\s+\S+:$ListeningPort\s+\S+\s+LISTENING\s+(\d+)\s*$") {
            return [int]$Matches[1]
        }
    }

    return $null
}

Write-Step "Checking local tools"
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go is not installed or not on PATH"
}
if (-not (Get-Command cargo -ErrorAction SilentlyContinue)) {
    throw "Cargo is not installed or not on PATH"
}

if (-not $SkipTests) {
    Write-Step "Running Go tests"
    Invoke-Native -FilePath "go" -Arguments @("test", "./...")

    Write-Step "Running Rust tests"
    Invoke-Native -FilePath "cargo" -Arguments @("test", "--workspace")
}

Write-Step "Building local API binary"
Invoke-Native -FilePath "go" -Arguments @("build", "-o", $ApiBinary, "./cmd/twins-api")

$Port = Find-FreePort -StartPort $Port
$BaseUrl = "http://localhost:$Port"
$PostUrl = "$BaseUrl/v1/stablecoin-transactions"
$ApiLog = Join-Path $CacheDir "twins-api-local-$Port-$RunId.log"

Write-Step "Starting Twins API on $BaseUrl"
$serverCommand = "`$env:TWINS_HTTP_ADDR=':$Port'; `$env:TWINS_DATA_PATH='$DataPath'; Set-Location '$Root'; & '$ApiBinary' *> '$ApiLog'"
$server = Start-Process -WindowStyle Hidden -FilePath "powershell" -ArgumentList @(
    "-NoProfile",
    "-ExecutionPolicy",
    "Bypass",
    "-Command",
    $serverCommand
) -PassThru

Wait-ForHealth -BaseUrl $BaseUrl
Wait-ForReady -BaseUrl $BaseUrl
$ListenerPid = Get-ListeningPid -ListeningPort $Port
if ($null -eq $ListenerPid) {
    $ListenerPid = $server.Id
}

Write-Step "Creating local business and wallet"
$businessPayload = @{ name = "Local Verification" } | ConvertTo-Json
$business = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/businesses" -ContentType "application/json" -Body $businessPayload
$apiKey = $business.api_key

Write-Step "Verifying security controls and tenant readiness"
$securityPolicy = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/security-policy" -Headers @{ Authorization = "Bearer $apiKey" }
if ($securityPolicy.security_policy.require_scoped_api_keys -ne $true) {
    throw "Expected security policy require_scoped_api_keys true"
}
if ($securityPolicy.security_policy.rate_limit_per_minute -lt 10) {
    throw "Expected security policy rate_limit_per_minute to be at least 10"
}

$userPayload = @{
    email = "ops.local@example.com"
    name  = "Local Ops"
    role  = "operator"
} | ConvertTo-Json
$createdUser = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/users" -Headers @{ Authorization = "Bearer $apiKey" } -ContentType "application/json" -Body $userPayload
if ($createdUser.user.role -ne "operator") {
    throw "Expected created user role operator, got $($createdUser.user.role)"
}
$users = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/users" -Headers @{ Authorization = "Bearer $apiKey" }
$storedUsers = @($users.users)
if ($storedUsers.Count -ne 2) {
    throw "Expected 2 users including owner and operator, got $($storedUsers.Count)"
}

$limitedKeyPayload = @{
    name   = "Local payment request reader"
    scopes = @("payment_requests:read")
} | ConvertTo-Json -Depth 4
$limitedKeyResult = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/api-keys" -Headers @{ Authorization = "Bearer $apiKey" } -ContentType "application/json" -Body $limitedKeyPayload
$limitedApiKey = $limitedKeyResult.secret
if ([string]::IsNullOrWhiteSpace($limitedApiKey)) {
    throw "Expected limited API key secret"
}
if ($limitedKeyResult.api_key.PSObject.Properties.Match("secret").Count -ne 0) {
    throw "API key metadata must not expose the raw secret"
}

Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/payment-requests" -Headers @{ Authorization = "Bearer $limitedApiKey" } | Out-Null
Invoke-ExpectHttpStatus -StatusCode 403 -Description "limited API key wallet write" -Script {
    $forbiddenWalletPayload = @{
        label   = "Forbidden wallet"
        chain   = "solana"
        address = "8xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDH"
    } | ConvertTo-Json
    Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/wallets" -Headers @{ Authorization = "Bearer $limitedApiKey" } -ContentType "application/json" -Body $forbiddenWalletPayload
}

$apiKeys = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/api-keys" -Headers @{ Authorization = "Bearer $apiKey" }
$storedApiKeys = @($apiKeys.api_keys)
if ($storedApiKeys.Count -ne 2) {
    throw "Expected 2 API keys before revoke, got $($storedApiKeys.Count)"
}

$revokedKey = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/api-keys/$($limitedKeyResult.api_key.id)/revoke" -Headers @{ Authorization = "Bearer $apiKey" } -ContentType "application/json" -Body "{}"
if ($null -eq $revokedKey.api_key.revoked_at) {
    throw "Expected revoked API key to have revoked_at"
}
Invoke-ExpectHttpStatus -StatusCode 401 -Description "revoked API key read" -Script {
    Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/payment-requests" -Headers @{ Authorization = "Bearer $limitedApiKey" }
}

$policyPatchPayload = @{
    rate_limit_per_minute     = 120
    data_retention_days       = 400
    access_log_retention_days = 120
    webhook_retention_days    = 45
    incident_retention_days   = 400
} | ConvertTo-Json
$updatedPolicy = Invoke-RestMethod -Method Patch -Uri "$BaseUrl/v1/security-policy" -Headers @{ Authorization = "Bearer $apiKey" } -ContentType "application/json" -Body $policyPatchPayload
if ($updatedPolicy.security_policy.rate_limit_per_minute -ne 120) {
    throw "Expected updated security policy rate_limit_per_minute 120, got $($updatedPolicy.security_policy.rate_limit_per_minute)"
}

$incidentPayload = @{
    title       = "Local webhook endpoint refused connection"
    severity    = "medium"
    description = "Expected local verification failure because no receiver is listening on port 9."
} | ConvertTo-Json
$incident = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/incidents" -Headers @{ Authorization = "Bearer $apiKey" } -ContentType "application/json" -Body $incidentPayload
if ($incident.incident.status -ne "open") {
    throw "Expected incident status open, got $($incident.incident.status)"
}
$resolveIncidentPayload = @{ summary = "Failure is expected in local webhook verification." } | ConvertTo-Json
$resolvedIncident = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/incidents/$($incident.incident.id)/resolve" -Headers @{ Authorization = "Bearer $apiKey" } -ContentType "application/json" -Body $resolveIncidentPayload
if ($resolvedIncident.incident.status -ne "resolved") {
    throw "Expected incident status resolved, got $($resolvedIncident.incident.status)"
}

$walletPayload = @{
    label   = "Fixture Solana wallet"
    chain   = "solana"
    address = $WalletAddress
} | ConvertTo-Json
$wallet = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/wallets" -Headers @{ Authorization = "Bearer $apiKey" } -ContentType "application/json" -Body $walletPayload

Write-Step "Creating local webhook subscription"
$webhookPayload = @{
    url         = "http://127.0.0.1:9/twins-local-webhook"
    secret      = "whsec_local_verify"
    event_types = @("payment.confirmed")
} | ConvertTo-Json -Depth 4
$webhookSubscription = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/webhook-subscriptions" -Headers @{ Authorization = "Bearer $apiKey" } -ContentType "application/json" -Body $webhookPayload

Write-Step "Creating local USDC payment request"
$fixture = Get-Content -Raw -Path $FixturePath | ConvertFrom-Json
$fixtureTime = [DateTimeOffset]::FromUnixTimeSeconds([int64]$fixture.blockTime).UtcDateTime
if ($fixtureTime -lt [DateTime]::UtcNow) {
    $fixtureTime = [DateTime]::UtcNow
}
$expiresAt = $fixtureTime.AddDays(1).ToString("yyyy-MM-ddTHH:mm:ssZ")
$paymentRequestPayload = @{
    wallet_id   = $wallet.wallet.id
    customer_id = "cust_local_verify"
    invoice_id  = "INV-LOCAL-VERIFY"
    amount      = "500.00"
    token       = "USDC"
    chain       = "solana"
    expires_at  = $expiresAt
    metadata    = @{
        source = "verify-local"
        run_id = $RunId
    }
} | ConvertTo-Json -Depth 4
$paymentRequest = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/payment-requests" -Headers @{
    Authorization       = "Bearer $apiKey"
    "Idempotency-Key"   = "verify-local-$RunId"
} -ContentType "application/json" -Body $paymentRequestPayload

$paymentRequests = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/payment-requests" -Headers @{ Authorization = "Bearer $apiKey" }
$storedPaymentRequests = @($paymentRequests.payment_requests)
if ($storedPaymentRequests.Count -ne 1) {
    throw "Expected 1 stored payment request, got $($storedPaymentRequests.Count)"
}
if ($paymentRequest.payment_request.status -ne "awaiting_payment") {
    throw "Expected payment request status awaiting_payment, got $($paymentRequest.payment_request.status)"
}

Write-Step "Verifying inbound USDC fixture and posting evidence"
Invoke-Native -FilePath "cargo" -Arguments @(
    "run",
    "-q",
    "-p",
    "twins-solana-watcher",
    "--",
    "verify-fixture",
    "--input",
    $FixturePath,
    "--wallet",
    $WalletAddress,
    "--post-url",
    $PostUrl,
    "--api-key",
    $apiKey
)

$transactions = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/stablecoin-transactions" -Headers @{ Authorization = "Bearer $apiKey" }
$storedTransactions = @($transactions.stablecoin_transactions)
if ($storedTransactions.Count -ne 1) {
    throw "Expected 1 stored stablecoin transaction, got $($storedTransactions.Count)"
}

$tx = $storedTransactions[0]
if ($tx.status -ne "matched_to_request") {
    throw "Expected transaction status matched_to_request, got $($tx.status)"
}
if ($tx.token -ne "USDC") {
    throw "Expected token USDC, got $($tx.token)"
}
if ($tx.amount_atomic -ne "500000000") {
    throw "Expected amount_atomic 500000000, got $($tx.amount_atomic)"
}

$matchedRequests = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/payment-requests" -Headers @{ Authorization = "Bearer $apiKey" }
$matchedPaymentRequests = @($matchedRequests.payment_requests)
if ($matchedPaymentRequests.Count -ne 1) {
    throw "Expected 1 matched payment request, got $($matchedPaymentRequests.Count)"
}
$matchedRequest = $matchedPaymentRequests[0]
if ($matchedRequest.status -ne "confirmed") {
    throw "Expected matched payment request status confirmed, got $($matchedRequest.status)"
}

$matches = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/transaction-matches" -Headers @{ Authorization = "Bearer $apiKey" }
$storedMatches = @($matches.transaction_matches)
if ($storedMatches.Count -ne 1) {
    throw "Expected 1 transaction match, got $($storedMatches.Count)"
}
if ($storedMatches[0].status -ne "confirmed") {
    throw "Expected transaction match status confirmed, got $($storedMatches[0].status)"
}

$exceptions = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/exceptions" -Headers @{ Authorization = "Bearer $apiKey" }
$storedExceptions = @($exceptions.exceptions)
if ($storedExceptions.Count -ne 0) {
    throw "Expected 0 exceptions for exact payment, got $($storedExceptions.Count)"
}

Write-Step "Verifying receipt timeline and webhook delivery log"
$receiptEvents = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/receipt-events" -Headers @{ Authorization = "Bearer $apiKey" }
$storedReceiptEvents = @($receiptEvents.receipt_events)
if ($storedReceiptEvents.Count -ne 5) {
    throw "Expected 5 receipt events, got $($storedReceiptEvents.Count)"
}
if ($storedReceiptEvents[0].type -ne "payment_request.created") {
    throw "Expected first receipt event payment_request.created, got $($storedReceiptEvents[0].type)"
}
if ($storedReceiptEvents[-1].type -ne "payment.confirmed") {
    throw "Expected final receipt event payment.confirmed, got $($storedReceiptEvents[-1].type)"
}
$confirmedEvents = @($storedReceiptEvents | Where-Object { $_.type -eq "payment.confirmed" })
if ($confirmedEvents.Count -ne 1) {
    throw "Expected one payment.confirmed receipt event, got $($confirmedEvents.Count)"
}

$privateReceipt = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/payment-requests/$($matchedRequest.id)/receipt" -Headers @{ Authorization = "Bearer $apiKey" }
$privateReceiptEvents = @($privateReceipt.receipt.events)
if ($privateReceiptEvents.Count -ne 5) {
    throw "Expected private receipt to have 5 events, got $($privateReceiptEvents.Count)"
}
if ($privateReceiptEvents[-1].type -ne "payment.confirmed") {
    throw "Expected final private receipt event payment.confirmed, got $($privateReceiptEvents[-1].type)"
}

$publicReceipt = Invoke-RestMethod -Method Get -Uri "$BaseUrl/receipts/$($matchedRequest.id)"
$publicReceiptEvents = @($publicReceipt.receipt.events)
if ($publicReceiptEvents.Count -ne 5) {
    throw "Expected public receipt to have 5 events, got $($publicReceiptEvents.Count)"
}

$webhookDeliveries = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/webhook-deliveries" -Headers @{ Authorization = "Bearer $apiKey" }
$storedWebhookDeliveries = @($webhookDeliveries.webhook_deliveries)
if ($storedWebhookDeliveries.Count -ne 1) {
    throw "Expected 1 webhook delivery, got $($storedWebhookDeliveries.Count)"
}
$webhookDelivery = $storedWebhookDeliveries[0]
if ($webhookDelivery.event_type -ne "payment.confirmed") {
    throw "Expected webhook event_type payment.confirmed, got $($webhookDelivery.event_type)"
}
if (-not $webhookDelivery.signature.StartsWith("sha256=")) {
    throw "Expected webhook signature to start with sha256="
}
if ($webhookDelivery.attempts -ne 1) {
    throw "Expected webhook attempts 1, got $($webhookDelivery.attempts)"
}

$replayedWebhook = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/webhook-deliveries/$($webhookDelivery.id)/replay" -Headers @{ Authorization = "Bearer $apiKey" } -ContentType "application/json" -Body "{}"
if ($replayedWebhook.webhook_delivery.attempts -ne 2) {
    throw "Expected replayed webhook attempts 2, got $($replayedWebhook.webhook_delivery.attempts)"
}

$afterWebhookReplayRequests = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/payment-requests" -Headers @{ Authorization = "Bearer $apiKey" }
$afterWebhookReplayRequest = @($afterWebhookReplayRequests.payment_requests)[0]
if ($afterWebhookReplayRequest.status -ne "confirmed") {
    throw "Expected payment request to remain confirmed after webhook failure/replay, got $($afterWebhookReplayRequest.status)"
}

Write-Step "Creating reconciliation run and CSV settlement export"
$periodStart = ([DateTime]::UtcNow.AddHours(-1)).ToString("yyyy-MM-ddTHH:mm:ssZ")
$periodEnd = ([DateTime]::UtcNow.AddHours(1)).ToString("yyyy-MM-ddTHH:mm:ssZ")
$reconciliationPayload = @{
    period_start = $periodStart
    period_end   = $periodEnd
} | ConvertTo-Json
$reconciliation = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/reconciliation-runs" -Headers @{ Authorization = "Bearer $apiKey" } -ContentType "application/json" -Body $reconciliationPayload
$reconciliationRun = $reconciliation.reconciliation_report.reconciliation_run
if ($reconciliationRun.status -ne "completed") {
    throw "Expected reconciliation run status completed, got $($reconciliationRun.status)"
}
if ($reconciliationRun.total_payment_requests -ne 1) {
    throw "Expected reconciliation total_payment_requests 1, got $($reconciliationRun.total_payment_requests)"
}
if ($reconciliationRun.total_transactions -ne 1) {
    throw "Expected reconciliation total_transactions 1, got $($reconciliationRun.total_transactions)"
}
if ($reconciliationRun.matched_transactions -ne 1) {
    throw "Expected reconciliation matched_transactions 1, got $($reconciliationRun.matched_transactions)"
}
if ($reconciliationRun.unmatched_transactions -ne 0) {
    throw "Expected reconciliation unmatched_transactions 0, got $($reconciliationRun.unmatched_transactions)"
}
if ($reconciliationRun.total_received_usdc -ne "500") {
    throw "Expected reconciliation total_received_usdc 500, got $($reconciliationRun.total_received_usdc)"
}

$settlementRows = @($reconciliation.reconciliation_report.rows)
if ($settlementRows.Count -ne 1) {
    throw "Expected 1 settlement row, got $($settlementRows.Count)"
}
if ($settlementRows[0].reconciliation_status -ne "reconciled") {
    throw "Expected settlement row reconciliation_status reconciled, got $($settlementRows[0].reconciliation_status)"
}

$walletSnapshots = @($reconciliation.reconciliation_report.wallet_snapshots)
if ($walletSnapshots.Count -ne 1) {
    throw "Expected 1 wallet snapshot, got $($walletSnapshots.Count)"
}
if ($walletSnapshots[0].observed_inbound_amount -ne "500") {
    throw "Expected wallet snapshot observed_inbound_amount 500, got $($walletSnapshots[0].observed_inbound_amount)"
}

$reconciliationRuns = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/reconciliation-runs" -Headers @{ Authorization = "Bearer $apiKey" }
$storedReconciliationRuns = @($reconciliationRuns.reconciliation_runs)
if ($storedReconciliationRuns.Count -ne 1) {
    throw "Expected 1 reconciliation run, got $($storedReconciliationRuns.Count)"
}

$storedReport = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/reconciliation-runs/$($reconciliationRun.id)" -Headers @{ Authorization = "Bearer $apiKey" }
$storedSettlementRows = @($storedReport.reconciliation_report.rows)
if ($storedSettlementRows.Count -ne 1) {
    throw "Expected stored reconciliation report to have 1 row, got $($storedSettlementRows.Count)"
}

$exportPayload = @{
    reconciliation_run_id = $reconciliationRun.id
    format                = "csv"
} | ConvertTo-Json
$export = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/exports" -Headers @{ Authorization = "Bearer $apiKey" } -ContentType "application/json" -Body $exportPayload
if ($export.export.status -ne "ready") {
    throw "Expected export status ready, got $($export.export.status)"
}
if ($export.export.row_count -ne 1) {
    throw "Expected export row_count 1, got $($export.export.row_count)"
}
if (-not $export.export.content.Contains("INV-LOCAL-VERIFY")) {
    throw "Expected export content to include INV-LOCAL-VERIFY"
}

$exports = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/exports" -Headers @{ Authorization = "Bearer $apiKey" }
$storedExports = @($exports.exports)
if ($storedExports.Count -ne 1) {
    throw "Expected 1 export, got $($storedExports.Count)"
}

Write-Step "Verifying private beta onboarding, evidence, and usage metrics"
$designPartnerPayload = @{
    company_name            = "Local Beta AI Labs"
    segment                 = "AI API company"
    contact_name            = "Finance Lead"
    contact_email           = "finance.local@example.com"
    use_case                = "USDC invoice matching and reconciliation"
    status                  = "onboarding"
    agreed_to_test          = $true
    pricing_commitment      = $true
    expected_monthly_volume = 250
    notes                   = "Local verifier design partner record"
} | ConvertTo-Json
$designPartner = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/design-partners" -Headers @{ Authorization = "Bearer $apiKey" } -ContentType "application/json" -Body $designPartnerPayload
if ($designPartner.design_partner.status -ne "onboarding") {
    throw "Expected design partner status onboarding, got $($designPartner.design_partner.status)"
}

$activePartnerPayload = @{ status = "active" } | ConvertTo-Json
$activePartner = Invoke-RestMethod -Method Patch -Uri "$BaseUrl/v1/design-partners/$($designPartner.design_partner.id)" -Headers @{ Authorization = "Bearer $apiKey" } -ContentType "application/json" -Body $activePartnerPayload
if ($activePartner.design_partner.status -ne "active") {
    throw "Expected design partner status active, got $($activePartner.design_partner.status)"
}

$realTransactionEvidencePayload = @{
    design_partner_id          = $designPartner.design_partner.id
    type                       = "real_transaction"
    title                      = "Local verified USDC payment processed"
    description                = "Verifier proved payment request, transaction evidence, match, receipt, and reconciliation."
    payment_request_id         = $matchedRequest.id
    stablecoin_transaction_id  = $tx.id
} | ConvertTo-Json
$realTransactionEvidence = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/beta-evidence" -Headers @{ Authorization = "Bearer $apiKey" } -ContentType "application/json" -Body $realTransactionEvidencePayload
if ($realTransactionEvidence.beta_evidence.type -ne "real_transaction") {
    throw "Expected beta evidence type real_transaction, got $($realTransactionEvidence.beta_evidence.type)"
}

$testimonialEvidencePayload = @{
    design_partner_id = $designPartner.design_partner.id
    type              = "testimonial"
    title             = "Payment operations proof"
    quote             = "This makes USDC payment operations easier to prove."
} | ConvertTo-Json
$testimonialEvidence = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/beta-evidence" -Headers @{ Authorization = "Bearer $apiKey" } -ContentType "application/json" -Body $testimonialEvidencePayload
if ($testimonialEvidence.beta_evidence.type -ne "testimonial") {
    throw "Expected beta evidence type testimonial, got $($testimonialEvidence.beta_evidence.type)"
}

$usageMetrics = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/usage-metrics" -Headers @{ Authorization = "Bearer $apiKey" }
if ($usageMetrics.usage_metrics.payment_requests_created -ne 1) {
    throw "Expected usage payment_requests_created 1, got $($usageMetrics.usage_metrics.payment_requests_created)"
}
if ($usageMetrics.usage_metrics.transactions_matched -ne 1) {
    throw "Expected usage transactions_matched 1, got $($usageMetrics.usage_metrics.transactions_matched)"
}
if ($usageMetrics.usage_metrics.design_partners -ne 1) {
    throw "Expected usage design_partners 1, got $($usageMetrics.usage_metrics.design_partners)"
}
if ($usageMetrics.usage_metrics.beta_evidence_items -ne 2) {
    throw "Expected usage beta_evidence_items 2, got $($usageMetrics.usage_metrics.beta_evidence_items)"
}
if ($usageMetrics.usage_metrics.reconciled_business_records -ne 1) {
    throw "Expected usage reconciled_business_records 1, got $($usageMetrics.usage_metrics.reconciled_business_records)"
}

$privateBetaReport = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/private-beta-report" -Headers @{ Authorization = "Bearer $apiKey" }
if ($privateBetaReport.private_beta_report.design_partners_onboarded -ne 1) {
    throw "Expected private beta design_partners_onboarded 1, got $($privateBetaReport.private_beta_report.design_partners_onboarded)"
}
if ($privateBetaReport.private_beta_report.partners_with_real_transactions -ne 1) {
    throw "Expected private beta partners_with_real_transactions 1, got $($privateBetaReport.private_beta_report.partners_with_real_transactions)"
}
if ($privateBetaReport.private_beta_report.pricing_commitments -ne 1) {
    throw "Expected private beta pricing_commitments 1, got $($privateBetaReport.private_beta_report.pricing_commitments)"
}

Write-Step "Verifying access logs, audit logs, and incident records"
$accessLogs = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/access-logs" -Headers @{ Authorization = "Bearer $apiKey" }
$storedAccessLogs = @($accessLogs.access_logs)
if ($storedAccessLogs.Count -lt 10) {
    throw "Expected at least 10 access logs, got $($storedAccessLogs.Count)"
}
$walletForbiddenLogs = @($storedAccessLogs | Where-Object { $_.path -eq "/v1/wallets" -and $_.status_code -eq 403 })
if ($walletForbiddenLogs.Count -lt 1) {
    throw "Expected access logs to include forbidden limited-key wallet write"
}

$incidentList = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/incidents" -Headers @{ Authorization = "Bearer $apiKey" }
$storedIncidents = @($incidentList.incidents)
if ($storedIncidents.Count -ne 1) {
    throw "Expected 1 incident, got $($storedIncidents.Count)"
}
if ($storedIncidents[0].status -ne "resolved") {
    throw "Expected stored incident status resolved, got $($storedIncidents[0].status)"
}

$auditLogs = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/audit-logs" -Headers @{ Authorization = "Bearer $apiKey" }
$storedAuditLogs = @($auditLogs.audit_logs)
$requiredAuditActions = @("user.created", "api_key.created", "api_key.revoked", "incident.created", "incident.resolved", "security_policy.updated", "design_partner.created", "design_partner.updated", "beta_evidence.created")
foreach ($action in $requiredAuditActions) {
    $matchesAction = @($storedAuditLogs | Where-Object { $_.action -eq $action })
    if ($matchesAction.Count -lt 1) {
        throw "Expected audit logs to include $action"
    }
}

Write-Step "Verifying duplicate replay stays idempotent"
Invoke-Native -FilePath "cargo" -Arguments @(
    "run",
    "-q",
    "-p",
    "twins-solana-watcher",
    "--",
    "verify-fixture",
    "--input",
    $FixturePath,
    "--wallet",
    $WalletAddress,
    "--post-url",
    $PostUrl,
    "--api-key",
    $apiKey
)

$transactionsAfterReplay = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/stablecoin-transactions" -Headers @{ Authorization = "Bearer $apiKey" }
$storedAfterReplay = @($transactionsAfterReplay.stablecoin_transactions)
if ($storedAfterReplay.Count -ne 1) {
    throw "Expected duplicate replay to keep 1 stored transaction, got $($storedAfterReplay.Count)"
}

Write-Step "Verifying wrong-token path"
$wrongTokenJson = & cargo run -q -p twins-solana-watcher -- verify-fixture --input $WrongTokenFixturePath --wallet $WalletAddress
if ($LASTEXITCODE -ne 0) {
    throw "wrong-token fixture command failed with code $LASTEXITCODE"
}
$wrongToken = $wrongTokenJson | ConvertFrom-Json
$wrongTokenStable = @($wrongToken.stablecoin_transactions)
$wrongTokenRejected = @($wrongToken.rejected_transfers)
if ($wrongTokenStable.Count -ne 0) {
    throw "Expected wrong-token fixture to produce 0 stablecoin transactions"
}
if ($wrongTokenRejected.Count -ne 1 -or $wrongTokenRejected[0].reason -ne "wrong_token") {
    throw "Expected wrong-token fixture to produce one rejected transfer with reason wrong_token"
}

if (-not (Test-Path -LiteralPath $DataPath)) {
    throw "Expected persistent data file at $DataPath"
}
$dataFile = Get-Item -LiteralPath $DataPath
if ($dataFile.Length -le 0) {
    throw "Expected persistent data file to be non-empty"
}

Write-Host ""
Write-Host "Local verification passed." -ForegroundColor Green
Write-Host "API:       $BaseUrl"
Write-Host "Dashboard: $BaseUrl/dashboard"
Write-Host "Server PID: $ListenerPid"
Write-Host "Business:  $($business.business.id)"
Write-Host "Wallet:    $($wallet.wallet.id)"
Write-Host "Webhook:   $($webhookSubscription.webhook_subscription.id) [$($replayedWebhook.webhook_delivery.status), attempts=$($replayedWebhook.webhook_delivery.attempts)]"
Write-Host "API key:   $apiKey"
Write-Host "Request:   $($matchedRequest.id) [$($matchedRequest.status)]"
Write-Host "Tx status: $($tx.status)"
Write-Host "Receipt:   $($privateReceiptEvents.Count) events"
Write-Host "Recon:     $($reconciliationRun.id) [$($reconciliationRun.status), rows=$($settlementRows.Count), total=$($reconciliationRun.total_received_usdc) USDC]"
Write-Host "Export:    $($export.export.id) [$($export.export.format), rows=$($export.export.row_count)]"
Write-Host "Security:  users=$($storedUsers.Count), api_keys=$($storedApiKeys.Count), incident=$($storedIncidents[0].status), access_logs=$($storedAccessLogs.Count)"
Write-Host "Beta:      partners=$($usageMetrics.usage_metrics.design_partners), evidence=$($usageMetrics.usage_metrics.beta_evidence_items), reconciled_records=$($usageMetrics.usage_metrics.reconciled_business_records), ready=$($privateBetaReport.private_beta_report.ready_for_private_beta_evidence)"
Write-Host "Signature: $($tx.signature)"
Write-Host "Data file: $DataPath"
Write-Host "Log file:  $ApiLog"
Write-Host "Binary:    $ApiBinary"
Write-Host ""
Write-Host "To stop the API server:"
Write-Host "Stop-Process -Id $ListenerPid"
