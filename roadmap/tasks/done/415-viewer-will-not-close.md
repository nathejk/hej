# 415 — The viewer will not close, and a shared link opens it with no controls

**Status:** done
**Priority:** high
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

Two bugs found by the maintainer in the first real use of the viewer (PRD 023, tasks 402–405), on desktop macOS
in Brave:

1. *"There is no way to get out of the viewer. When the cross in the top right corner is clicked, the current
   photo disappears but the viewer persists. Nothing happens when ESC is pressed."*
2. *"There is no fullscreen icon, and no share icon."*

Both are real, both were invisible to the whole Go test suite, and they are unrelated to each other.

### 1. A closed `<dialog>` stayed on screen

`viewer.css` sets `.hv { display: grid }` on the dialog. The browser's own stylesheet hides a closed one with
`dialog:not([open]) { display: none }`, which *looks* like it should win — (0,1,1) against (0,1,0) on
specificity.

It does not. **The cascade compares origin before specificity**, and any author declaration beats a user-agent
one however specific. So the overlay was displayed whether or not the dialog was open, and `.close()` left a
full-viewport dark box over the page.

That accounts for the whole report, including the parts that look like separate faults:

- the photograph vanished because the JavaScript *did* run — `onClose` clears the image;
- the viewer stayed because the CSS won;
- `Esc` then did nothing because the dialog was already closed, and `Esc` only reaches an open one.

Pico ships the same rule for the elements it styles, which is the hint that this is a property of `dialog`
rather than a mistake peculiar to this file: anything that sets `display` on a dialog has to hide it again by
hand.

### 2. `start()` ran before the controls were registered

`start()` binds the containers and, crucially, **opens the viewer immediately when the address carries
`?foto=`** (task 401's deep link). It sat halfway up `viewer.js`, above the `register()` calls for fullscreen
(task 404) and share (task 405) — and because the script is deferred, `document.readyState` is already
`interactive`, so it ran synchronously at that point.

So a cold load of a `?foto=` address built its action row from an **empty registry**: a viewer with a close
button and nothing else.

Not a corner case. The viewer puts `?foto=` on the address as you move through an album, so **reloading the
page reproduces it every time** — which is how it was found.

## Acceptance Criteria

- [x] Closing the viewer hides it, and `Esc` closes it
- [x] Leaving fullscreen happens with the viewer rather than after it
- [x] A cold load carrying `?foto=` shows the same controls a click does
- [x] Both failures are guarded by tests that were verified to fail

## Progress Log

- 2026-09-25 — Reported by the maintainer from Brave on macOS.
- 2026-09-25 — `.hv:not([open]) { display: none }` added, with the cascade-origin reasoning written next to it.
  The specificity arithmetic is the trap here: it looks like the browser's rule should win, and reading it that
  way is how the bug got written in the first place.
- 2026-09-25 — The `start()` block moved to the very bottom of `viewer.js`, after every `register()` call.
- 2026-09-25 — Also fixed while in there: closing the viewer now leaves fullscreen. Without it, closing while
  fullscreen left the browser filling the screen with the album page behind it — no chrome, no viewer, and no
  obvious way back.
- 2026-09-25 — `TestAClosedViewerIsHidden` and `TestTheViewerRegistersItsControlsBeforeItCanOpen` added, and both
  **verified to fail** against the broken versions before being trusted. The second asserts by source position,
  which is unusual and deliberate: both orderings look equally sensible in review, and no Go test can execute
  this file.
- 2026-09-25 — Lesson worth keeping, because task 411 was supposed to catch these and had not run yet: every
  guard written for tasks 402–408 was about *decisions* — no surface check, no `files` in a share, the filmstrip
  hidden by media query. All of them passed while the viewer could not be closed. Source-reading guards cannot
  see a cascade conflict or an initialisation order, and **the QA pass is not belt-and-braces on this feature,
  it is the only test of most of it**. Two of its checks are cheap enough to do on every change: open the viewer
  and close it, and reload the page it leaves in the address bar.
- 2026-09-25 — ✅ All criteria met. `gofmt`, `go vet`, full `go test ./cmd/api/` clean.
