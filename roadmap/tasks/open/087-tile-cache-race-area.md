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

Race area 428 km² for 2026, and **roughly the same size every year** (maintainer) — so this
sizing is durable rather than annual. Tile sizes measured **in the race area** (North
Zealand), not extrapolated: its cartography is denser than rural Zealand and topo tiles run
up to 43% larger there.

| zoom ceiling | tiles | topo | + aerial (JPEG) | total |
|---|---|---|---|---|
| z12–14 | 404 | 50 MB | 6 MB | 56 MB |
| z12–15 | 1,430 | 110 MB | 19 MB | 129 MB |
| **z12–16 (recommended)** | **5,291** | **264 MB** | **60 MB** | **324 MB** |

Plan for **~450 MB** to leave headroom for a larger year (600 km² is 446 MB); expect ~325 MB.

Per-zoom, z16 alone is 3,861 tiles and 195 MB — **60% of the total**.

**Do not treat z15 as a cheap saving.** DTK25 is a 508 DPI 1:25.000 product, so its native
resolution is **1.25 m/px**, which is **z16** (1.34 m/px). z15 is *half* the linear
resolution of the source map — a real loss of detail. z17, by contrast, is 2× oversampled,
which is why its tiles shrink and carry no new information. So **z16 is the correct ceiling**
and dropping to z15 is a fidelity decision, not housekeeping.

**Cache the aerial layer as JPEG** (see `fix(040)`) — as PNG it costs ~15× for the same
imagery.

## How it downloads matters as much as how big it is

Maintainer, 2026-08-26: *"we should be aware if we start downloading several 100 MB on a
mobile connection — at least we should cache if relevant map is being browsed."* 325 MB
pulled silently over rural mobile data is a real cost to a participant, possibly against a
data cap, on a battery that has to last the night.

**The platform will not help decide.** The Network Information API (`navigator.connection`)
is **not available in Safari**, so on iOS — the primary platform — the app cannot tell WiFi
from cellular. "Download only on WiFi" is not implementable; `navigator.onLine` distinguishes
only online from offline. The decision therefore has to be the user's, made with the size in
front of them.

Three tiers:

| tier | tiles | size | when |
|---|---|---|---|
| **cache while browsing** | — | **free** | always on, unconditional |
| z12–14 (orientation, 5.4 m/px) | 404 | 56 MB | candidate for automatic |
| z15 (+detail) | 1,026 | +73 MB | explicit opt-in |
| z16 (native scale) | 3,861 | +195 MB | explicit opt-in |

**Cache-while-browsing is the important one and the cheapest to build.** Every tile fetched
to draw the map gets stored; the bytes are already being downloaded, so it costs nothing
extra. A participant who looks at the map around the assembly area arrives with that area
cached without being asked. Build this first — it delivers most of the practical benefit
before any bulk download exists.

The expensive 268 MB of z15–z16 should be user-initiated, with the size stated and progress
shown, and ideally prompted while the participant is at home on WiFi. PRD 009 §11.7 already
proposes a "prepare for offline" push a few hours before the start; this is what it is for.

**Do not cache z17.** DTK25 is a 1:25.000 map; its tiles are the same cartography upsampled
past its design scale, so z17 adds bytes and no map information.

## Acceptance Criteria

- [ ] **Tiles are cached as they are browsed**, unconditionally and with no bulk download —
      this alone must give useful offline coverage for wherever the user has looked.
      *Implemented 2026-08-26 and verified in the generated `sw.js`; the "gives useful
      offline coverage" half needs a browser against a production build, see the log below.*
- [ ] Bulk download of the race area (from task 088), topo z12–16, aerial as JPEG
- [ ] The bulk download is **user-initiated, with the size stated before it starts** — not
      triggered silently, since on iOS the app cannot tell WiFi from cellular
- [ ] Progress is visible: 5,291 tiles over rural mobile data is minutes, not seconds, and a
      silent multi-minute download is indistinguishable from a hang
- [ ] The download is **resumable**: interrupted at 60% it continues rather than restarting
- [ ] The download can be **cancelled**, and cancelling keeps what has already been fetched
- [ ] Zoom tiers are separable, so z12–14 (56 MB) can be offered independently of z15–16
      (268 MB)
- [ ] Cached tiles are served when offline; areas outside the cache degrade with a clear
      notice rather than blank grey
- [ ] Declared in PRD 009's priority order with its size, drawing on the global budget —
      tiles are the largest dataset by far (~99% of it), so keeping them outside it would
      defeat the budget. *Not a registration call: PRD 009's dataset registry and generic
      sync engine were cut on 2026-09-01 (009 §4, §11.2), precisely because this cache had
      already shipped and works. What is owed is a declared size, a rank — **last, evicted
      first, highest zoom first** (009 §6) — and reporting into `offline.store`.*
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

- **Task 088** — the race area itself. **Done**, and served by `GET /api/race-area`. The area
  is derived from this year's checkpoints and **does not change during an event** (maintainer,
  2026-09-01), so the download has a fixed target: 5,291 tiles / 324 MB for 2026. Pre-event it
  can still move, since the checkpoint set is edited until late — hence the re-sync criterion
  above.
- PRD 009's **budget** — **decided 2026-09-01** (009 §6), so this is no longer blocked on it.
  Tiles get ~500 MB planned inside a ~1 GB origin ceiling, and they sit **last** in the
  priority order: tiles are the dataset that gets evicted to protect the app shell, the
  directory, portraits and — above all — unshipped local writes, highest zoom first. The
  counter-argument that eviction is irreversible in the field is recorded and was weighed:
  a lost tile costs a map the participant can survive without; a lost track never existed.

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
- 2026-08-26 — Download strategy added as the task's centre of gravity, per the maintainer's
  concern about pulling several hundred MB over a mobile connection. Cache-while-browsing is
  unconditional and free; the bulk download is user-initiated with the size shown, because
  `navigator.connection` is unavailable in Safari so the app cannot detect WiFi on iOS.
  Also corrected earlier advice in this file: z15 is *not* a cheap saving — z16 is DTK25's
  native 1.25 m/px scale, so z15 halves the source map's resolution.
- 2026-08-26 — **Cache-while-browsing implemented.** A Workbox `runtimeCaching` rule in
  `vite.config.ts` stores every tile fetched to draw the map, plus `crossOrigin` on the tile
  layer and a new `src/config/cache.ts` holding the tuning in one place.

  Four decisions worth the reasoning:

  - **`CacheFirst`, not `StaleWhileRevalidate`.** Checked the response headers: the service
    sends **no `cache-control`, `etag` or `expires` at all**, so there is nothing to
    revalidate against and no upstream freshness signal to respect. SWR would re-download
    every visible tile on every view — precisely the mobile-data cost this exists to avoid.
    Our `maxAgeSeconds` is therefore the only freshness policy there is; a year is right for
    maps revised on a scale of years (DTK 1:50.000 has not changed since 2017).
  - **`crossOrigin: 'anonymous'` on the tile layer.** Without it, `<img>` tiles are fetched
    no-cors and the responses are **opaque**; browsers pad opaque responses for quota
    accounting to prevent cross-origin size leaks, so a few thousand tiles would consume far
    more than their real ~324 MB. Verified the live service sends
    `Access-Control-Allow-Origin` reflecting whatever Origin is presented — tested for the
    dev hostname, the production hostname and a nonsense one, all reflected — so this is
    safe. `cacheableResponse: { statuses: [200] }` then refuses to store opaque responses at
    all, so a future CORS regression fails visibly through the existing tile-retry and
    "Kortbilleder kunne ikke hentes" notice rather than silently filling the cache with
    padded entries.
  - **The cache key strips `token` and `_retry`.** Both would otherwise fragment the cache:
    a rotated Dataforsyningen token would silently miss several hundred megabytes and
    re-download the lot, and the existing tile-retry appends `&_retry=N` to force an `<img>`
    reload, so a tile that failed once and succeeded on retry would be stored under a URL
    its normal request never asks for.
  - **Removed `purgeOnQuotaError`** after adding it. It deletes the *entire* cache on a quota
    error, which contradicts the principle already written into PRD 009 §11.9: offline, a
    purged tile cannot be re-fetched, so losing every tile mid-race is unrecoverable. A full
    cache that cannot grow is much better than an empty one. Whether tiles should be
    sacrificed to protect genuinely unrecoverable data like portraits is a cross-dataset
    priority question and belongs to PRD 009 §11.1.

  **A bug found by inspecting the generated worker rather than trusting the build.** The
  `cacheKeyWillBeUsed` callback originally referenced the imported
  `TILE_CACHE_KEY_IGNORED_PARAMS`. It type-checked, built cleanly, and produced a `sw.js`
  containing `for(const s of TILE_CACHE_KEY_IGNORED_PARAMS)` — a free identifier that does
  not exist in worker scope, so every tile request would have thrown `ReferenceError` at
  runtime. Workbox's generateSW mode **stringifies** callbacks verbatim instead of bundling
  them; only values outside function bodies (`cacheName`, `maxEntries`, `urlPattern`) are
  evaluated at build time. The list is now inlined, with the constraint documented at both
  ends. Nothing short of reading `sw.js` would have caught this.

  **Verification, and its limit.** Confirmed in the built `sw.js`: the route
  `/^https:\/\/api\.dataforsyningen\.dk\//` bound to `CacheFirst`, cache
  `nathejk-map-tiles-v1`, `ExpirationPlugin{maxEntries:12000, maxAgeSeconds:31536000}`,
  `CacheableResponsePlugin{statuses:[200]}`, and a self-contained key callback. `crossOrigin`
  present in the `EventMap` chunk. Precache unchanged at 24 entries / 491 KiB, so no shell
  bloat. `vue-tsc` and `npm run build` clean.

  What is **not** verified is runtime behaviour, and it cannot be from here: the service
  worker is disabled in dev (no `devOptions.enabled` in `vite.config.ts`), so the dev server
  cannot exercise this at all, and there is no browser in this environment. Someone needs to
  load a production build and confirm tiles are served from `nathejk-map-tiles-v1` with the
  network offline. That is the same production-build check task 036 is already waiting on.

- 2026-09-01 — PRD 009 review settled the two inputs this task was waiting on: the priority
  order (tiles last, evicted first, highest zoom first) and the race area (derived from
  checkpoints, fixed during an event). No longer blocked on 009 for anything but its readiness
  surface; the bulk download can be built.
- 2026-09-02 — PRD 002's checkboxes reconciled against the shipped source (not against task status).
  This task stays the only unbuilt code PRD 002 owes; recorded there as `[~]` with the reason. Nothing
  blocks it now: the race area is served by `GET /api/race-area` (task 088) and PRD 009 shipped with a
  budget and a rank for tiles — evicted first, highest zoom first, ~500 MB planned.
