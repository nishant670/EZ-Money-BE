# Manual Mobile QA

This runbook covers mobile behaviors that need real device or simulator checks
because they depend on OS permissions, microphone input, audio file handling, or
native UI behavior. Record the date, app build, backend commit, device, OS, and
tester for every run.

## Voice Capture QA

### Scope

Voice capture is part of the Phase 1.2 habit loop:

1. Continue as guest.
2. Capture a spend by microphone from the Home tab.
3. Send temporary audio to `POST /v1/parse`.
4. Review the returned draft in the confirmation modal.
5. Confirm and save an account-linked transaction.
6. Verify the transaction list and dashboard update.

This QA does not replace automated API, component, or flow tests. It verifies the
native recording surface and OS integration that Jest and backend tests cannot
cover.

### Required Matrix

Run the happy path on at least:

- One iOS physical device or iOS simulator with microphone input enabled.
- One Android physical device or Android emulator with microphone input enabled.

Before external beta, include at least one low-end Android device and one device
using a non-English keyboard/locale.

### Setup

- Backend is running with a reachable `EXPO_PUBLIC_API_BASE_URL` for the device.
  Physical devices must use the machine LAN IP or a tunnel, not `localhost`.
- Backend has a valid AI/STT provider configuration for live voice parsing.
- Mobile app is a fresh install or has app data cleared.
- Microphone permission can be reset from OS settings before the permission
  tests.
- Test account starts with no saved transactions for the current day, or the
  tester records the expected baseline counts before starting.

### Happy Path

1. Launch the mobile app.
2. Continue as guest.
3. Confirm the Home tab shows the capture card with the microphone action.
4. Tap the microphone button.
5. Grant microphone permission when prompted.
6. Confirm the UI enters recording state and shows "Listening...".
7. Speak a clear transaction such as "chai 80 cash today".
8. Tap the stop button.
9. Confirm the card changes to "Recording ready" with Process and record-again
   actions.
10. Tap Process.
11. Confirm the transaction review modal opens and no transaction has been saved
    yet.
12. Confirm the draft is populated with amount, type, mode, category, date, and
    source text/transcript. Edit any uncertain or missing fields.
13. Confirm an owned account is selected or auto-created for the payment mode.
14. Tap Confirm & Save.
15. Confirm the modal closes and a saved confirmation appears.
16. Confirm the transaction appears in recent activity with source `voice` and
    the selected account.
17. Open the dashboard/insights tab and refresh if needed.
18. Confirm today's spend, recent transactions, top categories, and account
    spending include the saved transaction.

### Permission And Recovery Cases

- Deny microphone permission, tap the microphone again, and confirm the app shows
  "Microphone permission is required to record audio." without crashing.
- Re-enable permission in OS settings, return to the app, and confirm recording
  can start.
- Start recording, stop, tap the record-again action, and confirm the old
  recording is cleared.
- Start recording, navigate away from the Home tab or background the app, return,
  and confirm there is no stuck recording state.
- Submit with neither text nor recording and confirm the app asks the user to
  type or record an expense.

### Parse And Network Cases

- With the backend unreachable, process a recording and confirm a recoverable
  error is shown and the user can retry or record again.
- With provider/STT failure simulated or observed, confirm the app shows a
  recoverable parse error and does not save a transaction.
- Record silence or unusable audio and confirm the app does not open a misleading
  completed transaction; it should ask for retry or correction.
- Type text into "I prefer to write" and process it to confirm text fallback
  still works after a voice attempt.

### Privacy Checks

- Confirm raw audio is not displayed in transaction details after save.
- Confirm only the transcript/source text is attached to the confirmed saved
  entry.
- Confirm backend and mobile logs do not print raw audio, provider payloads,
  bearer tokens, or full transaction request bodies.
- Confirm account deletion removes saved transactions and source text according
  to `docs/AI_PARSING.md`.

### Pass Criteria

- A guest can complete the voice capture -> review -> confirm -> save ->
  dashboard loop on each required platform.
- The parse draft is never saved before explicit confirmation.
- Permission denial, bad audio, backend failure, and provider failure all leave
  the user in a recoverable state.
- Recording state does not get stuck after stop, retry, navigation, or app
  backgrounding.
- No raw audio is retained or exposed by default.

### Run Record Template

Use this template for each manual QA run:

```text
Date:
Tester:
Mobile commit/build:
Backend commit:
Device and OS:
Backend base URL:
AI/STT provider:

Happy path:
Permission recovery:
Parse/network recovery:
Privacy checks:
Notes and follow-ups:
```
