# Milestone 8: Private Beta With Real Businesses

Milestone 8 turns Twins from an internal MVP into a private-beta operating system. It does not claim that design partners already exist. It gives the product a way to track design partners, real usage, exception cases, testimonials, pricing commitments, and the evidence needed before a serious beta launch.

## What Changed

- Design partner records
- Design partner status updates
- Beta evidence records
- Usage metrics API
- Private beta readiness report
- Dashboard sections for partners, evidence, usage, and readiness
- Verification coverage in the local smoke test

## New API Surface

```text
GET   /v1/design-partners
POST  /v1/design-partners
PATCH /v1/design-partners/{id}

GET   /v1/beta-evidence
POST  /v1/beta-evidence

GET   /v1/usage-metrics
GET   /v1/private-beta-report
```

## Design Partner Example

```json
{
  "company_name": "Beta AI Labs",
  "segment": "AI API company",
  "contact_name": "Finance Lead",
  "contact_email": "finance@example.com",
  "use_case": "USDC invoice matching and reconciliation",
  "status": "onboarding",
  "agreed_to_test": true,
  "pricing_commitment": true,
  "expected_monthly_volume": 250
}
```

## Beta Evidence Example

```json
{
  "design_partner_id": "dsp_123",
  "type": "real_transaction",
  "title": "First real USDC payment processed",
  "payment_request_id": "prq_123",
  "stablecoin_transaction_id": "txn_123"
}
```

Supported evidence types:

- `real_transaction`
- `exception_case`
- `testimonial`
- `pricing_commitment`
- `workflow_pain`
- `integration_request`

## Readiness Logic

The private beta report tracks whether the evidence target is met:

- 5 design partners onboarded
- 2 design partners with real transaction evidence
- 1 pricing commitment
- at least 1 exception case collected

This is intentionally evidence-based. A private beta should be driven by real usage, not vibes.

## Why This Matters

The product already proves the stablecoin payment flow:

```text
request -> on-chain evidence -> match -> receipt -> webhook -> reconciliation -> export
```

Milestone 8 adds the operating question:

```text
Are real teams using it, where are they getting value, and what proof do we have?
```

That is how Twins moves from a working MVP into a credible private beta.

## Migration

```text
migrations/0007_milestone8_private_beta.sql
```

## Local Verification

Run:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-local.ps1
```

The verifier creates a design partner, records real transaction and testimonial evidence, checks usage metrics, checks the private beta report, and still verifies the full payment truth flow.
