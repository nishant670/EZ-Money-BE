ALTER TABLE users
    ADD COLUMN IF NOT EXISTS converted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_active_at TIMESTAMPTZ;

ALTER TABLE feedback
    ADD COLUMN IF NOT EXISTS admin_notes TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS resolved_by BIGINT REFERENCES users(id) ON DELETE SET NULL;

UPDATE feedback SET status = 'triaged' WHERE status = 'reviewing';
UPDATE feedback SET status = 'declined' WHERE status = 'closed';

ALTER TABLE plans
    ADD COLUMN IF NOT EXISTS list_price_minor BIGINT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS admin_users (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(24) NOT NULL DEFAULT 'viewer',
    disabled_at TIMESTAMPTZ,
    created_by VARCHAR(120) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Short-lived admin-console sessions are separated from ordinary app logins so
-- a 30-day app token cannot authenticate the admin API.
ALTER TABLE auth_sessions
    ADD COLUMN IF NOT EXISTS kind VARCHAR(16) NOT NULL DEFAULT 'user';

CREATE INDEX IF NOT EXISTS idx_auth_sessions_kind ON auth_sessions (kind);

-- admin_user_id is nullable: an action taken with the machine token records that
-- no human performed it, rather than borrowing a real owner's identity.
CREATE TABLE IF NOT EXISTS admin_audit_log (
    id BIGSERIAL PRIMARY KEY,
    admin_user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    actor VARCHAR(40) NOT NULL DEFAULT 'admin_user',
    action VARCHAR(80) NOT NULL,
    subject_type VARCHAR(40) NOT NULL DEFAULT '',
    subject_id VARCHAR(120) NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_hash CHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS admin_daily_metrics (
    id BIGSERIAL PRIMARY KEY,
    metric_date DATE NOT NULL UNIQUE,
    active_users INTEGER NOT NULL DEFAULT 0,
    ai_events INTEGER NOT NULL DEFAULT 0,
    ai_credits INTEGER NOT NULL DEFAULT 0,
    ai_cost_usd_micros BIGINT NOT NULL DEFAULT 0,
    successful_ai_events INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE admin_audit_log
    ALTER COLUMN admin_user_id DROP NOT NULL;

ALTER TABLE admin_audit_log
    ADD COLUMN IF NOT EXISTS actor VARCHAR(40) NOT NULL DEFAULT 'admin_user';

CREATE INDEX IF NOT EXISTS idx_admin_users_role
    ON admin_users (role) WHERE disabled_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_admin_audit_log_actor ON admin_audit_log (actor);
CREATE INDEX IF NOT EXISTS idx_admin_audit_log_created ON admin_audit_log (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_admin_audit_log_admin ON admin_audit_log (admin_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_users_last_active_at ON users (last_active_at);
CREATE INDEX IF NOT EXISTS idx_users_converted_at ON users (converted_at);
