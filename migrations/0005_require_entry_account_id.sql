-- 0003 temporarily allowed nullable entry accounts. Re-apply the locked
-- transaction contract after ensuring every user and legacy entry has a Cash
-- account to reference.
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
    ALTER COLUMN account_id SET NOT NULL;
