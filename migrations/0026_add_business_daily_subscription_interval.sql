ALTER TABLE subscriptions
    DROP CONSTRAINT IF EXISTS subscriptions_billing_interval_check;

ALTER TABLE subscriptions
    ADD CONSTRAINT subscriptions_billing_interval_check
    CHECK (billing_interval IN (
        'daily',
        'business_daily',
        'weekly',
        'biweekly',
        'monthly',
        'quarterly',
        'yearly'
    ));
