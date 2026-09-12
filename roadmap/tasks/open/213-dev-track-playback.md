# 213 — Fake track playback over an event-area polyline

**Status:** open
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] A committed polyline of hand-picked coordinates in the event area
- [ ] Interpolated movement at a configurable, walking-pace-by-default speed
- [ ] Start / pause / reset
- [ ] Loops or stops cleanly at the end, without emitting a discontinuity that looks
      like a GPS jump
- [ ] `track.store` records points from it, and the map marker follows
- [ ] No real participant track data is committed

## Depends on

- **Task 212** — the provider this drives.

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 3.
