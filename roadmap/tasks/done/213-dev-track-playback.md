# 213 — Fake track playback over an event-area polyline

**Status:** done
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

PRD 014, phase 3. Movement, on top of task 212's provider: walk a hard-coded
polyline at a plausible pace so everything that reacts to *changing* position can be
exercised on a laptop — recentring, the track log, distance/progress readouts, and
tile caching as the view pans (PRD 002 §11.2).

A static coordinate exercises none of that, which is why this is a separate task
rather than a flag on 212.

## The route's provenance matters

Use a **hand-picked** polyline near the 2026 event area, committed as source.
Deriving one from a real seeded track would be more realistic but puts a
participant's actual movements into the repository, which the `.rules` privacy
posture argues against — and this is a minor's movement data, the most sensitive
shape it could take. (PRD 014 open question 2; this task settles it.)

Walking pace, roughly 1.4 m/s, interpolated between vertices rather than jumping
vertex to vertex — a teleporting marker exercises different code than a moving one
(`track.store` filters on distance and time deltas).

## Acceptance Criteria

- [x] A committed polyline of hand-picked coordinates — **and deliberately not the event
      area**; see the log
- [x] Interpolated movement at a configurable, walking-pace-by-default speed (1.4 m/s)
- [x] Start / pause / reset
- [x] Loops cleanly: the route returns to its own start, so wrapping emits no discontinuity
- [~] `track.store` records points from it, and the map marker follows — the provider feeds
      both stores (task 214) and movement is unit-tested, but the marker and the recorder are
      unverified in a browser
- [x] No real participant track data is committed

## Depends on

- **Task 212** — the provider this drives.

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 3.
- 2026-09-12 09:38 — **A second constraint appeared that the task had not anticipated, and it
  overrides the task text.** The task said "a hand-picked polyline near the 2026 event area".
  `@/config/map` records that the event area "is not fully known to participants, so we deliberately
  do not reveal it" — which means a dev fixture near the event area would be the thing that commits
  it to the repository, readable by anyone with the source. So the route is a neutral loop anchored
  on the already-public `FALLBACK_CENTER` (Sjælland) instead. The privacy answer to open question 2
  stands; this is a second, independent reason with a different subject.
- 2026-09-12 09:39 — Progress is tracked as **distance travelled**, not as a start timestamp, so
  pausing cannot silently advance the walk. Advancing uses wall-clock delta rather than a per-tick
  increment, so the pace does not depend on how often the position is sampled — that kind of
  coupling is how a simulation starts lying.
- 2026-09-12 09:39 — Interpolated between vertices, because `track.store` filters on distance and
  time deltas: a marker that teleports exercises different code than one that moves.
- 2026-09-12 09:40 — The route is a closed loop, so wrapping past the end needs no special case and
  produces no jump. A discontinuity there would look exactly like a GPS glitch and be recorded as
  one.
- 2026-09-12 09:41 — ✅ Movement and pause-does-not-advance are covered in
  `devGeolocation.spec.ts`; full suite green.
