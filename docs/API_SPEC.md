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

## Dashboard
`GET /v1/dashboard?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD` returns only
deterministic values calculated from the authenticated user's transactions:
period totals, daily average, top categories, top merchants, account-wise
spending, five recent transactions, and insight cards. Date ranges are capped
at 366 days. `/v1/insights` remains a temporary compatibility alias.

## Important Rule
`POST /v1/parse` returns a draft only. It must not save a transaction.

## Auth Response
Auth endpoints that create a session return an opaque bearer `token`,
`expires_at`, and the user object. The token is stored server-side only as a
hash and must be treated as revocable.

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

## Parse Response Should Include
amount, currency, type, merchant, category, account_hint, date, note, tags, recurring_candidate, split_candidate, confidence, missing_fields.
