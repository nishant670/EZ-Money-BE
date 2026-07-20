CREATE TABLE IF NOT EXISTS ai_abuse_blocks (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    guest_device_id_hash CHAR(64) NOT NULL DEFAULT '',
    scope VARCHAR(32) NOT NULL DEFAULT 'ai_parse',
    reason_code VARCHAR(80) NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    expires_at TIMESTAMPTZ,
    created_by VARCHAR(120) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_abuse_blocks_user_active
    ON ai_abuse_blocks (user_id, active, expires_at)
    WHERE user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ai_abuse_blocks_guest_active
    ON ai_abuse_blocks (guest_device_id_hash, active, expires_at)
    WHERE guest_device_id_hash <> '';

CREATE INDEX IF NOT EXISTS idx_ai_abuse_blocks_scope_active
    ON ai_abuse_blocks (scope, active, created_at DESC);

ALTER TABLE ai_abuse_blocks
    DROP CONSTRAINT IF EXISTS ai_abuse_blocks_identity_check;
ALTER TABLE ai_abuse_blocks
    ADD CONSTRAINT ai_abuse_blocks_identity_check
    CHECK (user_id IS NOT NULL OR guest_device_id_hash <> '');

ALTER TABLE ai_abuse_blocks
    DROP CONSTRAINT IF EXISTS ai_abuse_blocks_scope_check;
ALTER TABLE ai_abuse_blocks
    ADD CONSTRAINT ai_abuse_blocks_scope_check
    CHECK (scope IN ('ai_parse', 'all_ai'));
