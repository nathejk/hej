# 283 — Version derivations for profile and race area

**Status:** doing
**Priority:** high
**Created:** 2026-09-16
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-16
**Completed:**

## Description

PRD 017 phase 1. Task 269 landed `go/cmd/api/mapversion.go` with cached, per-patrol
versions for checkpoints, handouts and scans, and `contactsVersionFor` has existed
since the contacts pane. That leaves two of the PRD's six datasets without one:

- **profile** — the caller's own record. Scoped to the user, not a shared set, so
  unlike the others its cache key is genuinely the user id.
- **race area** — the event's boundary/tile area. Shared by everyone in the event, so
  one cache entry serves every device; key it on the event year.

Follow `mapversion.go`'s header, which documents the rules and the two traps:

1. **A wrongly-unstable version is expensive** — nothing time-varying in the hash. No
   expiry, no `now`, no per-request presentation flag. One that changes per request
   makes every device refetch every payload every interval.
2. **A wrongly-stable version is silent** — a device that quietly never updates, the
   hardest failure to notice during an event. So each hash must cover every field the
   corresponding payload can expose, and needs a test that it **changes** when its data
   changes, not merely that it is stable.

Answer from a projection read or a short-lived cache. Never build the payload and hash
it.

## Acceptance Criteria

- [ ] `profileVersionFor(viewer)` — covers every field `/api/profile` exposes,
      including the portrait version and confirmation state.
- [ ] `raceAreaVersionFor(viewer)` — covers every field the race-area payload exposes.
- [ ] Both answered from a projection read or a `versionCache`; neither builds a
      payload.
- [ ] Profile cache keyed by user; race area keyed by event year (shared).
- [ ] Nothing time-varying in either hash.
- [ ] Per dataset: a test that the version **changes** when the data changes.
- [ ] Per dataset: a test that it is **stable** across repeated calls, unchanged data.
- [ ] A test that the race-area version is shared across two different viewers.
- [ ] All four Go gates green.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1. Sibling of task 269, which did
  the other three.
- 2026-09-16 11:10 — Picked up. Plan: new `syncversion.go` next to `mapversion.go`.
  Profile hashes the `users.User` fields `/me/profile` exposes plus the `person.Person`
  facts the derived flags read (`PortraitRef`, `PhoneParent`, verification, member
  status) — one `Users.Get` and one `People.Get`, both single-row projection reads.
  Race area hashes the hull the payload carries and is keyed on the event year alone,
  since every device in the event shares it.
