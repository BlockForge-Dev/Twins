# Milestone 7: Security And Multi-Tenant Readiness

Milestone 7 makes Twins safer to operate as business infrastructure. The goal is not to turn the MVP into a compliance product yet. The goal is to make the core payment truth system tenant-aware, permission-aware, observable, and ready for design partners.

## What Changed

- Business owner user is created with each business account
- API keys now have explicit scopes and can be revoked
- API routes enforce required scopes
- Webhook signing secrets are stored as protected secret material and never returned by list APIs
- Access logs record authenticated API traffic
- Security policy stores rate limit and retention settings
- Incident log tracks operational security or reliability events
- Audit logs cover sensitive actions like users, API keys, incidents, and policy changes
- Dashboard shows security policy, API keys, users, incidents, and access logs

## New API Surface

```text
GET   /v1/api-keys
POST  /v1/api-keys
POST  /v1/api-keys/{id}/revoke

GET   /v1/users
POST  /v1/users

GET   /v1/security-policy
PATCH /v1/security-policy

GET   /v1/access-logs

GET   /v1/incidents
POST  /v1/incidents
POST  /v1/incidents/{id}/resolve
```

## API Key Scopes

The default business key has all scopes for local development. New keys can be narrowed, for example:

```json
{
  "name": "Payment request reader",
  "scopes": ["payment_requests:read"]
}
```

That key can read payment requests, but it cannot register wallets, create transactions, manage webhooks, export reports, or modify security settings.

## Why This Matters

Milestones 2 through 6 proved the payment truth flow: create a request, detect a USDC transaction, match it, generate receipts, deliver webhooks, and reconcile a period.

Milestone 7 answers the enterprise question:

```text
Can real businesses safely operate this with multiple teams, scoped credentials, logs, and incident records?
```

This milestone gives Twins the control layer around the truth layer.

## Done Means

- One business cannot see another business's data through the API
- API keys can be scoped and revoked
- Scoped keys are rejected when they attempt unauthorized operations
- Sensitive webhook secrets are not returned in API responses
- Access logs show API activity for the business
- Incidents can be opened and resolved
- Security policy settings can be viewed and updated
- Sensitive actions are audit logged

## Migration

```text
migrations/0006_milestone7_security_multitenancy.sql
```

## Local Verification

Run:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-local.ps1
```

The verifier now checks the existing payment truth flow plus Milestone 7 controls: scoped key rejection, key revocation, users, policy updates, incidents, access logs, and dashboard data.
