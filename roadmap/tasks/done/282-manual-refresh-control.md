# 282 — Manual refresh control on the panes holding synced data

**Status:** done
**Priority:** medium
**Created:** 2026-09-16
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-16
**Completed:** 2026-09-16

## Description

PRD 017 phase 1, §7. The interval decides how fresh the app is when nobody is asking;
this control decides how fresh it is when somebody is. It is what makes the interval's
exact value low-stakes, so it ships with the mechanism rather than after it.

Wire it to the loop's returned `check` (forced, per task 281) so there is one path to
`/api/sync` rather than a second one that drifts.

**It must acknowledge the tap even when nothing changed.** A check that finds nothing
is the expected outcome, and a control that gives no feedback in the common case reads
as broken and gets tapped repeatedly — which is exactly the traffic the debounce
exists to prevent.

Placement: the panes that hold synced data (contacts, map/scan drawer, profile).
Pull-to-refresh alone is not sufficient — it collides with a scrolling list and has no
natural home on the map — so use an explicit control. Prefer a standard shadcn-vue
component; use Lucide `RefreshCw` for the icon (repo rules).

## Acceptance Criteria

- [x] A refresh control on the panes holding synced data, from shadcn-vue primitives.
- [x] Calls the sync loop's forced `check`; no second fetch path to `/api/sync`.
- [x] A busy state while the check and any resulting refetch are in flight.
- [x] A nothing-changed result is acknowledged visibly (and briefly).
- [x] Offline: the control says so rather than appearing to succeed (PRD 017 §5) — but in
      its own words, about the refresh rather than the connection. See the log.
- [x] Disabled or no-op while a check is already in flight.
- [x] Nothing shifts under a reading finger when a refetch applies (PRD 017 §7).
- [x] Component test: tap → forced check; offline tap → no request, notice shown.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1 (§7 manual refresh decision).
- 2026-09-16 16:00 — Picked up.
- 2026-09-16 16:05 — **Two placements, not three.** The task said "the panes holding synced
  data", but contacts, profile and every other non-map pane share the app bar, so one
  instance there covers them — and the app bar is where a user looks for "is this current?".
  The map needs its own because that route is full-bleed and has no header; it went into the
  scan drawer's header, which is the surface whose content it refreshes. Three per-view
  copies would have been three integrations of one tap with three chances to drift.
- 2026-09-16 16:10 — **`refreshNow()` is a module-level entry to the app's single loop**, not
  a second loop. A control anywhere can reach *this* loop; with no loop running it answers
  `'skipped'` rather than starting one, because a loop created by a button press would live
  outside the app's lifecycle and never be stopped. Module-level rather than provide/inject
  because the loop is a singleton by design (PRD 017 §6) and a stray provider could otherwise
  create a second silently.
- 2026-09-16 16:15 — **A `SyncOutcome` was needed, and "skipped" had to be distinguishable
  from "unchanged".** Telling somebody "alt er opdateret" on the strength of a check that
  never ran (dropped by the overlap guard) is exactly the confident wrong answer this control
  exists to remove, so `refreshNow` counts completions rather than reading the last result.
- 2026-09-16 16:20 — **A test caught a real flaw:** after a 401 the loop is gated off, so a
  tap reported `'skipped'` — i.e. "Opdaterer allerede …" — to a user whose session had
  expired. Wrong, and wrong in the direction that keeps them tapping. `refreshNow` now
  answers `'unauthenticated'` without attempting a check.
- 2026-09-16 16:25 — **The offline wording broke `offlineIndicator.spec.ts`, correctly.**
  My first draft said "Ingen forbindelse. Prøv igen, når du har signal." — duplicating the
  one global offline vocabulary that PRD 009 §6 (task 188) reserves for `OfflineNotice`, and
  that a structural test enforces. PRD 017 §7 says the same thing from the other direction:
  do not add a second vocabulary for staleness. Reworded to speak only about the *action*
  ("Kunne ikke opdatere."), leaving the banner above it to say why. The `offline` and `error`
  statuses are still distinct in the API, because "prøv igen" is advice and giving it to
  someone whose signal is fine is a lie they will act on.
- 2026-09-16 16:30 — Completed. `SyncRefreshButton.vue` (shadcn `Button`, Lucide `RefreshCw`,
  `aria-live="polite"`, spinner while busy, message cleared after 4 s so the pane never
  carries a stale claim), placed in `App.vue`'s header and `ScanList.vue`'s drawer header;
  `syncRefresh.spec.ts` with 7 tests on the seam underneath it. 708 tests green, type-check
  clean.
