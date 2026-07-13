CREATE TABLE IF NOT EXISTS subscriptions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id BIGINT,
    name VARCHAR(120) NOT NULL,
    merchant VARCHAR(120) NOT NULL DEFAULT '',
    category VARCHAR(80) NOT NULL DEFAULT '',
    amount NUMERIC(19,2) NOT NULL,
    currency CHAR(3) NOT NULL DEFAULT 'INR',
    billing_interval VARCHAR(16) NOT NULL DEFAULT 'monthly',
    next_due_date DATE NOT NULL,
    last_charged_date VARCHAR(10) NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    reminder_days INTEGER NOT NULL DEFAULT 3,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_user_status_due
    ON subscriptions (user_id, status, next_due_date);

CREATE INDEX IF NOT EXISTS idx_subscriptions_user_merchant
    ON subscriptions (user_id, merchant);

CREATE INDEX IF NOT EXISTS idx_subscriptions_user_category
    ON subscriptions (user_id, category);

ALTER TABLE subscriptions
    DROP CONSTRAINT IF EXISTS subscriptions_amount_positive_check;
ALTER TABLE subscriptions
    ADD CONSTRAINT subscriptions_amount_positive_check CHECK (amount > 0);

ALTER TABLE subscriptions
    DROP CONSTRAINT IF EXISTS subscriptions_currency_check;
ALTER TABLE subscriptions
    ADD CONSTRAINT subscriptions_currency_check CHECK (currency = 'INR');

ALTER TABLE subscriptions
    DROP CONSTRAINT IF EXISTS subscriptions_billing_interval_check;
ALTER TABLE subscriptions
    ADD CONSTRAINT subscriptions_billing_interval_check
    CHECK (billing_interval IN ('weekly', 'monthly', 'yearly'));

ALTER TABLE subscriptions
    DROP CONSTRAINT IF EXISTS subscriptions_status_check;
ALTER TABLE subscriptions
    ADD CONSTRAINT subscriptions_status_check
    CHECK (status IN ('active', 'paused', 'cancelled'));

ALTER TABLE subscriptions
    DROP CONSTRAINT IF EXISTS subscriptions_reminder_days_check;
ALTER TABLE subscriptions
    ADD CONSTRAINT subscriptions_reminder_days_check
    CHECK (reminder_days BETWEEN 0 AND 30);

ALTER TABLE subscriptions
    DROP CONSTRAINT IF EXISTS fk_subscriptions_owned_account;
ALTER TABLE subscriptions
    ADD CONSTRAINT fk_subscriptions_owned_account
    FOREIGN KEY (user_id, account_id) REFERENCES accounts(user_id, id);

CREATE TABLE IF NOT EXISTS subscription_reminders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subscription_id BIGINT NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    due_date DATE NOT NULL,
    kind VARCHAR(24) NOT NULL,
    notification_id BIGINT REFERENCES notifications(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_reminder_once_due
    ON subscription_reminders (user_id, subscription_id, due_date, kind);

CREATE INDEX IF NOT EXISTS idx_subscription_reminders_user_subscription
    ON subscription_reminders (user_id, subscription_id);

ALTER TABLE subscription_reminders
    DROP CONSTRAINT IF EXISTS subscription_reminders_kind_check;
ALTER TABLE subscription_reminders
    ADD CONSTRAINT subscription_reminders_kind_check CHECK (kind IN ('due', 'overdue'));
