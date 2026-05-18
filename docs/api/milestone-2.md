# Milestone 2 API

Milestone 2 provides the business-intent side of Twins:

- Business account creation
- API key issuance
- Wallet registry
- USDC payment request creation
- Payment request status/listing
- Idempotency keys
- Audit logs
- Basic payment request dashboard

The local Go service uses in-memory storage. The durable Postgres schema is in:

```text
migrations/0001_milestone2_core.sql
```

## Run Locally

```powershell
go run ./cmd/twins-api
```

Default address:

```text
http://localhost:8080
```

Override:

```powershell
$env:TWINS_HTTP_ADDR=":8081"
go run ./cmd/twins-api
```

## Create Business

```powershell
Invoke-RestMethod -Method Post `
  -Uri http://localhost:8080/v1/businesses `
  -ContentType "application/json" `
  -Body '{"name":"Acme Labs"}'
```

The response includes an API key. Store it somewhere temporary for local testing:

```powershell
$apiKey = "twins_test_example"
```

## Register Wallet

```powershell
Invoke-RestMethod -Method Post `
  -Uri http://localhost:8080/v1/wallets `
  -Headers @{ Authorization = "Bearer $apiKey" } `
  -ContentType "application/json" `
  -Body '{
    "label": "Main Solana wallet",
    "chain": "solana",
    "address": "7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDH"
  }'
```

## Create Payment Request

Use the wallet ID returned by the wallet registry endpoint.

```powershell
Invoke-RestMethod -Method Post `
  -Uri http://localhost:8080/v1/payment-requests `
  -Headers @{
    Authorization = "Bearer $apiKey"
    "Idempotency-Key" = "INV-1001-001"
  } `
  -ContentType "application/json" `
  -Body '{
    "wallet_id": "wal_example",
    "customer_id": "cust_123",
    "invoice_id": "INV-1001",
    "amount": "500.00",
    "token": "USDC",
    "chain": "solana",
    "expires_at": "2026-05-20T12:00:00Z",
    "metadata": {
      "source": "local-test"
    }
  }'
```

New payment requests start in:

```text
awaiting_payment
```

## List Payment Requests

```powershell
Invoke-RestMethod -Method Get `
  -Uri http://localhost:8080/v1/payment-requests `
  -Headers @{ Authorization = "Bearer $apiKey" }
```

## Get Payment Request

```powershell
Invoke-RestMethod -Method Get `
  -Uri http://localhost:8080/v1/payment-requests/prq_example `
  -Headers @{ Authorization = "Bearer $apiKey" }
```

## List Audit Logs

```powershell
Invoke-RestMethod -Method Get `
  -Uri http://localhost:8080/v1/audit-logs `
  -Headers @{ Authorization = "Bearer $apiKey" }
```

## Dashboard

Open:

```text
http://localhost:8080/dashboard
```

Paste the API key to view wallet count and payment requests.

## Milestone 2 Done Criteria

- A business can be created.
- A business receives an API key.
- A business can register a Solana wallet.
- A business can create a USDC payment request.
- Payment request data includes amount, token, chain, expiry, metadata, customer ID, invoice ID, destination wallet, and status.
- Duplicate payment request creation with the same idempotency key returns the original request.
- Reusing an idempotency key with a different body returns a conflict.
- The dashboard lists created payment requests.
- Sensitive actions are audit logged.

