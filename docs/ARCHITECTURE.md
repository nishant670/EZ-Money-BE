# FINNRI Architecture

## Recommended Stack
- Mobile: React Native + Expo
- Backend: Go/Gin if already implemented
- Database: PostgreSQL
- ORM: GORM if already implemented
- AI/STT: provider abstraction

## Principles
- Do not let AI directly persist final transactions.
- Keep parsing separate from transaction services.
- Use provider interfaces for AI and speech-to-text.
- Validate all transaction mutations server-side.
- Preserve existing stack unless there is a clear blocker.

## Suggested Modules
- auth
- accounts
- transactions
- categories
- ai_parser
- insights
- settings
- audit
