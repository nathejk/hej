# 258 — Regression test: an un-revealed checkpoint never leaves the BFF

**Status:** done
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

## Description

PRD 016 phase 2. The top risk of this whole feature (PRD 016 §8): widening the checkpoint
projection puts the positions of *un-revealed* posts one struct field away from a response.
The event area is deliberately not fully known to participants (PRD 002), and the race-area
hull exists precisely so that positions never have to leave.

The type-level boundary (task 255) is the primary defence. This task is the test that the
boundary holds, and it must be the kind of test that fails when someone adds a field in six
months rather than only when someone rewrites the query.

Scenario worth building: a year with checkpoints in three checkgroups, a patrol holding one
sheet covering two of them, and at least one positioned checkpoint that no rule reveals.
Then assert that its coordinates appear in **no** response body — `/api/checkpoints`,
`/api/patrol/handouts`, `/api/patrol/scans`, and `/api/race-area`'s polygon is a hull, not a
point list.

Also assert the guardian rule while we are here: no `phoneParent` in any of these payloads
(repo rule).

## Acceptance Criteria

- [x] Test asserts an un-revealed checkpoint's latitude/longitude appear in no response.
- [x] Test scans the serialised JSON, not just the Go structs, so a field added later is
      caught.
- [x] Test covers all endpoints this PRD touches.
- [x] Test asserts no `phoneParent` in any of them — except `/api/me/profile`, the one
      sanctioned surface, which is now excluded **by name** and pinned by its own test.
- [x] A deliberately broken reveal rule makes the test fail (verified by hand — 8 leak
      assertions fired across 3 endpoints).

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 2.
- 2026-09-15 — Done after task 259, since the assertions are against response bodies and the endpoints had
  to exist first. Landed as `go/cmd/api/revealboundary_test.go`.
- 2026-09-15 — Decision: the test wires the **real** `reveal.Rule` over fake projections and drives the
  **real** HTTP handlers. Only the data is faked. A fake rule would have tested the handler's plumbing and
  nothing about the guarantee — which is the way this kind of test usually ends up worthless.
- 2026-09-15 — The fixture world contains `cp-secret`: positioned, real, in its own checkgroup, and listed
  on a sheet the patrol has **never been handed**. That last detail is the point — the sheet exists and
  names the checkpoint, so the rule has to decline to follow it rather than simply not find it.
- 2026-09-15 — Greps the serialised JSON rather than inspecting structs, and looks for **truncated**
  coordinates as well as exact ones. A field added to a Go struct in six months would still serialise, and a
  reformatted or rounded coordinate is just as much of a leak.
- 2026-09-15 — Added the positive half (`TestRevealedCheckpointsDoReachThePatrol`). Without it the secrecy
  assertion would be satisfied by an endpoint that returns nothing at all — passing while the feature was
  broken.
- 2026-09-15 — Added `TestRaceAreaIsAHullNotAPointList`, which tests a claim PRD 002 makes rather than
  assumes: the race area is derived from *every* checkpoint including the secret one, and is published
  anyway because a convex hull plus a 3 km buffer is not a position. If the buffer were dropped or the hull
  replaced by a point list, the main test would start failing on `/api/race-area` and this one explains why.
- 2026-09-15 — **The guardian assertion failed, and the code was right.** Written the obvious way — every
  surface — it failed on `/api/me/profile`, which carries `phone_parent` by design: it is the single
  sanctioned exception in `.rules`, a user confirming *their own* guardian's number (PRD 003/005). Excluded
  it **by name with the reasoning**, rather than silently, so the next reader can tell a sanctioned
  exception from a gap in the test — and added `TestProfileIsTheOnlyGuardianPhoneSurface` so that if the
  profile ever stops carrying it, that is a deliberate change to the PRDs rather than something this file
  quietly permitted.
- 2026-09-15 — **Verified the test can fail.** Temporarily sabotaged `revealsFor` to reveal every sheet
  regardless of possession: 8 leak assertions fired across `/api/checkpoints`, `/api/patrol/scans` and
  `/api/race-area`. Restored, suite green. A secrecy test nobody has seen fail is a decoration.
- 2026-09-15 — ✅ All criteria complete. Done.
