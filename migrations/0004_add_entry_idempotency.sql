ALTER TABLE entries
    ADD COLUMN IF NOT EXISTS idempotency_key varchar(128);

CREATE UNIQUE INDEX IF NOT EXISTS idx_entries_user_idempotency
    ON entries (user_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
