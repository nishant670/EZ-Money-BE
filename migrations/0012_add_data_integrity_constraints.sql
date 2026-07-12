ALTER TABLE entries
    ALTER COLUMN account_id SET NOT NULL,
    ALTER COLUMN amount SET NOT NULL,
    ALTER COLUMN source SET DEFAULT 'manual',
    ALTER COLUMN source SET NOT NULL;

UPDATE entries
SET type = LOWER(type)
WHERE LOWER(type) IN ('expense', 'income');

UPDATE entries
SET source = LOWER(source)
WHERE LOWER(source) IN ('manual', 'text', 'voice');

DO $$
BEGIN
    ALTER TABLE accounts
        ADD CONSTRAINT accounts_user_id_id_unique UNIQUE (user_id, id);
EXCEPTION
    WHEN duplicate_object THEN NULL;
END
$$;

ALTER TABLE accounts
    DROP CONSTRAINT IF EXISTS accounts_type_check;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_type_check
    CHECK (type IN ('cash', 'upi', 'bank', 'credit_card', 'debit_card', 'wallet', 'other'));

ALTER TABLE accounts
    DROP CONSTRAINT IF EXISTS accounts_credit_limit_non_negative_check;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_credit_limit_non_negative_check
    CHECK (credit_limit >= 0);

ALTER TABLE entries
    DROP CONSTRAINT IF EXISTS entries_amount_positive_check;

ALTER TABLE entries
    ADD CONSTRAINT entries_amount_positive_check
    CHECK (amount > 0);

ALTER TABLE entries
    DROP CONSTRAINT IF EXISTS entries_type_check;

ALTER TABLE entries
    ADD CONSTRAINT entries_type_check
    CHECK (type IN ('expense', 'income'));

ALTER TABLE entries
    DROP CONSTRAINT IF EXISTS entries_source_check;

ALTER TABLE entries
    ADD CONSTRAINT entries_source_check
    CHECK (source IN ('manual', 'text', 'voice'));

ALTER TABLE entries
    DROP CONSTRAINT IF EXISTS fk_entries_owned_account;

ALTER TABLE entries
    ADD CONSTRAINT fk_entries_owned_account
    FOREIGN KEY (user_id, account_id) REFERENCES accounts(user_id, id);
