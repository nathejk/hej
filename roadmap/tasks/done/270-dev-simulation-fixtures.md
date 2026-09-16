# 270 — Dev-simulation fixtures for every verdict and reveal state

**Status:** done
**Priority:** low
**Created:** 2026-09-15
**Completed:** 2026-09-16

## Description

PRD 016 phase 4, using PRD 014's device-simulation machinery. Most of this feature's states
are unreachable in development without a race happening: a relative window with an anchoring
scan, a sheet reassigned to a successor team, a skitse revealed at a post, a scan nobody can
attribute.

Seed the `internal/scans` mock and the dev fixtures so every state can be looked at on a
device:

**Verdicts:** on time; late with a delta; early; `fixed` window; `relative` window with an
anchor; `relative` without an anchor (no verdict); `none` (no verdict); unattributable scan
(no checkpoint at all).

**Reveal states:** revealed by QR-bound sheet; revealed by a sheet's handout checkgroup;
revealed by having scanned a checkgroup; a positioned checkpoint revealed by nothing (must
never appear); a revealed checkpoint with no position (listed nowhere on the map); a sheet
with an unknown `mapId`; a reassigned sheet whose checkpoints stay revealed.

**Handouts:** QR-bound; synthesised skitse; "afleveret".

This is also the fixture set the reveal regression test (task 258) and the arrow tests
(task 263) want, so build it where both can reach it.

## Acceptance Criteria

- [x] Every verdict state reachable in dev.
- [x] Every reveal state reachable in dev, including the must-never-appear one.
- [x] Fixtures shared with tasks 258 and 263 rather than duplicated.
- [x] Documented in the dev simulation notes so the states are discoverable.
- [x] Fixtures cannot leak into production builds.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 4.
- 2026-09-16 — Added `go/internal/mapfixture`: one coherent fixture world (central Jutland, matching
  `scans.NewMockSource`'s checkpoint ids so scanned posts show as visited) plus fakes for the five
  projections, with `NewRule()` running the **real** `reveal.Rule` over them. Faking only the data matters:
  `cp-secret` is absent from the dev map because the rule filters it, not because a mock declined to
  mention it. States covered: revealed by QR-bound sheet, by a skitse's handout checkgroup, by a scanned
  checkgroup, by a sheet since reassigned away (revealing is monotonic — `heldSheetIDs` ignores `Current`,
  confirmed in the source); a revealed-but-unsited post (filtered, drawn nowhere); an unknown `mapId`;
  and a positioned post revealed by nothing. Checkgroup schemes span fixed / relative-with-anchor / none.
- 2026-09-16 — Wired as the dev fallback via `mapReadsFor` (`cmd/api/mapsource.go`), which has **three**
  outcomes rather than two: real rule when it built; fixtures when there is no eventing at all (the
  supported no-database mode); and untyped nil — an honest 503 — when a broker *is* configured but a
  projection failed. That third branch is the important one: serving a plausible-looking fixture map to a
  misconfigured production deployment would be worse than serving nothing. Chosen by absence of eventing,
  never by a flag, mirroring `scanSourceFor`.
- 2026-09-16 — Made the fixture genuinely *shared* rather than duplicated: `revealboundary_test.go`
  (task 258) now builds its app from `mapfixture.NewRule()` and hunts `mapfixture.SecretLat/SecretLng`,
  deleting ~90 lines of parallel fixture definitions. One world in one place — two copies would drift, and
  the copy that drifted would be the one guarding the reveal guarantee. Extended the `internal/scans` mock
  with the one verdict state it lacked: an unattributable registration (no checkpoint, no verdict).
- 2026-09-16 — Documented in `README.md` under "The map fixture world (PRD 016)", as a table mapping each
  fixture row to the state it exists to demonstrate, so the states are discoverable rather than folklore.
  `internal/mapfixture/fixture_test.go` asserts the fixture keeps its promises (secret never revealed,
  every trigger fires, next line is cg-3, another patrol sees nothing, all four handout states present,
  every scheme represented, bounded checkpoint reads, coordinates inside the default view) — a fixture
  nobody checks drifts silently while still looking plausible. All Go gates green, incl. `GOWORK=off`.
