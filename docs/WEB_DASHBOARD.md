# FINNRI Web Dashboard

## Product role

The web dashboard is FINNRI's big-screen analysis and management surface. It
extends the same confirm-first transaction model used by mobile; it is not a
separate source of financial truth.

The first active web phase prioritizes:

1. Explainable insights from confirmed transactions.
2. Searchable transaction review and account context.
3. Practical tools already supported by the backend.
4. Honest empty, loading, unavailable, and plan-gated states.

## Implemented routes

| Web route | Purpose | Backend APIs |
| --- | --- | --- |
| `/dashboard` | Period overview, top categories and merchants, recent activity, key insight | `GET /v1/dashboard` |
| `/dashboard/insights` | Custom-period analysis, account usage, recurring review, formula explanation | `GET /v1/dashboard` |
| `/dashboard/transactions` | Search, filters, pagination, CSV of the visible page, entry detail, capture, and inline splits | `/v1/entries`, `/v1/parse`, `/v1/accounts`, `/v1/split/*` |
| `/dashboard/accounts` | Real account list and CRUD | `/v1/accounts` |
| `/dashboard/splits` | Friends, groups, bills, balances, settlements, and activity | `/v1/split/*` |
| `/dashboard/tools` | EMI, monthly budgets, recurring-payment schedules | `/v1/tools/emi/calculate`, `/v1/budgets`, `/v1/subscriptions` |
| Dashboard shell | Notification review and global transaction search | `/v1/notifications` |
| `/dashboard/settings` | Basic profile update | `PUT /v1/user` |

## Insight rules

- The web client renders backend dashboard values instead of recalculating or
  inventing financial health scores.
- Period comparisons use the backend's immediately preceding equal-length
  range.
- Recurring candidates remain review suggestions. They are not automatically
  saved as subscriptions.
- Charts have adjacent labels and numeric summaries so color is not the only
  information carrier.
- The web product does not present investment, tax, lending, or legal advice.

## Tool rules

- EMI results come from the stateless backend calculator and are labelled as
  estimates.
- Budgets create in-app alerts from confirmed expenses. Plan-gated responses
  are shown as unavailable states rather than broken controls.
- Marking a recurring payment paid advances its schedule but does not create a
  transaction.
- Account balances are manually maintained; the UI does not claim bank sync.
- Split balances are calculated from user-recorded friend shares and
  settlements; FINNRI does not move money or notify friends.

## Deferred web power-user work

- Cross-page or bulk transaction editing.
- Full-history export and advanced reports generated server-side.
- Statement import and reconciliation.
- Bank, UPI, and Account Aggregator connectivity.
- Open-ended AI financial advice.

## Deployment configuration

The web build requires `NEXT_PUBLIC_API_URL` to point to a reachable FINNRI API.
The backend must allow the deployed web origin through its restrictive CORS
configuration. Do not publish a production dashboard that silently falls back
to localhost.
