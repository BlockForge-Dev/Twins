# Milestone 2: Core Payment Request And Wallet Registry

Milestone 2 builds the business-intent side of Twins.

This milestone does not watch blockchains yet. Its job is to let a business define what should happen before any on-chain transaction is detected.

## What This Milestone Includes

- Business account creation
- API key issuance
- API key authentication
- Solana wallet registry
- USDC payment request creation
- Payment request status
- Payment request listing
- Idempotency keys for duplicate-safe creation
- Audit logs for sensitive actions
- Basic dashboard list for payment requests
- Postgres schema for the durable data model

## What This Milestone Does Not Include

- Solana transaction watching
- USDC transfer detection
- Finality verification
- Matching engine
- Exception engine
- Receipt timeline
- Webhook delivery
- Real Postgres persistence in the Go runtime

Those begin in later milestones.

## Current Runtime

The Go API runs locally with in-memory storage.

The Postgres schema exists in:

```text
migrations/0001_milestone2_core.sql
```

This lets the API behavior move quickly while still preserving the database contract we will wire up next.

## Run

```powershell
go run ./cmd/twins-api
```

Default URL:

```text
http://localhost:8080
```

If that port is busy:

```powershell
$env:TWINS_HTTP_ADDR=":8082"
go run ./cmd/twins-api
```

Dashboard:

```text
http://localhost:8080/dashboard
```

or, if using port 8082:

```text
http://localhost:8082/dashboard
```

## API Guide

Use the API guide for local request examples:

```text
docs/api/milestone-2.md
```

## Core Endpoints

```text
GET  /healthz
POST /v1/businesses
GET  /v1/wallets
POST /v1/wallets
GET  /v1/payment-requests
POST /v1/payment-requests
GET  /v1/payment-requests/{id}
GET  /v1/audit-logs
GET  /dashboard
```

## Payment Request Status

New payment requests are created as:

```text
awaiting_payment
```

Milestone 3 will introduce on-chain detection and move requests through later states.

## Done Criteria

- A business can be created.
- A business receives an API key.
- A business can register a Solana wallet.
- A business can create a USDC payment request.
- The payment request stores amount, token, chain, expiry, metadata, customer reference, invoice reference, destination wallet, and status.
- Duplicate creation with the same idempotency key returns the original request.
- Reusing an idempotency key with a different body returns a conflict.
- The dashboard lists created payment requests.
- Sensitive actions are audit logged.
- Tests pass with `go test ./...`.

## Verification

Run:

```powershell
$env:GOCACHE="C:\Users\hp\Twins\.cache\go-build"
$env:GOTELEMETRY="off"
go test ./...
```

Expected result:

```text
ok twins/internal/api
ok twins/internal/core
```

## Next Milestone

Milestone 3 starts the Rust/Solana side:

- Solana wallet watcher
- USDC transaction detection
- Confirmation/finality tracking
- Destination wallet verification
- Amount verification
- Token mint verification
- Transaction evidence storage

