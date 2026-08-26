# 088 — Derive the race area from checkpoints and serve it to the client

**Status:** open
**Priority:** medium
**Created:** 2026-08-26
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 002 §11.2. The tile cache is scoped to the race area, which is defined as **the convex
hull of this year's checkpoints plus a 3 km buffer** (every checkpoint is inside the race
area, by definition). The client cannot cache that area without being told what it is, so
this task supplies it.

Blocks task 087.

## Why this cannot be a constant

The obvious shortcut is to paste today's bounding box into `map.ts`. It would be wrong: the
checkpoint set is **still being edited** — the most recent `checkgroups.sorted` event is 16
days old — and the area moves from year to year. A hardcoded area silently under-caches, and
the symptom appears at 02:00 in a forest with no coverage.

What *is* stable is the area's **size** — "roughly the same every year" (maintainer) — so the
storage budget is a one-time decision even though the polygon is not. This task supplies the
*where*; PRD 009's budget is settled from the *how big*.

So the area is derived from live data, and re-derived whenever it is asked for.

## Precision required: low

Worth stating so this does not get over-built. The output feeds a tile cache with a 3 km
buffer already baked in; being a kilometre out changes the download by a few percent and
changes nothing a participant would notice. A convex hull over whatever checkpoints have
coordinates is sufficient. It does not need geodesic exactness, a concave hull, or the
missing checkpoints filled in.

So the area is derived from live data, and re-derived whenever it is asked for.

## The data

`NATHEJK.<year>.checkpoint.<id>.updated` carries what is needed:

```json
{"checkpointId":"...","name":"Post 4A","position":{"latitude":55.716595,"longitude":12.264819}}
```

The matching `.created` events carry no name or position, so `.updated` is the one to
project. As of 2026-08-26 there are 12 checkpoints for 2026, of which **9 have coordinates** —
Post 3A, Post 3B and "Til Gøgl" have none.

**That is normal and not a problem to solve** (maintainer, 2026-08-26): the derivation gives
an indication of the area, not a survey. The 3 km buffer absorbs it, since checkpoints in a
night hike sit within one bounded area, so a hull around 9 of 12 plus 3 km very likely
contains the other three.

Still worth **counting and logging** how many lack coordinates — not to chase individual
gaps, but so that a systematic collapse (a year where the field stops being filled in at
all, or where only two checkpoints have positions) is visible rather than silently producing
a tiny area.

## Scope

- A `checkpoint` projection in `nathejk/table/` following the same conventions as `person`
  (see the `go-bff-layout` skill and PRD 006's projection for the pattern: no
  `internal/...` imports, ports declared locally, idempotent statements, per-year keying).
  Small — id, name, lat, lng, deleted.
- The race area computed from it: convex hull + 3 km buffer.
- Served to the client. `GET /api/config` already exists and already delivers runtime map
  configuration, so it is the natural home rather than a new endpoint — but the area is a
  polygon and `config` is currently flat, so decide deliberately whether to extend it or add
  `GET /api/race-area`.

**Do not** expose checkpoint coordinates themselves to participants through this. The area
is a hull; the individual post positions are race-sensitive (PRD 002 notes the event area is
deliberately not fully known to participants) and are not this task's to publish. Serve the
polygon, not the points.

## Acceptance Criteria

- [ ] A checkpoint projection consuming `NATHEJK.*.checkpoint.*.updated`, registered in the
      three-way pattern (construct → register → expose)
- [ ] Checkpoints without coordinates are **counted and logged**, so a systematic collapse in
      coordinate coverage is visible — individual gaps are expected and fine
- [ ] The race area is computed as convex hull + 3 km buffer, in a function with unit tests
      covering: a single checkpoint, two checkpoints (degenerate hull), collinear points, and
      the real 9-point set
- [ ] The buffer is applied in metres on the ground, not in degrees — a naive degree buffer
      is ~1.8× wider east–west than north–south at this latitude
- [ ] The area is served to the client, with OpenAPI annotations
- [ ] Individual checkpoint coordinates are **not** exposed to participants
- [ ] Behaves sanely with zero checkpoints (early in the year): no area, and the client
      falls back rather than caching the whole of Denmark
- [ ] A sanity bound on the result — an area far outside the expected few-hundred km² means
      bad input (one stray checkpoint in another country would balloon the hull), and should
      be refused rather than handed to the cache
- [ ] Verified against the live stream: the computed area is close to the 428 km² measured by
      hand on 2026-08-26 (hull 220 km², perimeter 60 km, extent 22.4 × 15.3 km)

## Progress Log

- 2026-08-26 — Task created from PRD 002 §11.2, after deriving this year's race area from
  the stream by hand: 9 of 12 checkpoints carry positions, hull + 3 km buffer = 428 km².
