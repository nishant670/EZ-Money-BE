ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS auto_created BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_accounts_auto_created
    ON accounts (user_id, auto_created)
    WHERE auto_created = TRUE;
