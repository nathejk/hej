# 282 — Manual refresh control on the panes holding synced data

**Status:** open
**Priority:** medium
**Created:** 2026-09-16
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] A refresh control on the panes holding synced data, from shadcn-vue primitives.
- [ ] Calls the sync loop's forced `check`; no second fetch path to `/api/sync`.
- [ ] A busy state while the check and any resulting refetch are in flight.
- [ ] A nothing-changed result is acknowledged visibly (and briefly).
- [ ] Offline: the control says so rather than appearing to succeed (PRD 017 §5).
- [ ] Disabled or no-op while a check is already in flight.
- [ ] Nothing shifts under a reading finger when a refetch applies (PRD 017 §7).
- [ ] Component test: tap → forced check; offline tap → no request, notice shown.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1 (§7 manual refresh decision).
