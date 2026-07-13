# FINNRI API Spec - MVP

## Endpoints
- POST /v1/parse
- POST /v1/entries
- GET /v1/entries
- GET /v1/entries/:id
- PUT /v1/entries/:id
- DELETE /v1/entries/:id
- POST /v1/auth/guest
- GET /v1/accounts
- POST /v1/accounts
- PUT /v1/accounts/:id
- DELETE /v1/accounts/:id
- GET /v1/dashboard
- GET /v1/insights
- GET /v1/notifications
- GET /v1/notifications/unread-count
- PATCH /v1/notifications/:id/read
- PATCH /v1/notifications/read-all
- DELETE /v1/notifications/:id
- GET /v1/budgets
- POST /v1/budgets
- PUT /v1/budgets/:id
- DELETE /v1/budgets/:id
- GET /v1/subscriptions
- POST /v1/subscriptions
- PUT /v1/subscriptions/:id
- DELETE /v1/subscriptions/:id
- POST /v1/subscriptions/:id/mark-paid
- POST /v1/subscriptions/reminders
- POST /v1/split/friends
- GET /v1/split/friends
- PUT /v1/split/friends/:id
- DELETE /v1/split/friends/:id
- POST /v1/split/groups
- GET /v1/split/groups
- PUT /v1/split/groups/:id
- DELETE /v1/split/groups/:id
- POST /v1/split/bills
- GET /v1/split/bills
- POST /v1/split/settlements
- GET /v1/split/settlements
- GET /v1/split/balances
- POST /v1/tools/emi/calculate
- DELETE /v1/user

## Dashboard
`GET /v1/dashboard?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD` returns only
deterministic values calculated from the authenticated user's transactions:
period totals, daily average, top categories, top merchants, account-wise
spending, five recent transactions, and insight cards. Date ranges are capped
at 366 days. The payload also includes lightweight `recurring_candidates`
computed from stable weekly or monthly merchant/category repeats, with
`review_due` set when the next expected occurrence falls inside the selected
period or the following seven-day review window. `/v1/insights` remains a
temporary compatibility alias.

## Important Rule
`POST /v1/parse` returns a draft only. It must not save a transaction.

## Auth Response
Auth endpoints that create a session return an opaque bearer `token`,
`expires_at`, and the user object. The token is stored server-side only as a
hash and must be treated as revocable.

## Account/Data Deletion
`DELETE /v1/user` permanently deletes the authenticated user's transactions,
accounts, budgets, subscriptions, split ledger records, quick prompts,
notifications, sessions, profile record, and matching OTP/claim verification
records. Legacy local upload files referenced by the user's entries are removed
only when they resolve safely under `uploads/`.

## OTP Verification
`POST /v1/auth/otp/send` creates a random, expiring OTP challenge for an email
or phone identifier and stores only the OTP hash. `POST /v1/auth/otp/verify`
exchanges a valid OTP for an opaque, expiring, one-time `claim_token`.
Registration and contact updates must consume that claim token; plaintext
tokens such as `claim_email:...` or `claim_phone:...` are invalid.

## Canonical Contract
Phase 1.2 keeps the implemented versioned routes as the authoritative MVP API:
transaction persistence is exposed as `/v1/entries`, and parsing is exposed as
`/v1/parse`. Product copy may still call these records "transactions", but docs,
clients, and OpenAPI should use the versioned route names above.

## Transaction Account Rule
Transaction create and update payloads require `account_id`. The account must belong to the authenticated user. Transaction responses include the linked account summary, and account deletion must fail while transactions still reference it.

## Notifications
Notifications are authenticated, user-owned records exposed through `/v1/notifications`.
Notification sources include transaction create, update, and delete events plus
budget warning/exceeded alerts. Clients can list by `status=all|unread|read`,
fetch an unread count, mark one notification read, mark all read, or delete one
notification. Push delivery is not part of this contract yet; these are in-app
notifications.

## Budgets
Budgets are authenticated, user-owned records exposed through `/v1/budgets`.
Budgets are monthly INR limits and may target either all expenses or one category.
When a new or updated expense causes current-month spending to reach the configured
warning threshold or the full limit, the backend creates one in-app notification
per budget, threshold, and month.

## Subscriptions
Subscriptions are authenticated, user-owned records exposed through
`/v1/subscriptions`. Records track merchant/category, INR amount, optional owned
account, billing interval (`weekly`, `monthly`, or `yearly`), next due date,
status, reminder window, and notes. List responses include `days_until_due` and
`due_state` (`scheduled`, `due_soon`, `overdue`, `paused`, or `cancelled`).
`POST /v1/subscriptions/:id/mark-paid` records a paid date and advances the next
due date by the billing interval without creating a transaction. `POST
/v1/subscriptions/reminders` creates deduplicated in-app due/overdue notifications
for active subscriptions inside their reminder window.

## Split Ledger
Split endpoints are authenticated and user-owned. Friends are local contacts for
bill splitting. Groups are named friend collections for common trips,
households, or recurring crews. Split bills contain one or more participant shares where
`friend_owes_user` increases the friend's balance and `user_owes_friend`
increases what the user owes. Settlements use `friend_paid_user` or
`user_paid_friend` to reduce outstanding balances. `GET /v1/split/balances`
returns positive `net_balance` when the friend owes the user, and negative when
the user owes the friend.

`POST /v1/entries` may include an optional `split` object. When present, the
transaction and linked split bill are created atomically. Participants can
reference existing `friend_id` values or include a new `friend.name`; `group_id`
can attach the bill to an existing group, while `group_name` creates a new group
from the transaction's participants.

## EMI Tools
`POST /v1/tools/emi/calculate` is an authenticated, stateless INR calculator. It
accepts `principal_amount`, `annual_interest_rate_percent`, `tenure_months`, and
optional `currency`, then returns `monthly_emi`, `total_payment`,
`total_interest`, and a month-by-month amortization `schedule`. Tenure is capped
at 360 months and annual interest is capped at 100%.

## Parse Response Should Include
amount, currency, type, merchant, category, account_hint, date, note, tags, recurring_candidate, split_candidate, confidence, missing_fields.
