-- Canonicalize entry categories onto a single vocabulary.
--
-- The mobile app added "Food & Drinks" and "Transport" to its picker, but
-- schemas/expense_entry.schema.json, internal/ai/prompt.txt, and the
-- canonicalCategory() normalizer still only knew the original six categories.
-- The parser could not emit either new value, so AI-captured meals were filed
-- as "Food" while manual entries used "Food & Drinks" — the same spend split
-- across two keys, fragmenting every category rollup, budget, and comparison.
--
-- "Finance" and "Split" also reached the table because POST /v1/entries never
-- validated the category at all. It does now.
--
-- Canonical set: Food & Drinks, Transport, Travel, Shopping, Bills,
--                Family/Gifts, Misc
--
-- Safe to re-run: every statement is idempotent.

-- The straight duplicate.
UPDATE entries SET category = 'Food & Drinks' WHERE category = 'Food';

-- Strays the old parser invented before it was held to the schema enum.
-- "Finance" was an investment entry; the Investment tag carries that meaning,
-- so the category falls back rather than inventing a bucket for one row.
UPDATE entries SET category = 'Misc' WHERE category IN ('Finance', 'Split');

-- Deliberately NOT a catch-all. The category picker lets people add their own
-- names ("Pet Care", "Tuition"), and those are legitimate user data — rewriting
-- every non-canonical value to Misc would delete them. Only the two names the
-- parser produced on its own are corrected above.

-- Quick prompts are seeded per user and carry categories too.
UPDATE quick_prompts SET category = 'Food & Drinks' WHERE category = 'Food';

-- NOT rewritten: existing "Travel" rows. Some are genuinely commutes that the
-- old prompt forced into Travel, but there is no reliable way to tell those
-- apart from real trips without guessing at the user's own data. Going forward
-- the parser files commutes as Transport.
