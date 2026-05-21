-- Twins Milestone 7 security and multi-tenant readiness.
-- This migration tightens tenant operations around scoped API keys,
-- protected webhook secrets, access logging, incidents, and retention policy.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_role_check;

ALTER TABLE users
    ADD CONSTRAINT users_role_check
    CHECK (role IN ('owner', 'admin', 'operator', 'viewer'));

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS created_by TEXT REFERENCES users(id);

CREATE TABLE security_policies (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL UNIQUE REFERENCES businesses(id) ON DELETE CASCADE,
    require_scoped_api_keys BOOLEAN NOT NULL DEFAULT TRUE,
    rate_limit_per_minute INTEGER NOT NULL DEFAULT 240 CHECK (rate_limit_per_minute BETWEEN 10 AND 10000),
    data_retention_days INTEGER NOT NULL DEFAULT 365 CHECK (data_retention_days >= 30),
    access_log_retention_days INTEGER NOT NULL DEFAULT 90 CHECK (access_log_retention_days >= 7),
    webhook_retention_days INTEGER NOT NULL DEFAULT 30 CHECK (webhook_retention_days >= 7),
    incident_retention_days INTEGER NOT NULL DEFAULT 365 CHECK (incident_retention_days >= 30),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX security_policies_business_id_idx
    ON security_policies (business_id);

CREATE TABLE access_logs (
    id TEXT PRIMARY KEY,
    business_id TEXT REFERENCES businesses(id) ON DELETE CASCADE,
    api_key_id TEXT REFERENCES api_keys(id),
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    status_code INTEGER NOT NULL,
    remote_addr TEXT NOT NULL,
    user_agent TEXT,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    rate_limited BOOLEAN NOT NULL DEFAULT FALSE,
    accessed_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX access_logs_business_id_accessed_at_idx
    ON access_logs (business_id, accessed_at DESC);

CREATE INDEX access_logs_rate_limited_idx
    ON access_logs (rate_limited, accessed_at DESC);

CREATE TABLE incidents (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    severity TEXT NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    status TEXT NOT NULL CHECK (status IN ('open', 'resolved')),
    description TEXT,
    resolution_summary TEXT,
    created_by TEXT REFERENCES api_keys(id),
    resolved_by TEXT REFERENCES api_keys(id),
    created_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ
);

CREATE INDEX incidents_business_id_created_at_idx
    ON incidents (business_id, created_at DESC);

CREATE INDEX incidents_business_id_status_idx
    ON incidents (business_id, status);
