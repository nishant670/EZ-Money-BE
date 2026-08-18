-- Card EMI plans.
--
-- Converting a purchase to instalments blocks its full principal against the
-- card's limit immediately, and releases it a slice at a time as each
-- instalment's principal is paid. Only the principal component releases limit:
-- interest is a charge, not a repayment of what was borrowed.
--
-- The three instalment states exist to stop the same money reducing the limit
-- twice. Once an instalment is billed onto a statement it lives inside that
-- statement's total_due, which is already inside the card's outstanding, so it
-- must stop counting as blocked at the same moment it starts being billed.

CREATE TABLE IF NOT EXISTS card_emi_plans (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  title VARCHAR(120) NOT NULL,
  merchant VARCHAR(120),
  category VARCHAR(80),
  principal NUMERIC(19,2) NOT NULL,
  annual_rate_pct DOUBLE PRECISION NOT NULL DEFAULT 0,
  tenure_months INTEGER NOT NULL,
  monthly_amount NUMERIC(19,2) NOT NULL,
  total_interest NUMERIC(19,2) NOT NULL DEFAULT 0,
  currency CHAR(3) NOT NULL DEFAULT 'INR',
  purchased_on DATE NOT NULL,
  first_installment DATE NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'active',
  source_entry_id BIGINT,
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT card_emi_plans_principal_positive CHECK (principal > 0),
  CONSTRAINT card_emi_plans_tenure_sane CHECK (tenure_months BETWEEN 1 AND 360),
  CONSTRAINT card_emi_plans_rate_sane CHECK (annual_rate_pct >= 0 AND annual_rate_pct <= 100),
  CONSTRAINT card_emi_plans_status_check CHECK (status IN ('active', 'closed', 'foreclosed'))
);

CREATE INDEX IF NOT EXISTS idx_card_emi_plans_account
  ON card_emi_plans (account_id, status);
CREATE INDEX IF NOT EXISTS idx_card_emi_plans_user
  ON card_emi_plans (user_id, status);

CREATE TABLE IF NOT EXISTS card_emi_installments (
  id BIGSERIAL PRIMARY KEY,
  plan_id BIGINT NOT NULL REFERENCES card_emi_plans(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  seq INTEGER NOT NULL,
  due_date DATE NOT NULL,
  amount NUMERIC(19,2) NOT NULL,
  principal_part NUMERIC(19,2) NOT NULL,
  interest_part NUMERIC(19,2) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'scheduled',
  statement_id BIGINT REFERENCES card_statements(id) ON DELETE SET NULL,
  entry_id BIGINT REFERENCES entries(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT card_emi_installments_amounts_non_negative
    CHECK (amount >= 0 AND principal_part >= 0 AND interest_part >= 0),
  CONSTRAINT card_emi_installments_status_check
    CHECK (status IN ('scheduled', 'billed', 'paid'))
);

-- One row per month of a plan. Also the guard that makes schedule generation
-- safe to retry.
CREATE UNIQUE INDEX IF NOT EXISTS idx_card_emi_installment_once
  ON card_emi_installments (plan_id, seq);

-- Serves the blocked-principal sum, which runs on every accounts read.
CREATE INDEX IF NOT EXISTS idx_card_emi_installments_blocked
  ON card_emi_installments (account_id, status);

-- Serves the billing sweep.
CREATE INDEX IF NOT EXISTS idx_card_emi_installments_due
  ON card_emi_installments (due_date)
  WHERE status = 'scheduled';
