# 216 — Surface the dev PIN in the dev panel

**Status:** done
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

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

- [x] The panel fetches `/api/dev/pin` for the number in the login form — picked up
      automatically via an observer on `session.store.requestPin`, with a manual field as well
- [x] Copy-to-clipboard
- [~] Autofill — **deliberately not built**; see the log
- [x] A missing PIN (404) reads as "none issued yet", not as an error
- [x] Absent in production builds along with the rest of the panel
- [x] No member data displayed
- [~] Works for the shared-number `/auth/choose` and switch-profile flows — both go through
      the same `requestPin`, so the mechanism covers them; not exercised end to end

## Depends on

- **Task 208** — the panel.
- **Task 215** — the endpoint.

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 4.
- 2026-09-12 09:47 — Picked up. The number is picked up **automatically**: `session.store.requestPin`
  notifies a registered dev observer *after* the POST resolves (there is no PIN to read until the
  BFF has issued one). Registration, not import, like every other dev seam. A manual field is there
  too, for a number the app did not request.
- 2026-09-12 09:48 — **Autofill deliberately not built.** The task allowed it "if it still routes
  through the input", but doing that from the panel means the panel reaching into the login view's
  internals — coupling a dev overlay to a product component, to save one paste. The PIN is displayed
  in large type with a copy button instead. If autofill is wanted later it belongs in the login view,
  behind `import.meta.env.DEV`, not here.
- 2026-09-12 09:50 — 🚧 Blocked verifying this: the API was not running. Root-caused to the dev
  loop's 10s `go test` gate timing out on a cold cache — **task 220**, fixed and recorded separately.
  The image had to be rebuilt, because `docker/init/api-dev` is baked in rather than bind-mounted.
- 2026-09-12 10:01 — ✅ **Verified end to end against the running stack**, which is the first such
  verification in this PRD. Found a seeded number in the dev database, requested a PIN, and read it
  back:
  * `POST /api/auth/request-pin {"phone":"+4542453977"}` → 200
  * `GET /api/dev/pin?phone=42453977` → `{"pin":"830254"}` — note the un-normalised input works,
    confirming `phone.Normalize` is being applied
  * an unknown number → 404 with the same body as "no PIN outstanding"
  * the response carries the PIN and nothing else
- 2026-09-12 10:02 — A 404 is surfaced as "no pin issued yet" rather than an error, with a comment
  noting that a *missing route* (API not in development mode) looks identical from the client — which
  is worth saying, because that is what a developer would actually be looking at.
- 2026-09-12 10:02 — ✅ Suite 472/472 across 40 files; `vue-tsc` clean.
