# 087 — Cache map tiles for the race area

**Status:** open
**Priority:** medium
**Created:** 2026-08-26
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 002 §11.2. Cache the map tiles for **the whole race area** — the convex hull of this
year's checkpoints plus a 3 km buffer — on PRD 009's shared offline layer.

**Depends on task 088**, which derives the area and serves it to the client.

## Why the whole area, not a radius

The scope moved twice before landing here, and the reasoning is worth keeping because it
applies again if the area grows:

1. A 10 km radius of the current location — 188 MB.
2. Shrunk to 8 km with eviction — 160 MB using race-area tile sizes.
3. **The whole race area**, once it was actually measured: 428 km², **324 MB**.

Twice the bytes of the 8 km radius, and better on every other axis:

- **Complete coverage** instead of a moving window.
- **No eviction logic at all** — the area is fixed, so the cache has a known size. The
  radius needed a byte cap plus a policy that never evicted tiles near the user, which is
  fiddly and only correct if it is exactly right.
- **Deterministic contents** — "is the map ready?" has a true answer.
- **All of it is fetchable before the start**, while the participant still has coverage.
  This is the decisive one: a follow-me radius can only ever be filled where the network
  already works, which is precisely where the cache is not needed.

324 MB also sits comfortably inside the ~1 GB iOS 16 floor alongside portraits, the
directory, the app shell and the unshipped position track.

## Measured cost

Race area 428 km². Tile sizes measured **in the race area** (North Zealand), not
extrapolated — its cartography is denser than rural Zealand and topo tiles run up to 43%
larger there.

| zoom ceiling | tiles | topo | + aerial (JPEG) | total |
|---|---|---|---|---|
| z12–14 | 404 | 50 MB | 6 MB | 56 MB |
| z12–15 | 1,430 | 110 MB | 19 MB | 129 MB |
| **z12–16 (recommended)** | **5,291** | **264 MB** | **60 MB** | **324 MB** |

Per-zoom, z16 alone is 3,861 tiles and 195 MB — **60% of the total**. Two levers if budget
pressure appears later:

- Cap the topo at **z15** (which is the app's own `LOCATE_ZOOM`): whole area drops to 129 MB.
- Cap the **aerial** at z14 while keeping topo at z16: ~270 MB. Nobody navigates by aerial
  imagery at night, so this is the cheaper thing to give up first.

Neither is needed to fit. Start at z12–16.

**Cache the aerial layer as JPEG** (see `fix(040)`) — as PNG it costs ~15× for the same
imagery.

**Do not cache z17.** DTK25 is a 1:25.000 map; its tiles are the same cartography upsampled
past its design scale, so z17 adds bytes and no map information.

## Acceptance Criteria

- [ ] Tiles cached for the area supplied by task 088, topo z12–16, aerial as JPEG
- [ ] Pre-cached during the first sync, while the participant has coverage — the cache must
      be useful in a dead spot, not only where it was never needed
- [ ] Progress is visible during the sync: 5,291 tiles over rural mobile data is minutes,
      not seconds, and a silent multi-minute download is indistinguishable from a hang
- [ ] Resumable: a sync interrupted at 60% continues rather than restarting
- [ ] Cached tiles are served when offline; areas outside the race area degrade with a clear
      notice rather than blank grey
- [ ] Registered as a dataset on PRD 009's shared layer, drawing on the global budget —
      tiles are the largest dataset, so keeping them outside it would defeat the budget
- [ ] Actual usage reported via `navigator.storage.estimate()` in the readiness view, not
      inferred from tile counts
- [ ] `QuotaExceededError` handled: the cache stops growing and says so, rather than failing
      writes silently
- [ ] Re-syncing when the race area changes does not re-download tiles already held
- [ ] Post-event purge on next launch, with its unreliability documented rather than papered
      over (PRD 009 §11.5 — a service worker may never run again)

## Notes

No eviction requirement, deliberately — the fixed area removes the need. If the area ever
grows beyond the budget, the earlier radius-plus-eviction design is recorded in this task's
history and in PRD 002 §11.2, including the property that made it delicate: **eviction is
irreversible until coverage returns**, because a tile discarded in a dead spot cannot be
re-fetched.

## Depends on

- **Task 088** — the race area itself.
- PRD 009's shared offline layer (still in `draft/`). If 009 has not landed when this is
  picked up, decide deliberately whether to wait or build map-local caching and migrate; the
  PRD warns that retrofitting is real work.

## Progress Log

- 2026-08-26 — Task created from PRD 002 §11.2 as a 10 km radius, sizing measured against
  the live service.
- 2026-08-26 — Radius reduced to 8 km and eviction added, per the maintainer. Corridor ruled
  out: the route a team takes is not known even though the area is.
- 2026-08-26 — **Re-scoped to the whole race area** after deriving it from the stream
  (428 km², task 088). At 324 MB it is twice the 8 km radius and removes the eviction
  problem, the incomplete-coverage problem, and the "can only be filled where it is not
  needed" problem together. Also re-measured tile sizes inside the actual race area rather
  than reusing the rural sample, which turned out to matter: +43% at z15.
