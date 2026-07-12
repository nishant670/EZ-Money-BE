package database

func EnsureRuntimeSchema() error {
	statements := []string{
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
	}

	for _, statement := range statements {
		if err := DB.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}
