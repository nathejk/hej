# 436 — The photographs being dragged still held their place in the grid

**Status:** done
**Priority:** low
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

Follow-up to task 435, from the same curator: once the gap opens, the originals should not take up space
in the grid — they should float over it, following the mouse.

Task 396 dimmed the moving cells in place (`opacity: 0.35`) and carried a text pill reading "Flytter 12
billeder". Task 435 then opened a gap for them to land in. The two together produce an arrangement that
**cannot ever exist**: the grid is showing every photograph *and* a hole for twelve of them, so it is
longer than the album by the size of the gap, and every cell after the gap sits one gap-width away from
where it will actually end up. The gap was supposed to show the curator the result, and it was showing
the result plus the thing the result removes.

## What changed

**The moving cells leave the layout.** `#sheet .cell.dragging` is now `display: none`. What is on screen
during a drag is the album minus the selection, with one gap in it — which is exactly what releasing the
pointer will commit. Still only a class, never a detach, so Escape or a cancelled pointer puts the cells
back with their selection intact and nothing to rebuild.

**The pointer carries the photographs.** `carry()` replaces the text pill with a small fanned stack of
thumbnail clones and puts the count underneath it. `cloneNode(false)` on the cell's own `<img>`: the
browser has already decoded that image, so the copy costs nothing and cannot disturb the cell it came
from. The grabbed photograph is carried first, so the one under the pointer is the one the curator took
hold of.

**At most three thumbnails** (`MAXCARRIED`). A stack is a handful, not an inventory: beyond three the
fan stops being legible, and a selection may include photographs that are not loaded and have no
thumbnail to carry at all. The label underneath holds the true number, which is also why it is still
there.

**One frame per vacated cell** (corrected the same day — see the log). The gap is as big as the hole the
selection left, so the album keeps its length: the grid is rearranged, not resized, nothing below it
moves, and the scroll position keeps meaning what it meant. Task 435's cap of four was written while the
cells were still dimmed in place, when the gap was pure addition and a two-hundred-frame gap really would
have pushed the drop target off the screen. Once the cells leave the flow the arithmetic cancels, and the
cap stopped being a safeguard and started being the thing that made the grid shrink.

The fan (`rotate(-5deg)` / `rotate(4deg)`, straight on top) is what makes three read as three; squared up
they would look like one photograph with a thick border. A single photograph is carried straight
(`:only-child`).

Nothing about the request changed. The `dragging` test in `move()` is now belt and braces — a cell out of
the layout cannot be returned by `elementFromPoint` — and was kept and commented, because the server
refuses a move whose target is one of the photographs moving and this is the client side of that rule.

## Acceptance Criteria

- [x] The photographs being moved take up no space in the grid while in flight
- [x] They follow the pointer as the photographs themselves, not as a label
- [x] At most three are carried, with the true count shown
- [x] The photograph under the pointer is the one that was grabbed
- [x] A cancelled drag restores the cells and the selection exactly
- [x] The grid keeps its length: one frame per cell taken out of it
- [x] A page arriving mid-drag cannot put an in-flight photograph back in the grid
- [x] Pinned by test, mutation-checked

## Progress Log

- 2026-09-28 — Picked up. Reported against 435: with the gap open, the dimmed originals mean the grid
  shows the album *and* a hole for part of it, so the layout under the pointer is one that will never
  exist.
- 2026-09-28 — `display: none` on `.dragging` rather than removing the nodes. Detaching would have meant
  rebuilding them to cancel a drag, and the selection is painted on those cells; a class costs nothing
  and is reversible by construction.
- 2026-09-28 — Replaced the text ghost with `carry()`: cloned thumbnails, grabbed one first, count label
  below. Clones rather than moved nodes for the same reversibility reason.
- 2026-09-28 — Capped at three and fanned them. Four or more overlap into an unreadable smear, and the
  count was already the authority on how many are moving — the stack only has to say "these, and
  several".
- 2026-09-28 — ✅ Guard extended in `adminmove_test.go`: the clone, the cap, the grabbed-first ordering,
  and that `#sheet .cell.dragging` takes the cell out of the grid. Mutation-checked by putting
  `opacity: 0.35` back — it fails.
- 2026-09-28 — `gofmt` clean, `go test ./cmd/api/ -count=1` green.
- 2026-09-28 — Reopened immediately: shipped with 435's four-frame cap still in place, so the grid got
  *shorter* by the selection minus four. I had even written that up as acceptable, on the grounds that the
  jump was honest. The curator's point stands and mine did not: the frames exist to stand in the vacated
  positions, and the whole intent was to keep the grid's length rather than change it in either direction.
  The cap's original justification died when the cells left the flow — with them gone the arithmetic
  cancels — and I carried it across without re-deriving it.
- 2026-09-28 — Cap removed. `makeSlots(n)` builds one frame per cell now classed `dragging`, counted off
  the grid rather than off the selection: "vælg alle der matcher filteret" names photographs that were
  never loaded, and those vacated nothing.
- 2026-09-28 — The frames are now built once per drag and *moved* between gaps (`showGap` appends them to
  a fragment, which detaches them, then inserts). At two hundred frames, rebuilding on every pointermove
  would have been two hundred elements per event. `gapShown()` reads the DOM (`slots[0].parentNode`)
  instead of a second flag, so there is no "is it shown" state to fall out of step.
- 2026-09-28 — Found a consequence while reasoning about the count: the auto-scroll can trigger the
  infinite scroll, so a page can arrive mid-drag carrying photographs that are in flight. They would have
  landed as ordinary cells — droppable onto themselves, refused by the server, and one cell more than the
  album has room for, which would have made the grid grow after all. An `htmx:afterSwap` listener takes
  them out on arrival and gives the gap a frame each.
- 2026-09-28 — ✅ Guard updated to the count rather than the cap. Mutation-checked with `makeSlots(4)`.
  `go test ./cmd/api/ -count=1` green again.

## Worth knowing

The grid's length is now invariant across a drag, which is the property to protect if this code is
touched: it holds only because every hidden cell has exactly one frame, and both the `htmx:afterSwap`
listener and the count being read off `.cell.dragging` rather than off the selection exist to keep that
true in the awkward cases.

As with 435, the gesture itself is not executed by any test: the suite has no JavaScript runtime.
