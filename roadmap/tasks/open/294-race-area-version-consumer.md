# 294 — Tell a user when their downloaded map area no longer matches the event

**Status:** open
**Priority:** low
**Created:** 2026-09-16
**Picked up by:**
**Started:**
**Completed:**

## Description

Split out of task 287 (PRD 017). The sync check reports a `race_area` version, and unlike
the other five datasets there is **nothing on the client to refresh with it**: the race
area is fetched on demand by `helpers/offline/tileBulk.ts` at the moment a user starts a
bulk tile download, and no copy is kept. So `refreshIfVersionDiffers` had nothing to do
for it, and inventing a store just to have somewhere to put the version would have been
the wrong shape.

What a changed race area actually means is a **user-visible fact, not a refresh**: the
tiles on this device were downloaded for a smaller (or differently-shaped) area than the
event now has, so there is map the user thinks they have offline and does not. That
matters at exactly the moment it cannot be fixed — out of signal, at night, at the edge of
the area.

The honest scope is therefore: record the race-area version the tiles were downloaded
against, compare it to the one the sync check reports, and if it moved, tell the user on
the offline/readiness surface that more map is available to download. Not a silent
re-download — a few hundred megabytes must stay a decision the user makes, which is the
existing contract of that screen.

Worth noting the version only moves when the *hull* moves, i.e. when a checkpoint gains a
position or the set changes. Early in the year that is often; during the race it should be
never. If it moves mid-race, that is itself worth an organizer knowing about.

## Acceptance Criteria

- [ ] The race-area version is recorded when a bulk tile download completes.
- [ ] The sync loop's `race_area` version is compared against it.
- [ ] A difference is surfaced on the offline/readiness screen as "more map available",
      in Danish, alongside the existing affordances.
- [ ] **No automatic re-download.** The user decides; the screen's existing contract.
- [ ] Nothing is claimed when no tiles have ever been downloaded (no area held → nothing
      to be stale).
- [ ] A device that downloaded tiles before versions existed does not nag on first run —
      absent is not stale.
- [ ] Test: version moves after a download → prompt; version unchanged → silence.

## Progress Log

- 2026-09-16 13:30 — Task created, split out of task 287. See that task's log: the
  race-area dataset has no client store, so it needs a feature rather than a refresh.
