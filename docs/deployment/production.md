# Production Hosting Guide

This is the production baseline for hosting Twins as a private MVP or design-partner preview.

## What This Baseline Provides

- Durable single-node file storage through `TWINS_DATA_PATH`
- Business creation protected by `TWINS_BUSINESS_CREATION_TOKEN`
- HTTP server timeouts
- Graceful shutdown on SIGTERM
- `/healthz` liveness endpoint
- `/readyz` readiness endpoint
- Dockerfile and Compose deployment
- Non-root container user
- Persistent Docker volume

This is suitable for a private preview where a small number of trusted people can test the product. For larger production traffic or multi-instance deployment, replace file storage with Postgres.

## Environment Variables

```text
TWINS_ENV=production
TWINS_HTTP_ADDR=:8080
TWINS_DATA_PATH=/data/twins-store.json
TWINS_BUSINESS_CREATION_TOKEN=<long random setup token>
```

When `TWINS_ENV=production`, `TWINS_DATA_PATH` is required. Without it, the API refuses to start.

## Run With Docker Compose

Create `.env` from `.env.example` and set a long setup token:

```powershell
Copy-Item .env.example .env
```

Then run:

```powershell
docker compose up --build
```

Open:

```text
http://localhost:8080/dashboard
```

## Create The First Business

Business creation is locked behind the setup token in hosted mode:

```powershell
$body = @{ name = "Your Company"; owner_email = "you@example.com" } | ConvertTo-Json
Invoke-RestMethod `
  -Method Post `
  -Uri "http://localhost:8080/v1/businesses" `
  -Headers @{ "X-Twins-Setup-Token" = $env:TWINS_BUSINESS_CREATION_TOKEN } `
  -ContentType "application/json" `
  -Body $body
```

Save the returned API key securely. It is only shown once.

## Health Checks

```text
GET /healthz
GET /readyz
```

`/healthz` means the process is alive. `/readyz` means storage is usable.

## Current Production Boundary

This baseline is intentionally single-node. The file store is durable across restarts, but it is not a multi-writer database.

Use this for:

- private demos
- design partner previews
- low-volume hosted MVP testing

Do not use this yet for:

- public self-serve signup
- regulated production workloads
- multi-region or multi-instance deployments
- large customer data volumes

## Next Hardening Step

The next storage milestone should wire the existing migration files into a real Postgres store. That is the upgrade path from hosted MVP to production SaaS.
