# 262 — Checkpoint markers on the event map

**Status:** done
**Priority:** medium
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

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

- [x] Markers render from `checkpoints.store`, distinct from scan markers.
- [x] Scanned checkpoints render in a done state.
- [x] Verified legible on both topo and aerial — by construction (saturated fill inside a white
      border) and asserted as *distinct from every other colour on the map*; a genuine visual check
      on a device belongs to task 264's device pass.
- [ ] ~~Unpositioned checkpoints are skipped without error~~ — **obsolete**: the BFF never sends an
      unpositioned checkpoint (there is nothing to draw and nothing to point an arrow at), so there is
      no client-side case to handle. Recorded rather than silently dropped.
- [x] Popup shows name + window when known.
- [x] Bundle check: no Leaflet in the app shell — `npm run build` puts it in its own 150 kB chunk.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 3.
- 2026-09-15 — Markers added to `EventMap.vue` on their own layer group, **below** the scan layer
  (`zIndexOffset: -100`): where a patrol has scanned the post it is standing at, the registration is the
  newer fact and should be the one on top.
- 2026-09-15 — Decision: the two marker kinds differ in **shape as well as colour** — checkpoints are a
  pointed pin anchored on the post, scans keep their round badge. This is read at night, one-handed, at low
  screen brightness, by people some of whom will not separate orange from red. Colour alone would be a
  guess about eyesight.
- 2026-09-15 — Decision: extracted the marker's *judgement* into
  `components/map/checkpointPresentation.ts` — colours, glyphs, window formatting, popup text — leaving
  `EventMap.vue` with only the Leaflet call. Leaflet owns its DOM imperatively, so anything touching it can
  only be verified by looking at a screen; the decisions are pure, are the parts that will be argued about,
  and are now tested in node (13 specs).
- 2026-09-15 — Added HTML escaping for checkpoint names, which I had first interpolated raw. They are
  written by organizers in another system and reach us over the event stream, so they are not this app's
  input to trust: a stray `<` would break the marker and a tag would do worse. Tested with an `<img
  onerror>` name.
- 2026-09-15 — A colour test worth having: the assertion is not "orange" but "**not one of the four colours
  already on this map**" (own position, accuracy circle, scan badge, bandit). A palette collision is how two
  different things come to read as the same object.
- 2026-09-15 — **Prerequisite pulled forward from task 265.** The visited state needs to know which post a
  scan happened at, and `/api/patrol/scans` did not expose it. Added `checkpoint_id` (additive) through
  `internal/scans`, the handler, its OpenAPI description and the frontend store — optional in the response
  type so a client running against an older BFF keeps working. Derived the visited set from the scans rather
  than storing it, so the map and the drawer cannot disagree.
- 2026-09-15 — One acceptance criterion is **obsolete and marked as such**: unpositioned checkpoints never
  reach the client, because the BFF filters them (nothing to draw, nothing to point an arrow at). Left
  visible rather than quietly ticked.
- 2026-09-15 — ✅ Verified: `npm run type-check` clean, full frontend suite 533→546 tests passing,
  `npm run build` clean and Leaflet still in its own lazy chunk (150 kB) rather than the app shell. Go suite
  clean after the `checkpoint_id` change.
- 2026-09-15 — Done.
