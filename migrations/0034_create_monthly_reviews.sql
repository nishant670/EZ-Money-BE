-- One monthly review per user per month. The unique index is the delivery
-- guarantee: the job that emits reviews runs hourly through the send window and
-- relies on the second insert failing rather than on knowing whether it has
-- already run.
CREATE TABLE IF NOT EXISTS monthly_reviews (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  month CHAR(7) NOT NULL,
  total_spent NUMERIC(19,2) NOT NULL DEFAULT 0,
  transaction_count INTEGER NOT NULL DEFAULT 0,
  notification_id BIGINT REFERENCES notifications(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_monthly_review_once
  ON monthly_reviews (user_id, month);

CREATE INDEX IF NOT EXISTS idx_monthly_reviews_notification
  ON monthly_reviews (notification_id);
