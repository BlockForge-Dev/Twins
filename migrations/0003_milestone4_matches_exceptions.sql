-- Twins Milestone 4 matching and exception schema.
-- Connects confirmed on-chain evidence to payment requests and records reviewable exceptions.

CREATE TABLE transaction_matches (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    payment_request_id TEXT NOT NULL REFERENCES payment_requests(id),
    stablecoin_transaction_id TEXT NOT NULL REFERENCES stablecoin_transactions(id),
    status TEXT NOT NULL CHECK (status IN ('confirmed', 'underpaid', 'overpaid', 'expired')),
    expected_amount NUMERIC(38, 6) NOT NULL CHECK (expected_amount > 0),
    received_amount NUMERIC(38, 6) NOT NULL CHECK (received_amount > 0),
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (business_id, payment_request_id, stablecoin_transaction_id)
);

CREATE TABLE exceptions (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    payment_request_id TEXT REFERENCES payment_requests(id),
    stablecoin_transaction_id TEXT REFERENCES stablecoin_transactions(id),
    type TEXT NOT NULL CHECK (
        type IN (
            'underpaid',
            'overpaid',
            'expired',
            'orphan',
            'ambiguous_match',
            'wrong_token',
            'wrong_chain'
        )
    ),
    status TEXT NOT NULL CHECK (status IN ('open', 'resolved')),
    severity TEXT NOT NULL CHECK (severity IN ('low', 'medium', 'high')),
    reason TEXT NOT NULL,
    resolution_reason TEXT,
    resolved_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX idx_transaction_matches_business_created_at
    ON transaction_matches (business_id, created_at DESC);

CREATE INDEX idx_transaction_matches_payment_request
    ON transaction_matches (payment_request_id);

CREATE INDEX idx_transaction_matches_transaction
    ON transaction_matches (stablecoin_transaction_id);

CREATE INDEX idx_exceptions_business_status_created_at
    ON exceptions (business_id, status, created_at DESC);

CREATE INDEX idx_exceptions_payment_request
    ON exceptions (payment_request_id);

CREATE INDEX idx_exceptions_transaction
    ON exceptions (stablecoin_transaction_id);

