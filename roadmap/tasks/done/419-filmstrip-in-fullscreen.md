# 419 — The filmstrip is missing from the fullscreen view

**Status:** done
**Priority:** high
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

> *"Now the row of thumbnails has gone from the fullscreen view."* — maintainer, desktop macOS, Brave

Task 418 made `.hv-stage` the fullscreen element and wrote this consequence down as *"the right trade rather
than a regrettable one: fullscreen is for looking at one photograph as large as the screen allows, and the strip
is how you leave it."*

That was wrong, and the mistake is worth naming rather than just fixing. The filmstrip is **how you see where
you are in an album**, and fullscreen is precisely when an album is being looked *through* rather than glanced
at. Arguing that a missing feature is a virtue, in a task whose job was to fix a different missing feature, is
how the last three rounds happened.

### The fix: a frame that is neither the dialog nor the stage

The fullscreen element has to be an ordinary div — not in the top layer — that **contains the whole viewer**. So
the viewer now has one:

```
<dialog class="hv">            ← modal, top layer, refused for fullscreen
  <div class="hv-frame">       ← the fullscreen element: grid, two rows
    <div class="hv-stage">…    ← photograph, arrows, action row, caption
    <div class="hv-strip">…    ← the filmstrip
```

The layout moved from the dialog to the frame, which is the point: **one layout, two containers**, so the same
two rows apply whether the frame is filling the dialog or filling the screen.

### Four targets, three of them wrong

Recorded in the code and asserted by test, because each one is the obvious next guess for somebody who has just
met one of the others:

| Target | Outcome |
|---|---|
| The `<dialog>` | **Refused** — already in the top layer (task 417) |
| `document.documentElement` | **Accepted, renders wrong** — joins the top layer above the dialog (task 416) |
| `.hv-stage` | **Loses the filmstrip** — the strip is its sibling (task 418) |
| `.hv-frame` | **Works** |

## Acceptance Criteria

- [x] The filmstrip is visible in fullscreen, on a viewport wide and tall enough for it
- [x] The photograph, arrows, action row and caption are unchanged in both modes
- [x] The frame carries its own background and backdrop as the fullscreen element
- [x] The guard names all three rejected targets, and asserts the strip is inside the frame

## Progress Log

- 2026-09-25 — Reported. Diagnosed immediately: the strip is a sibling of the stage, and the stage was the
  fullscreen element.
- 2026-09-25 — `.hv-frame` added inside the dialog, holding the stage and the strip; the two-row grid moved from
  the dialog to the frame; `:fullscreen` background and `::backdrop` moved with it.
- 2026-09-25 — `ctx.stage` in the action context became `ctx.frame`. Nothing else used it — the editor finds the
  stage by selector — so the narrow context stays narrow.
- 2026-09-25 — The backdrop-click-to-close check now accepts the frame as well as the dialog, since the frame
  covers the dialog's box entirely and a click that used to land on one now lands on the other.
- 2026-09-25 — `TestTheFilmstripIsInsideTheFullscreenElement` asserts the structure rather than the appearance:
  "is it visible" is not a question this suite can ask, but "is the strip a child of the frame" is, and it is the
  property that makes the answer yes.
- 2026-09-25 — Four rounds on one button. Two lessons, and the second one is mine rather than the platform's:
  **a swallowed rejection cost two rounds** (tasks 416–417, fixed by reporting it), and **this round was caused by
  writing a limitation up as a virtue.** When a fix removes something, that is a cost to report, not a design to
  justify — the person testing it will notice within the minute either way.
- 2026-09-25 — ✅ All criteria met. `gofmt`, `go vet`, full `go test ./cmd/api/` clean.
