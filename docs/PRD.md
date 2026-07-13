# FINNRI Updated PRD v2.0 - Codex Ready

**Tagline:** Your money, understood.  
**Supporting line:** Speak your expenses. Confirm in seconds. Understand where your money goes.

## Product Decision
FINNRI should start as a focused MVP, not a broad personal finance suite. The MVP is: effortless AI capture, confirm-first trust, account-linked transactions, and simple actionable insights. Later-phase split ledger and EMI calculator APIs can exist as isolated utilities, while full planning workflows, full subscription management, web dashboard, bank integrations, and open-ended AI financial advice remain deferred until the core habit loop is proven.

## Executive Summary
FINNRI is an AI-powered personal finance companion for young Indians. Users speak or type what happened financially, AI extracts draft transaction details, the user confirms or edits, and FINNRI turns the data into clear insights.

## Vision
Become the trusted AI-powered financial companion that helps young Indians capture money movement effortlessly, understand spending behaviour, and make smarter everyday financial decisions.

## Primary Persona
- Age: 24-32
- Market: India; Bangalore, Pune, Hyderabad, Mumbai, Delhi NCR, Chennai and similar cities
- Occupation: salaried professional, IT/startup employee, early-career corporate worker
- Behaviour: UPI daily, food delivery, subscriptions, rent, one or more bank accounts, possibly credit cards
- Pain: wants to save more but does not track consistently

## Core Product Principles
1. Capture should be effortless.
2. AI assists; it never assumes.
3. Accounts/payment sources matter from day one.
4. Insights should drive decisions, not just charts.
5. Guest-first reduces friction.

## MVP Hypothesis
Users will build a daily habit of logging transactions if FINNRI makes capture fast, uses confirm-first AI to maintain trust, and gives them useful insights that make the habit feel worthwhile.

## MVP Scope
Included:
- Guest-first onboarding
- Voice/text AI transaction capture
- Editable confirm transaction sheet
- Manual transaction entry with smart defaults
- Accounts/payment sources: cash, UPI, bank, credit card, wallet
- Transaction list, search, filter, edit, delete
- Basic dashboard
- Deterministic insights with optional AI wording
- Lightweight recurring/subscription candidate detection
- Optional simple split metadata
- Later-phase EMI calculator API

Deferred:
- Full bill splitting groups and settlements
- Full EMI planning workflow beyond the calculator API
- Full subscription manager
- Web dashboard
- Open-ended AI financial advisor
- Bank/UPI integrations
- Statement imports/reconciliation

## Primary User Loop
1. User opens FINNRI.
2. User speaks or types a financial event.
3. AI extracts transaction details.
4. User reviews and edits confirmation sheet.
5. User saves transaction.
6. Dashboard updates.
7. FINNRI shows a useful insight.
8. User returns because the product gave clarity with low effort.

## Core Features
### AI Transaction Capture
Extract amount, type, merchant, category, date, account/payment source, note/tags, recurring candidate, and split candidate from natural language.

### Confirm Transaction Sheet
Mandatory before save. User can edit amount, type, category, merchant, account, date, note, and tags.

### Accounts
Support cash, UPI, bank, credit card, and wallet accounts. Every transaction should support account linking.

### Transactions
Grouped by date; searchable and filterable by merchant, category, note, account, date range, and type. Supports edit and delete.

### Dashboard
Monthly spend, daily average, top categories, account-wise spend, recent transactions, and smart insights.

### Insights
Use deterministic calculations first, AI wording second. Examples: monthly comparison, category increase, top merchant, account usage, anomaly, recurring candidate.

## Recommended Stack
- Mobile: React Native + Expo
- Backend: Continue Go/Gin if already working
- Database: PostgreSQL
- ORM: GORM if already implemented
- AI parsing: provider abstraction
- Speech-to-text: provider abstraction
- Auth: guest first, optional login later

## Data Model
- User
- Account
- Category string on each transaction for MVP; normalized category model later
- Transaction
- ParseAttempt
- Insight
- RecurringCandidate
- SplitMetadata

## API Requirements
- POST /v1/parse
- POST /v1/entries
- GET /v1/entries
- GET /v1/entries/:id
- PUT /v1/entries/:id
- DELETE /v1/entries/:id
- POST /v1/auth/guest
- GET /v1/accounts
- POST /v1/accounts
- PUT /v1/accounts/:id
- DELETE /v1/accounts/:id
- GET /v1/dashboard
- GET /v1/insights

`/v1/entries` is the canonical Phase 1.2 transaction persistence route. Product
language may say "transactions", but client and backend contracts should use the
versioned route names above.

## AI Rules
- AI proposes; user confirms.
- Uncertainty must be visible.
- Do not hallucinate merchants/accounts.
- Do not save without explicit user action.
- MVP insights are spending analysis, not investment/tax/legal advice.
- Do not permanently store raw voice recordings by default.

## Suggested Repo Docs
- README.md
- docs/PRD.md
- docs/MVP_SCOPE.md
- docs/ARCHITECTURE.md
- docs/DATA_MODEL.md
- docs/API_SPEC.md
- docs/AI_PARSING.md
- docs/UI_UX_GUIDELINES.md
- docs/CODE_REVIEW_CHECKLIST.md
- docs/ROADMAP.md

## Suggested First Codex Prompt
Read the FINNRI PRD v2 and inspect the existing repository. Do not rewrite the stack. First create/update README.md and docs/ARCHITECTURE.md, docs/MVP_SCOPE.md, docs/DATA_MODEL.md, docs/API_SPEC.md, docs/AI_PARSING.md, docs/UI_UX_GUIDELINES.md, and docs/CODE_REVIEW_CHECKLIST.md. Then provide a code review of the existing mobile app and backend against the PRD, listing gaps, risks, and a recommended implementation plan in small PR-sized steps.

## MVP Acceptance Criteria
- New user can start as guest and save a first transaction without login.
- Natural-language input returns a draft transaction.
- No AI-parsed transaction is saved unless user confirms.
- User can create/select payment source and link it to transaction.
- User can list, search, filter, edit, and delete transactions.
- Dashboard shows monthly spend, daily average, top categories, account-wise spend, recent transactions, and insights.
- At least 5 reliable insight templates exist.
- Raw voice recordings are not stored permanently by default.
- Core flows have automated tests or manual QA steps.
