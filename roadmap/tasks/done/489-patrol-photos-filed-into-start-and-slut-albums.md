# 489 — Patrol photographs filed automatically into "Start" and "Slut" albums

**Status:** done
**Priority:** medium
**Created:** 2026-10-02
**Picked up by:** agent session
**Started:** 2026-10-02
**Completed:** 2026-10-02

**Follows:** task 397 (the diploma-album button, replaced here)

## Description

Maintainer: *"We are listening for `NATHEJK.<year>.patrulje.<teamId>.photographed`; these messages carry a type which
for now can be "start" or "maal". When clicking "Læg diplombilleder i album" these photos should end in one of two
albums: "start" and "slut". Remove button and automatically add new photos to albums. Create album if not already
exists. Only one photo per team per album, last photo wins. Photos should be named after team number (with leading
zeros), album should be sorted by filename low-to-high."*

## Acceptance Criteria

- [x] `start` photographs go to the album "Start", `maal` to "Slut"; each created as a draft if missing.
- [x] One photograph per patrol per album: the newest by capture time. A newer one replaces the older membership.
- [x] Library photographs are named after the patrol number, zero-padded to three digits (`007.jpg`).
- [x] A new album is created sorted `filename-asc`, and the sort mode is applied after every run.
- [x] The button, its route and its handler are gone.
- [x] Runs automatically: once at boot after the projections it reads have caught up, and debounced after every live
      `photographed`, `photopurged` and `photoconsented`.
- [x] Consent: refusing patrols are never filed, and are swept out of every album on each run.

## Progress Log

- 2026-10-02 — `cmd/api/diplomaalbums.go` replaces `admindiplomaalbum.go`. New read `patrolphoto.Latest(year, type)`;
  `photo.Updated` gained `fileName` so a photograph already in the library can be renamed without re-uploading (a
  second `uploaded` would blank its 800px rendition). The boot run waits for the album, photo, public-patrol and
  patrol-photo projections via the stream library's `CatchupListener`, wrapped around them in `main.go`.
- 2026-10-02 — Verified on dev: the boot run created "Start" and "Slut" as `filename-asc` drafts and walked every
  patrol's newest photograph per type. **Every fetch failed because foto is not running locally**, so no photograph
  was filed on dev; the filing path itself is unverified end to end. The old "Diplombilleder" album is left as is.
- Decisions taken without asking: titles are capitalised ("Start", "Slut", slugs `start`/`slut`); the albums are
  managed, so a curator's hand removal of a patrol's current photograph is undone by the next run (deleting the
  photograph from the library is how to keep it out); a curator-added photograph with no patrol tag is left alone.
