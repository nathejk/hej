# 266 — Emphasise checkpoint scans and render verdict badges in the drawer

**Status:** open
**Priority:** medium
**Created:** 2026-09-15

## Description

PRD 016 phase 4. `ScanList.vue` already exists and is already wired into `MapsView.vue`,
using the shadcn-vue `Drawer` primitive — so this is an upgrade of a live component, not a
resurrection (PRD 016 §11.10).

- **Checkpoint scans get the emphasis**: stronger icon treatment than other registrations,
  plus a verdict badge — "på tid" (green) or "for sent" with the delta (amber). Bandit
  catches keep their current red skull styling.
- **No badge at all** when the checkpoint has no window, or when a relative window has no
  anchor yet. Absence is the honest rendering; a grey "ukendt" badge on half the rows is
  noise.
- An unattributable scan is listed plainly, with no checkpoint name and no verdict.
- Keep the existing behaviour: tapping a positioned row pans the map underneath and closes
  the drawer; un-positioned rows show `MapPinOff` and are not tappable.
- Danish copy; `da-DK` time formatting (already in the file).
- Lucide icons only. Prefer standard shadcn-vue components — use `Badge` rather than
  hand-rolling one.
- Touch targets ≥ 44 px (the existing rows are `min-h-[3.25rem]`).

## Acceptance Criteria

- [ ] Checkpoint rows visually distinct from bandit rows.
- [ ] Verdict badge renders on time / late-with-delta, and is absent when unknown (tests).
- [ ] Unattributable scans render without a checkpoint or verdict.
- [ ] Existing pan-and-close and un-positioned behaviour unchanged (tests still pass).
- [ ] Uses shadcn-vue `Badge`; no PrimeVue anywhere.
- [ ] Long checkpoint names truncate rather than wrap the row.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 4.
