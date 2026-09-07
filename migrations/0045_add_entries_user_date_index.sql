-- The insights, reports, and transaction history paths all scope by owner and
-- order by the legacy ISO date string. This index removes the per-user sort
-- while the separate varchar-to-date migration is planned and validated.
CREATE INDEX IF NOT EXISTS idx_entries_user_date
    ON entries (user_id, date DESC);
