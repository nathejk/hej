# 461 — The frontpage's album covers are blurry

**Status:** done
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

Reported by the maintainer:

> the album cover photo is somewhat blury on the album list page

## Which page, and the arithmetic

Checked both pages that list albums, because "album list page" is ambiguous here:

- **The curator's album list** (`/{year}/albums`) — covers are `2.75rem` (44px) from a 320px thumbnail. A 7x
  downscale; it cannot be the blur.
- **The public frontpage's album grid** — this is it, and it is measurable rather than a matter of taste:

  | | |
  |---|---|
  | Card width | `minmax(12rem, 1fr)`, so 192px at its narrowest and **the whole column on a phone** — about 330px |
  | Device pixels wanted | ~660 at the 2x every phone has |
  | Thumbnail | 320px **on its longest edge** |
  | Crop | `aspect-ratio: 1/1` + `object-fit: cover` |
  | Detail actually available | **240px** — a landscape photograph's short edge |

  660 wanted against 240 delivered is a **2.7x upscale**. The 800px rendition has existed since task 409; the
  frontpage simply never offered it.

The square crop is the part that is easy to miss, and it is worth recording: the thumbnail's *stated* size is 320px
but a square crop of a landscape photograph only ever yields its short edge. Task 398 fixed the album page's grid by
reasoning about "the 320px thumbnail" against 10rem tiles — that conclusion still holds at 10rem (240 against 320
wanted is barely soft, and offering the medium there would have an album of 300 photographs fetch and decode 300 of
them), but the same reasoning applied to a 12rem card that stretches to 330px does not.

## What shipped

**`srcset` with both renditions, and a `sizes` that matches the grid** — 320w and 800w, so a 2x screen gets the
800px file and a 1x desktop keeps the 320px one. `src` stays the thumbnail: it is what a browser ignoring `srcset`
gets.

The medium candidate is emitted **only when the rendition exists**. This follows the rule the album page's tile
already records: a photograph uploaded before task 409 has no `mediumRef`, and naming a URL the server would answer
by *falling back* puts 1600px bytes behind an `800w` descriptor. That is correct at the byte level and a lie at the
label level, and a browser does arithmetic with those numbers — it would pick the "smaller" candidate and get the
largest file on the narrowest screen, which is the opposite of the point.

### The cover is now addressed by ref, and the ordinal is gone

Not scope creep — two things forced it, and both are answered by the same widening of the query:

1. **`srcset` needs to know whether the medium exists**, and only the photograph's row can say. The frontpage's
   album summary knew a *position*, not a photograph.
2. **The cover's URL had quietly stopped being cacheable.** Task 456 had to drop `immutable` for ordinal-addressed
   media, because PRD 024 made an ordinal's meaning move under a re-sort. The frontpage addressed its covers by
   ordinal — so the one page an entire event opens at once on the Sunday morning had a **60-second** cache on its
   largest assets. Shipping bigger images into that would have been a straight downgrade.

So `album.Album` carries `CoverRef` + `CoverMediumRef` instead of `CoverOrdinal`, and the cover is picked once as a
photograph id with its renditions joined on — rather than three near-identical correlated subqueries differing only
in their SELECT list. `coverOrder` remains the single statement of the cover rule.

`CoverOrdinal` was **removed** rather than kept alongside: which bytes is one fact, and "which slot, then which
bytes" was two that could disagree. It had exactly one reader.

The view model carries `CoverHasMedium bool`, **not** the medium rendition's ref — mirroring
`publicAlbumItem.HasMedium`. One display ref plus `?variant=` addresses all three renditions, so the derived hashes
stay on the server and the narrowed invariant (PRD 022 §8.4) is not widened for nothing.

## Cost

A 2x visitor now fetches ~90KB per cover instead of ~25KB, so a frontpage with four albums moves from ~100KB to
~360KB of cover imagery. Bounded by three things: `loading="lazy"`, a 1x desktop still taking the thumbnail, and the
bytes being `immutable` for a year again instead of 60 seconds — which is a net improvement for the repeat views
that dominate an event weekend. Worth re-measuring under task 400 all the same.

## Acceptance Criteria

- [x] The frontpage cover is sharp on a 2x display
- [x] A 1x desktop still gets the small file
- [x] No `srcset` candidate is named that the server would answer by falling back
- [x] The cover's bytes are `immutable` again
- [x] Only the display ref reaches the HTML
- [x] The album page's grid is left as task 398 sized it
- [x] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-09-28 — Walked into the backtick trap in `publicsite.go` for the **sixth** recorded time, writing
  `immutable` in backticks inside the raw string — while adding a comment about the frontpage. The comment now says
  so at the site, next to the warning that was already there and that I had just read.
- 2026-09-28 — Mutation-checked by forcing the `srcset` on unconditionally, which fails the no-medium test.
- 2026-09-28 — Deliberately left `.photos` on the thumbnail. Recorded at the code, because "fix the other grid
  too" is the obvious next edit and it would cost 300 decodes per album to fix nothing.
