# 088 — Derive the race area from checkpoints and serve it to the client

**Status:** done
**Priority:** medium
**Created:** 2026-08-26
**Picked up by:** agent
**Started:** 2026-08-26
**Completed:** 2026-08-26

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

- [x] A checkpoint projection consuming `NATHEJK.*.checkpoint.*.updated`, registered in the
      three-way pattern (construct → register → expose)
- [x] Checkpoints without coordinates are **counted and logged**, so a systematic collapse in
      coordinate coverage is visible — individual gaps are expected and fine
- [x] The race area is computed as convex hull + 3 km buffer, in a function with unit tests
      covering: a single checkpoint, two checkpoints (degenerate hull), collinear points, and
      the real 9-point set
- [x] The buffer is applied in metres on the ground, not in degrees — a naive degree buffer
      is ~1.8× wider east–west than north–south at this latitude
- [x] The area is served to the client, with OpenAPI annotations
- [x] Individual checkpoint coordinates are **not** exposed to participants — and this needed
      more than not returning them; see the disclosure grid in the log
- [x] Behaves sanely with zero checkpoints (early in the year): no area, and the client
      falls back rather than caching the whole of Denmark
- [x] A sanity bound on the result — an area far outside the expected few-hundred km² means
      bad input (one stray checkpoint in another country would balloon the hull), and should
      be refused rather than handed to the cache
- [x] Verified against the live stream: the computed area is close to the 428 km² measured by
      hand on 2026-08-26 (hull 220 km², perimeter 60 km, extent 22.4 × 15.3 km)

## Progress Log

- 2026-08-26 — Task created from PRD 002 §11.2, after deriving this year's race area from
  the stream by hand: 9 of 12 checkpoints carry positions, hull + 3 km buffer = 428 km².
- 2026-08-26 — **Done.** A `checkpoint` projection, `ComputeRaceArea`, and an authenticated
  `GET /api/race-area`. Verified against the live stream: 12 checkpoints projected, 9
  positioned, 0 dead letters, and the endpoint returns 476 km² — the 428 km² measured by hand
  plus the disclosure grid below.

  **The endpoint is authenticated and separate from `/api/config`.** That question decided
  itself: `config.go`'s own comment says "Everything here is public by definition: it is
  handed to any browser that asks. Never add a value that must stay server-side." The race
  area is precisely such a value. Since a participant must sign in to use the app at all
  (PRD 005), requiring a session costs nothing real.

  **A leak found by its own test, and the most important thing in this task.** The first
  implementation returned the buffered hull unsnapped, and the test asserting "no checkpoint
  coordinate appears in the response" failed — the response contained input latitudes at full
  float precision (`55.830986000000003`, `55.716594999999998`).

  Not a coincidence, and worse than it first looked. A buffered hull is **invertible**: every
  vertex lies on a circle of radius `BufferKm` around a vertex of the *original* hull, and
  those vertices **are** checkpoint positions. The buffer distance has to be published too, so
  the client does not add its own — so anyone could offset inward and recover the outermost
  posts exactly. The exact-coordinate matches were just the visible symptom, caused by
  sampling each arc from angle 0, which puts a vertex at its centre's own latitude.

  Fixed by snapping every published vertex **outward** onto a ~1.1 km grid
  (`GridLat`/`GridLng`), which destroys the inversion's precision. The cost is explicit:
  **+48 km² and +34 MB of tiles (+11%)** to reduce "there is a post at this spot" to "the
  event is in this region" — which the client must know regardless, since it caches tiles for
  it. Outward, so coverage is never lost; a test asserts every checkpoint plus its full 3 km
  buffer still falls inside the published bounds.

  Three other things worth recording:

  - **`.created` is not consumed.** Checked the live events: they carry only the id and
    checkgroup — no name, no position. Subscribing would add a family that could only ever
    write an empty row. `.deleted` *is* subscribed although no such events exist yet, because
    a removed checkpoint must not leave a phantom point stretching the area, and discovering
    that during an event is worse than a handler that never fires.
  - **Every field on `NathejkCheckpointUpdated` is a pointer**, so the message expresses a
    partial update. Only the columns actually present are written — otherwise an event that
    merely renames a checkpoint would erase its position, silently shrinking the cached
    region. Pinned by a test.
  - **0,0 is stored as NULL.** It is what an unset coordinate serialises to, and it is the
    Atlantic off Ghana. Stored literally it would stretch the hull across two continents,
    trip the plausibility bound, and lose the whole race area because one post had not been
    sited yet.

  Also added `ServiceUnavailableResponse` (503) to `cmd/api/app`, because this handler needs
  to distinguish two things a 404 would conflate: "no area derived yet", a normal state early
  in the year, from "no projection at all", a dependency the client should retry for. And
  `raceAreasOrNil`, because assigning a nil `*checkpoint.Table` to the interface would produce
  a non-nil interface and turn the handler's nil check into a nil-pointer dereference — the
  same trap `publisherOrNil` already guards.

  20 tests. Gates green on the workspace and `GOWORK=off` paths.

## Files

- `go/nathejk/table/checkpoint/` (new) — `table.sql`, `table.go`, `consumer.go`, `querier.go`,
  `racearea.go` + tests
- `go/cmd/api/racearea.go` (new) + `racearea_test.go` — `GET /api/race-area`
- `go/cmd/api/app/errors.go` — `ServiceUnavailableResponse`
- `go/cmd/api/main.go`, `routes.go`, `internal/data/models.go` — wiring
- `roadmap/prd/doing/002-event-map-with-position-and-scan-history.md` — published-area
  figures and the disclosure-grid reasoning
