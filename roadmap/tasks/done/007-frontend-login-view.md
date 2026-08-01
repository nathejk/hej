# 007 — Frontend `LoginView` (phone + PIN)

**Status:** done
**Priority:** high
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

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

- [x] `src/views/LoginView.vue` with phone step and PIN step, branded
      "Hej Nathejk" (Lucide `KeyRound` mark).
- [x] PIN input uses `inputmode="numeric"` + `autocomplete="one-time-code"`
      (maxlength 6).
- [x] Resend control with 60s cooldown countdown + "Skift nummer" (change
      number); clear error states (wrong PIN → 401 copy, lockout → 429 copy).
- [x] Reassurance copy present with a `tel:` nødtelefon link (number from
      `@/config/contact` placeholder until the real number is provided).
- [x] No password field anywhere.
- [x] API calls go through the session store (which uses `@/helpers`
      `fetchWrapper`), not bare fetch.

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 19:00 — Productionized `LoginView.vue` (task 008 had shipped a minimal version). Added: 60-second resend cooldown with live countdown, "Skift nummer" back-to-phone, Lucide icons (`KeyRound`/`Phone`/`ArrowLeft`), Danish copy, safe-area padding, 401/429-specific error messages, and the nødtelefon `tel:` link. Added `@/config/contact.ts` for the nødtelefon number (placeholder — real number is a PRD open item).
- 2026-07-30 19:01 — ✅ Verified in `node:20-alpine`: `npm run build` (LoginView code-split, ~5 kB) and `npm run type-check` clean.
- 2026-07-30 19:01 — Completed. Note: nødtelefon number is a placeholder (`+4500000000`) pending the real value.
