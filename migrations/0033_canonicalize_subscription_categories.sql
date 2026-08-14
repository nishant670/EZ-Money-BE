-- Move subscriptions onto the canonical entry categories.
--
-- Subscriptions had their own category list — Entertainment, Productivity,
-- Cloud, Bills, Membership, Learning — that existed nowhere else in the product.
-- That mattered because autopay copies a subscription's category straight onto
-- the entry it generates (internal/http/subscription_automation.go), so those
-- values landed in the ledger where no screen could render or filter them.
--
-- The subscription picker now offers the canonical set, and autopay routes the
-- value through categoryForSave(). This rewrites the retired names.
--
-- "Entertainment" is unchanged: it was added to the canonical set precisely so
-- streaming subscriptions keep a real home instead of being lumped into Bills.
--
-- Safe to re-run.

UPDATE subscriptions
SET category = 'Bills'
WHERE category IN ('Productivity', 'Cloud', 'Membership', 'Learning');

-- Same for any entries autopay already generated from them.
UPDATE entries
SET category = 'Bills'
WHERE source = 'recurring'
  AND category IN ('Productivity', 'Cloud', 'Membership', 'Learning');

-- Deliberately not a catch-all: categories people typed themselves
-- ("Stock trading") are their data and stay as they are.
