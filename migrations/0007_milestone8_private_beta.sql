-- Twins Milestone 8 private beta readiness.
-- This migration tracks design partners, beta evidence, and usage proof
-- needed before the product can be taken into a real private beta.

CREATE TABLE design_partners (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    company_name TEXT NOT NULL,
    segment TEXT,
    contact_name TEXT,
    contact_email TEXT,
    use_case TEXT,
    status TEXT NOT NULL CHECK (status IN ('prospect', 'invited', 'onboarding', 'active', 'paused', 'churned')),
    agreed_to_test BOOLEAN NOT NULL DEFAULT FALSE,
    pricing_commitment BOOLEAN NOT NULL DEFAULT FALSE,
    expected_monthly_volume INTEGER NOT NULL DEFAULT 0 CHECK (expected_monthly_volume >= 0),
    notes TEXT,
    created_by TEXT REFERENCES api_keys(id),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX design_partners_business_id_created_at_idx
    ON design_partners (business_id, created_at DESC);

CREATE INDEX design_partners_business_id_status_idx
    ON design_partners (business_id, status);

CREATE TABLE beta_evidence (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    design_partner_id TEXT REFERENCES design_partners(id),
    type TEXT NOT NULL CHECK (
        type IN (
            'real_transaction',
            'exception_case',
            'testimonial',
            'pricing_commitment',
            'workflow_pain',
            'integration_request'
        )
    ),
    title TEXT NOT NULL,
    description TEXT,
    payment_request_id TEXT REFERENCES payment_requests(id),
    stablecoin_transaction_id TEXT REFERENCES stablecoin_transactions(id),
    exception_id TEXT REFERENCES exceptions(id),
    quote TEXT,
    created_by TEXT REFERENCES api_keys(id),
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX beta_evidence_business_id_created_at_idx
    ON beta_evidence (business_id, created_at DESC);

CREATE INDEX beta_evidence_design_partner_id_idx
    ON beta_evidence (design_partner_id);

CREATE INDEX beta_evidence_business_id_type_idx
    ON beta_evidence (business_id, type);
