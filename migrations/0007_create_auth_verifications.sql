CREATE TABLE IF NOT EXISTS auth_verifications (
    id BIGSERIAL PRIMARY KEY,
    identifier_type VARCHAR(16) NOT NULL,
    identifier TEXT NOT NULL,
    otp_hash TEXT NOT NULL,
    claim_token_hash CHAR(64),
    otp_expires_at TIMESTAMPTZ NOT NULL,
    verified_at TIMESTAMPTZ,
    claim_expires_at TIMESTAMPTZ,
    claim_used_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_auth_verifications_identifier
    ON auth_verifications (identifier_type, identifier, otp_expires_at);

CREATE INDEX IF NOT EXISTS idx_auth_verifications_verified_at
    ON auth_verifications (verified_at);

CREATE INDEX IF NOT EXISTS idx_auth_verifications_claim_expires_at
    ON auth_verifications (claim_expires_at);

CREATE INDEX IF NOT EXISTS idx_auth_verifications_claim_used_at
    ON auth_verifications (claim_used_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_verifications_claim_token_hash
    ON auth_verifications (claim_token_hash)
    WHERE claim_token_hash IS NOT NULL;
