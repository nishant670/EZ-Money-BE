# FINNRI AI Parsing

## Behaviour Rules
- AI proposes, user confirms.
- Never save without explicit user action.
- Do not hallucinate merchant, account, or category.
- Make uncertainty visible.
- Do not persist parse attempts in MVP. Return source text/transcript only in
  the parse draft and persist it only if the user confirms and saves the entry.
- Avoid permanent voice recording storage by default.

## Source Text And Transcript Retention

The MVP retention policy is data minimization:

- Voice audio is transient. The API holds uploaded audio in memory only long
  enough to send it to the speech-to-text provider, then discards it.
- `/v1/parse` does not create a `ParseAttempt` record and must not write raw
  text, transcripts, provider prompts, or provider responses to application
  logs.
- Parse failures may echo the transcript in the API response so the client can
  let the user edit or retry, but that response is not stored by the backend.
- Confirmed entries may store `source_text` as provenance for the saved
  transaction. Users can remove or edit that text by editing the entry.
- Deleting an entry deletes its stored `source_text` with the entry. Full
  account/data deletion is exposed through `DELETE /v1/user`.
- Future parse-attempt audit storage is deferred because raw financial text can
  include sensitive personal details. It must be opt-in from a product/privacy
  perspective or have a short explicit retention window, access controls, and a
  deletion job before it is enabled.

## Provider Abstraction
Implement an internal parser interface so OpenAI, Claude, local models, or rule engines can be swapped without changing transaction persistence.

## Output Contract
amount, currency, type, merchant, category, account_hint, date, note, tags, recurring_candidate, split_candidate, confidence, missing_fields.
