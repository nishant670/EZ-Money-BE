ALTER TABLE entries
    ADD COLUMN IF NOT EXISTS account_id bigint;

CREATE INDEX IF NOT EXISTS idx_entries_account_id
    ON entries (account_id);

DO $$
BEGIN
    ALTER TABLE entries
        ADD CONSTRAINT fk_entries_account
        FOREIGN KEY (account_id) REFERENCES accounts(id);
EXCEPTION
    WHEN duplicate_object THEN NULL;
END
$$;

-- Legacy rows remain nullable. The API requires an owned account_id for every
-- new create or update; users select an account when editing legacy rows.
