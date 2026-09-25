# 416 — The caption pushes the arrows around, and fullscreen does nothing

**Status:** done
**Priority:** high
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

Three things from the maintainer's second pass over the viewer (PRD 023), desktop macOS, Brave:

1. *"Put the caption/credit on top of the photo in a semi transparent grey box, rounded corners — right now it
   takes up space and changes the position of the left/right arrows."*
2. *"It's fine that right/left arrows are relatively small, but make sure that I can click a larger area."*
3. *"The fullscreen icon is not working, share icon is fine."*

### 1. The caption was moving the arrows

The info panel was a row of the dialog's grid, so it took its own height: a captioned photograph made the stage
shorter than an uncaptioned one, and the arrows — centred in the stage — sat somewhere else from one photograph
to the next. Furniture moving under the cursor between one press and the next, which feels broken without being
nameable.

It is now positioned over the photograph, inside the stage, so the geometry is identical for every photograph.
Semi-transparent grey, rounded, centred at the bottom, and `pointer-events: none`: nothing in it is interactive,
and it sits where a swipe starts, so a caption that swallowed a swipe would be a caption that broke the album.

### 2. The visible arrow and the clickable arrow are different sizes

There was never a reason for them to be the same number. Each arrow is now a tall column down the side of the
photograph — 22% of the width, capped — with the visible circle painted on the icon inside it. Easy to hit with a
thumb on a phone and with a wandering cursor on a laptop, while looking exactly as small as it did.

### 3. Fullscreen asked the wrong element

`requestFullscreen()` was called on the `<dialog>`. That dialog is in the **top layer**, put there by
`showModal()`, and a top-layer element is the awkward case for the fullscreen API: two mechanisms that both mean
"render this above everything", with engines disagreeing about what the combination means. In Brave on macOS the
result was a button that appeared and did nothing.

It now asks `document.documentElement`, which sidesteps the argument and looks identical — the dialog already
covers the viewport and stays in the top layer over it, so there is nothing of the page left to see.

**And the refusal is now reported.** The first version caught the rejected promise and did nothing, which is
exactly how this became a bug report instead of a message on screen. That swallowed `catch` is the more
important half of this fix: the wrong element was a mistake, and the empty catch block is what hid it.

## Acceptance Criteria

- [x] The caption and credit sit over the photograph in a rounded, semi-transparent grey box
- [x] The arrows do not move when a photograph has no caption
- [x] The arrows keep their size and gain a much larger clickable area
- [x] Fullscreen works from the viewer, and a refusal says so rather than passing in silence
- [x] Each of the three is guarded by a test

## Progress Log

- 2026-09-25 — Reported by the maintainer, second pass in Brave on macOS.
- 2026-09-25 — Info panel moved into the stage and positioned over it; the dialog's grid is now two rows rather
  than three. `pointer-events: none` on it, so it cannot eat a swipe or an arrow press.
- 2026-09-25 — Arrows: the button is the hit area and the icon carries the visible circle. Keeping those as two
  separate properties is the whole trick, and doing it in CSS means the markup stays one button with one icon in
  it.
- 2026-09-25 — Fullscreen retargeted to the document element, with a note in the code about why the obvious
  target was wrong. The swallowed `catch` replaced by a Danish line in the panel.
- 2026-09-25 — The admin editor panel now inserts before the filmstrip rather than at the end of the dialog:
  "under the info panel" stopped meaning anything once the info panel moved over the photograph, and an editor
  below the strip reads as belonging to the strip.
- 2026-09-25 — Guards: `TestTheCaptionOverlaysThePhotographRatherThanMovingIt`,
  `TestTheViewerArrowsHaveALargerHitAreaThanIcon`, `TestFullscreenAsksThePageAndReportsRefusal`, plus a small
  `ruleFor` helper so a guard about one CSS decision reads as one. All three verified to fail against the broken
  versions.
- 2026-09-25 — Process note for me, twice now: `git checkout <file>` to undo a deliberate break-test also threw
  away the uncommitted fix it was testing, costing a full reapply both times. **Commit the fix first, then
  break-test against the commit.**
- 2026-09-25 — Worth stating plainly, since this is the second round of bugs found by looking rather than by
  testing: all three of these were about *rendered geometry and platform behaviour*, which no test in this repo
  can observe. The guards added here stop a regression, but they could not have found any of it. Task 411 is not
  optional on this feature.
- 2026-09-25 — ✅ All criteria met. `gofmt`, `go vet`, full `go test ./cmd/api/` clean.
