# 240 — Frontend: vehicles store and API client

**Status:** open
**Priority:** medium
**Created:** 2026-09-14
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `vehicles.store.ts` with `load`, `register`, `update`, `remove`
- [ ] No optimistic insertion; the list reflects what the server confirmed
- [ ] A failed write leaves a retryable error in state and loses no form input
- [ ] `409` is distinguishable from a generic failure by the caller
- [ ] Nothing persisted to localStorage or IndexedDB
- [ ] Cleared on sign-out and on profile switch
- [ ] Unit tests: load, empty list, register success, `409`, offline failure, remove, and
      that a profile switch empties it
- [ ] `npm run type-check` and `npm run test:unit` green

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
