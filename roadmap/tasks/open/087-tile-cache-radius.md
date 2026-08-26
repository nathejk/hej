# 087 — Cache map tiles within an 8 km radius, with eviction

**Status:** open
**Priority:** medium
**Created:** 2026-08-26
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 002 §11.2. Cache map tiles **within an 8 km radius of the current location**, on
PRD 009's shared offline layer, and **evict stale tiles** so the cache stays bounded.

A corridor along the route was considered and ruled out: the race area is known, but **the
route a given team will follow is not**, so there is no corridor to draw.

## Measured cost

An 8 km radius is 201 km². Real tile sizes from the live service (256 px tiles, central
Zealand):

| zoom ceiling | topo | + aerial (JPEG) | total |
|---|---|---|---|
| **z12–16 (recommended)** | 100 MB | 24 MB | **124 MB** |
| z12–17 | 262 MB | 63 MB | 325 MB |

Rule of thumb for any shape: **0.45 MB/km²** topo z12–16, plus **0.11 MB/km²** aerial.

**Cap the topo at z16.** DTK25 is a 1:25.000 map; its tiles shrink with zoom (140 kB at
z12 → 20 kB at z17) because they are the same cartography upsampled. z17 costs ~200 MB more
and contains no additional map information.

**Cache the aerial layer as JPEG** (see `fix(040)`) — as PNG it costs ~15× for the same
imagery.

## Eviction: the part that needs care

A radius that follows the walker sweeps a capsule, not a circle. The union of every 8 km
circle along a route is `2·R·L + πR²`:

| route walked | area | total at z12–16 |
|---|---|---|
| 0 km (static) | 201 km² | 124 MB |
| 20 km | 521 km² | 303 MB |
| 40 km | 841 km² | 478 MB |
| 60 km | 1,161 km² | 651 MB |

So the radius is a *fetch* policy and the bound has to come from eviction under a byte cap.

**The safety property that matters more than the policy: eviction is irreversible until
coverage returns.** Discarding a tile in a dead spot means it cannot be re-fetched — the
cache exists precisely because the network is not there. Consequences:

- Never evict tiles near the current position, whatever the cap says.
- Prefer evicting what is furthest behind the walker, then least recently used.
- If a pre-cached region exists (see below), treat it as **protected** and only evict from
  the opportunistic top-up. A two-tier cache — protected floor, evictable margin — is
  simpler to reason about than one LRU pool where the guaranteed coverage can be evicted by
  ordinary movement.

Post-event cleanup is a separate need: reclaiming a few hundred MB once the race is over.
PRD 009 §11.5 already asks whether a purge can be made reliable when the service worker may
never run again; the honest answer is that it cannot be guaranteed, so purge on next launch
rather than promising it happens at a time.

## First, one decision that may remove most of this work

**If the race area is known — and it is — pre-caching the whole area is strictly better than
a moving radius**, and probably cheaper. The route being unknown is exactly why a fixed area
works where a corridor does not.

| known race area | total (z12–16) | vs. 8 km radius |
|---|---|---|
| 10×10 km | 66 MB | cheaper than the static radius |
| 15×15 km | 138 MB | ≈ the static radius (124 MB) |
| 20×20 km | 236 MB | cheaper than the radius swept over 20 km (303 MB) |
| 30×30 km | 510 MB | dearer — the radius starts to pay |

An 8 km radius is 201 km², about a **14×14 km square**. So if the race area is near that
size, the radius and the area are nearly the same set of tiles, and the radius is machinery
for no gain.

Caching the whole area also wins on properties, not just bytes: complete coverage instead of
a window, no eviction logic, deterministic contents, and **all of it fetchable before the
start while the participant still has coverage**. A follow-me radius can only be filled
where there is network — which is exactly where the cache is not needed.

**So: get the race area's size before building the radius.** At ~20×20 km or smaller, cache
the area and drop the radius. Larger, and the radius plus eviction earns its complexity. The
two compose if needed: pre-cache the area, use the radius only beyond it.

## Acceptance Criteria

- [ ] Race area size established, and the choice between "whole area" and "8 km radius"
      made on that basis and recorded here
- [ ] Tiles cached for the chosen region, topo capped at z16, aerial as JPEG
- [ ] A **total byte cap** is enforced with eviction, verified by simulating a swept route
      rather than only a static radius
- [ ] Eviction never removes tiles near the current position, and any pre-cached region is
      protected from it
- [ ] A pre-race sync populates the region while online, so the cache is useful in a dead
      spot rather than only where it was never needed
- [ ] Cached tiles are served when offline; areas outside the cache degrade with a clear
      notice rather than blank grey
- [ ] Registered as a dataset on PRD 009's shared layer, drawing on the global budget —
      tiles are the largest dataset, so keeping them outside it would defeat the budget
- [ ] Actual usage reported via `navigator.storage.estimate()` in the readiness view, not
      estimated from tile counts
- [ ] `QuotaExceededError` handled: the cache stops growing and says so, rather than
      failing writes silently
- [ ] Post-event purge implemented on next launch, with its unreliability documented rather
      than papered over

## Depends on

PRD 009's shared offline layer (still in `draft/`). If 009 has not landed when this is
picked up, decide deliberately whether to wait or build map-local caching and migrate — the
PRD warns that retrofitting is real work.

## Progress Log

- 2026-08-26 — Task created from PRD 002 §11.2, sizing measured against the live service.
- 2026-08-26 — Radius reduced 10 km → 8 km (124 MB static, down from 188 MB) and eviction
  added as a requirement, per the maintainer. Corridor ruled out: the route a team takes is
  not known even though the area is. Recorded the open question of whether a known race area
  removes the need for a radius at all, since that decision could delete most of this task.
