# 462 — The album grid and the viewer showed different photographs

**Status:** done
**Priority:** high
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

Reported by the maintainer, with two screenshots of the album "Test": the page's grid in one order, the viewer's
filmstrip in another.

> there is a different sorting order when opening an album than the overview

**Neither order was wrong. They were different photographs at the same positions**, which is worse than a sorting
bug and reads exactly like one.

## Diagnosis

Ruled out by looking rather than by reasoning, in this order:

1. **Duplicate ordinals.** `SELECT COUNT(*), COUNT(DISTINCT ordinal)` on the album — 10 and 10. Not it.
2. **The viewer sorting its items.** `itemsIn` is DOM order and `fillStrip` walks `ui.items`; nothing sorts. Not it.
3. **The two surfaces disagreeing.** Fetched both the public page and the admin fragment for the real album: both
   came back in album order, identically.
4. **Which items are which.** With no image library to hand, the two flat gradient placeholders were identified by
   compressed size (≈4 KB against ≈15–23 KB for real photographs). In album order they sit at positions **3 and 8**.
   The **filmstrip** had them at 3 and 8. The **grid** had them at 8 and 9.

So the filmstrip was right and the grid was not — on the same page, from the same DOM. That narrows it to the one
thing that can differ between a tile and the viewer: **the URL each fetches.**

```
<a class="tile" href=".../media/{{.Ref}}"          ← task 447
   data-full=".../media/{{.Ref}}"                  ← task 447
   data-medium=".../media/{{.Ref}}?variant=medium" ← task 447
   data-thumb=".../media/{{.Ref}}?variant=thumb">  ← task 447
  <img src=".../media/{{.Ordinal}}?variant=thumb"> ← missed
```

**My own bug, from task 447.** I converted the anchor's four attributes to refs and left the `<img src>` one line
below on the ordinal.

### Why it was invisible, and why it looked like a sorting problem

- **A cold cache renders both correctly.** The ordinal URL resolves through the album, so a first visit gets the
  right bytes from either address.
- The damage needs one re-sort plus a warm cache. This route served ordinal URLs `immutable, max-age=1y` until
  task 456, so every browser that had loaded the album before the re-sort keeps answering *each position* with the
  photograph that used to be there — for a year, with no revalidation. The grid is stale, the viewer is live, and
  they disagree.
- This is precisely the failure task 456 wrote up ("the page updates, the images do not"). Task 456 fixed the
  *header*; it could not un-poison caches, and refs were the actual remedy — applied to four of five places.
- **The tests pinned the bug.** `TestAlbumPageRendersItsPhotographs` and two credit tests asserted
  `media/0?variant=thumb` and `media/1?variant=thumb`. And the structural guard,
  `TestTheAlbumPageNeverPutsADerivedRenditionRefInItsHTML`, requires *a* display ref to be **present** — which the
  anchor satisfied. Every test agreed with the broken line.

## What shipped

The tile's `<img src>` uses the display ref, like the four attributes beside it. Three tests moved off the ordinal
form, and two guards were added that would have caught it:

- **`TestNoPublicPageAddressesAlbumMediaByPosition`** — the rule stated as a rule, over the whole album page *and*
  the frontpage, rather than per attribute. A position is not an address for bytes; `?foto=N` is untouched, because
  that is the page's own state (task 401).
- **`TestTheTileImageAndTheViewerAgreeOnThePhotograph`** — per tile, the thumbnail URL appears **twice** (the
  `img src` and `data-thumb`) and `data-full` names the same photograph. The failure was never a missing ref, it
  was two addresses for one tile that stopped agreeing, so that is what is asserted.

Mutation-checked by putting `{{.Ordinal}}` back: five failures across the two tests.

### A side effect worth having

The grid is the heaviest thing on an album page — up to 300 thumbnails — and it was the one part still addressed by
position, so task 456 had reduced it to a 60-second cache. Content-addressed, those tiles are `immutable` again.

## For the maintainer

A **hard reload** (⇧⌘R) will fix what you are looking at. The stale entries are keyed by the old ordinal URLs, and
nothing on the page requests those any more — but a normal reload may still serve the page itself from cache.

## Acceptance Criteria

- [x] The tile image and the viewer show the same photograph at the same position
- [x] No public page addresses album media by ordinal
- [x] A guard that states that as a rule, not per attribute
- [x] A guard that the tile's two addresses agree
- [x] Album tiles are `immutable` again
- [x] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-09-28 — Found by fingerprinting the images by compressed size, after the DB and both surfaces' HTML all
  came back in the correct order. The evidence that mattered was "the grid and the strip disagree **within one
  render**", which leaves only the URLs.
- 2026-09-28 — The first draft of the new regex, `/media/\d+`, reported the fixture's own refs: a ref is hex and
  may begin with digits. Anchored with a trailing `["?]`.
- 2026-09-28 — Worth recording as a general lesson: **four out of five is a silent state.** The four converted
  attributes were the ones a test could see, and the fifth was the one a person could see.
