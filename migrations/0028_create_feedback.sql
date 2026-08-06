CREATE TABLE IF NOT EXISTS feedback (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type VARCHAR(32) NOT NULL,
  area VARCHAR(64) NOT NULL DEFAULT '',
  title VARCHAR(140) NOT NULL,
  message TEXT NOT NULL,
  impact VARCHAR(32) NOT NULL DEFAULT 'nice_to_have',
  status VARCHAR(32) NOT NULL DEFAULT 'new',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_feedback_user_created
  ON feedback (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_feedback_status_created
  ON feedback (status, created_at DESC);

ALTER TABLE feedback
  DROP CONSTRAINT IF EXISTS feedback_type_check;

ALTER TABLE feedback
  ADD CONSTRAINT feedback_type_check
  CHECK (type IN ('bug', 'idea', 'improvement', 'feature_request', 'other'));

ALTER TABLE feedback
  DROP CONSTRAINT IF EXISTS feedback_impact_check;

ALTER TABLE feedback
  ADD CONSTRAINT feedback_impact_check
  CHECK (impact IN ('critical', 'high', 'medium', 'nice_to_have'));

ALTER TABLE feedback
  DROP CONSTRAINT IF EXISTS feedback_status_check;

ALTER TABLE feedback
  ADD CONSTRAINT feedback_status_check
  CHECK (status IN ('new', 'reviewing', 'planned', 'shipped', 'closed'));
