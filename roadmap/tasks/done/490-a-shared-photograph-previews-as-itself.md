# 490 — A shared photograph previews as itself, not as the album cover

**Status:** done
**Priority:** high
**Created:** 2026-10-02
**Picked up by:** agent session
**Started:** 2026-10-02
**Completed:** 2026-10-02

**Follows:** task 467 (the album's share card is its cover), task 447/486/487 (the permalink)

## Description

Maintainer, relaying pushback from users: *"The open graph image for when you share a photo within an album should be
the photo itself, not the cover photo, the link should also be to that specific image. When they share an image,
they expect that image to show, not an album cover."*

The viewer's share button already sent the photograph's permalink (`…/album/{slug}/f/{ref12}`). The permalink 302s
onto `album?foto={ordinal}`, and that page always built its card from the **cover**, with `og:url` naming the bare
album, which a platform then re-scrapes, so the preview collapsed back to the album.

## Acceptance Criteria

- [x] An album page opened with `?foto=` naming a live item uses that photograph (800px rendition) as `og:image`.
- [x] Its `og:url` and canonical link are the photograph's permalink, so a re-scrape comes back to the photograph.
- [x] Title and description stay the album's; never a caption or a credit (`TestNoShareCardNamesAPerson` still holds).
- [x] An unknown or taken-down ordinal falls back to the album's own card.
- [x] Tested through the real path: permalink → redirect → page tags.

## Progress Log

- 2026-10-02 — `publicPageData.CanonicalPath` overrides `og:url` when set; `publicAlbumPageData.Shared` is the item
  `?foto=` names. Reverses the "query is dropped, every share presents as the album" decision on `Path` for this one
  case: the canonical is the ref permalink, not the ordinal, so a re-sort cannot change which photograph it means.
- Facebook caches cards for days: links shared before this deploy keep showing the cover until re-scraped
  (Sharing Debugger → "Scrape again").
