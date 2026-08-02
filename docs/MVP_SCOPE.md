# FINNRI MVP Scope

## MVP Goal
Prove that users will form a daily finance-tracking habit when capture is effortless, confirmation is trusted, and insights are useful.

## Included
- Guest-first onboarding
- Voice/text AI transaction capture
- Confirm-before-save transaction sheet
- Manual entry with smart defaults
- Accounts/payment sources: cash, UPI, bank, credit card, wallet
- Transaction list/search/filter/edit/delete
- Dashboard
- Basic deterministic insights
- Lightweight recurring and split candidates
- Later-phase split ledger API for friends, balances, and settlements
- Later-phase EMI calculator API

## Deferred
- Full Splitwise-style groups with friend-to-friend ledgers
- Full EMI planning workflow beyond the calculator API
- Full subscription manager
- Web dashboard (deferred from the mobile MVP; active later-phase work is
  documented in `WEB_DASHBOARD.md`)
- Open-ended AI advisor
- Bank integrations and statement imports

## Acceptance Criteria
- User saves first transaction as guest.
- AI parse result is editable and requires confirmation.
- Every transaction supports account linking.
- Dashboard and insights update after save.
