# Twins V1 Milestones

## Milestone 1: Founder Validation And Design Partner Proof

Execution kit:

- [Milestone 1 Validation Kit](validation/README.md)
- [Interview Script](validation/interview-script.md)
- [Pain Evidence Log](validation/pain-evidence-log.csv)
- [Design Partner Tracker](validation/design-partner-tracker.csv)

Build or do:

- Interview 20-30 stablecoin-using businesses and builders.
- Find 5 design partners.
- Collect real examples of reconciliation pain.
- Map current workflows.
- Define the exact first workflow.

Done means:

- At least 5 serious teams confirm stablecoin reconciliation or payment matching pain.
- At least 2 agree to test the product when the MVP is ready.
- There are 10+ real examples of mismatches, support issues, wrong payments, or manual reconciliation.
- The first chain and token are confirmed.

## Milestone 2: Core Payment Request And Wallet Registry

Implementation:

- [Milestone 2 README](milestones/milestone-2/README.md)
- [Milestone 2 API](api/milestone-2.md)
- [Postgres core schema](../migrations/0001_milestone2_core.sql)

Build:

- Business account model
- API keys
- Wallet registry
- Payment request creation
- Payment request status
- Basic dashboard list
- PostgreSQL schema
- Idempotency keys
- Audit logs

Done means:

- A business can register a wallet.
- A business can create a USDC payment request.
- The system persists amount, token, chain, expiry, metadata, and customer reference.
- Duplicate payment requests with the same idempotency key are blocked.
- Dashboard shows created and awaiting payment requests.

## Milestone 3: Solana USDC Watcher And Verifier

Implementation:

- [Milestone 3 README](milestones/milestone-3/README.md)
- [Stablecoin transaction schema](../migrations/0002_milestone3_stablecoin_transactions.sql)

Build:

- Solana wallet watcher
- USDC transaction detection
- Confirmation and finality tracking
- Destination wallet verification
- Amount verification
- Token mint verification
- Transaction evidence storage

Done means:

- USDC entering a registered wallet is detected.
- The system stores transaction signature, amount, token, source, destination, slot or block time, and confirmation status.
- The system rejects or warns on wrong token.
- The system does not mark payment as confirmed until the required finality rule is met.

## Milestone 4: Matching Engine And Exception Engine

Implementation:

- [Milestone 4 README](milestones/milestone-4/README.md)
- [Matches and exceptions schema](../migrations/0003_milestone4_matches_exceptions.sql)

Build:

- Transaction-to-payment-request matching
- Underpayment detection
- Overpayment detection
- Wrong token detection
- Wrong chain detection
- Expired payment detection
- Orphan transaction detection
- Manual review queue

Done means:

- Correct payments automatically match to the correct request.
- Underpaid payments become underpaid, not failed.
- Overpaid payments become overpaid, not silently accepted.
- Payments after expiry become late or expired exceptions.
- Incoming transactions without matching requests become orphan transactions.
- Operators can manually resolve exceptions with a reason.

## Milestone 5: Receipt Timeline And Webhook Delivery

Implementation:

- [Milestone 5 README](milestones/milestone-5/README.md)
- [Receipts and webhooks schema](../migrations/0004_milestone5_receipts_webhooks.sql)

Build:

- Receipt event timeline
- Public and private receipt views
- Webhook subscriptions
- Webhook signing
- Webhook retries
- Webhook delivery logs
- Webhook replay

Done means:

- Every payment request has a full receipt timeline.
- Businesses can see when a request was created, payment detected, verified, matched, confirmed, reconciled, or exceptioned.
- Businesses receive signed webhooks when payment status changes.
- Failed webhook delivery does not change payment truth.
- Businesses can replay webhooks after failure.

## Milestone 6: Reconciliation Dashboard And Export

Implementation:

- [Milestone 6 README](milestones/milestone-6/README.md)
- [Reconciliation and exports schema](../migrations/0005_milestone6_reconciliation_exports.sql)

Build:

- Daily reconciliation run
- Wallet balance snapshot
- Matched vs unmatched transactions
- Payment request vs transaction report
- CSV export
- API export
- Exception summary
- Settlement report

Done means:

- Businesses can see all transactions for a period.
- Businesses can see which transactions matched payment requests.
- Businesses can see orphan, underpaid, overpaid, and expired exceptions.
- Businesses can export a clean settlement report.
- Finance or operations can close a day without manually checking a wallet explorer for every transaction.

## Milestone 7: Security And Multi-Tenant Readiness

Implementation:

- [Milestone 7 README](milestones/milestone-7/README.md)
- [Security and multi-tenant schema](../migrations/0006_milestone7_security_multitenancy.sql)

Build:

- Tenant isolation
- RBAC
- API key scopes
- Encrypted secrets
- Audit logs
- Rate limits
- Admin and operator roles
- Basic incident log
- Data retention rules
- Access logging

Done means:

- One business cannot access another business's data.
- API keys can be scoped and revoked.
- Sensitive actions are audit logged.
- Operators have roles.
- Webhook secrets are protected.
- There is a basic customer-facing security document.

## Milestone 8: Private Beta With Real Businesses

Implementation:

- [Milestone 8 README](milestones/milestone-8/README.md)
- [Private beta schema](../migrations/0007_milestone8_private_beta.sql)

Build or do:

- Onboard 5-10 design partners.
- Process real stablecoin payment requests.
- Collect exception cases.
- Improve UX and API docs.
- Track usage metrics.
- Charge at least some customers.

Done means:

- At least 5 businesses are onboarded.
- At least 2 businesses process real stablecoin transactions.
- At least 1 business pays or agrees to pay.
- The system handles real confirmed, underpaid, overpaid, orphan, or late payment cases.
- There are testimonials or concrete usage evidence.
