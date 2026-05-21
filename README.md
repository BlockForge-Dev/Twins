# Twins

Twins is stablecoin payment truth infrastructure for businesses.

It helps companies turn stablecoin wallet activity into verified receipts, reconciled records, actionable exceptions, webhook events, and settlement reports.

Stablecoins move money fast. Twins explains what happened.

## Category

Stablecoin Payment Truth Infrastructure.

Twins is not a wallet, exchange, remittance app, custody provider, or full accounting suite. It sits between on-chain stablecoin movement and business operations.

## First Wedge

Twins v1 focuses on USDC payment matching, receipts, reconciliation, and exception infrastructure for businesses.

A business creates a payment request, the customer sends USDC, Twins detects and verifies the transaction, matches it to the business intent, generates a receipt, sends signed webhooks, and raises exceptions when something is wrong.

## Product Doctrine

- Wallets move the money. Twins proves what happened.
- Wallet activity is not business truth. Matched, verified, reconciled wallet activity is business truth.
- Every stablecoin payment needs a receipt.
- Unknown is not failed. Unknown requires evidence.
- Wrong amount, wrong token, wrong chain, and late payment are first-class states.
- The exception dashboard is not secondary. The exception dashboard is the product.

## V1 Scope

- USDC first.
- Solana first.
- Base or Polygon second.
- Business-owned wallets.
- Non-custodial by design.
- Developer-first APIs and webhooks.
- Operator-ready receipts, exceptions, and reconciliation exports.
- Security-ready tenant controls, scoped API keys, access logs, and incident records.
- Private beta readiness tracking for design partners, usage metrics, and evidence.

## Core Flow

```text
Business creates payment request
        |
Customer sends stablecoin
        |
Twins detects transaction
        |
Twins verifies token, amount, chain, destination, and finality
        |
Twins matches payment to invoice, customer, or order
        |
Twins generates receipt
        |
Twins sends webhook
        |
Twins reconciles payment
        |
Exception is raised if there is a mismatch
```

## Docs

- [Product Brief](docs/product-brief.md)
- [V1 Milestones](docs/v1-milestones.md)
- [Milestone 1 Validation Kit](docs/validation/README.md)
- [Milestone 2 README](docs/milestones/milestone-2/README.md)
- [Milestone 2 API Guide](docs/api/milestone-2.md)
- [Milestone 3 README](docs/milestones/milestone-3/README.md)
- [Milestone 4 README](docs/milestones/milestone-4/README.md)
- [Milestone 5 README](docs/milestones/milestone-5/README.md)
- [Milestone 6 README](docs/milestones/milestone-6/README.md)
- [Milestone 7 README](docs/milestones/milestone-7/README.md)
- [Milestone 8 README](docs/milestones/milestone-8/README.md)
- [Production Hosting Guide](docs/deployment/production.md)

## Run The API

```powershell
go run ./cmd/twins-api
```

Then open:

```text
http://localhost:8080/dashboard
```

For durable local hosting, set a data path:

```powershell
$env:TWINS_DATA_PATH=".cache/twins-store.json"
go run ./cmd/twins-api
```

For hosted production mode, also set a setup token:

```powershell
$env:TWINS_ENV="production"
$env:TWINS_DATA_PATH=".cache/twins-store.json"
$env:TWINS_BUSINESS_CREATION_TOKEN="replace-with-a-long-random-token"
go run ./cmd/twins-api
```

See the [Production Hosting Guide](docs/deployment/production.md).

## Verify Locally

Run the full local smoke test:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-local.ps1
```

This runs Go and Rust tests, starts the API on a free local port, creates a webhook subscription and USDC payment request, posts verified Solana USDC fixture evidence from the Rust watcher into the Go API, confirms the payment through the matching engine, checks receipt events and webhook delivery logs, creates a reconciliation run and CSV settlement export, verifies scoped API keys and security records, records private beta evidence, checks duplicate handling, and verifies the wrong-token rejection path.
The dashboard shows business intent, on-chain evidence, transaction matches, receipt timeline events, webhook delivery logs, reconciliation runs, wallet snapshots, settlement rows, exports, exceptions, API keys, users, security policy, incidents, access logs, usage metrics, design partners, beta evidence, and private beta readiness.

## Start Here

The first milestone is customer validation. The second milestone is the core business-intent API.

For validation, begin with the [Milestone 1 Validation Kit](docs/validation/README.md):

1. Add target companies to `docs/validation/target-accounts.csv`.
2. Use the outreach templates to book interviews.
3. Run calls with the interview script.
4. Log real pain examples in `docs/validation/pain-evidence-log.csv`.
5. Score design partners in `docs/validation/design-partner-tracker.csv`.
6. Update the scorecard every Friday.

For the first build milestone, use the [Milestone 2 README](docs/milestones/milestone-2/README.md) and [Milestone 2 API guide](docs/api/milestone-2.md).

For the first chain milestone, use the [Milestone 3 README](docs/milestones/milestone-3/README.md).

For matching and exceptions, use the [Milestone 4 README](docs/milestones/milestone-4/README.md).

For receipts and webhooks, use the [Milestone 5 README](docs/milestones/milestone-5/README.md).

For reconciliation and exports, use the [Milestone 6 README](docs/milestones/milestone-6/README.md).

For security and multi-tenant readiness, use the [Milestone 7 README](docs/milestones/milestone-7/README.md).

For private beta readiness, use the [Milestone 8 README](docs/milestones/milestone-8/README.md).
