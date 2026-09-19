# 342 — The map island

**Status:** open
**Priority:** medium
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `GET /api/public/patrol/{number}/map` returns the merged track and plottable scans; gated by task
      330's shared check. OpenAPI annotated.
- [ ] `GET /api/public/albums` returns albums' located items for plotting; bounds-rejected coordinates are
      absent. OpenAPI annotated.
- [ ] The island loads only where JS runs; with JS off the container shows the scan list and no broken
      frame.
- [ ] Merged track drawn as multi-segment polyline; a test or fixture demonstrates a gap is not bridged.
- [ ] Scan pins, photograph markers and clusters are visually distinguishable; clusters show counts.
- [ ] Base layers shared in configuration with the app's map (PRD 002), not duplicated by hand.
- [ ] Legible on topographic and aerial backgrounds.
- [ ] No glimt appears on the map (task 336) — only curated album photographs.
- [ ] The island holds no application state; no router, no store.
- [ ] Bundle cost of Leaflet + clustering recorded in this task's log, so §11 Q8 can be decided on numbers.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §8 / §10 (Phase 2). Depends on 330, 340, 341, 333.
