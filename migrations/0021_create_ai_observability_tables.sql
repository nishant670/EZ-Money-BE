CREATE TABLE IF NOT EXISTS ai_usage_limit_events (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    guest_device_id_hash CHAR(64) NOT NULL DEFAULT '',
    action_code VARCHAR(80) NOT NULL,
    reason VARCHAR(64) NOT NULL,
    required_credits INTEGER NOT NULL DEFAULT 0,
    available_credits INTEGER NOT NULL DEFAULT 0,
    daily_limit INTEGER NOT NULL DEFAULT 0,
    used_today INTEGER NOT NULL DEFAULT 0,
    daily_remaining INTEGER NOT NULL DEFAULT 0,
    plan_code VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_usage_limit_events_user_created
    ON ai_usage_limit_events (user_id, created_at DESC)
    WHERE user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ai_usage_limit_events_guest_created
    ON ai_usage_limit_events (guest_device_id_hash, created_at DESC)
    WHERE guest_device_id_hash <> '';

CREATE INDEX IF NOT EXISTS idx_ai_usage_limit_events_reason_created
    ON ai_usage_limit_events (reason, created_at DESC);

ALTER TABLE ai_usage_limit_events
    DROP CONSTRAINT IF EXISTS ai_usage_limit_events_identity_check;
ALTER TABLE ai_usage_limit_events
    ADD CONSTRAINT ai_usage_limit_events_identity_check
    CHECK (user_id IS NOT NULL OR guest_device_id_hash <> '');

ALTER TABLE ai_usage_limit_events
    DROP CONSTRAINT IF EXISTS ai_usage_limit_events_reason_check;
ALTER TABLE ai_usage_limit_events
    ADD CONSTRAINT ai_usage_limit_events_reason_check
    CHECK (reason IN ('insufficient_ai_credits', 'daily_ai_limit_reached', 'feature_locked', 'guest_not_allowed', 'subject_required'));

ALTER TABLE ai_usage_limit_events
    DROP CONSTRAINT IF EXISTS ai_usage_limit_events_non_negative_check;
ALTER TABLE ai_usage_limit_events
    ADD CONSTRAINT ai_usage_limit_events_non_negative_check
    CHECK (
        required_credits >= 0
        AND available_credits >= 0
        AND daily_limit >= 0
        AND used_today >= 0
        AND daily_remaining >= 0
    );

CREATE TABLE IF NOT EXISTS ai_model_pricings (
    id BIGSERIAL PRIMARY KEY,
    provider VARCHAR(40) NOT NULL,
    model VARCHAR(80) NOT NULL,
    operation VARCHAR(32) NOT NULL,
    input_token_usd_micros BIGINT NOT NULL DEFAULT 0,
    output_token_usd_micros BIGINT NOT NULL DEFAULT 0,
    audio_minute_usd_micros BIGINT NOT NULL DEFAULT 0,
    request_usd_micros BIGINT NOT NULL DEFAULT 0,
    credit_usd_micros BIGINT NOT NULL DEFAULT 100,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_model_pricings_provider_model_operation
    ON ai_model_pricings (provider, model, operation);

CREATE INDEX IF NOT EXISTS idx_ai_model_pricings_active
    ON ai_model_pricings (active, provider, model);

ALTER TABLE ai_model_pricings
    DROP CONSTRAINT IF EXISTS ai_model_pricings_operation_check;
ALTER TABLE ai_model_pricings
    ADD CONSTRAINT ai_model_pricings_operation_check
    CHECK (operation IN ('llm', 'transcription', 'credit_fallback'));

ALTER TABLE ai_model_pricings
    DROP CONSTRAINT IF EXISTS ai_model_pricings_non_negative_check;
ALTER TABLE ai_model_pricings
    ADD CONSTRAINT ai_model_pricings_non_negative_check
    CHECK (
        input_token_usd_micros >= 0
        AND output_token_usd_micros >= 0
        AND audio_minute_usd_micros >= 0
        AND request_usd_micros >= 0
        AND credit_usd_micros >= 0
    );
