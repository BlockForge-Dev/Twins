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
$CargoHome = Join-Path $CacheDir "cargo-home"
$CargoTarget = Join-Path $CacheDir "cargo-target"
$RunId = Get-Date -Format "yyyyMMdd-HHmmss-fff"
$ApiBinary = Join-Path $CacheDir "twins-api-local-$RunId.exe"
$ApiLog = $null

$WalletAddress = "7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDH"
$FixturePath = "workers/solana-watcher/fixtures/inbound_usdc_transfer.json"
$WrongTokenFixturePath = "workers/solana-watcher/fixtures/wrong_token_transfer.json"

New-Item -ItemType Directory -Force -Path $CacheDir, $GoCache, $CargoHome, $CargoTarget | Out-Null

$env:GOCACHE = $GoCache
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
$serverCommand = "`$env:TWINS_HTTP_ADDR=':$Port'; Set-Location '$Root'; & '$ApiBinary' *> '$ApiLog'"
$server = Start-Process -WindowStyle Hidden -FilePath "powershell" -ArgumentList @(
    "-NoProfile",
    "-ExecutionPolicy",
    "Bypass",
    "-Command",
    $serverCommand
) -PassThru

Wait-ForHealth -BaseUrl $BaseUrl
$ListenerPid = Get-ListeningPid -ListeningPort $Port
if ($null -eq $ListenerPid) {
    $ListenerPid = $server.Id
}

Write-Step "Creating local business and wallet"
$businessPayload = @{ name = "Local Verification" } | ConvertTo-Json
$business = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/businesses" -ContentType "application/json" -Body $businessPayload
$apiKey = $business.api_key

$walletPayload = @{
    label   = "Fixture Solana wallet"
    chain   = "solana"
    address = $WalletAddress
} | ConvertTo-Json
$wallet = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/wallets" -Headers @{ Authorization = "Bearer $apiKey" } -ContentType "application/json" -Body $walletPayload

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

Write-Host ""
Write-Host "Local verification passed." -ForegroundColor Green
Write-Host "API:       $BaseUrl"
Write-Host "Dashboard: $BaseUrl/dashboard"
Write-Host "Server PID: $ListenerPid"
Write-Host "Business:  $($business.business.id)"
Write-Host "Wallet:    $($wallet.wallet.id)"
Write-Host "API key:   $apiKey"
Write-Host "Request:   $($matchedRequest.id) [$($matchedRequest.status)]"
Write-Host "Tx status: $($tx.status)"
Write-Host "Signature: $($tx.signature)"
Write-Host "Log file:  $ApiLog"
Write-Host "Binary:    $ApiBinary"
Write-Host ""
Write-Host "To stop the API server:"
Write-Host "Stop-Process -Id $ListenerPid"
