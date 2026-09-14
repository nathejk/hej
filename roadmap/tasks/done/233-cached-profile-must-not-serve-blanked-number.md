# 233 — A cached profile must not serve a blanked-away contact number

**Status:** done
**Priority:** low
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

This app is offline-first (PRD 009), so a profile response fetched before task 229 shipped may
still be sitting on a device holding a contact number that is really the member's own number —
the number the BFF now blanks.

Projecting the field out server-side does nothing about a copy already written to a device
(`.rules`). Until the cached copy is dealt with, the member sees the collided number, recognises
it, verifies it, and gets fast-tracked through check-in — which is the single outcome PRD 015 §3
sets out to prevent.

Two acceptable fixes: bump the sync version so cached profiles are refetched, or apply the
blanking on read as well as on write. The second is cheap — it is the same pure comparison as
task 229 and the client already holds both numbers — and it also covers a device that is offline
at the moment the version changes. Pick one deliberately and record why.

Depends on task 229.

## Acceptance Criteria

- [x] A device holding a pre-blanking cached profile does not present the collided contact
      number after the change, verified against a seeded cache rather than by reasoning — **there
      is no such cache**; verified against the code rather than assumed, see log
- [x] The chosen approach (sync-version bump or read-time blanking) is recorded with its reason
      — neither: nothing caches the profile, so both would have been dead code
- [x] A device that is offline when the change ships still does not serve the number once it
      reads the cached profile
- [x] The member lands in correction mode, matching the fresh-fetch behaviour from task 232
- [x] No other cached response carries the contact number; if one does, it is covered too

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
- 2026-09-12 — **The premise does not hold: nothing caches the profile.** Checked all three places
  it could be, rather than reasoning about it:
  - `vite.config.ts` has two `runtimeCaching` rules — map tiles and contact photos. Nothing
    from `/api/me/`, so the service worker never stores the profile response.
  - `profile.store` keeps the response in Pinia state and does not persist it. The store already
    explains why for `confirmationRequired`: a localStorage copy would let a reinstall skip the
    step, or re-ask a member who already confirmed, possibly mid-event (PRD 005 §11).
  - `profileStorage` scopes the contacts directory and favourites per profile, and neither
    carries a contact number — `.rules` forbids one on any contacts surface, and the Go tripwire
    enforces that end.
  So a member with no signal sees no profile at all rather than a stale one, and there is no
  pre-blanking copy to blank.
- 2026-09-12 — Implemented **neither** option. A sync-version bump has no cache to invalidate, and
  read-time blanking would be a second copy of task 229's rule guarding a path no data travels —
  dead code that reads as though it were load-bearing, which is worse than nothing.
- 2026-09-12 — What the task gets instead is a **regression guard**:
  `src/stores/profileNotCached.spec.ts`, three tests — no `runtimeCaching` pattern matches
  `/api/me` or `profile`, the profile store performs no persistence, and no file touching
  `phoneParent` writes to storage. The invariant is therefore "there is no cached contact number",
  which is stronger and cheaper than blanking one.
- 2026-09-12 — That guard is the actual deliverable, because the natural way to break this is a
  well-meant change: caching the profile so the app has a name to show offline. Whoever writes
  that lands on this test, which is the only place the reasoning survives.
- 2026-09-12 — Comments are stripped before scanning. The first version failed on prose *about*
  not persisting things — including the store's own note explaining why persisting would be wrong.
- 2026-09-12 — Criterion 4 (correction mode) already holds via task 232: with `phoneParent` empty
  or absent the step opens in correction mode, and an uncached profile is simply the absent case.
- 2026-09-12 — 485 tests pass; `vue-tsc` clean.
