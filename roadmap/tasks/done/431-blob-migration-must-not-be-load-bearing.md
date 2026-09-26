# 431 — The blob layout migration must never be load-bearing

**Status:** done
**Priority:** high
**Created:** 2026-09-26
**Picked up by:** agent
**Started:** 2026-09-26
**Completed:** 2026-09-26

## Description

Task 429 split the blob store into `original/` and `cache/` and migrated existing objects into
`original/` on boot. The first deploy of that change **served no photographs at all**, and the
symptoms reported were exactly diagnostic: *files moved into `/original`, no `/cache` created, no
photos served.*

Two mistakes, and the second is the one that mattered.

**1. A failed migration took the whole store down.** `NewFileStore` returned the migration's error.
`cmd/api` (main.go:758) treats that as "blob store unavailable" and falls back to an **empty
in-memory store** — deliberately, and correctly, for the case it was written for. So a single
un-renameable directory meant no photographs anywhere in the service, and no `cache/` would ever
appear because nothing was writing to a filesystem at all.

Before task 429, `NewFileStore` could only fail on `MkdirAll`/`Chmod` of the root. The migration
added a large new surface of ways to fail — and pointed it at the one error path that disables the
entire feature.

**2. `locate` searched only the two new subtrees.** This made a *successful* migration a
precondition for reading anything. A migration that failed, or got halfway, silently hid objects
that were sitting right there on disk. I considered exactly this when writing task 429 and chose
"migrate rather than fall back", on the reasoning that the migration runs every boot and converges.
That was wrong: it converted housekeeping into a precondition.

**3. Found by the regression test, not by me:** `locate` also treated any non-`ErrNotExist` stat
error as fatal to the whole lookup, so one unreadable location (a file where `original/` should be a
directory, an `EIO` on a failing disk) made **every** object in the store unreadable.

## The rule this restores

**The layout migration is housekeeping for backup hygiene, and nothing may depend on it having
run.** Reads look in the legacy location too, so the store works whether or not it ever ran — which
also covers a case with no migration in it at all: an operator restoring an old backup lands objects
in exactly that layout.

## What changed

- `locate` searches `original/`, `cache/`, then the legacy root. `legacyDir` is the empty string, so
  `filepath.Join` drops it and it costs no special case.
- A stat error in one location no longer aborts the search: it is remembered and returned only if
  the object was found nowhere, so a genuine I/O fault still surfaces.
- `NewFileStore` no longer fails when the migration does. It records the error; `MigrationError()`
  exposes it and `cmd/api` logs it at ERROR, naming the real consequence — *those objects are
  outside the `original/` backup path* — without taking the store offline.
- `Delete` reaches the legacy location, or a takedown would leave an un-migrated photograph on disk.
- **`PutAs` now refuses a ref that exists in the legacy location too.** The subtle one: a legacy
  object is an original as far as anything can tell, and `locate` checks `cache/` *before* the
  legacy path — so without this, a rebuilt rendition would shadow an un-migrated original and every
  read would quietly return the re-encode instead of the photograph.

Deliberate asymmetry worth keeping: `locate` degrades on a stat error, `PutAs` fails closed on the
same error. A read degrading to "no photo" is documented behaviour (PRD 008 §8); a write guard that
degraded would let a rebuild overwrite something irreplaceable.

## Two things fixed alongside

- **Log spam from task 430.** A rendition whose *source* is also gone is permanently unrebuildable,
  and glimt media is purged on retention (PRD 019) — so dev logs filled with `WARN rebuilding a
  missing rendition … blob not found`, once per request per dead item. That case is now `DEBUG` and
  says what it means; genuine failures stay `WARN`. This is the negative-caching gap task 430 noted,
  in its cheap form.
- **`cqrs.EnsureIndex` already existed.** Task 409 added a private `ensureIndex` to
  `nathejk/table/photo` with a comment asserting the library had no such helper. It does, with the
  same signature. Replaced; the duplicate is gone.

## Acceptance Criteria

- [x] An object in the legacy location is readable, and deletable, without having been migrated
- [x] A failed migration yields a working store, reports itself, and is visible in the boot log
- [x] One unreadable location cannot hide objects in the others
- [x] `PutAs` cannot shadow an un-migrated original
- [x] A permanently unrebuildable rendition does not log at WARN on every request
- [x] Both original mistakes are covered by tests that fail when reintroduced

## Progress Log

- 2026-09-26 — Reported: no photos on the public site after the task 429/430/409/410 deploy, with
  `/original` populated and no `/cache`.
- 2026-09-26 — Investigated rather than guessed. Ruled out the schema: dev's `photo` table has
  `mediumRef` with its key, and every media variant (`full`, `medium`, `thumb`) answers 200 locally.
  Audited all 23 photo refs against the volume — 3 absent, none of them the ones in the log. The
  logged repair failures turned out to be **glimt** items whose media was purged on retention, i.e.
  pre-existing absence surfaced as new noise.
- 2026-09-26 — Found the mechanism by reading main.go:758: a failed `NewFileStore` falls back to an
  empty memory store, which explains all three reported symptoms together, and `migrateLegacyLayout`
  is a new way to fail there.
- 2026-09-26 — ✅ Fixed the fragility: legacy fallback in `locate`, non-fatal migration with
  `MigrationError()` surfaced in the boot log, `Delete` and the `PutAs` guard extended to the legacy
  location.
- 2026-09-26 — `TestAFailedMigrationStillYieldsAWorkingStore` then failed for a *third* reason and
  found the stat-error bug: one broken location made every object unreadable. Fixed by remembering
  the first error and only returning it when nothing was found anywhere.
- 2026-09-26 — Mutation-checked both original mistakes: removing the legacy fallback fails
  `TestObjectsAreReadableBeforeTheyAreMigrated` ("served zero photographs") and
  `TestPutAsRefusesToShadowAnUnmigratedOriginal`; making the migration fatal again fails
  `TestAFailedMigrationStillYieldsAWorkingStore`.
- 2026-09-26 — `gofmt`, `GOWORK=off go test -timeout 5m ./...` and `go tool staticcheck ./...` clean.

## Still to confirm on the production host

The fix makes the store correct regardless of which failure occurred, but **the production cause was
not directly observed** — the dev environment serves photographs correctly both before and after.
Worth reading from the deploy log, because it distinguishes the two cases and one of them means
something else is also wrong:

```
grep -E 'blob store (ready|unavailable)|layout migration' <the api log>
```

- `blob store unavailable, falling back to memory` → this was it, and this fix resolves it.
- `blob store ready` with no migration error → the blob store was fine and the cause is elsewhere;
  reopen with the media route's status code for one photograph.
