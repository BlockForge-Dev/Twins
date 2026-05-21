CREATE TABLE receipt_events (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id),
    payment_request_id TEXT NOT NULL REFERENCES payment_requests(id),
    stablecoin_transaction_id TEXT REFERENCES stablecoin_transactions(id),
    transaction_match_id TEXT REFERENCES transaction_matches(id),
    exception_id TEXT REFERENCES exceptions(id),
    type TEXT NOT NULL,
    status TEXT NOT NULL,
    description TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX receipt_events_business_id_created_at_idx
    ON receipt_events (business_id, created_at DESC);

CREATE INDEX receipt_events_payment_request_id_created_at_idx
    ON receipt_events (payment_request_id, created_at ASC);

CREATE TABLE webhook_subscriptions (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id),
    url TEXT NOT NULL,
    secret_encrypted TEXT NOT NULL,
    event_types TEXT[] NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX webhook_subscriptions_business_id_created_at_idx
    ON webhook_subscriptions (business_id, created_at DESC);

CREATE TABLE webhook_deliveries (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id),
    webhook_subscription_id TEXT NOT NULL REFERENCES webhook_subscriptions(id),
    receipt_event_id TEXT NOT NULL REFERENCES receipt_events(id),
    event_type TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    payment_request_id TEXT REFERENCES payment_requests(id),
    stablecoin_transaction_id TEXT REFERENCES stablecoin_transactions(id),
    exception_id TEXT REFERENCES exceptions(id),
    payload JSONB NOT NULL,
    signature TEXT NOT NULL,
    status TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_status_code INTEGER,
    last_error TEXT,
    next_attempt_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX webhook_deliveries_business_id_created_at_idx
    ON webhook_deliveries (business_id, created_at DESC);

CREATE INDEX webhook_deliveries_status_next_attempt_at_idx
    ON webhook_deliveries (status, next_attempt_at);
