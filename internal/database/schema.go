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
		`CREATE TABLE IF NOT EXISTS budgets (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name VARCHAR(120) NOT NULL,
			period VARCHAR(16) NOT NULL DEFAULT 'monthly',
			category VARCHAR(80) NOT NULL DEFAULT '',
			limit_amount NUMERIC(19,2) NOT NULL,
			currency CHAR(3) NOT NULL DEFAULT 'INR',
			alert_threshold_percent INTEGER NOT NULL DEFAULT 80,
			active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_budgets_user_active
			ON budgets (user_id, active, period)`,
		`CREATE INDEX IF NOT EXISTS idx_budgets_user_category
			ON budgets (user_id, category)`,
		`ALTER TABLE budgets
			DROP CONSTRAINT IF EXISTS budgets_period_check`,
		`ALTER TABLE budgets
			ADD CONSTRAINT budgets_period_check CHECK (period = 'monthly')`,
		`ALTER TABLE budgets
			DROP CONSTRAINT IF EXISTS budgets_limit_amount_positive_check`,
		`ALTER TABLE budgets
			ADD CONSTRAINT budgets_limit_amount_positive_check CHECK (limit_amount > 0)`,
		`ALTER TABLE budgets
			DROP CONSTRAINT IF EXISTS budgets_currency_check`,
		`ALTER TABLE budgets
			ADD CONSTRAINT budgets_currency_check CHECK (currency = 'INR')`,
		`ALTER TABLE budgets
			DROP CONSTRAINT IF EXISTS budgets_alert_threshold_percent_check`,
		`ALTER TABLE budgets
			ADD CONSTRAINT budgets_alert_threshold_percent_check
			CHECK (alert_threshold_percent BETWEEN 1 AND 100)`,
		`CREATE TABLE IF NOT EXISTS budget_alerts (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			budget_id BIGINT NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
			period_start DATE NOT NULL,
			kind VARCHAR(24) NOT NULL,
			spend_amount NUMERIC(19,2) NOT NULL,
			limit_amount NUMERIC(19,2) NOT NULL,
			notification_id BIGINT REFERENCES notifications(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_budget_alert_once_period
			ON budget_alerts (user_id, budget_id, period_start, kind)`,
		`CREATE INDEX IF NOT EXISTS idx_budget_alerts_user_budget
			ON budget_alerts (user_id, budget_id)`,
		`ALTER TABLE budget_alerts
			DROP CONSTRAINT IF EXISTS budget_alerts_kind_check`,
		`ALTER TABLE budget_alerts
			ADD CONSTRAINT budget_alerts_kind_check CHECK (kind IN ('warning', 'exceeded'))`,
		`ALTER TABLE budget_alerts
			DROP CONSTRAINT IF EXISTS budget_alerts_spend_amount_non_negative_check`,
		`ALTER TABLE budget_alerts
			ADD CONSTRAINT budget_alerts_spend_amount_non_negative_check CHECK (spend_amount >= 0)`,
		`ALTER TABLE budget_alerts
			DROP CONSTRAINT IF EXISTS budget_alerts_limit_amount_positive_check`,
		`ALTER TABLE budget_alerts
			ADD CONSTRAINT budget_alerts_limit_amount_positive_check CHECK (limit_amount > 0)`,
		`CREATE TABLE IF NOT EXISTS subscriptions (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			account_id BIGINT,
			name VARCHAR(120) NOT NULL,
			merchant VARCHAR(120) NOT NULL DEFAULT '',
			category VARCHAR(80) NOT NULL DEFAULT '',
			amount NUMERIC(19,2) NOT NULL,
			currency CHAR(3) NOT NULL DEFAULT 'INR',
			billing_interval VARCHAR(16) NOT NULL DEFAULT 'monthly',
			next_due_date DATE NOT NULL,
			last_charged_date VARCHAR(10) NOT NULL DEFAULT '',
			status VARCHAR(16) NOT NULL DEFAULT 'active',
			reminder_days INTEGER NOT NULL DEFAULT 3,
			notes TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_subscriptions_user_status_due
			ON subscriptions (user_id, status, next_due_date)`,
		`CREATE INDEX IF NOT EXISTS idx_subscriptions_user_merchant
			ON subscriptions (user_id, merchant)`,
		`CREATE INDEX IF NOT EXISTS idx_subscriptions_user_category
			ON subscriptions (user_id, category)`,
		`ALTER TABLE subscriptions
			DROP CONSTRAINT IF EXISTS subscriptions_amount_positive_check`,
		`ALTER TABLE subscriptions
			ADD CONSTRAINT subscriptions_amount_positive_check CHECK (amount > 0)`,
		`ALTER TABLE subscriptions
			DROP CONSTRAINT IF EXISTS subscriptions_currency_check`,
		`ALTER TABLE subscriptions
			ADD CONSTRAINT subscriptions_currency_check CHECK (currency = 'INR')`,
		`ALTER TABLE subscriptions
			DROP CONSTRAINT IF EXISTS subscriptions_billing_interval_check`,
		`ALTER TABLE subscriptions
			ADD CONSTRAINT subscriptions_billing_interval_check
			CHECK (billing_interval IN ('weekly', 'monthly', 'yearly'))`,
		`ALTER TABLE subscriptions
			DROP CONSTRAINT IF EXISTS subscriptions_status_check`,
		`ALTER TABLE subscriptions
			ADD CONSTRAINT subscriptions_status_check CHECK (status IN ('active', 'paused', 'cancelled'))`,
		`ALTER TABLE subscriptions
			DROP CONSTRAINT IF EXISTS subscriptions_reminder_days_check`,
		`ALTER TABLE subscriptions
			ADD CONSTRAINT subscriptions_reminder_days_check CHECK (reminder_days BETWEEN 0 AND 30)`,
		`ALTER TABLE subscriptions
			DROP CONSTRAINT IF EXISTS fk_subscriptions_owned_account`,
		`ALTER TABLE subscriptions
			ADD CONSTRAINT fk_subscriptions_owned_account
			FOREIGN KEY (user_id, account_id) REFERENCES accounts(user_id, id)`,
		`CREATE TABLE IF NOT EXISTS subscription_reminders (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			subscription_id BIGINT NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
			due_date DATE NOT NULL,
			kind VARCHAR(24) NOT NULL,
			notification_id BIGINT REFERENCES notifications(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_reminder_once_due
			ON subscription_reminders (user_id, subscription_id, due_date, kind)`,
		`CREATE INDEX IF NOT EXISTS idx_subscription_reminders_user_subscription
			ON subscription_reminders (user_id, subscription_id)`,
		`ALTER TABLE subscription_reminders
			DROP CONSTRAINT IF EXISTS subscription_reminders_kind_check`,
		`ALTER TABLE subscription_reminders
			ADD CONSTRAINT subscription_reminders_kind_check CHECK (kind IN ('due', 'overdue'))`,
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
			IF NOT EXISTS (
				SELECT 1
				FROM pg_constraint
				WHERE conrelid = 'accounts'::regclass
					AND conname = 'accounts_user_id_id_unique'
			) AND to_regclass('accounts_user_id_id_unique') IS NULL THEN
				ALTER TABLE accounts
					ADD CONSTRAINT accounts_user_id_id_unique UNIQUE (user_id, id);
			END IF;
		EXCEPTION
			WHEN duplicate_object OR duplicate_table THEN NULL;
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
		`ALTER TABLE split_bills
			ADD COLUMN IF NOT EXISTS group_id BIGINT`,
		`CREATE TABLE IF NOT EXISTS split_groups (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			archived BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS split_group_members (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			group_id BIGINT NOT NULL REFERENCES split_groups(id) ON DELETE CASCADE,
			friend_id BIGINT NOT NULL REFERENCES split_friends(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_split_groups_user_archived
			ON split_groups (user_id, archived, name)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_split_group_members_unique
			ON split_group_members (user_id, group_id, friend_id)`,
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
