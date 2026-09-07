-- Payments and the webhook ledger.
--
-- A signed, deduplicated webhook is the only thing permitted to move a
-- subscription to active or write a credit grant, so these two tables are the
-- entire trust boundary between Razorpay and the ledger.

CREATE TABLE IF NOT EXISTS payments (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id BIGINT NOT NULL REFERENCES plans(id),
    provider VARCHAR(40) NOT NULL,
    -- The order id is the idempotency key for a whole purchase: two
    -- subscriptions must never be granted against one order.
    provider_order_id VARCHAR(120) NOT NULL UNIQUE,
    provider_payment_id VARCHAR(120) NOT NULL DEFAULT '',
    status VARCHAR(24) NOT NULL,
    amount_minor BIGINT NOT NULL,
    amount_refunded_minor BIGINT NOT NULL DEFAULT 0,
    currency CHAR(3) NOT NULL DEFAULT 'INR',
    method VARCHAR(40) NOT NULL DEFAULT '',
    receipt VARCHAR(64) NOT NULL DEFAULT '',
    failure_reason VARCHAR(255) NOT NULL DEFAULT '',
    subscription_id BIGINT REFERENCES user_subscriptions(id) ON DELETE SET NULL,
    captured_at TIMESTAMPTZ,
    refunded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payments_user_created ON payments (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments (status);
CREATE INDEX IF NOT EXISTS idx_payments_provider_payment
    ON payments (provider, provider_payment_id) WHERE provider_payment_id <> '';

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check
    CHECK (status IN ('created', 'captured', 'failed', 'refunded'));

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_amount_check;
-- A captured payment can be refunded in part, but never for more than was
-- taken - that would be money invented by an arithmetic slip.
ALTER TABLE payments ADD CONSTRAINT payments_amount_check
    CHECK (amount_minor > 0 AND amount_refunded_minor >= 0 AND amount_refunded_minor <= amount_minor);

CREATE TABLE IF NOT EXISTS payment_webhook_events (
    id BIGSERIAL PRIMARY KEY,
    provider VARCHAR(40) NOT NULL,
    event_id VARCHAR(160) NOT NULL,
    event_type VARCHAR(80) NOT NULL DEFAULT '',
    -- Enough to spot a replay reusing an event id under a different body,
    -- without retaining the payload itself.
    payload_hash CHAR(64) NOT NULL DEFAULT '',
    status VARCHAR(24) NOT NULL,
    detail VARCHAR(255) NOT NULL DEFAULT '',
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The replay guard. Inserting this row before the event is handled means a
-- concurrent redelivery loses the insert and returns without granting twice.
CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_webhook_events_identity
    ON payment_webhook_events (provider, event_id);
CREATE INDEX IF NOT EXISTS idx_payment_webhook_events_status_created
    ON payment_webhook_events (status, created_at DESC);
