-- Credit card statement tracking.
--
-- A statement is a second source of truth alongside the ledger: authoritative
-- for how much is owed, while the ledger stays authoritative for what the
-- money was spent on. The card's outstanding is derived from the latest
-- statement plus anything logged since it closed, which is why no synthetic
-- ledger entry is written against the card when a bill is paid.

ALTER TABLE accounts
  ADD COLUMN IF NOT EXISTS statement_day INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS reminder_days_before INTEGER NOT NULL DEFAULT 3,
  ADD COLUMN IF NOT EXISTS autopay_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS card_statements (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  cycle_start DATE NOT NULL,
  cycle_end DATE NOT NULL,
  statement_date DATE NOT NULL,
  due_date DATE NOT NULL,
  total_due NUMERIC(19,2) NOT NULL DEFAULT 0,
  minimum_due NUMERIC(19,2) NOT NULL DEFAULT 0,
  paid_amount NUMERIC(19,2) NOT NULL DEFAULT 0,
  currency CHAR(3) NOT NULL DEFAULT 'INR',
  status VARCHAR(16) NOT NULL DEFAULT 'draft',
  source VARCHAR(16) NOT NULL DEFAULT 'manual',
  unitemized_entry_id BIGINT REFERENCES entries(id) ON DELETE SET NULL,
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT card_statements_amounts_non_negative
    CHECK (total_due >= 0 AND minimum_due >= 0 AND paid_amount >= 0),
  CONSTRAINT card_statements_cycle_ordered
    CHECK (cycle_end >= cycle_start AND due_date > statement_date)
);

-- One statement per card per statement date. This is the idempotency key that
-- lets statement parsing re-run safely: a re-parsed message updates the
-- existing row instead of creating a duplicate.
CREATE UNIQUE INDEX IF NOT EXISTS idx_card_statement_once
  ON card_statements (user_id, account_id, statement_date);

-- Serves both the account history screen and the reminder job's due sweep.
CREATE INDEX IF NOT EXISTS idx_card_statements_account_date
  ON card_statements (account_id, statement_date DESC);
CREATE INDEX IF NOT EXISTS idx_card_statements_due
  ON card_statements (due_date)
  WHERE status <> 'paid';

CREATE TABLE IF NOT EXISTS card_statement_payments (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  statement_id BIGINT NOT NULL REFERENCES card_statements(id) ON DELETE CASCADE,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  from_account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
  bank_entry_id BIGINT REFERENCES entries(id) ON DELETE SET NULL,
  amount NUMERIC(19,2) NOT NULL,
  paid_on DATE NOT NULL,
  method VARCHAR(24),
  note TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT card_statement_payments_amount_positive CHECK (amount > 0)
);

CREATE INDEX IF NOT EXISTS idx_card_statement_payments_statement
  ON card_statement_payments (statement_id, paid_on DESC);
CREATE INDEX IF NOT EXISTS idx_card_statement_payments_user
  ON card_statement_payments (user_id, paid_on DESC);

-- One reminder of each kind per statement. As with subscription_reminders,
-- the job depends on the second insert failing rather than on knowing whether
-- it has already run.
CREATE TABLE IF NOT EXISTS card_statement_reminders (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  statement_id BIGINT NOT NULL REFERENCES card_statements(id) ON DELETE CASCADE,
  kind VARCHAR(24) NOT NULL,
  notification_id BIGINT REFERENCES notifications(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_card_statement_reminder_once
  ON card_statement_reminders (user_id, statement_id, kind);
