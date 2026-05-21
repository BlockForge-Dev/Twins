CREATE TABLE reconciliation_runs (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id),
    status TEXT NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    wallet_id TEXT REFERENCES wallets(id),
    total_payment_requests INTEGER NOT NULL DEFAULT 0,
    confirmed_payment_requests INTEGER NOT NULL DEFAULT 0,
    total_transactions INTEGER NOT NULL DEFAULT 0,
    matched_transactions INTEGER NOT NULL DEFAULT 0,
    unmatched_transactions INTEGER NOT NULL DEFAULT 0,
    total_matches INTEGER NOT NULL DEFAULT 0,
    exception_count INTEGER NOT NULL DEFAULT 0,
    open_exception_count INTEGER NOT NULL DEFAULT 0,
    total_received_usdc NUMERIC(38, 6) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ
);

CREATE INDEX reconciliation_runs_business_id_created_at_idx
    ON reconciliation_runs (business_id, created_at DESC);

CREATE TABLE wallet_balance_snapshots (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id),
    reconciliation_run_id TEXT NOT NULL REFERENCES reconciliation_runs(id),
    wallet_id TEXT NOT NULL REFERENCES wallets(id),
    wallet_address TEXT NOT NULL,
    chain TEXT NOT NULL,
    token TEXT NOT NULL,
    observed_inbound_amount NUMERIC(38, 6) NOT NULL DEFAULT 0,
    transaction_count INTEGER NOT NULL DEFAULT 0,
    captured_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX wallet_balance_snapshots_reconciliation_run_id_idx
    ON wallet_balance_snapshots (reconciliation_run_id);

CREATE TABLE settlement_report_rows (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id),
    reconciliation_run_id TEXT NOT NULL REFERENCES reconciliation_runs(id),
    payment_request_id TEXT REFERENCES payment_requests(id),
    customer_id TEXT,
    invoice_id TEXT,
    order_id TEXT,
    payment_status TEXT,
    expected_amount NUMERIC(38, 6),
    received_amount NUMERIC(38, 6),
    token TEXT,
    chain TEXT,
    wallet_id TEXT REFERENCES wallets(id),
    wallet_address TEXT,
    stablecoin_transaction_id TEXT REFERENCES stablecoin_transactions(id),
    signature TEXT,
    transaction_status TEXT,
    match_id TEXT REFERENCES transaction_matches(id),
    match_status TEXT,
    exception_id TEXT REFERENCES exceptions(id),
    exception_type TEXT,
    exception_status TEXT,
    reconciliation_status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX settlement_report_rows_reconciliation_run_id_idx
    ON settlement_report_rows (reconciliation_run_id);

CREATE TABLE exports (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id),
    reconciliation_run_id TEXT NOT NULL REFERENCES reconciliation_runs(id),
    type TEXT NOT NULL,
    format TEXT NOT NULL,
    status TEXT NOT NULL,
    file_name TEXT NOT NULL,
    row_count INTEGER NOT NULL DEFAULT 0,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX exports_business_id_created_at_idx
    ON exports (business_id, created_at DESC);
