# 283 — Version derivations for profile and race area

**Status:** done
**Priority:** high
**Created:** 2026-09-16
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-16
**Completed:** 2026-09-16

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

- [x] `profileVersionFor(viewer)` — covers every field `/api/profile` exposes,
      including the portrait version and confirmation state.
- [x] `raceAreaVersionFor(viewer)` — covers every field the race-area payload exposes.
- [x] Both answered from a projection read or a `versionCache`; neither builds a
      payload.
- [x] Profile cache keyed by user; race area keyed by event year (shared).
- [x] Nothing time-varying in either hash.
- [x] Per dataset: a test that the version **changes** when the data changes.
- [x] Per dataset: a test that it is **stable** across repeated calls, unchanged data.
- [x] A test that the race-area version is shared across two different viewers.
- [x] All four Go gates green.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1. Sibling of task 269, which did
  the other three.
- 2026-09-16 11:10 — Picked up. Plan: new `syncversion.go` next to `mapversion.go`.
  Profile hashes the `users.User` fields `/me/profile` exposes plus the `person.Person`
  facts the derived flags read (`PortraitRef`, `PhoneParent`, verification, member
  status) — one `Users.Get` and one `People.Get`, both single-row projection reads.
  Race area hashes the hull the payload carries and is keyed on the event year alone,
  since every device in the event shares it.
- 2026-09-16 11:30 — **Decision: profile hashes its inputs, not its rendered response.**
  Five of the payload's fields are derived (`has_photo`, `phone_parent`,
  `confirmation_required`, `verified_at`, `contact_settled`) and each is a *pure function*
  of fields that are hashed, so covering the inputs covers the outputs — while keeping
  this out of the business of rebuilding the response, which is the one thing a version
  derivation must never do. The mapping input→field is written into the doc comment so
  the next person adding a profile field knows what to add here.
- 2026-09-16 11:35 — **The guardian number is deliberately in the profile hash.**
  `.rules` keeps `phoneParent` out of every payload except this one — a member
  confirming their *own* guardian's number — so it is legitimately part of this payload
  and therefore has to be part of this version. Omitting it would mean a corrected
  guardian number never reaching the member it belongs to: the silent failure PRD 017
  exists to prevent, on the most consequential field in the record. Noted in the comment
  that the rule forbids the number *leaving*, and a hash is not a way out of the server.
- 2026-09-16 11:40 — Two nil-distinctness helpers, both load-bearing rather than tidy:
  `hashStringPtr` keeps nil ("this population has no contact number") apart from ""
  ("one is expected and missing"), which the profile page renders differently and
  `confirmationRequired` branches on; `hashTime` hashes UTC unix seconds so a projection
  replayed in another zone does not mint versions for untouched data. Both have tests.
- 2026-09-16 11:45 — **Decision: an unavailable race-area projection is not cached.**
  It answers like "no area" (the client holds nothing either way) but is deliberately
  left out of the cache, so freshness resumes the instant the projection does instead of
  being pinned to a stale outage for the TTL. A read *error* propagates rather than
  being hashed — a plausible-looking version meaning "we could not tell" is worse than
  an error, because the client would store it as fact. Both have tests.
- 2026-09-16 11:50 — Completed. `go/cmd/api/syncversion.go` + `syncversion_test.go`
  (13 tests, incl. 20 table-driven field cases); two new `versionCache` fields on
  `application` at the same 5 s TTL as the others. All four Go gates green:
  `go test ./...`, `go vet`, `go tool staticcheck`, `go build`.
