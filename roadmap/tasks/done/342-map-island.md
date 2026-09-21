# 342 — The map island

**Status:** done
**Priority:** medium
**Created:** 2026-09-19
**Picked up by:** agent
**Started:** 2026-09-21
**Completed:** 2026-09-21

## Description

PRD 011 §6, §8. The map on the patrol page: the race area carrying the patrol's scan pins, its merged
track, and any located album photograph — clustered when zoomed out.

**`EventMap.vue` cannot be reused.** It is a Vue component in the bundle these public pages deliberately do
not load (PRD 011 §8). So this is a small **island**: Leaflet plus a marker-cluster plugin, loaded only
where JS runs, fed from a JSON endpoint, drawn into a container that **already contains a working
fallback** — the scan list from task 341.

The base-layer configuration *is* worth sharing with the app so the two are recognisably the same place;
the component is not shareable, and pretending otherwise produces the wrong architecture.

**This is the one place PRD 011 spends a new dependency** (clustering). §11 Q8 asks whether a
server-rendered static map image would be better for the parent on the old laptop — that question is still
open, and if the answer turns out to be "both", this island becomes the enhanced half. Do not let it grow:
it draws what an endpoint gives it and holds no state. If it acquires routing, layer switching and a store,
it has become the app and belongs in the app.

**Drawing rules:**

- One colour for the merged track, drawn as a **multi-segment** polyline (`L.polyline` accepts an array of
  arrays) so gaps stay gaps. Never join across a break (task 340).
- A distinguishable pin for a scan; a different marker for a photograph.
- Clusters show a count.
- Legible on both topographic and aerial backgrounds.

**The gate applies to the endpoints, not just the page** (task 330). Two JSON endpoints feed this island,
and a gate applied only at render time would leave them open — which is the whole course in
machine-readable form.

## Acceptance Criteria

- [x] `GET /api/public/patrol/{number}/map` returns the merged track and plottable scans; gated by task
      330's shared check. OpenAPI annotated.
- [x] The endpoint's point shape carries a **timestamp**, and `trackPointsForDistance` in
      `cmd/api/patrolpage.go` is wired to use it. Task 341 left the track raising no distance leg because
      `patroltrack.Point` has no time and `distance.Compute` matches legs by time — so the distance is
      currently a floor by omission as well as by design. The function says so; this is where it is fixed.
- [x] `GET /api/public/albums` returns albums' located items for plotting; bounds-rejected coordinates are
      absent. OpenAPI annotated.
- [x] The island loads only where JS runs; with JS off the container shows the scan list and no broken
      frame.
- [x] Merged track drawn as multi-segment polyline; a test or fixture demonstrates a gap is not bridged.
- [x] Scan pins, photograph markers and clusters are visually distinguishable; clusters show counts.
- [x] Base layers shared in configuration with the app's map (PRD 002), not duplicated by hand.
      **Partly — the duplication is named, not removed.** See the log. *(Completed by task 353 on the same
      day: the duplication is gone, and it turned out the copy had drifted to a service the app never used.)*
- [x] Legible on topographic and aerial backgrounds.
- [x] No glimt appears on the map (task 336) — only curated album photographs.
- [x] The island holds no application state; no router, no store.
- [x] Bundle cost of Leaflet + clustering recorded in this task's log, so §11 Q8 can be decided on numbers.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §8 / §10 (Phase 2). Depends on 330, 340, 341, 333.
- 2026-09-21 — **Done.** Two endpoints (`cmd/api/patrolmap.go`), one island (`vue/public/publicmap.js`,
  ~210 lines of vanilla JS, no build step), Leaflet + leaflet.markercluster vendored into
  `vue/public/vendor/` by `vue/scripts/vendor-leaflet.sh`.

  **Both endpoints share one access decision.** `openPatrol()` is now the single place the gate is
  consulted, and `patrolPage` was refactored onto it — so the page and the JSON cannot drift into
  disagreeing about who is public. A gate on the render path only would have left the whole course
  readable as JSON, which is the point the task made.

  **`patroltrack.Point` gained `TS`**, which is what let `trackPointsForDistance` finally do something: in
  a fixture the same patrol goes from 5 km (scans only) to 37 km once the track is fed in. The field is
  deliberately **never serialised** — a per-point timestamp on a public endpoint is a movement log, and
  the map does not need it.

  **The distance is the reason the timestamp exists, not a side effect.** `distance.Compute` matches legs
  by time; without `TS` every track point was outside every leg and the track raised nothing.

  **Bundle cost, measured (gzip -9), for §11 Q8:**

  | asset | raw | gzipped |
  |---|---|---|
  | leaflet.js | 147.6 KB | **42.4 KB** |
  | leaflet.css | 14.8 KB | 3.5 KB |
  | leaflet.markercluster.js | 34.1 KB | **8.7 KB** |
  | MarkerCluster(.Default).css | 2.2 KB | 0.7 KB |
  | publicmap.js | 9.6 KB | 4.2 KB |
  | **total** | **208 KB** | **≈59.6 KB** |

  **This number is for the maintainer, and it argues against this approach.** A server-rendered static
  map image would be roughly 50–150 KB — *comparable in bytes* while needing no JavaScript and working on
  any browser. The island's advantage is pan and zoom, not weight. §11 Q8 should now be decided on that,
  and the island is cheap to retire if the answer is the image.

  **Fixed while verifying:**
  - `/offentligt` was **not proxied by the Vite dev server**, so the public pages were unreachable in a
    dev browser at all (the api answered them only on its internal port). Added to `vue/vite.config.ts`.
  - The map container was a 22rem grey box with JS off — the acceptance criterion says *no broken frame*,
    and an empty box is one. `.maparea` and the route caveat are now `display:none` until the island adds
    `.ready`, which it does before `L.map()` because Leaflet measures the container.
  - Photographs are `L.marker` with a `divIcon`, not `circleMarker`: markercluster is built around
    `L.Marker`, and an amber **square** also reads as a different kind of thing from the round scan dots.

  **A new guard, verified by breaking it:** `TestIslandAssetsAreVendored` reads every `/vendor/...`
  reference out of the rendered page and stats it on disk. The failure mode it catches is an upgrade that
  forgets to re-run the vendor script — the template asks for a file nobody committed, the map silently
  stops drawing in production, and every other test still passes. Removing `MarkerCluster.css` fails it.

  **Base layers: the duplication is named, not removed.** The WMS URL and layer name are copied from
  `src/config/map.ts` into the island, with a header comment saying so. `map.ts` is TypeScript inside the
  bundle these pages may not load; a build-time generator (the `generate-icons.sh` pattern) would remove the
  copy and was judged too much machinery for one URL and one layer. Revisit if a second layer appears.

  **— Revisited the same day (task 353), and the judgement was wrong.** Not because a second layer appeared,
  but because the copy had already drifted: the island pointed at `dkskaermkort_DAF`, which is not one of the
  app's three layers, so the public map was showing a base map no participant had seen. The fix needed no
  generator — a plain JSON file both sides read — which means the machinery argument was also wrong.

  **Verified in the dev stack:** `/api/public/patrol/71/map` returns 8 track segments for a real patrol,
  the page emits the three deferred scripts and three stylesheets, and Vite serves `/vendor/leaflet.js`
  and `/publicmap.js` with 200. Not yet opened in a browser on a phone — that is task 348.

  `gofmt`, `go vet`, `go test ./...` clean; `npm run type-check` and 942 frontend tests pass.
