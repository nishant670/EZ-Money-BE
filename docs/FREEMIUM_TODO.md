# FINNRI Freemium And AI Usage Accounting TODO

This is the implementation checklist for turning FINNRI into a freemium product
with a robust AI usage accounting system. Pick tasks from top to bottom. Do not
ship paid plans before Phase 1 and Phase 2 are complete, because pricing without
accurate usage accounting creates uncontrolled AI cost risk.

## Product Rules

- [ ] Keep manual finance tracking free: manual entries, accounts, basic
  transaction list, basic dashboard, security settings, and user data deletion.
- [ ] Meter every AI-assisted action through Finnri AI credits.
- [ ] Charge credits for provider attempts even when parsing fails after the
  provider call, because provider cost was still incurred.
- [ ] Do not charge credits for validation failures that happen before any AI
  provider call.
- [ ] Never trust client-reported credit cost, plan, subscription status, model,
  token count, or audio duration.
- [ ] Enforce all limits on the backend before calling OpenAI.
- [ ] Make credits understandable in the app, but keep the provider-cost formula
  server-side.
- [ ] Use hard server-side safety caps even for paid users.
- [ ] Treat guest allowances as abuse-prone and intentionally small.

## Current AI Cost Surfaces

- [ ] Text AI transaction parse via `POST /v1/parse` with `hint_text`.
  - Cost: one LLM call.
  - Current model: `gpt-4o-mini`.
  - Suggested debit: 5 credits.
- [ ] Voice AI transaction parse via `POST /v1/parse` with `audio`.
  - Cost: speech-to-text call plus LLM parse call.
  - Current models: `gpt-4o-mini-transcribe` plus `gpt-4o-mini`.
  - Suggested debit: 12 credits for short audio, 18+ credits for longer audio.
- [ ] AI-produced metadata inside the parse response.
  - Category, merchant, tags, account hint, subscription candidate, split
    candidate, confidence, missing fields, and clarifications are included in
    the same parse call. Do not double-charge for these yet.
- [ ] Future AI features must be registered before implementation.
  - Examples: AI advisor, weekly AI summary, bulk categorization, statement
    import, OCR, receipt parsing, voice assistant, AI reports.

## Initial Credit Policy

- [ ] Define `1 Finnri AI credit` as an internal budget unit, not a provider
  token.
- [ ] Use this initial planning target:
  - 1000 free credits should cost no more than USD 0.10 per user.
  - 1 credit should represent roughly USD 0.00008 to USD 0.00010 of maximum
    allowed AI cost.
- [ ] Logged-in free users:
  - Grant 1000 one-time free credits.
  - Expire the grant after 30 days.
  - Limit usage to 50 credits per day.
  - Allow manual tracking after credits expire.
- [ ] Guest users:
  - Grant 150-300 trial credits per device for 30 days.
  - Limit usage to 10-15 credits per day.
  - Require login before buying a subscription.
- [ ] Paid users:
  - Monthly, quarterly, and yearly plans are public.
  - Weekly can be added as a short paid pass only after payment fees are
    reviewed.
  - Lifetime is visible as "quote after 3 paid months"; it must not be directly
    purchasable.
- [ ] Lifetime subscription rule:
  - User must complete at least 3 paid months.
  - Quote from actual 90-day AI cost, expected support/storage cost, and risk
    buffer.
  - Apply a fair-use cap even to lifetime users.

## Suggested Plan Limits

- [ ] Free logged-in: 1000 trial credits, 50/day, expires in 30 days.
- [ ] Guest: 150-300 trial credits, 10-15/day, expires in 30 days.
- [ ] Weekly pass: 700 credits/week, 150/day.
- [ ] Monthly: 3000 credits/month, 200/day.
- [ ] Quarterly: 10000 credits/quarter, 250/day.
- [ ] Yearly: 45000 credits/year, 300/day.
- [ ] Lifetime quoted: custom allowance with fair-use cap and abuse review.

## Phase 1 - AI Usage Registry

- [x] Create a backend registry of all metered AI actions.
- [x] Include stable action codes:
  - `transaction_parse_text`
  - `transaction_parse_voice_short`
  - `transaction_parse_voice_medium`
  - `transaction_parse_voice_long`
  - `future_ai_advisor_message`
  - `future_ai_weekly_summary`
  - `future_ai_bulk_categorization`
  - `future_ai_statement_import`
- [x] For every action, store:
  - Action code.
  - User-visible label.
  - Whether guest is allowed.
  - Default credit estimate.
  - Maximum allowed credit debit.
  - Required provider operations.
  - Input limits.
  - Whether paid plan is required.
- [x] Keep this registry in backend code, not in the frontend.
- [x] Add tests proving unregistered AI actions cannot be charged or executed.

## Phase 2 - Database Schema

- [x] Add `plans`.
  - `id`
  - `code`
  - `name`
  - `billing_interval`: `weekly`, `monthly`, `quarterly`, `yearly`,
    `lifetime_quote`
  - `price_minor`
  - `currency`
  - `included_credits`
  - `daily_credit_limit`
  - `is_public`
  - `requires_login`
  - `requires_prior_paid_months`
  - `created_at`
  - `updated_at`
- [x] Add `user_subscriptions`.
  - `id`
  - `user_id`
  - `plan_id`
  - `status`: `trialing`, `active`, `past_due`, `cancelled`, `expired`
  - `current_period_start`
  - `current_period_end`
  - `provider`
  - `provider_customer_id`
  - `provider_subscription_id`
  - `cancel_at_period_end`
  - `created_at`
  - `updated_at`
- [x] Add `credit_grants`.
  - `id`
  - `user_id` nullable for guest-device grants if needed.
  - `guest_device_id_hash` nullable.
  - `source`: `free_trial`, `subscription_period`, `manual_adjustment`,
    `refund`, `promo`, `lifetime_quote`
  - `credits_granted`
  - `credits_remaining`
  - `valid_from`
  - `expires_at`
  - `subscription_id` nullable.
  - `created_at`
  - `updated_at`
- [x] Add `credit_ledger`.
  - `id`
  - `user_id` nullable.
  - `guest_device_id_hash` nullable.
  - `grant_id` nullable.
  - `direction`: `grant`, `debit`, `refund`, `adjustment`, `expiry`
  - `credits`
  - `balance_after`
  - `reason_code`
  - `idempotency_key`
  - `ai_usage_event_id` nullable.
  - `created_at`
- [x] Add `ai_usage_events`.
  - `id`
  - `user_id` nullable.
  - `guest_device_id_hash` nullable.
  - `session_id` nullable.
  - `request_id`
  - `idempotency_key`
  - `action_code`
  - `input_kind`: `text`, `voice`, `image`, `file`, `chat`
  - `status`: `reserved`, `succeeded`, `failed_before_provider`,
    `failed_after_provider`, `refunded`, `cancelled`
  - `provider`
  - `model`
  - `secondary_provider` nullable.
  - `secondary_model` nullable.
  - `estimated_credits`
  - `reserved_credits`
  - `final_credits`
  - `estimated_cost_usd_micros`
  - `actual_cost_usd_micros` nullable.
  - `prompt_tokens` nullable.
  - `completion_tokens` nullable.
  - `total_tokens` nullable.
  - `audio_duration_ms` nullable.
  - `audio_bytes` nullable.
  - `input_chars` nullable.
  - `response_bytes` nullable.
  - `error_code` nullable.
  - `started_at`
  - `provider_started_at` nullable.
  - `finished_at` nullable.
- [x] Add `daily_credit_usage`.
  - `id`
  - `user_id` nullable.
  - `guest_device_id_hash` nullable.
  - `usage_date`
  - `credits_used`
  - `created_at`
  - `updated_at`
  - Unique key across user/device and date.
- [x] Add `guest_usage_keys`.
  - `id`
  - `guest_device_id_hash`
  - `ip_hash`
  - `first_seen_at`
  - `last_seen_at`
  - `trial_grant_id` nullable.
  - `abuse_score`
- [x] Add indexes for:
  - Active subscriptions by user.
  - Unexpired credit grants by user/device.
  - Ledger idempotency keys.
  - AI usage request IDs.
  - Daily usage lookups.
- [x] Add constraints so balances cannot go negative except through explicit
  admin adjustment paths.

## Phase 3 - Credit Granting

- [x] On logged-in user creation, grant 1000 free credits once.
- [x] Set free credit expiry to 30 days from grant creation.
- [x] On guest creation, grant the guest allowance once per hashed device key.
- [x] Prevent repeated guest grants from reinstall/login loops using:
  - Device ID hash.
  - IP hash.
  - Auth session history.
  - Rate limit history.
- [x] On subscription activation, grant included credits for the current billing
  period.
- [x] On subscription renewal, create a new period grant.
- [x] Decide whether paid credits roll over.
  - Recommended v1: no rollover; unused period credits expire.
- [x] Add a scheduled expiry job.
  - [x] Move expired remaining credits to an `expiry` ledger entry.
  - [x] Set remaining credits to zero.
- [x] Add tests for free grant once-only behavior.
- [x] Add tests for subscription renewal grant behavior.
- [x] Add tests for expiry behavior.

## Phase 4 - Credit Reservation And Finalization

- [x] Build a server-side `CreditService`.
- [x] Add `CheckAllowance(user/device, action_code)`:
  - Confirm action exists in registry.
  - Load user subscription state.
  - Determine free, guest, or paid allowance.
  - Check daily cap.
  - Check available unexpired credits.
  - Return allowed/denied plus reason.
- [x] Add `ReserveCredits(user/device, action_code, idempotency_key)`:
  - Use a database transaction.
  - Lock applicable credit grants.
  - Reserve estimated credits from soonest-expiring grants first.
  - Create `ai_usage_events` row with `reserved` status.
  - Create debit ledger rows linked to the event.
  - Increment `daily_credit_usage`.
  - Return the event ID.
- [x] Add `FinalizeUsage(event_id, provider_usage)`:
  - Store token/audio usage.
  - Estimate provider cost in USD micros.
  - Calculate final credits.
  - If final credits are lower than reserved, refund the difference.
  - If final credits are higher than reserved, optionally debit more up to the
    action's max debit.
  - Mark event `succeeded` or `failed_after_provider`.
- [x] Add `CancelReservation(event_id)`:
  - Use only for failures before provider call.
  - Refund reserved credits.
  - Mark event `failed_before_provider` or `cancelled`.
- [x] Ensure all operations are idempotent.
  - Repeated idempotency key should return the original reservation/event.
  - Repeated finalization should not double debit.
- [x] Add race-condition tests with parallel parse requests.
- [x] Add tests proving credits cannot go below zero.
- [x] Add tests proving daily limits cannot be bypassed by parallel requests.

## Phase 5 - Parse Endpoint Enforcement

- [x] Update `POST /v1/parse` to classify input as text or voice before calling
  the AI provider.
- [x] Reject empty or oversized input before reserving credits.
- [x] Estimate voice duration server-side from audio metadata or conservative
  byte-size fallback.
- [x] Map parse requests to actions:
  - Text: `transaction_parse_text`
  - Voice <= 15 sec: `transaction_parse_voice_short`
  - Voice 15-30 sec: `transaction_parse_voice_medium`
  - Voice > 30 sec: either `transaction_parse_voice_long` or reject for free
    users.
- [x] Reserve credits before transcription or LLM parsing.
- [x] If transcription fails after provider call, finalize as
  `failed_after_provider` and charge the voice reservation.
- [x] If transcription succeeds but LLM parse fails, finalize as
  `failed_after_provider` and charge because provider calls happened.
- [x] If schema validation fails after provider call, charge and record
  `error_code=schema_invalid`.
- [x] Return credit metadata in successful responses:
  - `credits_charged`
  - `credits_remaining_today`
  - `credits_remaining_total`
  - `plan_code`
- [x] Return a structured error on insufficient credits:
  - `error=insufficient_ai_credits`
  - `required_credits`
  - `available_credits`
  - `daily_limit_remaining`
  - `upgrade_required`
- [x] Return a structured error on daily limit:
  - `error=daily_ai_limit_reached`
  - `daily_limit`
  - `used_today`
  - `reset_at`

## Phase 6 - Subscription And Plan APIs

- [x] Add `GET /v1/billing/plans`.
  - [x] Return public plans and feature gates.
  - [x] Do not return internal provider cost formulas.
- [x] Add `GET /v1/billing/status`.
  - [x] Current plan.
  - [x] Subscription status.
  - [x] Current period.
  - [x] Total credits remaining.
  - [x] Daily credits used and remaining.
  - [x] Trial expiry.
  - [x] Lifetime eligibility status.
- [x] Add `POST /v1/billing/checkout`.
  - [x] Require logged-in non-guest user.
  - [x] Accept `plan_code`.
  - [x] Reject lifetime direct purchase.
  - [ ] Create checkout session with payment provider.
    - Endpoint currently returns `payment_provider_not_configured` after all
      product-rule validation. Implement after provider, keys, price IDs, and
      redirect URLs are chosen.
- [x] Add `POST /v1/billing/webhook`.
  - [ ] Verify provider signature.
  - [ ] Upsert subscription status.
  - [ ] Grant credits on successful payment/renewal.
  - [ ] Cancel or expire access on failed/cancelled subscription.
    - Endpoint currently returns `payment_webhook_not_configured` and performs
      no state mutation until a provider-specific signed webhook contract is
      available.
- [x] Add `POST /v1/billing/lifetime-quote/request`.
  - [x] Require at least 3 paid months.
  - [x] Store a quote request.
  - [x] Compute 90-day usage summary for admin review.
- [x] Add `GET /v1/ai/usage`.
  - [x] Paginated list of user-visible AI usage events.
  - [x] Hide raw prompts and raw provider responses.
- [x] Add `GET /v1/ai/credits`.
  - [x] Remaining credits.
  - [x] Grants and expiries.
  - [x] Daily limit state.

## Phase 7 - Feature Gates

- [x] Create a backend `EntitlementService`.
- [x] Define feature codes:
  - [x] `ai_text_capture`
  - [x] `ai_voice_capture`
  - [x] `advanced_insights`
  - [x] `weekly_review`
  - [x] `budgets`
  - [x] `subscription_reminders`
  - [x] `split_ledger`
  - [x] `web_dashboard`
  - [x] `exports`
  - [x] `bulk_edit`
  - [x] `future_ai_advisor`
- [x] Keep these free:
  - [x] Manual transaction CRUD.
  - [x] Basic account CRUD.
  - [x] Basic dashboard.
  - [x] Basic transaction search/filter.
  - [x] Security settings.
  - [x] User data deletion.
- [x] Gate these for paid or credit-backed access:
  - [x] Voice AI beyond trial.
  - [x] High-volume text AI.
  - [x] Advanced insights.
  - [x] Weekly/monthly reviews.
    - Entitlement is defined now; endpoint enforcement should be added with
      the first dedicated review endpoint.
  - [x] Budgets with alerts.
  - [x] Subscription reminders.
  - [x] Full split ledger.
    - Direct `/v1/split/*` routes and inline entry split creation are gated.
  - [x] Web dashboard.
    - Entitlement is defined now. The existing `/v1/dashboard` endpoint remains
      the free basic dashboard API.
  - [x] Exports and reports.
    - Entitlement is defined now; endpoint enforcement should be added with
      export/report endpoints.
  - [x] Bulk categorization/editing.
    - Entitlement is defined now; endpoint enforcement should be added with
      bulk-edit endpoints.
- [x] Make gated endpoints return `payment_required` or `feature_locked` with
  the required plan/entitlement.
- [x] Add tests for each gated endpoint.

## Phase 8 - Frontend UX

- [x] Add credit status to mobile home screen.
  - [x] Show remaining trial/plan credits.
  - [x] Show daily remaining credits.
  - [x] Show reset/expiry date.
- [x] Show estimated credits before AI parse.
  - [x] Text parse: "Uses 5 credits".
  - [x] Voice parse: show estimate after recording.
- [x] Add insufficient-credit state in the AI capture card.
- [x] Add daily-limit-reached state.
- [x] Add upgrade screen.
  - [x] Explain free trial.
  - [x] Compare monthly, quarterly, yearly.
  - [x] Show weekly only if payment economics are acceptable.
    - Weekly appears through the backend public plan catalog; backend can hide
      it by setting `is_public=false` or removing it from fallback defaults.
  - [x] Show lifetime as "available after 3 paid months".
- [x] Require login before checkout.
- [x] For guest users, show "Create account to keep data and subscribe".
- [x] Add billing status screen in profile/settings.
- [x] Add AI usage history screen.
- [x] Add renewal/cancel status copy.
- [x] Keep manual entry available when credits run out.

## Phase 9 - Observability And Cost Control

- [x] Log one structured backend event per AI usage event.
  - Logs `ai_usage_event` for finalized, refunded, cancelled, and
    failed-before-provider reservations without raw prompts, transcripts, audio,
    or provider responses.
- [x] Add daily cost dashboard metrics:
  - [x] AI cost by model.
  - [x] AI cost by action.
  - [x] AI cost by plan.
  - [x] Cost per active user.
  - [x] Cost per paid user.
  - [x] Parse success rate.
  - [x] Average credits charged per parse.
  - [x] Users hitting daily cap.
  - [x] Users hitting total credit cap.
  - Exposed through `GET /v1/admin/ai/metrics`.
- [x] Add alert when daily OpenAI estimated cost crosses configured threshold.
  - Uses `AI_DAILY_COST_ALERT_USD_MICROS`.
- [x] Add alert when one user/device exceeds abuse threshold.
  - Uses `AI_ABUSE_DAILY_CREDITS_THRESHOLD`.
- [x] Add alert when free-tier cost per activated user exceeds target.
  - Uses `AI_FREE_COST_PER_USER_ALERT_USD_MICROS`.
- [x] Add admin adjustment path for support refunds.
  - Exposed through `POST /v1/admin/credits/adjustments`.
- [x] Add admin view for lifetime quote candidates.
  - Exposed through `GET /v1/admin/billing/lifetime-quotes`.
- [x] Add model pricing config that can be updated without code deploy.
  - Exposed through `GET` and `PUT /v1/admin/ai/model-pricing`; runtime cost
    estimates prefer matching active pricing rows before the credit fallback.

## Phase 10 - Abuse Prevention

- [x] Keep existing IP rate limiting, but do not rely on it for cost control.
  - Route rate limiting remains active, while credit caps and abuse controls
    perform the real AI cost control.
- [x] Add user/device daily credit caps.
  - Enforced through `daily_credit_usage` before provider calls.
- [x] Add free-trial one-time grants.
  - User and guest free-trial grants are unique by user/device scope.
- [x] Hash device IDs and IPs before storing abuse keys.
  - Guest credit/usage keys store SHA-256 hashes, not raw identifiers.
- [x] Add max transcript characters per parse.
  - Uses `MAX_TRANSCRIPT_CHARS`.
- [x] Add stricter max voice duration for guests/free users.
  - Uses `AI_UNPAID_MAX_VOICE_BYTES`; unpaid users are limited to short voice
    capture unless they have an active paid plan.
- [x] Add cooldown after repeated failed parse attempts.
  - Uses `AI_FAILED_PARSE_COOLDOWN_THRESHOLD`,
    `AI_FAILED_PARSE_COOLDOWN_WINDOW_MINUTES`, and
    `AI_FAILED_PARSE_COOLDOWN_MINUTES`.
- [x] Add duplicate request idempotency.
  - Parse credit reservation idempotency uses `Idempotency-Key` per user/device.
- [x] Block guest AI if device fingerprint is missing or clearly invalid.
  - Guest AI requires a non-trivial device fingerprint before credit subject
    creation.
- [x] Add manual admin block for abusive users/devices.
  - Exposed through `GET`, `POST`, and `PATCH /v1/admin/ai/abuse-blocks`.
- [x] Add provider timeout and circuit breaker behavior.
  - Provider HTTP timeout follows `REQUEST_TIMEOUT_SECONDS`; repeated provider
    failures open an in-memory circuit using
    `AI_PROVIDER_FAILURE_THRESHOLD` and
    `AI_PROVIDER_CIRCUIT_BREAKER_SECONDS`.
- [x] Add global kill switch for AI parse if costs spike.
  - Uses `AI_PARSE_DISABLED`.

## Phase 11 - Privacy And Retention

- [x] Do not store raw voice audio.
- [x] Do not store raw provider prompts or raw provider responses by default.
- [x] Store usage metadata only:
  - Action.
  - Model.
  - Token counts.
  - Audio duration/bytes.
  - Cost estimate.
  - Credits charged.
  - Status/error code.
- [x] Keep confirmed transaction `source_text` only because the user saved it.
- [x] Ensure `DELETE /v1/user` deletes:
  - Subscriptions.
  - Credit grants.
  - Credit ledger.
  - AI usage events.
  - Guest usage keys linked to the user where possible.
  - Daily AI usage counters, AI usage limit events, AI abuse blocks, user
    subscription mirror rows, and lifetime quote requests.
- [x] Decide retention for anonymous guest usage events.
  - V1 policy: purge anonymous guest AI usage, AI limit events, daily usage,
    guest usage keys, expired guest grants, and linked credit ledger rows after
    90 days. Registered-user rows are retained until account deletion.
  - Scheduled backend maintenance runs credit expiry and anonymous guest
    retention daily by default.

## Phase 12 - Testing Checklist

- [x] Unit test credit math.
- [ ] Unit test plan entitlement resolution.
- [x] Unit test guest/free/paid allowance selection.
- [ ] Unit test credit grant expiry.
- [ ] Unit test idempotency.
- [ ] Unit test refunds for pre-provider failures.
- [ ] Unit test charges for post-provider failures.
- [ ] Integration test text parse debit.
- [ ] Integration test voice parse debit.
- [ ] Integration test insufficient credits.
- [ ] Integration test daily limit reached.
- [ ] Integration test subscription renewal grants.
- [ ] Integration test subscription cancellation.
- [ ] Integration test lifetime quote eligibility.
- [ ] Race test parallel parse requests against one remaining credit balance.
- [ ] Manual QA: guest trial.
- [ ] Manual QA: logged-in free trial.
- [ ] Manual QA: upgrade flow.
- [ ] Manual QA: paid renewal.
- [ ] Manual QA: expired credits.
- [ ] Manual QA: manual entry still works with zero credits.

## Phase 13 - Rollout Plan

- [ ] Ship usage accounting in shadow mode first.
  - Record estimated usage.
  - Do not block users.
  - Compare estimated costs with OpenAI billing.
- [ ] Run shadow mode for at least 7 days or 1000 parse attempts.
- [ ] Tune credit prices from real usage.
- [ ] Enable credit deduction for internal/test users.
- [ ] Enable credit deduction for new guest users.
- [ ] Enable credit deduction for all free users.
- [ ] Launch paid plans.
- [ ] Add in-app messaging before enforcement:
  - Trial credits available.
  - Daily cap.
  - Upgrade options.
- [ ] Review free-tier economics weekly for the first month.

## Phase 14 - Documentation Updates

- [ ] Update `DATA_MODEL.md` with billing, credit, and usage entities.
- [ ] Update `API_SPEC.md` with billing and AI credit endpoints.
- [ ] Update `AI_PARSING.md` with credit reservation/finalization behavior.
- [ ] Update `MVP_SCOPE.md` with free vs paid feature scope.
- [ ] Update `ROADMAP.md` with freemium rollout phases.
- [ ] Update mobile copy for free trial and credits.
- [ ] Update web landing page to avoid unlimited/free AI claims.

## Definition Of Done

- [ ] Every AI provider call creates exactly one `ai_usage_events` record.
- [ ] Every credit movement creates a `credit_ledger` record.
- [ ] Credits cannot be double-spent under parallel requests.
- [ ] Free users cannot exceed daily or total allowance.
- [ ] Guest users cannot repeatedly reset trial credits through normal app flows.
- [ ] Paid users receive credits only after verified payment events.
- [ ] Lifetime cannot be purchased directly.
- [ ] Manual tracking remains usable with zero credits.
- [ ] The app can show users where credits went.
- [ ] Backend metrics can show actual AI cost by action, user type, and plan.
