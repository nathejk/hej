# 435 — The drag indicator shows which side, not that anything will move

**Status:** done
**Priority:** low
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

Reported while rearranging an album: dragging a selection gives no sense of where the photographs are
going to end up. Asked for "empty placeholder images ... a rectangle with a dashed line" at the drop
position.

Task 396 drew the landing place as a 4px bar down one edge of the cell the photographs would land
beside — `.drop-before` / `.drop-after`, an inset `box-shadow`. That is enough to answer *which side of
this cell*, and it answers nothing else. In particular the grid looks identical before and after the
drop is committed, so a fifty-photograph move and a one-photograph move present the same, and the
curator only finds out what the gesture meant once the server has rebuilt the order and the sheet has
reloaded. The bar is also easy to lose against a thumbnail's own edge.

Opening a gap instead makes the drop's effect visible while it can still be reconsidered: the frames
stand where the photographs will land, and the cells after them shift along.

## What changed

`adminui/albumorder.js`: `.drop-before` / `.drop-after` replaced by `div.dropslot` elements inserted
into `#sheet` as grid items — `openGap(cell, after, count)` puts them before or after the target, and
`clearMarks()` removes them. `#sheet .dropslot` in `page.css` is `aspect-ratio: 4 / 3` (a thumbnail's
shape, so the gap is the size of the hole the move will make rather than a hint beside it), a 3px
dashed blue border and a faint blue fill.

**One frame per photograph being moved, capped at four.** "Vælg alle der matcher filteret" makes a
two-hundred-photograph selection one click, and two hundred frames would shove the cell the curator is
aiming at off the screen — the gesture would become unaimable at exactly the size where rearranging
matters. Four reads as "several", and the ghost under the pointer already carries the true count.

**Two guards against the indicator oscillating**, which are the whole difficulty of this and were not
obvious before it was built. The frames take up room, so inserting them moves the cell under the
pointer, which asks for a gap one place over, every `pointermove`:

- `gapIsOpen(cell, after)` — the same gap can be named two ways, "before the cell after the frames" and
  "after the cell before them". If the pointer resolves to the gap already open, nothing is touched.
- `onSlot(x, y)` — the pointer spends most of a drag over the gap, because the gap opens under it.
  `elementFromPoint` landing on a frame means "no change", not "no target". Deliberately *not* solved
  with `pointer-events: none`, which would have made `elementFromPoint` return `#sheet` and read as
  leaving the grid — closing the gap on every move.

The frames are `aria-hidden` and are not `.cell`, so `contactsheet.js` does not see them: `order`, the
selection re-paint and the listbox's arrow keys all walk `.cell`, and a hole in the order is not
something a curator can select or focus. The request is unchanged — still "these, before (or after)
that one", with `moveAlbumOrder` building the order server-side.

## Acceptance Criteria

- [x] A gap of dashed, thumbnail-shaped, empty frames opens where the photographs will land
- [x] One frame per photograph moved, capped so a bulk selection stays aimable
- [x] The gap does not oscillate as the pointer rests in it or near its edge
- [x] The frames are invisible to the selection, the display order and the keyboard
- [x] The edge bar is gone from both the script and the stylesheet
- [x] Pinned by test, mutation-checked

## Progress Log

- 2026-09-28 — Picked up from a report that the indicator says which side but not that anything moves.
  Plan: insert real grid items rather than paint a cell's edge, so the layout itself shows the drop.
- 2026-09-28 — Built `openGap` / `clearMarks` and the `.dropslot` rule. First pass thrashed exactly as
  expected: the frames move the cell under the pointer, so the gap walked away from the pointer a place
  at a time.
- 2026-09-28 — Fixed with `gapIsOpen` (the two namings of one gap) and `onSlot` (the pointer resting in
  the gap it opened). Considered `pointer-events: none` on the frames and rejected it — it makes
  `elementFromPoint` return `#sheet`, which reads as leaving the grid and closes the gap every move.
- 2026-09-28 — Capped the gap at four frames. A two-hundred-frame gap is technically the honest answer
  and practically unusable: the drop target leaves the screen. The ghost carries the real count.
- 2026-09-28 — ✅ Guard `TestTheDropIndicatorOpensAGapInTheGrid` in `adminmove_test.go`: the mechanism,
  the cap, both anti-oscillation checks, the dashed thumbnail-shaped rule, and that `drop-before` /
  `drop-after` are gone from both languages. Mutation-checked by removing the cap and the `gapIsOpen`
  check — both fail.
- 2026-09-28 — `gofmt` clean, `go test ./cmd/api/ -count=1` green.

## Worth knowing

The behaviour itself is **not** executed by any test: the suite has no JavaScript runtime, and the
pointer gesture is the part where this could still be wrong. The anti-oscillation logic is reasoned and
clicked through, not asserted. If the gap is ever seen to flicker or to drift away from the pointer, the
two functions named above are where to look, and the cap is the knob that changes how violently the grid
reflows.
