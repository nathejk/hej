# 269 — Version derivations for handouts, checkpoints and scans

**Status:** open
**Priority:** medium
**Created:** 2026-09-15

## Description

PRD 016 phase 4, feeding PRD 017. Each of this feature's three datasets needs a cheap
`Version(viewer)` so the foreground sync check can ask "did this change?" without
downloading it.

Follow `contactsVersionFor` in `go/cmd/api/contacts.go`, which is the reference:

- a **short opaque string**, scoped to the caller's permitted set;
- answered from a **projection read or a short-lived cache** — never by building the payload
  and hashing it;
- cached keyed by *permitted set* rather than by user, so devices sharing a role share the
  work.

Two traps, both already documented in `contacts.go`:

1. **A wrongly-unstable version is expensive.** The contacts expiry is deliberately *not*
   part of its hash, because if it were, every request would produce a new version and every
   device would refetch everything every minute. Nothing time-varying may enter these hashes.
2. **A wrongly-stable version is silent** — a device that quietly never updates. So each
   version needs a test that it **changes** when its data changes, not merely that it is
   stable.

Datasets: `handouts`, `checkpoints` (per patrol, since reveal is per patrol), `scans`.

## Acceptance Criteria

- [ ] `Version(viewer)` for each of the three datasets.
- [ ] Each is a projection read or cached; none builds a payload.
- [ ] Cache keyed by permitted set / patrol, not per user.
- [ ] Test per dataset that the version **changes** when the underlying data changes.
- [ ] Test per dataset that it is stable across repeated calls with unchanged data.
- [ ] Nothing time-varying in any hash.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 4, consumed by PRD 017.
