# Milestone 1 Validation Kit

This kit turns Twins Milestone 1 into an executable customer discovery process.

Milestone 1 is complete when Twins has evidence that real businesses already feel stablecoin payment matching, receipt, reconciliation, or exception pain strongly enough to test an MVP.

## Goal

Find 5 serious teams with this exact pain and get at least 2 to agree to test the MVP.

## Required Evidence

- 20-30 interviews completed.
- 5 qualified design partner candidates.
- 2 written or recorded agreements to test the MVP.
- 10+ real pain examples involving mismatches, support disputes, wrong payments, orphan transactions, spreadsheet reconciliation, delayed fulfillment, or finance close problems.
- A clear decision on first chain and token.
- A precise first workflow for v1.

## Files

- [Target Accounts](target-accounts.csv)
- [Outreach Templates](outreach-templates.md)
- [Interview Script](interview-script.md)
- [Pain Evidence Log](pain-evidence-log.csv)
- [Workflow Map Template](workflow-map-template.md)
- [Design Partner Tracker](design-partner-tracker.csv)
- [Qualification Rubric](qualification-rubric.md)
- [Milestone 1 Scorecard](scorecard.md)

## Weekly Operating Rhythm

Run this cycle every week until the milestone is complete:

1. Add 30-50 target accounts.
2. Send 20-30 targeted messages.
3. Book 5-8 calls.
4. Complete 3-5 customer interviews.
5. Log every concrete pain example.
6. Score each company.
7. Ask qualified prospects to become design partners.
8. Update the scorecard every Friday.

## Design Partner Standard

A design partner is not just someone who likes the idea.

A qualified design partner:

- Already receives or sends stablecoins for business activity.
- Has a recurring reconciliation, support, fulfillment, or reporting problem.
- Can show a current manual workflow.
- Can provide examples of past transaction ambiguity.
- Has a named owner who will test the MVP.
- Is willing to give feedback at least once per week during private beta.

## First Workflow Hypothesis

The first workflow to validate:

```text
Business creates a USDC payment request
        |
Customer pays to a business-owned Solana wallet
        |
Twins detects the transfer
        |
Twins matches it to invoice/customer/order intent
        |
Twins generates a receipt timeline
        |
Twins raises exceptions for underpaid, overpaid, late, wrong token, or orphan payments
        |
Twins sends a webhook and exports a settlement record
```

## Kill Criteria

Pause the v1 build if:

- Fewer than 5 of 30 qualified conversations have this pain.
- Nobody can provide real examples.
- Prospects only want custody, off-ramp, wallet, card, or full accounting features.
- The buyer does not care enough to test manually before polish.
- The pain only exists at volumes too small to support pricing.

## Proceed Criteria

Begin Milestone 2 when:

- 5 serious teams confirm the pain.
- 2 teams commit to testing.
- The evidence log has 10+ real cases.
- The MVP workflow is stable across multiple interviews.
- The first chain and token are confirmed.

