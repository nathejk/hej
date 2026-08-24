# 046 — Registrations list (bottom sheet)

**Status:** done
**Priority:** medium
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

The same registrations as a chronological list, reachable without leaving the map,
where tapping a row pans the map to that marker (PRD 002).

## Acceptance Criteria

- [x] Newest-first list in an overlay sheet, opened from an on-map handle showing
      the count.
- [x] Selecting an entry pans/zooms to the marker and opens its popup.
- [x] Empty state ("Ingen registreringer endnu"); handle hidden when there is
      nothing to show.
- [x] Entries without a position are listed and marked as unplottable.
- [x] Rows ≥44px; safe-area aware.

## Progress Log

- 2026-08-24 12:25 — Built `ScanList.vue` on the **shadcn `Drawer`** — the standard
  component, reusing exactly what task 032 standardised on for the nav overflow, so
  the app has one bottom-sheet idiom rather than two.
- 2026-08-24 12:26 — Selecting a row also turns *off* follow mode: the user has
  explicitly asked to look somewhere other than their own position, so the map
  should stop dragging them back.
- 2026-08-24 12:27 — Un-positioned entries get a `MapPinOff` marker and simply
  close the sheet without moving the map, rather than being hidden or appearing
  tappable-but-broken.
- 2026-08-24 13:05 — ✅ Verified in-browser: handle reads "Registreringer (5)", the
  sheet hit-tests as topmost (see task 040's z-index bug), and all five rows render
  newest-first at 60px with Danish weekday+time — "Bandit: Sorte Sofie | man. 23.32"
  down to "Post 1 – Silkeborg Sønderskov | man. 18.40". Tapping the first row panned
  the map and opened that marker's popup.
