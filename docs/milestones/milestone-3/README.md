# Milestone 3: Solana USDC Watcher And Verifier

Milestone 3 starts the on-chain truth side of Twins.

The goal is to detect USDC transfers into registered Solana business wallets, verify the transaction evidence, reject wrong-token transfers, and store normalized chain evidence.

## What This Milestone Includes

- Rust Solana watcher worker
- Solana JSON-RPC client
- `getSignaturesForAddress` polling
- `getTransaction` fetching with `jsonParsed` encoding
- SPL Token transfer parsing
- USDC mint verification
- Destination wallet owner verification
- Amount and decimals extraction
- Confirmation/finality status handling
- Wrong-token rejection evidence
- Go API ingestion endpoint for verified stablecoin transactions
- Postgres schema for stablecoin transaction evidence

## What This Milestone Does Not Include

- Matching transactions to payment requests
- Underpaid or overpaid detection
- Orphan transaction workflow
- Receipt timeline
- Webhook delivery
- Production-grade cursor persistence
- Reorg/backfill strategy

Those begin in Milestone 4 and Milestone 5.

## Rust Worker

Worker path:

```text
workers/solana-watcher
```

Run fixture verification:

```powershell
cargo run -p twins-solana-watcher -- verify-fixture `
  --input workers/solana-watcher/fixtures/inbound_usdc_transfer.json `
  --wallet 7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDH
```

Scan a wallet through Solana RPC:

```powershell
cargo run -p twins-solana-watcher -- scan-wallet `
  --rpc-url https://api.mainnet-beta.solana.com `
  --wallet <business-owner-wallet> `
  --limit 20 `
  --commitment finalized
```

Fetch one signature:

```powershell
cargo run -p twins-solana-watcher -- fetch-signature `
  --rpc-url https://api.mainnet-beta.solana.com `
  --signature <transaction-signature> `
  --wallet <business-owner-wallet> `
  --commitment finalized
```

Post verified evidence into the Go API:

```powershell
cargo run -p twins-solana-watcher -- verify-fixture `
  --input workers/solana-watcher/fixtures/inbound_usdc_transfer.json `
  --wallet 7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDH `
  --post-url http://localhost:8082/v1/stablecoin-transactions `
  --api-key $apiKey
```

## Go API Endpoint

```text
GET  /v1/stablecoin-transactions
POST /v1/stablecoin-transactions
```

The POST endpoint accepts verified chain evidence emitted by the Rust watcher.

New finalized transactions are stored as:

```text
confirmed_onchain
```

Non-finalized transactions are stored as:

```text
pending_finality
```

## Durable Schema

Migration:

```text
migrations/0002_milestone3_stablecoin_transactions.sql
```

## Done Criteria

- The watcher can parse a Solana `jsonParsed` transaction.
- The watcher extracts signature, slot, block time, source, destination, owner, mint, amount, decimals, and confirmation status.
- The watcher identifies inbound USDC transfers for a registered wallet owner.
- The watcher rejects/warns on wrong-token inbound transfers.
- The Go API stores verified USDC transaction evidence.
- Duplicate transaction signatures are idempotently replayed.
- Tests pass for Go and Rust.

## Next Milestone

Milestone 4 connects this evidence to business intent:

- Match transaction to payment request
- Detect underpayment
- Detect overpayment
- Detect expired payment
- Detect orphan transaction
- Add manual review queue

