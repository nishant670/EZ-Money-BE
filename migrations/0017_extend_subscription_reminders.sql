ALTER TABLE subscriptions
    ADD COLUMN IF NOT EXISTS cancel_before_due BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE subscriptions
    DROP CONSTRAINT IF EXISTS subscriptions_billing_interval_check;
ALTER TABLE subscriptions
    ADD CONSTRAINT subscriptions_billing_interval_check
    CHECK (billing_interval IN ('daily', 'weekly', 'biweekly', 'monthly', 'quarterly', 'yearly'));
