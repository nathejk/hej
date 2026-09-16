# 294 — Tell a user when their downloaded map area no longer matches the event

**Status:** done
**Priority:** low
**Created:** 2026-09-16
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-16
**Completed:** 2026-09-16

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

- [x] The race-area version is recorded when a bulk tile download completes.
- [x] The sync loop's `race_area` version is compared against it.
- [x] A difference is surfaced on the offline/readiness screen as "more map available",
      in Danish, alongside the existing affordances.
- [x] **No automatic re-download.** The user decides; the screen's existing contract.
- [x] Nothing is claimed when no tiles have ever been downloaded (no area held → nothing
      to be stale).
- [x] A device that downloaded tiles before versions existed does not nag on first run —
      absent is not stale.
- [x] Test: version moves after a download → prompt; version unchanged → silence.

## Progress Log

- 2026-09-16 13:30 — Task created, split out of task 287. See that task's log: the
  race-area dataset has no client store, so it needs a feature rather than a refresh.
- 2026-09-16 19:30 — **`/api/race-area` now carries its own `version`.** The first design took
  the version from the last sync check, which is a race: the two calls are seconds apart, and if
  the area moved in between, the device would store a version *newer* than its tiles and then
  never be told about that change — a silent failure of the exact notice this task adds. One
  field on a response the downloader already makes removes the race. The handler hashes the area
  it is returning rather than calling `raceAreaVersionFor`, which would consult a 5 s cache and
  could describe a different area than the one in the body; a test asserts the two agree.
- 2026-09-16 19:40 — **`updateAvailable` is a new field on `OfflineDatasetStatus`, not a
  `problem`.** Nothing has failed and what the device holds is still correct as far as it goes.
  It is not `stale` either — that says a copy may be old, where this says the *event* has grown
  past it. Only tiles use it, because tiles are the one dataset the app will not refetch on the
  user's behalf.
- 2026-09-16 19:45 — **Recorded only on a complete run.** A partial download does not cover the
  area it was told about, so storing its version would suppress the very notice the user needs:
  they would be told nothing when the area later moved, having never had all of it. Cleared when
  the tiles are cleared, for the mirror-image reason — an orphaned version could announce that a
  map the user no longer has is out of date.
- 2026-09-16 19:50 — **Two rules about not speaking**, both in `tileAreaIsStale` with tests: no
  held version means no claim (a device that never downloaded tiles has nothing to be stale, and
  one that downloaded them before this shipped must not be nagged on the strength of a value we
  never stored), and no *served* version means no claim either (the sync check omits `race_area`
  for a caller who cannot hold it, and lists it as unavailable when derivation failed — neither
  is evidence anything moved).
- 2026-09-16 19:55 — Device-wide storage key, deliberately not profile-scoped like the contacts
  and map caches: the race area is neither personal nor patrol-scoped, and a sibling switching
  profiles on a shared phone must not be told to re-download 324 MB that is already there and
  already correct.
- 2026-09-16 20:00 — The readiness line is suppressed while a download is running: "more is
  available" beside a progress bar reads as though the running download is short.
- 2026-09-16 20:05 — Completed. `helpers/offline/tileAreaVersion.ts` (+10 tests), `version` on
  the race-area response and type, `updateAvailable` through the offline store and
  `OfflineReadiness.vue`, and the `race_area` dispatch in `useSyncLoop` that was a no-op since
  task 286. Frontend 733 tests + type-check + production build green; all four Go gates green.
