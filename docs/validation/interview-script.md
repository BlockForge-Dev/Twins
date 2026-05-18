# Customer Interview Script

Use this for 30-minute founder validation calls.

The goal is not to pitch Twins. The goal is to understand whether stablecoin payment operations are painful enough that a business will test and eventually pay for a product.

## Opening

Thanks for taking the time. I am researching how businesses handle stablecoin payments after the transaction happens.

I am especially interested in matching wallet activity to invoices, customers, orders, receipts, support cases, and reconciliation records.

This is not a sales call. I am trying to learn what is painful, what is manual, and what would need to be true for this to be reliable in production.

## Qualification

Ask these first:

- Does your business currently receive or send stablecoins?
- Which stablecoins do you use?
- Which chains do you use?
- Roughly how many stablecoin transactions do you process per month?
- Are these customer payments, vendor payments, internal treasury movements, or something else?
- Who owns the operational workflow after a stablecoin payment is made?

## Current Workflow

Walk me through the last time a customer or partner paid you in stablecoins.

Follow-up questions:

- Where did the payment request start?
- How did the payer know where to send funds?
- How did your team know the payment arrived?
- How did you connect the transaction to the customer, invoice, order, or account?
- What tools did you use?
- Was anything copied into a spreadsheet?
- Was anyone checking a block explorer manually?
- Who had to be notified after payment?
- Did any system update automatically?
- What happened if the webhook, fulfillment, or internal update failed?

## Pain Discovery

Ask for concrete examples:

- Tell me about a time a stablecoin payment was hard to identify.
- Have you ever received the right amount but could not tell who paid?
- Have you ever received the wrong amount?
- Have you ever received the wrong token?
- Have you ever received payment on the wrong chain or wallet?
- Have you ever delivered late because payment confirmation was unclear?
- Have you ever had a support dispute where the customer said they paid?
- Have you ever had an orphan transaction sitting in a wallet with unclear business meaning?
- What happens at month-end close?
- What breaks when transaction volume increases?

Do not accept general answers. Ask:

- When did that happen?
- What was the transaction value?
- Who had to fix it?
- How long did it take?
- What was the business impact?
- What record or screenshot proved the answer?

## Existing Alternatives

Ask:

- What do you use today for this?
- Why does that not fully solve it?
- Have you tried crypto accounting tools?
- Have you tried invoice tools?
- Have you built internal scripts?
- Would you prefer to keep your existing wallet and have a tool verify and reconcile activity around it?
- What would make you reject a non-custodial reconciliation tool?

## MVP Test

Describe the MVP briefly:

Twins would let your team create a USDC payment request, watch a registered business wallet, detect the incoming transaction, verify token, amount, destination, and finality, match it to the request, generate a receipt timeline, send a webhook, and flag exceptions like underpaid, overpaid, late, wrong token, wrong chain, or orphan payments.

Then ask:

- Would this fit a real workflow you have today?
- Which part would be most valuable?
- Which part is unnecessary?
- What would need to integrate with it?
- Who on your team would use it?
- What would you need to trust it?
- Would you test this with real transactions?
- Could we onboard you as a design partner when the MVP is ready?

## Pricing Signal

Ask only after strong pain is confirmed:

- Is this painful enough that your company would pay to solve it?
- Would you expect to pay monthly, per wallet, per reconciled transaction, or by volume tier?
- What budget category would this come from?
- Who would approve it?
- What would make it worth $99/month?
- What would make it worth $499/month?

## Closing

Ask:

- Can you introduce me to one other founder, finance lead, developer, or operator dealing with stablecoin payment workflows?
- Can I follow up with a short summary of what I heard?
- If we build the first MVP around this workflow, would you be willing to test it?

## Notes Format

After the call, write:

- Company:
- Segment:
- Interviewee:
- Role:
- Stablecoin volume:
- Chains/tokens:
- Current workflow:
- Concrete pain examples:
- Existing tools:
- Strongest quote:
- Design partner fit:
- Follow-up action:

