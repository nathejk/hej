# 259 — `GET /api/checkpoints`

**Status:** done
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

## Description

PRD 016 phase 2. Serve the caller's patrol's revealed checkpoints (task 255).

- Behind `requireAuth`.
- **Empty list with 200** when nothing is revealed, including for users with no patrol
  (personnel). Deliberately not a 404: unlike `/api/race-area` — where "nothing" precedes a
  few-hundred-megabyte tile download and must be hard to misread — an empty checkpoint list
  is a benign, normal state early in the race.
- Each entry: id, name, checkgroup, sort order, position, and window when known.
- **OpenAPI annotations are mandatory** (repo rule), and must state that only revealed
  checkpoints are ever returned and that positions of others are never exposed — the
  contract is part of the guarantee.
- Use the `…OrNil` adapter pattern (see `raceAreasOrNil`) so a missing projection stays
  checkable: assigning a nil `*Table` to an interface produces a non-nil interface, and the
  first call would panic.
- 503 when the projection is absent (a server-side problem to retry), distinct from an
  empty list (a state of the event).

## Acceptance Criteria

- [x] Handler in `go/cmd/api/checkpoints.go` + route registered.
- [x] OpenAPI annotations incl. the reveal guarantee.
- [x] 200 + empty list for a user with no patrol (test).
- [x] 401 unauthenticated (test).
- [x] 503 with no projection, via the nil-interface trap test every handler here has.
- [x] Response contains no field beyond those listed above.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 2.
- 2026-09-15 — Picked up **before task 258**, deliberately reordering phase 2: 258 is the regression test
  that no un-revealed checkpoint leaves the BFF, and it asserts against response bodies — so the endpoints
  have to exist first. Doing 258 first would mean writing it against nothing, then rewriting it.
- 2026-09-15 — Plan: `revealOrNil` following `raceAreasOrNil`'s nil-interface discipline, the rule wired
  into `data.Models`, and a handler that resolves the caller's patrol from the session exactly as
  `listPatrolScansHandler` does.
- 2026-09-15 — Decision: `data.NewModels` gained **variadic options** rather than a sixth parameter. It
  already takes five sources, three nil-able, and it has seventeen call sites — a sixth positional nil
  would make every one read slightly worse while telling the reader nothing. `data.WithMapReads(...)`
  names what it supplies, and it is the shape the projections already use (`checkpoint.Option`,
  `kort.Option`). No test churn.
- 2026-09-15 — Introduced `data.MapReads` rather than typing the field as `*reveal.Rule`. Both its methods
  take the patrol id, so **there is no "all checkpoints" to ask for** — the facade states what handlers may
  ask rather than what the implementation happens to offer, which is where the reveal guarantee becomes
  unavoidable rather than merely intended.
- 2026-09-15 — Decision: the rule is constructed **only when all five projections exist**, and partial
  construction is refused rather than degraded. A rule missing its handout projection would evaluate to
  "nothing revealed" for every patrol — and the client caches that offline, so it would hold an empty map
  for the rest of the night. An honest 503 is recoverable; a cached empty map is not. Logged with which
  projection was missing, so the cause is in the service log rather than inferred from a blank screen.
- 2026-09-15 — Same reasoning drives the handler's three answers: 503 for "no projection" (a server problem
  to retry), 500 for a read failure, 200 + `[]` for "nothing revealed yet". `TestCheckpoints_ReadFailureIsNotAnEmptyMap`
  exists because collapsing the middle case into the last is the tempting mistake.
- 2026-09-15 — 200 + empty list, deliberately unlike `/api/race-area`'s 404. There, "nothing" precedes a
  few-hundred-megabyte tile download and an empty polygon mistaken for "cache everything" would try to
  cache the country. Here the client simply draws no markers, and an empty map is normal for the first hour.
- 2026-09-15 — Test note: `TestCheckpoints_PatrolComesFromTheSession` passes `?patrol=team-somebody-else`
  and asserts it is ignored. A patrol id taken from the request is the shape of bug that lets one patrol
  read another's map, and it would look entirely reasonable in review.
- 2026-09-15 — Test note: `TestCheckpoints_ResponseCarriesOnlyTheExpectedFields` asserts on the
  **serialised keys**, not the Go struct, so a field added later cannot reach the client unnoticed — and
  this is the endpoint where an extra field is most likely to concern a checkpoint the patrol has not
  earned.
- 2026-09-15 — ✅ All criteria complete. 7 endpoint tests; build, vet, gofmt and the full suite clean.
- 2026-09-15 — Done. Moving to done/; task 258 (the regression test) now has endpoints to assert against.
