# 007 — Frontend `LoginView` (phone + PIN)

**Status:** open
**Priority:** high
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

## Description

Build the two-step login screen, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. Step 1: phone-number entry
→ calls `POST /api/auth/request-pin`. Step 2: PIN entry → calls
`POST /api/auth/verify`. The phone step **always** advances to the PIN step
regardless of recognition (anti-enumeration).

The PIN screen shows reassurance copy: *"If we know you, we have sent you an
SMS. If you don't receive an SMS and you feel we should know you, please reach
out."* — where "reach out" is a `tel:` link to the **nødtelefon** (number TBD).

Depends on: 001, 004, 005 (can stub API while backend lands).

## Acceptance Criteria

- [ ] `src/views/LoginView.vue` with phone step and PIN step, branded
      "Hej Nathejk".
- [ ] PIN input uses `inputmode="numeric"` + `autocomplete="one-time-code"`.
- [ ] Resend control with 60s cooldown state; clear error states (wrong/expired
      PIN, lockout).
- [ ] Reassurance copy present with a `tel:` nødtelefon link (number as a config
      placeholder until provided).
- [ ] No password field anywhere.
- [ ] API calls go through `@/helpers` `fetchWrapper` (not bare fetch).

## Progress Log

- 2026-07-30 13:12 — Task created.
