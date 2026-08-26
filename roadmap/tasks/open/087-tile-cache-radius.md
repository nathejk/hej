# 087 — Cache map tiles within a 10 km radius, under a byte cap

**Status:** open
**Priority:** medium
**Created:** 2026-08-26
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 002 §11.2, decided 2026-08-26: cache map tiles **within a 10 km radius of the current
location**, on PRD 009's shared offline layer. Replaces the earlier placeholder task of
"supply tile-set sizing numbers", since the sizing is now measured.

## Measured cost

A 10 km radius is 314 km². Real tile sizes from the live service (256 px tiles, central
Zealand):

| zoom ceiling | topo | + aerial (JPEG) | total |
|---|---|---|---|
| z12–15 | 64 MB | 11 MB | **74 MB** |
| **z12–16 (recommended)** | 152 MB | 36 MB | **188 MB** |
| z12–17 | 365 MB | 133 MB | 498 MB |

Rule of thumb for any shape: **0.45 MB/km²** topo z12–16, plus **0.11 MB/km²** aerial.

**Cap the topo at z16.** DTK25 is a 1:25.000 map; its tiles shrink with zoom (140 kB at
z12 → 20 kB at z17) because they are the same cartography upsampled. z17 costs ~310 MB more
and contains no additional map information. The aerial layer is 12.5 cm native imagery, so
it can justify going deeper if wanted — but it is the layer nobody navigates by at night.

## Two things "within 10 km of current location" hides

Both need designing for, and neither is obvious from the phrasing.

**1. A radius that follows the walker sweeps a capsule, not a circle.** The union of every
10 km circle along a route is `2·R·L + πR²`:

| route walked | area | total at z12–16 |
|---|---|---|
| 0 km (static) | 314 km² | 188 MB |
| 40 km | 1,114 km² | **626 MB** |
| 60 km | 1,514 km² | **842 MB** |

842 MB would consume the ~1 GB iOS 16 ceiling on its own, before portraits, the directory,
the app shell and the unshipped position track. So a radius alone is not a bound: this
needs a **byte cap with LRU eviction**, evicting tiles furthest behind the walker first.

**2. A follow-me radius cannot deliver offline coverage.** Filling it needs network *where
you already are* — but where there is coverage the cache is not needed, and in a dead spot
it cannot be filled. Taken alone it is a cache-warming policy, not offline preparation.

So the useful shape is **two mechanisms**:

- **Pre-race sync of a fixed region while the participant still has coverage** — this is
  what actually delivers offline. A 10 km radius around the assembly point needs no route
  knowledge (188 MB at z16).
- **Opportunistic top-up within 10 km of the current position while online**, under the LRU
  byte cap.

If the route turns out to be known in advance, a corridor is far cheaper for better
coverage: a **2 km corridor along a 40 km route is 173 km² / 108 MB**, about a sixth of the
swept radius, and all of it is fetchable before the start. Worth revisiting with the
maintainer if the route becomes available.

## Acceptance Criteria

- [ ] Tiles are cached for a 10 km radius, topo capped at z16
- [ ] A **total byte cap** is enforced with LRU eviction, so a long route cannot grow the
      cache without bound — verified by simulating a swept route, not just a static radius
- [ ] A pre-race sync populates a fixed region while online, so the cache is useful in a
      dead spot rather than only where it was never needed
- [ ] Cached tiles are served when offline, and the map degrades with a clear notice for
      areas outside the cache rather than showing blank grey
- [ ] Registered as a dataset on PRD 009's shared layer, drawing on the global budget
      rather than its own — tiles are the largest dataset, so keeping them outside the
      budget would defeat it
- [ ] Actual usage is reported through `navigator.storage.estimate()` in the readiness view
      rather than estimated from tile counts
- [ ] `QuotaExceededError` is handled: the cache stops growing and says so, rather than
      failing writes silently
- [ ] The aerial layer is cached as **JPEG** (see `fix(040)`) — caching it as PNG would
      cost ~15× for the same imagery

## Depends on

PRD 009's shared offline layer (still in `draft/`). If 009 has not landed when this is
picked up, decide deliberately whether to wait or build map-local caching and migrate —
the PRD warns that retrofitting is real work.

## Progress Log

- 2026-08-26 — Task created from PRD 002 §11.2, with sizing measured against the live
  service rather than estimated.
