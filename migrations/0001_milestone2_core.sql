-- Twins Milestone 2 core schema.
-- This migration defines the durable business-intent side of the product:
-- businesses, API keys, wallet registry, payment requests, idempotency, and audit logs.

CREATE TABLE businesses (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    name TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'developer', 'operator', 'viewer')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    disabled_at TIMESTAMPTZ,
    UNIQUE (business_id, email)
);

CREATE TABLE api_keys (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    prefix TEXT NOT NULL,
    secret_hash TEXT NOT NULL UNIQUE,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);

CREATE TABLE wallets (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    chain TEXT NOT NULL CHECK (chain IN ('solana')),
    address TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at TIMESTAMPTZ,
    UNIQUE (business_id, chain, address)
);

CREATE TABLE payment_requests (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    wallet_id TEXT NOT NULL REFERENCES wallets(id),
    customer_id TEXT NOT NULL,
    invoice_id TEXT NOT NULL,
    order_id TEXT,
    amount NUMERIC(38, 6) NOT NULL CHECK (amount > 0),
    token TEXT NOT NULL CHECK (token IN ('USDC')),
    chain TEXT NOT NULL CHECK (chain IN ('solana')),
    destination_address TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL CHECK (
        status IN (
            'created',
            'awaiting_payment',
            'payment_detected',
            'verifying',
            'matched',
            'confirmed',
            'underpaid',
            'overpaid',
            'wrong_token',
            'wrong_chain',
            'expired',
            'reconciled',
            'exception',
            'manually_resolved'
        )
    ),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE idempotency_keys (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    operation TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    body_hash TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (business_id, operation, idempotency_key)
);

CREATE TABLE audit_logs (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    actor_type TEXT NOT NULL CHECK (actor_type IN ('system', 'user', 'api_key')),
    actor_id TEXT NOT NULL,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_api_keys_business_id ON api_keys (business_id);
CREATE INDEX idx_wallets_business_id ON wallets (business_id);
CREATE INDEX idx_payment_requests_business_id_created_at ON payment_requests (business_id, created_at DESC);
CREATE INDEX idx_payment_requests_business_id_status ON payment_requests (business_id, status);
CREATE INDEX idx_payment_requests_business_id_invoice_id ON payment_requests (business_id, invoice_id);
CREATE INDEX idx_idempotency_keys_business_operation ON idempotency_keys (business_id, operation);
CREATE INDEX idx_audit_logs_business_created_at ON audit_logs (business_id, created_at DESC);

