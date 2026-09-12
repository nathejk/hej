# 216 — Surface the dev PIN in the dev panel

**Status:** open
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 014, phase 4. The client half of laptop login: show the PIN from task 215's
endpoint in the dev panel, for the number currently being logged in.

## Do not skip the input

Autofill is acceptable, but the PIN must still go **through** the OTP input rather
than being posted around it. The input's own behaviour — paste, autofill,
validation, the resend timer — is under test too, and a dev shortcut that bypasses it
would leave the one part of login that most often breaks unexercised.

## Privacy boundary

The phone number shown here is the one the developer just typed. That is the **only**
personal datum the panel may ever display (PRD 014 §6, task 208): no name, no role,
and never a guardian number.

## Acceptance Criteria

- [ ] The panel fetches `/api/dev/pin` for the number in the login form
- [ ] Copy-to-clipboard, and optionally autofill that still routes through the input
- [ ] A missing PIN (404) reads as "none issued yet", not as an error
- [ ] Absent in production builds along with the rest of the panel
- [ ] No member data displayed
- [ ] Works for the shared-number `/auth/choose` and switch-profile flows

## Depends on

- **Task 208** — the panel.
- **Task 215** — the endpoint.

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 4.
