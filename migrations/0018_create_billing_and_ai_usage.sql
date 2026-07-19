CREATE TABLE IF NOT EXISTS plans (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(120) NOT NULL,
    billing_interval VARCHAR(24) NOT NULL,
    price_minor BIGINT NOT NULL DEFAULT 0,
    currency CHAR(3) NOT NULL DEFAULT 'INR',
    included_credits INTEGER NOT NULL DEFAULT 0,
    daily_credit_limit INTEGER NOT NULL DEFAULT 0,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    requires_login BOOLEAN NOT NULL DEFAULT TRUE,
    requires_prior_paid_months INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE plans
    DROP CONSTRAINT IF EXISTS plans_billing_interval_check;
ALTER TABLE plans
    ADD CONSTRAINT plans_billing_interval_check
    CHECK (billing_interval IN ('weekly', 'monthly', 'quarterly', 'yearly', 'lifetime_quote'));

ALTER TABLE plans
    DROP CONSTRAINT IF EXISTS plans_price_minor_non_negative_check;
ALTER TABLE plans
    ADD CONSTRAINT plans_price_minor_non_negative_check CHECK (price_minor >= 0);

ALTER TABLE plans
    DROP CONSTRAINT IF EXISTS plans_currency_check;
ALTER TABLE plans
    ADD CONSTRAINT plans_currency_check CHECK (currency = 'INR');

ALTER TABLE plans
    DROP CONSTRAINT IF EXISTS plans_credit_limits_non_negative_check;
ALTER TABLE plans
    ADD CONSTRAINT plans_credit_limits_non_negative_check
    CHECK (included_credits >= 0 AND daily_credit_limit >= 0 AND requires_prior_paid_months >= 0);

CREATE TABLE IF NOT EXISTS user_subscriptions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id BIGINT NOT NULL REFERENCES plans(id),
    status VARCHAR(24) NOT NULL,
    current_period_start TIMESTAMPTZ NOT NULL,
    current_period_end TIMESTAMPTZ NOT NULL,
    provider VARCHAR(40) NOT NULL DEFAULT '',
    provider_customer_id VARCHAR(120) NOT NULL DEFAULT '',
    provider_subscription_id VARCHAR(120) NOT NULL DEFAULT '',
    cancel_at_period_end BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_subscriptions_user_status_period
    ON user_subscriptions (user_id, status, current_period_end DESC);

CREATE INDEX IF NOT EXISTS idx_user_subscriptions_plan
    ON user_subscriptions (plan_id);

CREATE INDEX IF NOT EXISTS idx_user_subscriptions_provider_customer
    ON user_subscriptions (provider, provider_customer_id)
    WHERE provider_customer_id <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_subscriptions_provider_subscription
    ON user_subscriptions (provider, provider_subscription_id)
    WHERE provider_subscription_id <> '';

ALTER TABLE user_subscriptions
    DROP CONSTRAINT IF EXISTS user_subscriptions_status_check;
ALTER TABLE user_subscriptions
    ADD CONSTRAINT user_subscriptions_status_check
    CHECK (status IN ('trialing', 'active', 'past_due', 'cancelled', 'expired'));

ALTER TABLE user_subscriptions
    DROP CONSTRAINT IF EXISTS user_subscriptions_period_check;
ALTER TABLE user_subscriptions
    ADD CONSTRAINT user_subscriptions_period_check CHECK (current_period_end > current_period_start);

CREATE TABLE IF NOT EXISTS credit_grants (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    guest_device_id_hash CHAR(64) NOT NULL DEFAULT '',
    source VARCHAR(40) NOT NULL,
    credits_granted INTEGER NOT NULL,
    credits_remaining INTEGER NOT NULL,
    valid_from TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ,
    subscription_id BIGINT REFERENCES user_subscriptions(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_credit_grants_user_available
    ON credit_grants (user_id, expires_at, valid_from)
    WHERE user_id IS NOT NULL AND credits_remaining > 0;

CREATE INDEX IF NOT EXISTS idx_credit_grants_guest_available
    ON credit_grants (guest_device_id_hash, expires_at, valid_from)
    WHERE guest_device_id_hash <> '' AND credits_remaining > 0;

CREATE INDEX IF NOT EXISTS idx_credit_grants_subscription
    ON credit_grants (subscription_id);

ALTER TABLE credit_grants
    DROP CONSTRAINT IF EXISTS credit_grants_identity_check;
ALTER TABLE credit_grants
    ADD CONSTRAINT credit_grants_identity_check
    CHECK (user_id IS NOT NULL OR guest_device_id_hash <> '');

ALTER TABLE credit_grants
    DROP CONSTRAINT IF EXISTS credit_grants_source_check;
ALTER TABLE credit_grants
    ADD CONSTRAINT credit_grants_source_check
    CHECK (source IN ('free_trial', 'subscription_period', 'manual_adjustment', 'refund', 'promo', 'lifetime_quote'));

ALTER TABLE credit_grants
    DROP CONSTRAINT IF EXISTS credit_grants_amount_check;
ALTER TABLE credit_grants
    ADD CONSTRAINT credit_grants_amount_check
    CHECK (credits_granted > 0 AND credits_remaining >= 0 AND credits_remaining <= credits_granted);

ALTER TABLE credit_grants
    DROP CONSTRAINT IF EXISTS credit_grants_expiry_check;
ALTER TABLE credit_grants
    ADD CONSTRAINT credit_grants_expiry_check
    CHECK (expires_at IS NULL OR expires_at > valid_from);

CREATE TABLE IF NOT EXISTS ai_usage_events (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    guest_device_id_hash CHAR(64) NOT NULL DEFAULT '',
    session_id VARCHAR(120) NOT NULL DEFAULT '',
    request_id VARCHAR(120) NOT NULL UNIQUE,
    idempotency_key VARCHAR(120) NOT NULL DEFAULT '',
    action_code VARCHAR(80) NOT NULL,
    input_kind VARCHAR(20) NOT NULL,
    status VARCHAR(32) NOT NULL,
    provider VARCHAR(40) NOT NULL DEFAULT '',
    model VARCHAR(80) NOT NULL DEFAULT '',
    secondary_provider VARCHAR(40) NOT NULL DEFAULT '',
    secondary_model VARCHAR(80) NOT NULL DEFAULT '',
    estimated_credits INTEGER NOT NULL,
    reserved_credits INTEGER NOT NULL DEFAULT 0,
    final_credits INTEGER NOT NULL DEFAULT 0,
    estimated_cost_usd_micros BIGINT NOT NULL DEFAULT 0,
    actual_cost_usd_micros BIGINT,
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    total_tokens INTEGER,
    audio_duration_ms INTEGER,
    audio_bytes BIGINT,
    input_chars INTEGER,
    response_bytes INTEGER,
    error_code VARCHAR(80) NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL,
    provider_started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_usage_events_user_started
    ON ai_usage_events (user_id, started_at DESC)
    WHERE user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ai_usage_events_guest_started
    ON ai_usage_events (guest_device_id_hash, started_at DESC)
    WHERE guest_device_id_hash <> '';

CREATE INDEX IF NOT EXISTS idx_ai_usage_events_action_status
    ON ai_usage_events (action_code, status, started_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_usage_events_user_idempotency
    ON ai_usage_events (user_id, idempotency_key)
    WHERE user_id IS NOT NULL AND idempotency_key <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_usage_events_guest_idempotency
    ON ai_usage_events (guest_device_id_hash, idempotency_key)
    WHERE guest_device_id_hash <> '' AND idempotency_key <> '';

ALTER TABLE ai_usage_events
    DROP CONSTRAINT IF EXISTS ai_usage_events_identity_check;
ALTER TABLE ai_usage_events
    ADD CONSTRAINT ai_usage_events_identity_check
    CHECK (user_id IS NOT NULL OR guest_device_id_hash <> '');

ALTER TABLE ai_usage_events
    DROP CONSTRAINT IF EXISTS ai_usage_events_input_kind_check;
ALTER TABLE ai_usage_events
    ADD CONSTRAINT ai_usage_events_input_kind_check
    CHECK (input_kind IN ('text', 'voice', 'image', 'file', 'chat'));

ALTER TABLE ai_usage_events
    DROP CONSTRAINT IF EXISTS ai_usage_events_status_check;
ALTER TABLE ai_usage_events
    ADD CONSTRAINT ai_usage_events_status_check
    CHECK (status IN ('reserved', 'succeeded', 'failed_before_provider', 'failed_after_provider', 'refunded', 'cancelled'));

ALTER TABLE ai_usage_events
    DROP CONSTRAINT IF EXISTS ai_usage_events_credit_non_negative_check;
ALTER TABLE ai_usage_events
    ADD CONSTRAINT ai_usage_events_credit_non_negative_check
    CHECK (estimated_credits >= 0 AND reserved_credits >= 0 AND final_credits >= 0);

ALTER TABLE ai_usage_events
    DROP CONSTRAINT IF EXISTS ai_usage_events_cost_non_negative_check;
ALTER TABLE ai_usage_events
    ADD CONSTRAINT ai_usage_events_cost_non_negative_check
    CHECK (estimated_cost_usd_micros >= 0 AND (actual_cost_usd_micros IS NULL OR actual_cost_usd_micros >= 0));

ALTER TABLE ai_usage_events
    DROP CONSTRAINT IF EXISTS ai_usage_events_usage_non_negative_check;
ALTER TABLE ai_usage_events
    ADD CONSTRAINT ai_usage_events_usage_non_negative_check
    CHECK (
        (prompt_tokens IS NULL OR prompt_tokens >= 0)
        AND (completion_tokens IS NULL OR completion_tokens >= 0)
        AND (total_tokens IS NULL OR total_tokens >= 0)
        AND (audio_duration_ms IS NULL OR audio_duration_ms >= 0)
        AND (audio_bytes IS NULL OR audio_bytes >= 0)
        AND (input_chars IS NULL OR input_chars >= 0)
        AND (response_bytes IS NULL OR response_bytes >= 0)
    );

CREATE TABLE IF NOT EXISTS credit_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    guest_device_id_hash CHAR(64) NOT NULL DEFAULT '',
    grant_id BIGINT REFERENCES credit_grants(id) ON DELETE SET NULL,
    direction VARCHAR(20) NOT NULL,
    credits INTEGER NOT NULL,
    balance_after INTEGER NOT NULL,
    reason_code VARCHAR(64) NOT NULL,
    idempotency_key VARCHAR(120) NOT NULL DEFAULT '',
    ai_usage_event_id BIGINT REFERENCES ai_usage_events(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_credit_ledger_user_created
    ON credit_ledger (user_id, created_at DESC)
    WHERE user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_credit_ledger_guest_created
    ON credit_ledger (guest_device_id_hash, created_at DESC)
    WHERE guest_device_id_hash <> '';

CREATE INDEX IF NOT EXISTS idx_credit_ledger_grant
    ON credit_ledger (grant_id);

CREATE INDEX IF NOT EXISTS idx_credit_ledger_ai_usage_event
    ON credit_ledger (ai_usage_event_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_credit_ledger_grant_idempotency
    ON credit_ledger (grant_id, idempotency_key, direction)
    WHERE grant_id IS NOT NULL AND idempotency_key <> '';

ALTER TABLE credit_ledger
    DROP CONSTRAINT IF EXISTS credit_ledger_identity_check;
ALTER TABLE credit_ledger
    ADD CONSTRAINT credit_ledger_identity_check
    CHECK (user_id IS NOT NULL OR guest_device_id_hash <> '');

ALTER TABLE credit_ledger
    DROP CONSTRAINT IF EXISTS credit_ledger_direction_check;
ALTER TABLE credit_ledger
    ADD CONSTRAINT credit_ledger_direction_check
    CHECK (direction IN ('grant', 'debit', 'refund', 'adjustment', 'expiry'));

ALTER TABLE credit_ledger
    DROP CONSTRAINT IF EXISTS credit_ledger_amount_check;
ALTER TABLE credit_ledger
    ADD CONSTRAINT credit_ledger_amount_check
    CHECK (credits > 0 AND balance_after >= 0);

CREATE TABLE IF NOT EXISTS daily_credit_usage (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    guest_device_id_hash CHAR(64) NOT NULL DEFAULT '',
    usage_date DATE NOT NULL,
    credits_used INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_daily_credit_usage_user_date
    ON daily_credit_usage (user_id, usage_date)
    WHERE user_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_daily_credit_usage_guest_date
    ON daily_credit_usage (guest_device_id_hash, usage_date)
    WHERE guest_device_id_hash <> '';

ALTER TABLE daily_credit_usage
    DROP CONSTRAINT IF EXISTS daily_credit_usage_identity_check;
ALTER TABLE daily_credit_usage
    ADD CONSTRAINT daily_credit_usage_identity_check
    CHECK (user_id IS NOT NULL OR guest_device_id_hash <> '');

ALTER TABLE daily_credit_usage
    DROP CONSTRAINT IF EXISTS daily_credit_usage_amount_check;
ALTER TABLE daily_credit_usage
    ADD CONSTRAINT daily_credit_usage_amount_check CHECK (credits_used >= 0);

CREATE TABLE IF NOT EXISTS guest_usage_keys (
    id BIGSERIAL PRIMARY KEY,
    guest_device_id_hash CHAR(64) NOT NULL UNIQUE,
    ip_hash CHAR(64) NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    trial_grant_id BIGINT REFERENCES credit_grants(id) ON DELETE SET NULL,
    abuse_score INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_guest_usage_keys_ip_hash
    ON guest_usage_keys (ip_hash);

CREATE INDEX IF NOT EXISTS idx_guest_usage_keys_last_seen
    ON guest_usage_keys (last_seen_at DESC);

ALTER TABLE guest_usage_keys
    DROP CONSTRAINT IF EXISTS guest_usage_keys_abuse_score_check;
ALTER TABLE guest_usage_keys
    ADD CONSTRAINT guest_usage_keys_abuse_score_check CHECK (abuse_score >= 0);

ALTER TABLE guest_usage_keys
    DROP CONSTRAINT IF EXISTS guest_usage_keys_seen_order_check;
ALTER TABLE guest_usage_keys
    ADD CONSTRAINT guest_usage_keys_seen_order_check CHECK (last_seen_at >= first_seen_at);
