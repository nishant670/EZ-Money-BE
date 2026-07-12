# Accessibility QA

This runbook covers manual mobile accessibility checks for the Phase 1.2 MVP.
Run it on real devices or simulators whenever capture, confirmation, dashboard,
accounts, transactions, onboarding, authentication, or shared UI primitives
change.

## Scope

Cover these MVP surfaces:

- Onboarding and Continue as guest.
- Home capture card, typed capture, voice capture, and parse errors.
- Transaction review/confirmation modal.
- Manual transaction form.
- Transaction list, transaction detail, edit, and delete flows.
- Accounts list and account create/edit/default/delete flows.
- Dashboard/insights and period controls.
- Profile, lock, OTP, and security setup screens.
- Empty, loading, error, retry, destructive confirmation, and saved-success
  states.

## Required Matrix

Run the full checklist on at least:

- iOS with VoiceOver.
- Android with TalkBack.
- One small viewport around 320px wide.
- One large-font setting at 150% or the nearest OS equivalent.
- Light mode and dark mode.
- Reduced Motion enabled.

Before external beta, include one low-end Android device and one device using a
non-English locale or keyboard.

## Labels And Screen Reader Flow

- Every actionable icon-only control has a useful accessible name, role, and
  state where relevant. Examples: microphone/stop, Process, clear recording,
  edit, delete, retry, notification, account default, visibility toggles, and
  modal close buttons.
- Decorative icons, charts, and visual flourishes are hidden from the screen
  reader or do not interrupt reading order.
- Inputs announce their purpose and error state. Required financial fields
  announce enough context to complete the form without relying on placeholder
  text alone.
- Confirmation modal fields read in a logical order: AI review status,
  clarifications, amount, type, account, mode, category, merchant, date/time,
  notes, then actions.
- Dynamic status messages use polite announcements where appropriate, including
  parse progress, save success, validation errors, retryable network errors, and
  destructive confirmations.
- Screen reader focus moves into modals when opened and returns to the triggering
  context when closed.

## Touch Targets And Gestures

- Primary actions, icon buttons, tab items, filter chips, segmented controls,
  form controls, and destructive actions have at least 44x44 points of tappable
  area.
- Adjacent small controls have enough spacing to avoid accidental taps.
- All actions available through swipe, long-press, or custom gesture are also
  available through a visible tap target.
- Disabled controls are visually distinct and announced as disabled.
- Bottom-sheet and modal actions remain reachable with one hand on small
  screens and do not overlap system gesture areas.

## Font Scaling And Layout

- At 150% font size, no critical text is clipped, overlapped, hidden behind
  fixed-height containers, or truncated without an alternate full label.
- Buttons preserve readable labels or move text to another line. Text must not
  overflow pill, chip, or toolbar containers.
- Forms remain scrollable when the keyboard is open.
- Confirmation and account selection content remains reachable without losing
  the primary action buttons.
- Currency, dates, category labels, and validation errors stay readable in both
  list and detail views.

## Contrast And Color Dependence

- Text and meaningful icons meet WCAG AA contrast against their backgrounds in
  light and dark mode.
- Error, warning, success, and selected states are not communicated by color
  alone; they also use text, icon shape, border, or state copy.
- Disabled text and placeholders remain legible enough to understand the form.
- Charts, category swatches, account colors, and insight cards remain
  distinguishable for users with common color-vision deficiencies.
- Focus, selected, pressed, loading, and error states are visible in both themes.

## Reduced Motion

- With Reduced Motion enabled, pulse, loop, entrance, success, and bottom-sheet
  animations are disabled or shortened enough that the app remains calm and
  usable.
- Voice capture still clearly communicates idle, recording, recording-ready,
  processing, error, and saved states without relying on animation.
- Loading and success states do not flash rapidly.
- Navigating between tabs, opening modals, and submitting forms does not trigger
  large parallax, zoom, or bouncing motion.

## Keyboard And External Input

- Hardware keyboard users can move through fields and actions in a predictable
  order.
- Enter/Return does not accidentally submit incomplete financial forms.
- Escape/back closes modals or returns to the previous screen only when it will
  not discard unsaved data without confirmation.
- Numeric fields use appropriate keyboards and still accept valid decimal input.

## Pass Criteria

- A screen reader user can complete guest onboarding, voice or text parse,
  review, save, dashboard verification, edit, delete, and account management.
- No critical control is unlabeled, unreachable, clipped, contrast-failing, or
  motion-dependent.
- Validation and recovery paths announce what happened and how to proceed.
- The app remains usable at 150% font scale, in dark mode, and with Reduced
  Motion enabled.

## Run Record Template

```text
Date:
Tester:
Mobile commit/build:
Backend commit:
Device and OS:
Screen reader:
Font size:
Theme:
Reduced Motion:

Labels and reading order:
Touch targets:
Font scaling:
Contrast:
Reduced motion:
Keyboard/external input:
Issues filed:
Notes:
```
