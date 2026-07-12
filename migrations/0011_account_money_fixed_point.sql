ALTER TABLE accounts
    ALTER COLUMN credit_limit TYPE NUMERIC(19,2) USING ROUND(credit_limit::numeric, 2),
    ALTER COLUMN credit_limit SET DEFAULT 0,
    ALTER COLUMN credit_limit SET NOT NULL,
    ALTER COLUMN balance TYPE NUMERIC(19,2) USING ROUND(balance::numeric, 2),
    ALTER COLUMN balance SET DEFAULT 0,
    ALTER COLUMN balance SET NOT NULL;
