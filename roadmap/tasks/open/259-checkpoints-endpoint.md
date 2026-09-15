# 259 — `GET /api/checkpoints`

**Status:** open
**Priority:** high
**Created:** 2026-09-15

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

- [ ] Handler in `go/cmd/api/checkpoints.go` + route registered.
- [ ] OpenAPI annotations incl. the reveal guarantee.
- [ ] 200 + empty list for a user with no patrol (test).
- [ ] 401 unauthenticated (test).
- [ ] 503 with no projection, via the nil-interface trap test every handler here has.
- [ ] Response contains no field beyond those listed above.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 2.
