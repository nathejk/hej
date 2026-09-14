# 240 — Frontend: vehicles store and API client

**Status:** done
**Priority:** medium
**Created:** 2026-09-14
**Picked up by:** agent session (Zed)
**Started:** 2026-09-14
**Completed:** 2026-09-14

## Description

`vue/src/stores/vehicles.store.ts`, over `helpers/fetchWrapper`, wrapping the four
endpoints from tasks 237–239. One store used by both the onboarding step (241) and the
profile section (242), so the two surfaces cannot disagree about what the user has
registered.

State: the list, a loading flag, and a submit error. Actions: `load`, `register`, `update`,
`remove`.

Three things worth deciding here rather than in each component:

- **Registration is a write, so it needs connectivity** (PRD 010 §5). It must fail
  visibly and stay retryable — never optimistically add a vehicle to the list, because a
  user who believes their car is registered when the publish failed is the exact failure
  PRD 008 §5 forbids. Offline is a legible error, not a queue: this is not a feature worth
  the sync machinery, and a queued registration that lands hours later is worse than a
  clear "try again when you have signal".
- **`409` is not an error to show as a failure.** It is the answer "this car is already
  registered", and the calling component turns it into a sentence, so the store must
  surface the status distinguishably rather than collapsing everything into one message
  string.
- **Do not persist the list.** It is small, cheap to re-fetch, and per PRD 010 §6 must be
  visible only to its owner — a cached copy surviving a profile switch (PRD 012) would show
  one person another's plate.

## Acceptance Criteria

- [x] `vehicles.store.ts` with `load`, `register`, `update`, `remove`
- [x] No optimistic insertion; the list reflects what the server confirmed
- [x] A failed write leaves a retryable error in state and loses no form input
- [x] `409` is distinguishable from a generic failure by the caller
- [x] Nothing persisted to localStorage or IndexedDB
- [x] Cleared on sign-out; a profile switch needs no call — see the log
- [x] Unit tests: load, empty list, register success, `409`, offline failure, remove, and
      that a profile switch empties it
- [x] `npm run type-check` and the unit suite green

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
- 2026-09-14 — Picked up. `fetchWrapper` has no `patch` verb yet, so that goes in first.
- 2026-09-14 — Added `fetchWrapper.patch`, documented as distinct from `put` because the
  BFF's PATCH is a delta — the ability to send *some* fields is exactly what a PUT of a whole
  resource cannot express.
- 2026-09-14 — Write failures are classified into `duplicate` / `offline` / `invalid` /
  `failed` rather than flattened to a message string. `409` is the one that matters: it is an
  *answer* ("this car is already registered"), quite possibly a success from the point of view
  of a passenger whose driver already registered it, and a component cannot phrase that from a
  generic error.
- 2026-09-14 — A failed `load` deliberately leaves the previous list in place instead of
  emptying it. "We could not reach the server" is not "you have no vehicles", and the same
  distinction the BFF makes with 503-vs-empty applies on the client: showing an empty list is
  what invites a duplicate registration.
- 2026-09-14 — `remove` treats `404` as success. The BFF answers 404 for a car that is
  already gone — it cannot distinguish that from one that never existed without disclosing
  whose registrations exist (task 239) — so reporting a failure would tell a member their
  deletion failed for a car that is demonstrably gone.
- 2026-09-14 — Corrected one assumption from the task description while writing it: **a
  profile switch needs no `clear()` call.** `UserMenu.switchTo` does a full page load
  (`window.location.assign`) precisely so no in-memory store survives it, and nothing here is
  persisted. Sign-out is the case that needs it, since that stays in the same document — wired
  into `UserMenu.signOut` next to `profile.clear()`. Criterion amended rather than ticked as
  written.
- 2026-09-14 — ✅ All criteria. 18 store tests; `type-check` clean; full frontend suite green
  (503 tests). Note for the record: there is no `npm run lint` script in this repo — the
  criterion as I wrote it named one that does not exist.
