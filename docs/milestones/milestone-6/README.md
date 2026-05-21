# Milestone 6: Reconciliation Dashboard And Export

Milestone 6 turns payment truth into an operator-ready settlement report.

Before this milestone, Twins could prove payments, produce receipt timelines, and log webhook delivery. This milestone lets a business close a period by reconciling payment requests, on-chain transaction evidence, matches, and exceptions into one report.

## What This Milestone Includes

- Reconciliation runs for a reporting period
- Matched vs unmatched transaction counts
- Payment request vs transaction settlement rows
- Exception summary for the period
- Wallet-level observed inbound USDC snapshots
- CSV settlement export
- JSON settlement export
- Dashboard sections for reconciliation runs, wallet snapshots, settlement rows, and exports

## API Endpoints

```text
GET  /v1/reconciliation-runs
POST /v1/reconciliation-runs
GET  /v1/reconciliation-runs/{id}
GET  /v1/exports
POST /v1/exports
GET  /v1/exports/{id}
```

Example reconciliation request:

```json
{
  "period_start": "2026-05-18T00:00:00Z",
  "period_end": "2026-05-19T00:00:00Z"
}
```

Example export request:

```json
{
  "reconciliation_run_id": "rec_123",
  "format": "csv"
}
```

## Important Boundary

The wallet snapshot in this MVP is an observed inbound USDC snapshot from the evidence Twins has ingested. It is not yet a live RPC wallet balance. A live end-of-day chain balance can be added later when the reconciliation service connects directly to chain balance providers.

## Durable Schema

Migration:

```text
migrations/0005_milestone6_reconciliation_exports.sql
```

## Done Criteria

- Businesses can create a reconciliation run for a time period.
- The run shows payment requests, transactions, matches, unmatched transactions, and exceptions.
- The dashboard shows settlement rows and wallet snapshots.
- Businesses can create a CSV or JSON settlement export.
- A finance/operator person can review the period without manually checking wallet explorer for every transaction.
