UPDATE accounts
SET type = 'credit_card'
WHERE LOWER(type) = 'credit';

UPDATE accounts
SET type = 'debit_card'
WHERE LOWER(type) = 'debit';

UPDATE accounts
SET type = 'wallet'
WHERE LOWER(type) = 'wallets';
