CREATE TABLE IF NOT EXISTS split_friends (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    email TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    archived BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS split_bills (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entry_id BIGINT REFERENCES entries(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    total_amount NUMERIC(19,2) NOT NULL,
    currency CHAR(3) NOT NULL DEFAULT 'INR',
    date TEXT NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT split_bills_total_amount_positive_check CHECK (total_amount > 0),
    CONSTRAINT split_bills_currency_check CHECK (currency = 'INR')
);

CREATE TABLE IF NOT EXISTS split_participants (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bill_id BIGINT NOT NULL REFERENCES split_bills(id) ON DELETE CASCADE,
    friend_id BIGINT NOT NULL REFERENCES split_friends(id) ON DELETE CASCADE,
    share_amount NUMERIC(19,2) NOT NULL,
    direction VARCHAR(24) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT split_participants_share_amount_positive_check CHECK (share_amount > 0),
    CONSTRAINT split_participants_direction_check CHECK (direction IN ('friend_owes_user', 'user_owes_friend'))
);

CREATE TABLE IF NOT EXISTS split_settlements (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    friend_id BIGINT NOT NULL REFERENCES split_friends(id) ON DELETE CASCADE,
    amount NUMERIC(19,2) NOT NULL,
    direction VARCHAR(24) NOT NULL,
    date TEXT NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT split_settlements_amount_positive_check CHECK (amount > 0),
    CONSTRAINT split_settlements_direction_check CHECK (direction IN ('friend_paid_user', 'user_paid_friend'))
);

CREATE INDEX IF NOT EXISTS idx_split_friends_user_archived
    ON split_friends (user_id, archived, name);
CREATE INDEX IF NOT EXISTS idx_split_bills_user_date
    ON split_bills (user_id, date DESC, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_split_participants_user_friend
    ON split_participants (user_id, friend_id);
CREATE INDEX IF NOT EXISTS idx_split_settlements_user_friend
    ON split_settlements (user_id, friend_id, date DESC);
