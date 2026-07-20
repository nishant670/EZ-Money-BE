# FINNRI Data Model

## Entities
### User
id, guest_id, phone/email optional, display_name, locale, currency, created_at

### Plan
id, code, name, billing_interval (`weekly`, `monthly`, `quarterly`, `yearly`,
`lifetime_quote`), price_minor, currency, included_credits, daily_credit_limit,
is_public, requires_login, requires_prior_paid_months, timestamps.

Plans describe what a user can buy or request. Lifetime is represented as a
quote-only interval and must not be directly purchasable.

### UserSubscription
id, user_id, plan_id, status (`trialing`, `active`, `past_due`, `cancelled`,
`expired`), current_period_start, current_period_end, provider,
provider_customer_id, provider_subscription_id, cancel_at_period_end,
timestamps.

Subscription rows mirror verified payment-provider state. Credits are not
implied by status alone; subscription activation or renewal must create an
explicit `CreditGrant`.

### CreditGrant
id, nullable user_id, nullable guest_device_id_hash, source (`free_trial`,
`subscription_period`, `manual_adjustment`, `refund`, `promo`,
`lifetime_quote`), credits_granted, credits_remaining, valid_from, expires_at,
nullable subscription_id, timestamps.

Credit grants are spendable buckets. Debits consume the soonest-expiring
available grants first. Database constraints keep `credits_remaining`
non-negative and no larger than `credits_granted`.

### CreditLedger
id, nullable user_id, nullable guest_device_id_hash, nullable grant_id,
direction (`grant`, `debit`, `refund`, `adjustment`, `expiry`), credits,
balance_after, reason_code, idempotency_key, nullable ai_usage_event_id,
created_at.

Every credit movement must create a ledger row. Ledger idempotency keys prevent
double charging when a client retries or a webhook is delivered more than once.

### AIUsageEvent
id, nullable user_id, nullable guest_device_id_hash, session_id, request_id,
idempotency_key, action_code, input_kind, status, provider/model metadata,
estimated_credits, reserved_credits, final_credits, estimated and actual cost in
USD micros, token counts, audio duration/bytes, input and response sizes,
error_code, started/provider/finished timestamps.

Every AI provider attempt must create one usage event. Raw prompts, transcripts,
audio, and provider responses are not stored here.

### AIUsageLimitEvent
id, nullable user_id, nullable guest_device_id_hash, action_code, reason,
required_credits, available_credits, daily_limit, used_today, reset_at,
created_at.

Limit events are written when an AI action is denied before provider execution.
They support dashboard metrics for users hitting daily caps, users hitting total
credit caps, and feature-lock failures without depending only on request logs.

### AIModelPricing
id, provider, model, operation (`llm`, `transcription`, `credit_fallback`),
input_token_usd_micros, output_token_usd_micros, audio_minute_usd_micros,
request_usd_micros, credit_usd_micros, active, timestamps.

Pricing rows allow operations to update cost estimates without a code deploy.
The active `(provider, model, operation)` row is used first; when no provider
pricing matches, accounting falls back to a credit-based estimate.

### AIAbuseBlock
id, nullable user_id, nullable guest_device_id_hash, scope (`ai_parse`,
`all_ai`), reason_code, notes, active, expires_at, created_by, timestamps.

Abuse blocks are an admin support control for stopping AI usage by user or
guest-device hash without deleting the account or credit history. Expired or
inactive blocks are ignored by AI parse enforcement.

### DailyCreditUsage
id, nullable user_id, nullable guest_device_id_hash, usage_date, credits_used,
timestamps.

This table enforces per-day credit caps for free, guest, and paid users. Unique
indexes are split by user and guest identity so nullable IDs cannot bypass daily
limits.

### GuestUsageKey
id, guest_device_id_hash, ip_hash, first_seen_at, last_seen_at, nullable
trial_grant_id, abuse_score, timestamps.

Guest usage keys reduce repeated free-trial grants across reinstall/session
loops. Device IDs and IPs are stored only as hashes.

### LifetimeQuoteRequest
id, user_id, status (`requested`, `reviewed`, `quoted`, `declined`,
`cancelled`), paid_months_completed, usage_window_start, usage_window_end,
usage_event_count, credits_used, average_monthly_credits,
estimated_cost_usd_micros, average_monthly_cost_usd_micros, notes, timestamps.

Lifetime quote requests preserve the user's actual 90-day AI usage summary for
admin review. They do not grant credits or activate access by themselves.

### Account
id, user_id, name, type, institution_name, last_four_optional,
credit_limit fixed-point money, balance fixed-point money, is_default,
timestamps

Account `type` uses lowercase singular API values: `cash`, `upi`, `bank`,
`credit_card`, `debit_card`, `wallet`, or `other`. Legacy aliases `credit`,
`debit`, and `wallets` are normalized to the canonical values before storage.

### Category
Deferred as a separate table for MVP. Entries keep a required `category` string
so AI capture, manual entry, filters, dashboard rollups, and mobile forms remain
simple while category names are still changing.

Future normalized shape: id, user_id nullable, name, type, icon, color,
is_system, timestamps.

Migration plan:
- Create `categories` with seeded system rows and optional user rows.
- Backfill distinct `(user_id, lower(category), type)` values from entries.
- Add nullable `entries.category_id` while continuing to expose `category`.
- Dual-read by preferring `category_id` when present and falling back to the
  string column.
- Dual-write both fields for one client release, then make `category_id`
  required once old clients are no longer supported.

### Transaction
id, user_id, account_id required for new writes, amount, type, category string,
merchant, date, note, tags, source, optional source_text retained only after
user confirmation, ai_confidence, timestamps

Database constraints enforce positive transaction amounts, supported transaction
types (`expense`, `income`), supported sources (`manual`, `text`, `voice`), and
owned account references through a composite foreign key from
`entries(user_id, account_id)` to `accounts(user_id, id)`.

### ParseAttempt
Deferred for MVP to minimize sensitive raw financial text storage. Do not persist parse attempts, transcripts, provider prompts, or raw provider responses until a short retention window, access controls, and deletion job are defined.

### Insight
id, user_id, type, title, body, severity, related_entity_id, dismissed_at, created_at

### RecurringCandidate
Computed dashboard payload for now, not persisted: label, merchant, category,
average_amount, interval_guess, confidence, occurrences, last_seen_date,
next_expected_date, review_due. Users can convert recurring behavior into
explicit subscription records when they want reminders and lifecycle management.

### Subscription
id, user_id, optional account_id, name, merchant, category, amount fixed-point
money, currency, billing_interval (`weekly`, `monthly`, `yearly`),
next_due_date, last_charged_date, status (`active`, `paused`, `cancelled`),
reminder_days, notes, timestamps.

Subscriptions are user-owned and may reference an owned account. They do not
auto-create transaction entries; marking a subscription paid advances
`next_due_date` and stores `last_charged_date`.

### SubscriptionReminder
id, user_id, subscription_id, due_date, kind (`due`, `overdue`), optional
notification_id, created_at. A unique key on user, subscription, due date, and
kind prevents duplicate in-app reminders.

### SplitFriend
id, user_id, name, email, phone, archived, timestamps

### SplitGroup
id, user_id, name, archived, timestamps

### SplitGroupMember
id, user_id, group_id, friend_id, timestamps

### SplitBill
id, user_id, entry_id optional, group_id optional, title, total_amount fixed-point money,
currency, date, notes, timestamps

### SplitParticipant
id, user_id, bill_id, friend_id, share_amount fixed-point money, direction,
timestamps. Direction is `friend_owes_user` or `user_owes_friend`.

### SplitSettlement
id, user_id, friend_id, amount fixed-point money, direction, date, notes,
timestamps. Direction is `friend_paid_user` or `user_paid_friend`.

Friend balances are computed from participant shares minus settlements. Positive
net balance means the friend owes the user; negative net balance means the user
owes the friend.
