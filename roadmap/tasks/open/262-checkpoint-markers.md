# 262 — Checkpoint markers on the event map

**Status:** open
**Priority:** medium
**Created:** 2026-09-15

## Description

PRD 016 phase 3. Draw the patrol's revealed checkpoints on `EventMap.vue`.

- Visually **distinct from the patrol's own scan markers**, which already exist — the map
  will show both at once and "post we must reach" versus "thing we registered" must not read
  as the same object.
- A checkpoint the patrol has **already scanned** reads as done (muted / check), so the map
  doubles as a progress view (PRD 016 §11.4 makes this stable: reveals are monotonic, so
  markers never vanish).
- Legible on **both** base layers — the topo and the aerial have completely different
  contrast, and a marker tuned for one disappears on the other (PRD 002).
- Tap shows name and, when known, the window.
- Only positioned checkpoints are plotted; an unpositioned revealed checkpoint is not an
  error and appears nowhere on the map.
- Leaflet stays lazy-loaded — the markers must not drag it into the app-shell bundle.
- Icons from Lucide (`Flag`, `Check`).

## Acceptance Criteria

- [ ] Markers render from `checkpoints.store`, distinct from scan markers.
- [ ] Scanned checkpoints render in a done state.
- [ ] Verified legible on both topo and aerial.
- [ ] Unpositioned checkpoints are skipped without error (test).
- [ ] Popup shows name + window when known.
- [ ] Bundle check: no Leaflet in the app shell.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 3.
