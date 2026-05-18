# Twins Product Brief

## Positioning

Twins helps businesses turn stablecoin payments into verified receipts, reconciled records, and actionable exceptions.

More technically, Twins is a non-custodial stablecoin payment truth layer that matches on-chain transactions to invoices, customers, orders, and settlement records.

Sharper pain statement:

Stablecoin payments are easy to receive. They are hard to reconcile. Twins fixes that.

## Vision

Make stablecoin payments business-ready by giving every stablecoin movement a verified business meaning, receipt, reconciliation status, and exception trail.

## Mission

Help businesses accept, track, match, reconcile, and prove stablecoin payments without spreadsheets, manual wallet checks, or uncertain transaction records.

Twins is not trying to make crypto payments popular. Twins is making stablecoin operations reliable enough for real businesses.

## Problem

The real pain is not that people cannot send stablecoins. They can already send USDC or USDT.

The real pain is that businesses cannot easily turn stablecoin transactions into clean operational records.

Businesses receiving stablecoins need to answer:

- Who paid this?
- Which invoice, customer, or order does this belong to?
- Was the amount correct?
- Was the token correct?
- Was the chain correct?
- Was the payment late?
- Was it underpaid or overpaid?
- Was the transaction final?
- Did fulfillment happen after payment?
- Can support prove the customer paid?
- Can finance close the books?
- Can the records be exported cleanly?

## V1 Product

Twins v1 is USDC payment matching, receipt, reconciliation, and exception infrastructure for businesses.

Example payment request:

```json
{
  "customer_id": "cust_123",
  "invoice_id": "INV-1001",
  "amount": "500.00",
  "token": "USDC",
  "chain": "solana",
  "expires_at": "2026-05-20T12:00:00Z"
}
```

Twins detects the customer payment, verifies it, matches it to the request, creates a receipt, sends a webhook, reconciles the payment, and raises an exception if something is wrong.

## Boundaries

### Non-Custodial

Twins does not hold customer funds.

Twins does not start as a custodial wallet, exchange, stablecoin issuer, off-ramp provider, FX provider, or money transmitter.

The customer owns the wallet. The customer owns the funds. Twins watches, verifies, matches, records, reconciles, and proves.

Doctrine:

Wallets move the money. Twins proves what happened.

### Not Full Accounting Software

Twins does not replace Bitwave, Cryptio, QuickBooks, Xero, NetSuite, or ERP systems.

Twins makes stablecoin payment events clean before they reach accounting.

Twins produces:

- Matched payment records
- Receipts
- Reconciliation statuses
- Exception reports
- Webhook events
- CSV and API exports

Doctrine:

Twins is not the accounting ledger. Twins is the stablecoin payment truth layer.

### Deep Before Wide

Twins does not support every chain, token, and wallet first.

Start with:

- USDC
- Solana
- Business-owned wallets

Add later:

- USDT
- Ethereum
- Polygon
- Base
- Arbitrum
- Aptos
- Sui
- Celo
- Bank and off-ramp providers

Doctrine:

One stablecoin flow done correctly is better than ten shallow integrations.

## Target Users

The first users are businesses already receiving or sending stablecoins.

Early segments:

- Web3 SaaS companies
- AI API companies accepting USDC
- Digital product businesses
- Stablecoin-native agencies
- Small fintechs
- Cross-border service businesses
- Solana and Base builders
- Payment startups
- Remote global service companies

First buyer personas:

- Founder
- Finance or operations lead
- Developer
- Support lead

They care about customer payment uncertainty, wrong amounts, manual wallet checking, support disputes, spreadsheet reconciliation, invoice mismatch, failed webhooks, delayed fulfillment, and month-end reporting.

## Differentiation

### Compared With Enterprise Crypto Accounting

Enterprise crypto accounting platforms are broader and finance-heavy.

Twins focuses on the payment event layer:

- Payment request
- Wallet transaction
- Matching
- Receipt
- Exception
- Webhook
- Settlement record

Twins is not trying to become the whole accounting department first.

### Compared With Stablecoin Orchestration And Custody

Stablecoin orchestration and custody providers own more of the money movement stack.

Twins starts lighter:

Keep your wallet. Keep your provider. Twins makes the stablecoin payment operationally true.

### Compared With Invoice And Payment Platforms

Invoice platforms focus on creating, approving, and paying invoices.

Twins is developer-first and exception-first.

The killer job is:

Transaction came in. Match it to business intent. Prove whether it is correct. Raise an exception if wrong. Send a webhook. Produce a receipt. Export the settlement record.

## Unique Thesis

Stablecoin payments are easy to receive but hard to operationalize. Twins turns raw wallet activity into verified business truth.

The enemy is not another platform directly. The enemy is:

- Manual wallet checking
- Spreadsheet reconciliation
- Unexplained transactions
- Failed customer fulfillment
- Support disputes
- Finance uncertainty

## Core System Components

- API Gateway
- Business and Tenant Service
- Wallet Registry
- Payment Request Service
- Chain Watcher
- Transaction Verifier
- Matching Engine
- Receipt Service
- Exception Engine
- Webhook Delivery Service
- Reconciliation Service
- Dashboard
- Export Service

## Core Data Model

- businesses
- users
- api_keys
- wallets
- payment_requests
- stablecoin_transactions
- transaction_matches
- receipt_events
- exceptions
- webhook_deliveries
- reconciliation_runs
- exports
- audit_logs

## Payment Request States

- created
- awaiting_payment
- payment_detected
- verifying
- matched
- confirmed
- underpaid
- overpaid
- wrong_token
- wrong_chain
- expired
- reconciled
- exception
- manually_resolved

## Transaction States

- detected
- pending_finality
- confirmed_onchain
- matched_to_request
- orphan
- duplicate
- invalid
- reconciled

## What Not To Build In V1

- Custody
- Off-ramp
- FX conversion
- Debit cards
- Own stablecoin
- All-chain support
- All-token support
- Full accounting suite
- Tax reporting
- Consumer wallet app
- Mobile app
- DeFi yield
- Trading features

## Business Model

Sandbox:

- Free
- Limited test transactions
- Developer docs
- Test webhooks

Starter:

- $49-$99/month
- Small number of wallets
- Payment requests
- Receipts
- Basic dashboard
- CSV export

Growth:

- $199-$499/month
- More wallets
- Higher transaction volume
- Webhook replay
- Exception dashboard
- Reconciliation reports
- API access

Enterprise:

- Custom pricing
- Advanced roles
- Custom retention
- Custom reports
- Dedicated support
- Private deployment or VPC later
- Custom chain/provider support

Charge for monthly platform access, reconciled transactions, connected wallets, advanced reports, and custom integrations.

Do not rely only on tiny transaction fees. Twins sells operational reliability.

## Key Metrics

- Active businesses
- Connected wallets
- Payment requests created
- Transactions detected
- Transactions matched
- Orphan transactions
- Underpaid payments
- Overpaid payments
- Late or expired payments
- Webhooks delivered
- Webhooks failed
- Exceptions resolved
- Average exception resolution time
- Receipts generated
- Settlement reports exported
- Monthly recurring revenue

Power metric:

Number of stablecoin transactions converted into reconciled business records.

## Expansion Path

Start with USDC payment matching.

Then expand into:

- Multi-chain stablecoin reconciliation
- Stablecoin payables and receivables
- Treasury operations
- Accounting integrations
- Enterprise settlement infrastructure
- AI-agent stablecoin payment controls

