# 443 — The sort itself

**Status:** open
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 024 §6 R2/R4/R6. Needs 441 and 442.

One function from (the album's live items, their photographs, the mode) to an order — the same shape
`moveAlbumOrder` has, and testable the same way: a list in, a list out, no database.

- `time-asc` / `time-desc` on `shotAt`, falling back to `uploadedAt` where it is null.
- `filename-asc` / `filename-desc` on the filename, **case-insensitively**: `IMG_*.JPG` and `img_*.jpg`
  from two cameras in one album would otherwise separate into two blocks.
- **Every mode is a total order**, with ties broken on `photoId` — the rule the library read already uses.
  Without it, two recomputations of an unchanged album can differ, which would reshuffle unrelated
  photographs on an add and change the public page for no reason a curator could explain.
- `manual` returns the existing order untouched, so callers need no special case.

## Acceptance Criteria

- [ ] One pure function, table-driven tests per mode
- [ ] The `shotAt`-null fallback is covered, including a mix of null and set in one album
- [ ] Case-insensitive filename order is covered
- [ ] Sorting an already-sorted album is a no-op (the property an add depends on)
- [ ] Every photograph missing both keys still yields a stable order
- [ ] Items whose photograph is not in the library keep a defined position rather than vanishing

## Progress Log

- 2026-09-28 — Task created from PRD 024 §6 R2/R4/R6.
