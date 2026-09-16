# 269 — Version derivations for handouts, checkpoints and scans

**Status:** done
**Priority:** medium
**Created:** 2026-09-15
**Completed:** 2026-09-16

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

- [x] `Version(viewer)` for each of the three datasets.
- [x] Each is a projection read or cached; none builds a payload.
- [x] Cache keyed by permitted set / patrol, not per user.
- [x] Test per dataset that the version **changes** when the underlying data changes.
- [x] Test per dataset that it is stable across repeated calls with unchanged data.
- [x] Nothing time-varying in any hash.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 4, consumed by PRD 017.
- 2026-09-16 — Added `cmd/api/mapversion.go`: `checkpointsVersionFor`/`handoutsVersionFor`/`scansVersionFor`
  on `application`, each following `contactsVersionFor`. Versions are sha256 over the *projection rows*
  (never the rendered payload), truncated to 16 bytes hex. checkpoints hashes every field
  `/api/checkpoints` exposes plus `next_checkgroup`; handouts hashes sheet/name/format/qr/handedOut/stillHeld;
  scans hashes id/kind/label/checkpointId/lat/lng/scannedAt plus the verdict (a scan gaining a verdict is a
  change). Floats hashed via `FormatFloat(-1)`, nil position a distinct sentinel. Nothing time-varying is
  hashed — no expiry, no now, no per-viewer flag — which is why `HasStarted` (`MemberStatus != ""`, an event
  fact) is safe to fold into the checkpoints key. Three new `*versionCache` fields on `application`
  (5 s TTL, like contacts), keyed by patrol (checkpoints also by started-state). Tests in
  `mapversion_test.go`: per-dataset change-and-stable on the pure hashes, plus `*For` tests proving the
  cache is shared per patrol not per user and that a data change propagates with caching off. All four Go
  gates green.
