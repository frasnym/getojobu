# go-get-job — Business Plan and Monetization

## 1. Executive summary

**go-get-job** is a global job-search and application-assistance platform.

It helps job seekers discover opportunities, understand how well a role matches their career profile, prepare tailored application materials, and track their applications.

The initial product combines:
- Job discovery and aggregation
- AI-assisted job matching and evaluation
- CV tailoring and cover-letter generation
- Application tracking
- Approval-gated application assistance

The business model is **hybrid**:
- Subscription plans provide platform access, feature entitlements, and recurring monthly credit allowances.
- Credit top-ups let users pay for additional AI-intensive usage.

The product is designed for a global audience. Actual prices, supported currencies, credit quantities, and payment-provider selection remain to be validated.

## 2. Product positioning

### Problem

Job seekers often need to:
- Search across multiple job boards and company career pages.
- Decide which roles are worth applying to.
- Adapt their CV and cover letter to each opportunity.
- Keep track of applications, follow-ups, and outcomes.

These tasks are repetitive and time-consuming. go-get-job aims to bring the workflow into one place and use AI to assist with the parts that benefit from personalization.

### Value proposition

**Find relevant jobs, prepare stronger applications, and manage the job search in one workflow.**

The product should help users make better decisions and reduce repetitive work. It should not promise interviews, employment, or guaranteed application success.

## 3. Target market

### Market scope

- Target market: global job seekers.
- Product and billing architecture should support international users.
- Currency, localization, tax, and payment-method support should be designed for expansion.
- Initial supported markets and languages are still to be determined.

### Initial customer segments to validate

These are hypotheses for customer discovery, not confirmed market research:

1. **Active job seekers** — applying regularly and needing help prioritizing opportunities.
2. **International applicants** — searching across countries and needing help tailoring applications to different roles and markets.
3. **Career changers** — evaluating transferable skills and adapting their career materials.
4. **Time-constrained professionals** — wanting to reduce repetitive job-search and application preparation work.

Do not assume all segments have the same willingness to pay. Validate demand through interviews, landing-page tests, and early usage.

## 4. Product scope

### Core user journey

1. Register and manage an account.
2. Build a career profile, including experience and skills.
3. Discover jobs from supported sources.
4. Evaluate and rank jobs against the user's profile.
5. Review relevant jobs.
6. Generate or tailor a CV and cover letter.
7. Review and approve application materials.
8. Submit an application only after explicit user approval.
9. Track application status and activity.

### Application automation policy

Application submission is approval-gated.

- AI may prepare application materials.
- The user must review and explicitly approve the submission.
- A paid plan or credit balance does not constitute application consent.
- Ambiguous submission outcomes must be reconciled rather than blindly retried.
- Fully unattended application submission is outside the initial product policy.

## 5. Business model

### Hybrid monetization

The platform uses two complementary mechanisms.

**Subscription**
- Recurring payment for access to plan-specific features and limits.
- Includes a monthly AI credit allowance for paid plans.
- Subscription status controls entitlements, not authentication.

**Credit top-ups**
- One-time purchases of additional credits.
- Available to Free, Pro, and Premium users.
- Used for variable-cost AI operations.
- Purchased credits do not expire under the current policy.

Basic product capabilities should not consume credits unnecessarily. For example, profile management and application tracking can be plan entitlements rather than metered AI actions.

## 6. Proposed plans

The initial plan structure is Free, Pro, and Premium. Pricing and exact quotas are not finalized.

| Capability | Free | Pro | Premium |
|---|---|---|---|
| Job discovery | Limited | Higher limits | Higher limits |
| Basic job matching | Included, subject to limits | Included | Included |
| AI job evaluation | Limited allowance | Monthly credit allowance | Larger monthly credit allowance |
| CV tailoring | Credit-based | Credit-based | Credit-based |
| Cover-letter generation | Credit-based | Credit-based | Credit-based |
| Application tracking | Included | Included | Included |
| Advanced matching | To be defined | Proposed | Proposed |
| Application assistance | Limited | Higher allowance | Higher allowance |

The table is a product proposal. Feature entitlements and quotas must be validated before publishing a pricing page.

Avoid advertising unlimited AI usage. Set explicit fair-use and credit limits for operations with variable model or browser costs.

## 7. Credit policy

### Confirmed rules

| Credit type | Grant condition | Expiration |
|---|---|---|
| Welcome credits | One-time after account registration | 30 days |
| Subscription credits | At subscription activation and each successful renewal | End of the relevant billing period |
| Purchased credits | After confirmed successful top-up payment | Never |
| Administrative adjustment | Authorized support/operations action | Defined per adjustment |

### Credit purchase eligibility

All users, including Free users, may purchase top-ups.

### Spending priority

Consume credits with the earliest expiration first. Non-expiring purchased credits should normally be consumed after expiring credits.

### Failed operations

- Reserve credits before an expensive operation starts.
- Capture the charge after successful completion.
- Release reservations when an operation fails or is cancelled.
- Do not charge for infrastructure failures.
- Make operation retries idempotent to prevent duplicate charges.

### Refunds and negative balances

The current product decision is to allow a negative credit balance when a refund reverses credits that have already been spent.

- Reverse the original credit grant.
- If some credits remain, remove the remaining amount.
- Record the unrecoverable remainder as credit debt.
- Block further credit-consuming operations while the balance is negative.
- Apply future grants against the debt before making credits spendable.

Refund eligibility, consumer rights, and payment-provider requirements must still be reflected in the published refund policy and applicable local law. The negative-balance rule is an accounting behavior, not a substitute for legal refund obligations.

## 8. Credit-consuming features

Candidate metered operations:

| Feature | Proposed billing |
|---|---|
| Basic job browsing | Plan entitlement / usage limit |
| Basic matching | Included or plan-limited |
| AI job evaluation | Credits |
| CV tailoring | Credits |
| Cover-letter generation | Credits |
| AI application assistance | Credits |
| Application tracking | Included |
| Profile and skill management | Included |

Exact credit costs are not set yet.

Credit pricing should be based on measured unit economics, not arbitrary round numbers. Measure:
- Model/provider cost per operation
- Input and output token usage
- Browser/worker infrastructure cost where applicable
- Retry and failure rates
- Storage and other variable costs
- Gross margin target

Consider different model/provider costs and operation complexity when setting credit costs.

## 9. Pricing and revenue strategy

### Pricing principles

- Display prices clearly in a supported currency.
- Store monetary amounts as integers in currency minor units, with an ISO currency code.
- Separate plan identity from price so monthly, annual, and regional prices can coexist.
- Preserve historical prices and purchase amounts.
- Avoid promising a price or credit quota until unit economics have been measured.

### Revenue streams

1. Monthly subscriptions.
2. Annual subscriptions, potentially with a discount after validation.
3. One-time credit top-ups.
4. Potential future add-ons, only if customer demand supports them.

### Pricing experiments

Validate:
- Free-to-paid conversion.
- Subscription conversion by customer segment and country.
- Top-up purchase rate.
- Average revenue per paying user.
- Credit utilization and unused-credit patterns.
- Gross margin by AI feature.
- Subscription retention and cancellation reasons.

## 10. Payment provider strategy

The payment provider has not been finalized.

Evaluate providers that support:
- A business operating from Indonesia.
- Global customer payments.
- Recurring subscriptions.
- One-time credit purchases.
- Webhooks and reliable event identifiers.
- Refunds and disputes.
- Appropriate tax handling for a global digital product.
- Payouts to the business's supported bank account and currency.

A Merchant of Record provider may reduce the operational burden of international sales tax/VAT, but eligibility and the provider's rules for AI services and credit-based products must be confirmed directly.

Keep payment integration behind a provider-independent Go interface. Do not let provider-specific IDs become internal primary keys.

## 11. Billing architecture

Billing is a module inside the Go modular monolith. It is not a separate microservice.

### Core tables

| Table | Responsibility |
|---|---|
| `plans` | Free, Pro, Premium definitions |
| `plan_prices` | Currency, billing interval, provider price mapping |
| `plan_entitlements` | Feature access and limits |
| `subscriptions` | Subscription lifecycle and provider references |
| `billing_orders` | One-time credit purchases |
| `billing_events` | Idempotent payment-provider webhook processing |
| `credit_accounts` | Aggregate available and reserved balance |
| `credit_lots` | Individual grants with source and expiration |
| `credit_transactions` | Append-only credit accounting ledger |
| `usage_records` | Billable AI operation lifecycle and idempotency |

### Sources of truth

- Clerk: authentication and identity.
- Go API: business rules, entitlements, and billing orchestration.
- PostgreSQL: subscription mirror, orders, credit balances, ledger, and usage history.
- Payment provider: payment execution and provider-side subscription/payment state.
- Temporal: long-running AI workflows; not the source of truth for credit balances.

### Important accounting invariants

- All credit changes are transactional.
- Account balance, lot balances, and ledger entries must change together.
- Credit grants and webhook handling must be idempotent.
- Financial history is not deleted; corrections use compensating entries.
- The frontend cannot directly grant, deduct, or modify credits.
- A negative available balance blocks new credit-consuming operations.
- Reservations must be reconciled if a workflow becomes stuck.

## 12. Go-to-market approach

A practical staged approach:

### Stage 1 — Validate the problem

- Interview active job seekers in the target segments.
- Identify which parts of the job-search workflow are most painful.
- Test messaging and positioning with a landing page.
- Validate whether users value matching, CV tailoring, or application tracking most.

### Stage 2 — Private MVP

- Invite a small group of users.
- Measure job discovery quality and duplicate rates.
- Measure AI evaluation usefulness.
- Measure CV/cover-letter quality and edit rates.
- Test credit reservations, failures, and refunds internally.
- Keep application submission approval-gated.

### Stage 3 — Paid beta

- Introduce a small number of plans and credit packages.
- Test payment checkout and webhook reliability.
- Measure conversion, usage, cost, and support burden.
- Review credit costs against actual AI/provider expenses.

### Stage 4 — Public launch

- Publish transparent plan limits and credit costs.
- Add localized pricing or payment methods where justified.
- Expand job sources and markets based on data quality and customer demand.
- Improve retention through saved searches, alerts, and application workflow improvements.

## 13. Key metrics

### Acquisition
- Landing-page conversion
- Registration conversion
- Cost per acquired user, when paid acquisition is used

### Activation
- Percentage completing a career profile
- Time to first relevant job
- Percentage evaluating at least one job
- Percentage generating application material

### Engagement
- Weekly active job seekers
- Jobs saved per active user
- AI evaluations per active user
- Applications tracked per active user

### Monetization
- Free-to-paid conversion
- Monthly recurring revenue (MRR)
- Average revenue per paying user
- Top-up conversion and revenue
- Subscription churn
- Refund and dispute rates

### Unit economics
- AI cost per operation
- Average credits consumed per user
- Gross margin by plan and feature
- Failed-operation and retry cost
- Contribution margin per paying user

### Quality
- Duplicate job rate
- Job freshness and broken-link rate
- Match usefulness feedback
- CV/cover-letter edit rate
- Application submission failure rate

## 14. Risks and mitigations

| Risk | Mitigation |
|---|---|
| AI costs exceed subscription revenue | Meter expensive operations, measure unit economics, set limits |
| Poor job-source quality | Monitor duplicate, stale, and broken-link rates |
| Duplicate listings | Deterministic canonical slug and ingestion deduplication |
| Payment webhook retries | Unique provider event IDs and idempotent processing |
| Credit double-spending | Transactional reservations and row locking |
| Refund after credits are spent | Compensating ledger entries and negative-balance restrictions |
| International tax complexity | Evaluate Merchant of Record providers and obtain appropriate advice |
| Unsafe application automation | Explicit per-application user approval and audit trail |
| Overly complex pricing | Start with a small plan and package catalog |

## 15. Decisions still open

The following have not been finalized:

- Actual Free, Pro, and Premium prices.
- Monthly and annual credit allocations.
- Welcome credit quantity.
- Credit package sizes and prices.
- Credit cost per AI operation.
- Supported currencies and regional pricing.
- Final payment provider and merchant eligibility.
- Tax, refund, and dispute terms.
- Initial launch countries and languages.
- Exact feature limits per plan.
- Whether annual plans receive a discount.

## 16. Summary of agreed direction

go-get-job will launch as a global job-search and application-assistance platform with a hybrid subscription and credit model.

Users receive access and feature limits through plans. AI-intensive operations consume credits. Free users receive a one-time welcome allowance that expires after 30 days. Subscription credits expire at the end of their billing period. Purchased credits do not expire and are available to all users. If a refund reverses already-spent credits, the account can become negative and further credit usage is blocked until the debt is cleared.

Application submission always requires explicit user approval. Authentication remains with Clerk, while the Go API and PostgreSQL own application billing rules and credit accounting.
