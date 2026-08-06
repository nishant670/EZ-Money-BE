CREATE TABLE IF NOT EXISTS recurring_candidate_decisions (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  candidate_key VARCHAR(180) NOT NULL,
  merchant VARCHAR(120) NOT NULL DEFAULT '',
  category VARCHAR(80) NOT NULL DEFAULT '',
  decision VARCHAR(24) NOT NULL,
  snoozed_until DATE,
  last_reviewed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_recurring_decision_user_key
  ON recurring_candidate_decisions (user_id, candidate_key);

CREATE INDEX IF NOT EXISTS idx_recurring_decision_user_decision
  ON recurring_candidate_decisions (user_id, decision);

ALTER TABLE recurring_candidate_decisions
  DROP CONSTRAINT IF EXISTS recurring_candidate_decisions_decision_check;

ALTER TABLE recurring_candidate_decisions
  ADD CONSTRAINT recurring_candidate_decisions_decision_check
  CHECK (decision IN ('dismissed', 'snoozed', 'tracked'));
