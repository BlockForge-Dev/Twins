-- Twins Milestone 3 chain evidence schema.
-- Stores normalized Solana USDC transaction evidence emitted by the Rust watcher.

CREATE TABLE stablecoin_transactions (
    id TEXT PRIMARY KEY,
    business_id TEXT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    wallet_id TEXT NOT NULL REFERENCES wallets(id),
    chain TEXT NOT NULL CHECK (chain IN ('solana')),
    signature TEXT NOT NULL,
    slot BIGINT NOT NULL CHECK (slot > 0),
    block_time BIGINT,
    confirmation_status TEXT NOT NULL CHECK (confirmation_status IN ('processed', 'confirmed', 'finalized')),
    source_address TEXT NOT NULL,
    source_owner TEXT,
    destination_address TEXT NOT NULL,
    destination_owner TEXT NOT NULL,
    token TEXT NOT NULL CHECK (token IN ('USDC')),
    mint TEXT NOT NULL,
    amount NUMERIC(38, 6) NOT NULL CHECK (amount > 0),
    amount_atomic NUMERIC(38, 0) NOT NULL CHECK (amount_atomic > 0),
    decimals SMALLINT NOT NULL CHECK (decimals = 6),
    status TEXT NOT NULL CHECK (
        status IN (
            'detected',
            'pending_finality',
            'confirmed_onchain',
            'matched_to_request',
            'orphan',
            'duplicate',
            'invalid',
            'reconciled'
        )
    ),
    detected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (business_id, chain, signature)
);

CREATE INDEX idx_stablecoin_transactions_business_created_at
    ON stablecoin_transactions (business_id, created_at DESC);

CREATE INDEX idx_stablecoin_transactions_business_status
    ON stablecoin_transactions (business_id, status);

CREATE INDEX idx_stablecoin_transactions_wallet_created_at
    ON stablecoin_transactions (wallet_id, created_at DESC);

CREATE INDEX idx_stablecoin_transactions_destination_owner
    ON stablecoin_transactions (chain, destination_owner);

