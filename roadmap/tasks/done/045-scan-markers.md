# 045 — Scan markers on the map

**Status:** done
**Priority:** high
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

Plot the patrol's registrations, visually distinguishing checkpoint scans from
bandit catches (PRD 002).

## Acceptance Criteria

- [x] `scans.store.ts` fetches `/api/patrol/scans`, mapping snake_case → camelCase
      at the store boundary and parsing timestamps.
- [x] Markers distinguishable by kind, legible on both topo and aerial.
- [x] Popup shows label, kind and a Danish-formatted timestamp.
- [x] Registrations without coordinates are not plotted.
- [x] Fetching failures leave the map usable.

## Progress Log

- 2026-08-24 12:15 — `scans.store.ts` never throws: a failed fetch sets a message
  and leaves the map working, since the map is the primary feature and the
  registrations are an overlay on top of it.
- 2026-08-24 12:20 — Markers use a Leaflet `divIcon` rather than the default image
  marker: no icon-asset wrangling through Vite (a classic Leaflet-with-bundlers
  trap), and the two kinds stay distinct — dark circle + flag for a post, red
  circle + skull for a bandit catch, both with a white ring so they read against
  aerial imagery as well as pale topo.
- 2026-08-24 12:22 — An empty list is a normal state (personnel have no patrol), so
  it hides the UI rather than showing an error.
- 2026-08-24 13:05 — ✅ Verified in-browser: exactly **4** markers for the 5 seeded
  scans — the un-positioned "Post 4 – Gjern Bakker" is correctly absent from the map
  while still appearing in the list. Popup rendered "Bandit: Sorte Sofie / Fanget af
  bandit · man. 23.32" and was legible over the orthophoto.
