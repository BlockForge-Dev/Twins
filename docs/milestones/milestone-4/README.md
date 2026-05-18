# Milestone 4: Matching Engine And Exception Engine

Milestone 4 connects business intent to on-chain evidence.

Before this milestone, Twins could create payment requests and verify Solana USDC transaction evidence, but those records were separate. This milestone makes the system decide whether a transaction satisfies a payment request or should become an operational exception.

## What This Milestone Includes

- Transaction-to-payment-request matching
- Exact payment confirmation
- Underpayment detection
- Overpayment detection
- Expired payment detection
- Orphan transaction detection
- Ambiguous match detection
- Transaction match records
- Exception records
- Manual exception resolution
- Dashboard sections for matches and exceptions

## Conservative Matching Rule

Twins only auto-confirms a payment when exactly one active payment request matches the transaction by:

- Business
- Wallet
- Chain
- Token
- Amount
- Destination
- Finality
- Expiry window

If the system cannot prove the match safely, it creates an exception instead of guessing.

## API Endpoints

```text
GET  /v1/transaction-matches
GET  /v1/exceptions
POST /v1/exceptions/{id}/resolve
```

Existing transaction ingestion now also triggers the matching engine:

```text
POST /v1/stablecoin-transactions
```

## State Changes

Exact confirmed payment:

```text
payment_request: awaiting_payment -> confirmed
transaction: confirmed_onchain -> matched_to_request
match: confirmed
```

Underpaid payment:

```text
payment_request: awaiting_payment -> underpaid
transaction: confirmed_onchain -> matched_to_request
match: underpaid
exception: underpaid/open
```

Overpaid payment:

```text
payment_request: awaiting_payment -> overpaid
transaction: confirmed_onchain -> matched_to_request
match: overpaid
exception: overpaid/open
```

Expired payment:

```text
payment_request: awaiting_payment -> expired
transaction: confirmed_onchain -> matched_to_request
match: expired
exception: expired/open
```

Orphan transaction:

```text
transaction: confirmed_onchain -> orphan
exception: orphan/open
```

Manual resolution:

```text
exception: open -> resolved
payment_request: underpaid/overpaid/expired -> manually_resolved
```

## Durable Schema

Migration:

```text
migrations/0003_milestone4_matches_exceptions.sql
```

## Local Verification

Run:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-local.ps1
```

The local verifier now confirms the happy path:

- A USDC payment request is created.
- The Rust watcher emits verified transaction evidence.
- The Go API ingests the evidence.
- The matching engine confirms the payment request.
- A transaction match is created.
- No exception is created for the exact payment.

## Done Criteria

- Correct payments automatically match to the correct request.
- Underpaid payments become underpaid, not failed.
- Overpaid payments become overpaid, not silently accepted.
- Payments after expiry become expired exceptions.
- Incoming transactions without matching requests become orphan exceptions.
- Operators can manually resolve exceptions with a reason.

