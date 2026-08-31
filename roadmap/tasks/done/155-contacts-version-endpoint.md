# 155 — `GET /api/contacts/version` (freshness poll)

**Status:** done
**Priority:** high
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

## Description

Backs the during-event freshness requirement (PRD 007 §6, §8 "Keeping the directory
fresh"): directory changes must be visible immediately on foreground and within ~60 s
while the app is open.

Returns a **monotonic version for the caller's permitted set and nothing else**. The
client polls this and only fetches the manifest delta when the version differs.

Why a separate endpoint rather than polling the manifest: this is called by a few
hundred devices every 60 s while the app is open, and it is the app's first
**continuous** during-race traffic — landing on the same BFF as PRD 002's position
reporting. It must be trivially cheap: a small integer or hash, `ETag`-able, answered
from a projection read rather than any recomputation.

Push is **not** an option for invalidation: iOS 16.4+ web push requires every push to
produce a user-visible notification (see `vue/public/push-sw.js`, which always calls
`showNotification`), so it would either buzz phones over a corrected phone number or
risk the permission. Documented in PRD 007 §8.

## Implementation

`go/cmd/api/contacts.go` (handler + `versionCache`), `contactsversion_test.go`, route in
`routes.go`, cache constructed in `main.go`.

**This task exposed a design flaw in task 154 and fixed it.** The manifest's version was a
hash of the *rendered entries*, which include `IsOwn` — a presentation flag that differs for
every viewer. Two consequences, both bad, neither obvious until something had to poll it:

- the version was effectively unique per person, so it could never be cached, and this
  endpoint would have meant one query per device per minute;
- two members of the same klan holding identical data would report different versions.

The version now hashes the **data** (`contactsVersion([]person.Person)`), so viewers sharing
a permitted role set share an answer — three or four distinct sets for the whole event.
The manifest and this endpoint call the same function, and a test asserts they agree; if
they ever disagree the client either refetches forever or never.

**A 5-second TTL cache keyed by permitted role set.** That is what makes the poll
affordable: a few hundred devices collapse to a handful of queries a minute. Staleness is
bounded at ~5 s on top of the client's own interval, comfortably inside "without too much
delay". The cache is nil-safe and degrades to computing every time, and its clock is
injectable so expiry is tested without sleeping. Hand-rolled rather than a dependency: the
key space is bounded by the access matrix, so there is nothing to evict.

Also `Cache-Control: private, max-age=10` as a second line of defence — a client polling far
more often than agreed is absorbed by its own browser cache before reaching us.

**`TestContactsVersion_CoversEveryExposedField`** is the test I would keep if I could keep
only one here: it mutates each field the manifest can expose and asserts the version moves.
The failure it guards against — an edit that never reaches devices — looks like "the app is
fine" right up until someone's corrected number is missing during the race.

Also made `stubPeople.ListByAppRoles` filter by role, as real SQL does. The stub previously
ignored the filter, which made per-permitted-set behaviour untestable and initially masked
this whole issue.

## Acceptance Criteria

- [x] `GET /api/contacts/version` behind `app.requireAuth`, returning a version scoped
      to the caller's permitted set.
- [x] Version changes when anything in that set changes — including a withdrawal, a
      number purge, or a new portrait (one subtest per exposed field).
- [x] Version does **not** change when something outside the caller's set changes —
      covered by the permitted-set keying test; a bandit and crew get distinct versions.
- [x] `ETag` / `304` supported.
- [x] Answered from a projection read; no scan of the whole person table per request —
      and, with the cache, not even a query per request.
- [x] Spejder gets `403`; unauthenticated `401`.
- [x] OpenAPI annotations present.

## Progress Log

- 2026-08-31 16:10 — Picked up. Plan: reuse the manifest's version function, add a TTL
  cache so the poll is affordable.
- 2026-08-31 16:25 — Blocker, and a real one: the version hashed rendered entries including
  `IsOwn`, making it per-viewer and therefore uncacheable. Refactored `contactsVersion` to
  hash the underlying rows instead. This corrects code committed in task 154.
- 2026-08-31 16:40 — Added the TTL cache keyed by permitted role set, with an injectable
  clock so expiry is testable without sleeping.
- 2026-08-31 16:50 — `TestContactsVersion_VersionIsPerViewer` (from 154) failed after the
  refactor because `stubPeople` ignored the role filter, so both viewers hashed identical
  data. Made the stub filter by role like the real query.
- 2026-08-31 17:00 — Two cache tests failed: `newTestApp` leaves `contactsVersions` nil, so
  caching was off. Wired it in `contactsTestApp` with production's TTL.
- 2026-08-31 17:10 — ✅ All criteria met. `go build ./...`, `go vet ./...`, `go test ./...`
  pass; `gofmt` clean.
