CREATE TABLE IF NOT EXISTS budgets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(120) NOT NULL,
    period VARCHAR(16) NOT NULL DEFAULT 'monthly',
    category VARCHAR(80) NOT NULL DEFAULT '',
    limit_amount NUMERIC(19,2) NOT NULL,
    currency CHAR(3) NOT NULL DEFAULT 'INR',
    alert_threshold_percent INTEGER NOT NULL DEFAULT 80,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_budgets_user_active
    ON budgets (user_id, active, period);

CREATE INDEX IF NOT EXISTS idx_budgets_user_category
    ON budgets (user_id, category);

ALTER TABLE budgets
    DROP CONSTRAINT IF EXISTS budgets_period_check;
ALTER TABLE budgets
    ADD CONSTRAINT budgets_period_check CHECK (period = 'monthly');

ALTER TABLE budgets
    DROP CONSTRAINT IF EXISTS budgets_limit_amount_positive_check;
ALTER TABLE budgets
    ADD CONSTRAINT budgets_limit_amount_positive_check CHECK (limit_amount > 0);

ALTER TABLE budgets
    DROP CONSTRAINT IF EXISTS budgets_currency_check;
ALTER TABLE budgets
    ADD CONSTRAINT budgets_currency_check CHECK (currency = 'INR');

ALTER TABLE budgets
    DROP CONSTRAINT IF EXISTS budgets_alert_threshold_percent_check;
ALTER TABLE budgets
    ADD CONSTRAINT budgets_alert_threshold_percent_check
    CHECK (alert_threshold_percent BETWEEN 1 AND 100);

CREATE TABLE IF NOT EXISTS budget_alerts (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    budget_id BIGINT NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    period_start DATE NOT NULL,
    kind VARCHAR(24) NOT NULL,
    spend_amount NUMERIC(19,2) NOT NULL,
    limit_amount NUMERIC(19,2) NOT NULL,
    notification_id BIGINT REFERENCES notifications(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_budget_alert_once_period
    ON budget_alerts (user_id, budget_id, period_start, kind);

CREATE INDEX IF NOT EXISTS idx_budget_alerts_user_budget
    ON budget_alerts (user_id, budget_id);

ALTER TABLE budget_alerts
    DROP CONSTRAINT IF EXISTS budget_alerts_kind_check;
ALTER TABLE budget_alerts
    ADD CONSTRAINT budget_alerts_kind_check CHECK (kind IN ('warning', 'exceeded'));

ALTER TABLE budget_alerts
    DROP CONSTRAINT IF EXISTS budget_alerts_spend_amount_non_negative_check;
ALTER TABLE budget_alerts
    ADD CONSTRAINT budget_alerts_spend_amount_non_negative_check CHECK (spend_amount >= 0);

ALTER TABLE budget_alerts
    DROP CONSTRAINT IF EXISTS budget_alerts_limit_amount_positive_check;
ALTER TABLE budget_alerts
    ADD CONSTRAINT budget_alerts_limit_amount_positive_check CHECK (limit_amount > 0);
