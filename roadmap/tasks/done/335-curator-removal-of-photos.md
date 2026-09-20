# 335 — Curator removal of a photograph or an album

**Status:** done
**Priority:** high
**Created:** 2026-09-19
**Picked up by:** agent session (Zed)
**Started:** 2026-09-19
**Completed:** 2026-09-19

## Description

PRD 011 §0b.2, §6. Permission to publish the album photographs has been obtained upstream — *"if we do
not have permission to show a picture it will not be there"* (maintainer, 2026-09-19) — so this feature
builds **no rights model, no per-subject flag and no consent register**. The safety property is *the
set*: a photograph is in an album because somebody decided it may be shown.

What that leaves is narrow and non-negotiable: **permission is withdrawable, and mistakes happen.** So
a curator must be able to remove a photograph, or a whole album, and have it leave the public page
promptly.

"Promptly" means within the page's cache window, not "at the next deploy". The public pages set
`Cache-Control: public, max-age=60` (see `glimtpublic.go`, where the same short window is chosen so a
takedown lands quickly) — removal must be bounded by that, and the bound should be stated in the code
rather than inferred.

Note this is the one operation on this surface that is **irreversible in effect** even if the row
survives: once a photograph is down, somebody's objection has been honoured, and re-publishing it
should require the same deliberate act as publishing it did. Prefer a soft delete with a visible
state (the `deleted` flag pattern in `checkgroup`, `checkpoint` and `person`) over a destructive one,
so a re-add stays expressible and an accidental removal is recoverable.

Curation tooling as a whole is PRD 011 §11 Q5 and undecided; this task is only the removal path, which
is needed regardless of where the tool eventually lives.

## Acceptance Criteria

- [x] A curator can remove a single photograph from an album, and a whole album, by a documented
      mechanism (whatever §11 Q5 settles on — do not block on the full tool).
- [x] Removal is soft, with a visible state, following the `deleted` flag pattern already used in
      `checkgroup` / `checkpoint` / `person`.
- [x] A removed photograph disappears from every public surface — frontpage cover, album page, map
      markers (task 342), and the media endpoint — within the stated cache window.
- [x] The media endpoint stops serving the bytes too: a removed photograph must not remain fetchable
      by a URL somebody already has.
- [x] The bound on "promptly" is stated in the code, not left to be inferred from cache headers.
- [x] A test asserts a removed item is absent from every public read path, not just the album page.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §0b.2 / §6 / §10 (Phase 1).
- 2026-09-19 — Picked up. Two endpoints: `DELETE /api/albums/{id}/items/{ordinal}` and
  `DELETE /api/albums/{id}`. Authenticated, unlike every other album route — and that asymmetry is the
  feature: albums are *read* by the open web and *changed* by the Team section.
- 2026-09-19 — **Authorization reuses `isGlimtModerator` rather than inventing a curator role.** It
  already answers "does this member currently hold the Team section?", it is a per-request lookup rather
  than a session claim (so revoking the assignment revokes access), and the people who moderate
  photographs in the app are the people who should be able to take one off the public page. A parallel
  notion of curator would be a second thing to assign and a second thing to get wrong. This is
  deliberately narrower than §11 Q5, which is still open about where curation *lives* — removal could
  not wait for that answer, because it is the safety valve behind "permission is withdrawable".
- 2026-09-19 — Event first, bytes second, following `purgeOneGlimt`'s reasoning: if the bytes went first
  and the publish failed, a retry would find a row whose refs are already gone and anything sharing them
  would be blank permanently. Publishing first means a failure leaves the photograph visible — the wrong
  direction for a takedown, but the *recoverable* one.
- 2026-09-19 — Implemented the mirror of task 333's blob fix: removing an album photograph must not
  delete bytes a **glimt** still shows, since content addressing makes identical bytes one object in
  both directions. Same union, same rule — if either owner cannot be asked, nothing is deleted.
- 2026-09-19 — ✅ **A test caught a design bug that would never have failed visibly.** My first version
  called `RefsInUse(year, refs)`, which is a **year-wide** question — so the item being removed reported
  its *own* refs as in use. And because the fold is asynchronous, its row is still live at the moment the
  check runs. The check would have looked correct and **deleted nothing, ever**: no error, no failing
  test, just a disk filling quietly over years. `TestRemovingAnAlbumItemKeepsBytesAGlimtStillUses` failed
  on "the unshared object should have been deleted", which is how it surfaced.
- 2026-09-19 — Fixed by giving `RefsInUse` an **exclusion set** (`[]album.ItemKey`), mirroring glimt's
  `RefsUsedElsewhere(year, glimtID, refs)` but at *item* granularity rather than entity — because one
  photograph can be removed from an album that keeps the rest. The glimt delete path passes nil, with a
  comment saying why: no album item is being removed there, so every one that references the bytes is a
  reason to keep them. The exclusion is applied in Go rather than as a compound `NOT (...)` chain in SQL,
  since the set is one or two items and the alternative is a placeholder-counting bug waiting to happen.
- 2026-09-19 — Added `TestRemovalExcludesTheItemsBeingRemovedFromTheSharingCheck` to lock it in, and
  **verified it by re-breaking the fix**: passing nil again makes it fail with "the removed item's own
  bytes were kept: the sharing check is not excluding it, so nothing would ever be deleted". Restored
  and green.
- 2026-09-19 — An unpublished album's *items* are not removable through the item endpoint, because the
  lookup goes through the published read — one place the publication filter lives, and a second lookup
  would be a second place to forget it. Named in the code as an accepted consequence: a draft is not on
  the public web, and the whole album can be deleted instead.
- 2026-09-19 — A removal of something that does not exist answers 404 and publishes **nothing**, rather
  than putting a no-op on an append-only log. Asserted for unknown ordinals, unknown albums, non-numeric
  ordinals and draft albums.
- 2026-09-19 — "Promptly" is bounded by the page cache (60s, set in task 332), and
  `TestRemovalPromptnessIsBoundedByThePageCache` connects the two explicitly — so if somebody lengthens
  the cache window, a test says why they should not.
- 2026-09-19 — `gofmt`, `go vet ./...` and `go test ./...` clean. Moving to done.
