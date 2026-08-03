ALTER TABLE subscriptions
    ADD COLUMN IF NOT EXISTS cancel_on_date VARCHAR(10) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS autopay BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS payment_mode VARCHAR(24) NOT NULL DEFAULT 'Cash',
    ADD COLUMN IF NOT EXISTS transaction_tag VARCHAR(40) NOT NULL DEFAULT 'Subscription',
    ADD COLUMN IF NOT EXISTS purpose_type VARCHAR(40) NOT NULL DEFAULT 'normal_spend';

ALTER TABLE subscription_reminders
    DROP CONSTRAINT IF EXISTS subscription_reminders_kind_check;
ALTER TABLE subscription_reminders
    ADD CONSTRAINT subscription_reminders_kind_check
    CHECK (kind IN ('due', 'overdue', 'cancel'));

CREATE INDEX IF NOT EXISTS idx_subscriptions_autopay_due
    ON subscriptions (status, autopay, next_due_date);

CREATE TABLE IF NOT EXISTS subscription_occurrences (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subscription_id BIGINT NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    entry_id BIGINT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    due_date DATE NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    confirmed_at TIMESTAMPTZ,
    reverted_at TIMESTAMPTZ,
    notification_id BIGINT REFERENCES notifications(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_occurrence_once
    ON subscription_occurrences (user_id, subscription_id, due_date);
CREATE INDEX IF NOT EXISTS idx_subscription_occurrences_user_status
    ON subscription_occurrences (user_id, status, created_at DESC);

ALTER TABLE subscription_occurrences
    DROP CONSTRAINT IF EXISTS subscription_occurrences_status_check;
ALTER TABLE subscription_occurrences
    ADD CONSTRAINT subscription_occurrences_status_check
    CHECK (status IN ('pending', 'confirmed', 'reverted'));

CREATE TABLE IF NOT EXISTS push_devices (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(255) NOT NULL UNIQUE,
    platform VARCHAR(16) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_push_devices_user_active
    ON push_devices (user_id, active);
