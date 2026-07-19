CREATE UNIQUE INDEX IF NOT EXISTS idx_credit_grants_once_user_free_trial
    ON credit_grants (user_id, source)
    WHERE user_id IS NOT NULL AND source = 'free_trial';

CREATE UNIQUE INDEX IF NOT EXISTS idx_credit_grants_once_guest_free_trial
    ON credit_grants (guest_device_id_hash, source)
    WHERE guest_device_id_hash <> '' AND source = 'free_trial';

CREATE UNIQUE INDEX IF NOT EXISTS idx_credit_grants_subscription_period
    ON credit_grants (subscription_id, valid_from, source)
    WHERE subscription_id IS NOT NULL AND source = 'subscription_period';
