-- Web entries created before T3 labelled both bank and debit-card spending as
-- Credit Card. The linked account is authoritative here, so this correction is
-- deterministic rather than a guess. The NOTICE is the production audit trail:
-- it reports the affected row count before changing anything.
DO $$
DECLARE
    affected_rows BIGINT;
BEGIN
    SELECT count(*)
      INTO affected_rows
      FROM entries AS entry
      JOIN accounts AS account ON account.id = entry.account_id
     WHERE entry.mode = 'Credit Card'
       AND account.type IN ('bank', 'debit_card');

    RAISE NOTICE 'T3 audit: % Credit Card entries are linked to bank/debit-card accounts', affected_rows;

    UPDATE entries AS entry
       SET mode = 'Bank Account'
      FROM accounts AS account
     WHERE account.id = entry.account_id
       AND entry.mode = 'Credit Card'
       AND account.type IN ('bank', 'debit_card');
END $$;
