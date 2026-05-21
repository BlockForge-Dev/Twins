# Milestone 5: Receipt Timeline And Webhook Delivery

Milestone 5 makes payment truth observable and developer-ready.

Before this milestone, Twins could match a stablecoin transaction to a payment request and raise exceptions. This milestone turns those state changes into a receipt timeline and signed webhook delivery records.

## What This Milestone Includes

- Receipt events for payment lifecycle changes
- Private receipt API
- Public receipt API
- Webhook subscriptions
- HMAC-SHA256 webhook signing
- Webhook delivery logs
- Webhook replay
- Dashboard sections for receipt events and webhook deliveries

## Receipt Timeline

Every payment request now gets a timeline of evidence-backed events:

```text
payment_request.created
payment.detected
transaction.verified
transaction.matched
payment.confirmed
payment.exceptioned
exception.resolved
```

The timeline is the human and API proof of what happened to a payment.

## API Endpoints

```text
GET  /v1/receipt-events
GET  /v1/payment-requests/{id}/receipt
GET  /receipts/{payment_request_id}
GET  /v1/webhook-subscriptions
POST /v1/webhook-subscriptions
GET  /v1/webhook-deliveries
POST /v1/webhook-deliveries/{id}/replay
```

Webhook payloads are signed with:

```text
Twins-Signature: sha256=<hmac>
```

## Durable Schema

Migration:

```text
migrations/0004_milestone5_receipts_webhooks.sql
```

## Local Verification

Run:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-local.ps1
```

The local verifier now confirms:

- A webhook subscription can be created.
- A payment request receives receipt events.
- The Rust watcher posts verified transaction evidence.
- The matching engine confirms the payment.
- A receipt timeline exists for the request.
- Webhook delivery logs are created.
- Webhook replay can retry a delivery without changing payment truth.

## Done Criteria

- Every payment request has a full receipt timeline.
- Businesses can see created, detected, verified, matched, confirmed, exceptioned, and resolved states.
- Signed webhook delivery records are generated from receipt events.
- Failed webhook delivery does not change payment status or matching truth.
- Businesses can replay webhooks after failure.
