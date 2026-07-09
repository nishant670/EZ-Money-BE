-- Backfill every user with an account before making account_id mandatory.
INSERT INTO accounts (user_id, type, name, color, is_default, created_at, updated_at)
SELECT users.id, 'cash', 'Cash', '#2ECC71', true, NOW(), NOW()
FROM users
WHERE NOT EXISTS (
    SELECT 1 FROM accounts WHERE accounts.user_id = users.id
);

UPDATE entries
SET account_id = defaults.id
FROM (
    SELECT DISTINCT ON (user_id) id, user_id
    FROM accounts
    ORDER BY user_id, is_default DESC, id
) AS defaults
WHERE entries.user_id = defaults.user_id
  AND entries.account_id IS NULL;

ALTER TABLE entries
    ALTER COLUMN account_id SET NOT NULL,
    ALTER COLUMN amount TYPE numeric(19,2) USING ROUND(amount::numeric, 2),
    ALTER COLUMN amount SET NOT NULL,
    ALTER COLUMN currency SET DEFAULT 'INR',
    ALTER COLUMN currency SET NOT NULL,
    ADD COLUMN IF NOT EXISTS source varchar(16) NOT NULL DEFAULT 'manual';

ALTER TABLE entries
    DROP CONSTRAINT IF EXISTS entries_source_check;

ALTER TABLE entries
    ADD CONSTRAINT entries_source_check CHECK (source IN ('manual', 'text', 'voice'));
