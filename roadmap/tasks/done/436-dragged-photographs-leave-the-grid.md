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

## Worth knowing

The grid gets **shorter** the moment a drag starts, by the size of the selection minus the gap. On a
fifty-photograph move that is a visible jump, and it is the honest one: those fifty are in flight. If it
ever reads as disorienting rather than informative, the thing to reach for is not putting the cells back
into the flow — that reintroduces the impossible layout this task removed — but holding the grid's
height for the duration of the drag.

As with 435, the gesture itself is not executed by any test: the suite has no JavaScript runtime.
