package database

func EnsureRuntimeSchema() error {
	for _, statement := range runtimeSchemaStatements() {
		if err := DB.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func runtimeSchemaStatements() []string {
	return []string{
		`ALTER TABLE users
			ADD COLUMN IF NOT EXISTS failed_login_attempts INTEGER NOT NULL DEFAULT 0,
			ADD COLUMN IF NOT EXISTS login_locked_until TIMESTAMPTZ`,
		`DROP INDEX IF EXISTS idx_users_device_id`,
		`CREATE INDEX IF NOT EXISTS idx_users_device_id
			ON users (device_id)
			WHERE device_id IS NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_guest_device_id
			ON users (device_id)
			WHERE device_id IS NOT NULL AND is_guest = TRUE`,
		`CREATE INDEX IF NOT EXISTS idx_users_login_locked_until
			ON users (login_locked_until)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_read_created
			ON notifications (user_id, read_at, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_created
			ON notifications (user_id, created_at DESC)`,
		`UPDATE accounts
			SET type = 'credit_card'
			WHERE LOWER(type) = 'credit'`,
		`UPDATE accounts
			SET type = 'debit_card'
			WHERE LOWER(type) = 'debit'`,
		`UPDATE accounts
			SET type = 'wallet'
			WHERE LOWER(type) = 'wallets'`,
		`ALTER TABLE accounts
			ALTER COLUMN credit_limit TYPE NUMERIC(19,2) USING ROUND(credit_limit::numeric, 2),
			ALTER COLUMN credit_limit SET DEFAULT 0,
			ALTER COLUMN credit_limit SET NOT NULL,
			ALTER COLUMN balance TYPE NUMERIC(19,2) USING ROUND(balance::numeric, 2),
			ALTER COLUMN balance SET DEFAULT 0,
			ALTER COLUMN balance SET NOT NULL`,
		`INSERT INTO accounts (user_id, type, name, color, is_default, created_at, updated_at)
			SELECT users.id, 'cash', 'Cash', '#2ECC71', true, NOW(), NOW()
			FROM users
			WHERE NOT EXISTS (
				SELECT 1 FROM accounts WHERE accounts.user_id = users.id
			)`,
		`UPDATE entries
			SET account_id = defaults.id
			FROM (
				SELECT DISTINCT ON (user_id) id, user_id
				FROM accounts
				ORDER BY user_id, is_default DESC, id
			) AS defaults
			WHERE entries.user_id = defaults.user_id
				AND entries.account_id IS NULL`,
		`UPDATE entries
			SET source = 'manual'
			WHERE source IS NULL OR TRIM(source) = ''`,
		`ALTER TABLE entries
			ALTER COLUMN account_id SET NOT NULL,
			ALTER COLUMN amount SET NOT NULL,
			ALTER COLUMN source SET DEFAULT 'manual',
			ALTER COLUMN source SET NOT NULL`,
		`UPDATE entries
			SET type = LOWER(type)
			WHERE LOWER(type) IN ('expense', 'income')`,
		`UPDATE entries
			SET source = LOWER(source)
			WHERE LOWER(source) IN ('manual', 'text', 'voice')`,
		`DO $$
		BEGIN
			ALTER TABLE accounts
				ADD CONSTRAINT accounts_user_id_id_unique UNIQUE (user_id, id);
		EXCEPTION
			WHEN duplicate_object THEN NULL;
		END
		$$`,
		`ALTER TABLE accounts
			DROP CONSTRAINT IF EXISTS accounts_type_check`,
		`ALTER TABLE accounts
			ADD CONSTRAINT accounts_type_check
			CHECK (type IN ('cash', 'upi', 'bank', 'credit_card', 'debit_card', 'wallet', 'other'))`,
		`ALTER TABLE accounts
			DROP CONSTRAINT IF EXISTS accounts_credit_limit_non_negative_check`,
		`ALTER TABLE accounts
			ADD CONSTRAINT accounts_credit_limit_non_negative_check CHECK (credit_limit >= 0)`,
		`ALTER TABLE entries
			DROP CONSTRAINT IF EXISTS entries_amount_positive_check`,
		`ALTER TABLE entries
			ADD CONSTRAINT entries_amount_positive_check CHECK (amount > 0)`,
		`ALTER TABLE entries
			DROP CONSTRAINT IF EXISTS entries_type_check`,
		`ALTER TABLE entries
			ADD CONSTRAINT entries_type_check CHECK (type IN ('expense', 'income'))`,
		`ALTER TABLE entries
			DROP CONSTRAINT IF EXISTS entries_source_check`,
		`ALTER TABLE entries
			ADD CONSTRAINT entries_source_check CHECK (source IN ('manual', 'text', 'voice'))`,
		`ALTER TABLE entries
			DROP CONSTRAINT IF EXISTS fk_entries_owned_account`,
		`ALTER TABLE entries
			ADD CONSTRAINT fk_entries_owned_account
			FOREIGN KEY (user_id, account_id) REFERENCES accounts(user_id, id)`,
		`CREATE INDEX IF NOT EXISTS idx_split_friends_user_archived
			ON split_friends (user_id, archived, name)`,
		`CREATE INDEX IF NOT EXISTS idx_split_bills_user_date
			ON split_bills (user_id, date DESC, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_split_participants_user_friend
			ON split_participants (user_id, friend_id)`,
		`CREATE INDEX IF NOT EXISTS idx_split_settlements_user_friend
			ON split_settlements (user_id, friend_id, date DESC)`,
		`ALTER TABLE split_bills
			DROP CONSTRAINT IF EXISTS split_bills_total_amount_positive_check`,
		`ALTER TABLE split_bills
			ADD CONSTRAINT split_bills_total_amount_positive_check CHECK (total_amount > 0)`,
		`ALTER TABLE split_bills
			DROP CONSTRAINT IF EXISTS split_bills_currency_check`,
		`ALTER TABLE split_bills
			ADD CONSTRAINT split_bills_currency_check CHECK (currency = 'INR')`,
		`ALTER TABLE split_participants
			DROP CONSTRAINT IF EXISTS split_participants_share_amount_positive_check`,
		`ALTER TABLE split_participants
			ADD CONSTRAINT split_participants_share_amount_positive_check CHECK (share_amount > 0)`,
		`ALTER TABLE split_participants
			DROP CONSTRAINT IF EXISTS split_participants_direction_check`,
		`ALTER TABLE split_participants
			ADD CONSTRAINT split_participants_direction_check
			CHECK (direction IN ('friend_owes_user', 'user_owes_friend'))`,
		`ALTER TABLE split_settlements
			DROP CONSTRAINT IF EXISTS split_settlements_amount_positive_check`,
		`ALTER TABLE split_settlements
			ADD CONSTRAINT split_settlements_amount_positive_check CHECK (amount > 0)`,
		`ALTER TABLE split_settlements
			DROP CONSTRAINT IF EXISTS split_settlements_direction_check`,
		`ALTER TABLE split_settlements
			ADD CONSTRAINT split_settlements_direction_check
			CHECK (direction IN ('friend_paid_user', 'user_paid_friend'))`,
	}
}
