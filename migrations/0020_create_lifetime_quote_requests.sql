CREATE TABLE IF NOT EXISTS lifetime_quote_requests (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(24) NOT NULL DEFAULT 'requested',
    paid_months_completed INTEGER NOT NULL DEFAULT 0,
    usage_window_start TIMESTAMPTZ NOT NULL,
    usage_window_end TIMESTAMPTZ NOT NULL,
    usage_event_count INTEGER NOT NULL DEFAULT 0,
    credits_used INTEGER NOT NULL DEFAULT 0,
    average_monthly_credits INTEGER NOT NULL DEFAULT 0,
    estimated_cost_usd_micros BIGINT NOT NULL DEFAULT 0,
    average_monthly_cost_usd_micros BIGINT NOT NULL DEFAULT 0,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_lifetime_quote_requests_user_created
    ON lifetime_quote_requests (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_lifetime_quote_requests_status_created
    ON lifetime_quote_requests (status, created_at DESC);

ALTER TABLE lifetime_quote_requests
    DROP CONSTRAINT IF EXISTS lifetime_quote_requests_status_check;
ALTER TABLE lifetime_quote_requests
    ADD CONSTRAINT lifetime_quote_requests_status_check
    CHECK (status IN ('requested', 'reviewed', 'quoted', 'declined', 'cancelled'));

ALTER TABLE lifetime_quote_requests
    DROP CONSTRAINT IF EXISTS lifetime_quote_requests_non_negative_check;
ALTER TABLE lifetime_quote_requests
    ADD CONSTRAINT lifetime_quote_requests_non_negative_check
    CHECK (
        paid_months_completed >= 0
        AND usage_event_count >= 0
        AND credits_used >= 0
        AND average_monthly_credits >= 0
        AND estimated_cost_usd_micros >= 0
        AND average_monthly_cost_usd_micros >= 0
    );

ALTER TABLE lifetime_quote_requests
    DROP CONSTRAINT IF EXISTS lifetime_quote_requests_window_check;
ALTER TABLE lifetime_quote_requests
    ADD CONSTRAINT lifetime_quote_requests_window_check
    CHECK (usage_window_end > usage_window_start);
