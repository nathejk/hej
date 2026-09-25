# 404 — The fullscreen button, and the one platform that has no element fullscreen at all

**Status:** open
**Priority:** medium
**Created:** 2026-09-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 023 §7.5, answering the maintainer's question directly: *"Is it possible to have a button for entering,
not just full viewport but also a fullscreen view?"* Yes — `Element.requestFullscreen()` on the overlay,
`document.exitFullscreen()` to leave, driven by the button click as the user gesture. It escapes the browser
chrome — address bar, tabs, the lot — which the full-viewport overlay cannot do on its own. §2 makes this more
valuable than a phone-first reading suggests: looking through a photographer's album on a laptop is a
first-class case here.

Three things §7.5 says to build in rather than discover:

- **Safari needs the prefixed pair.** `el.requestFullscreen || el.webkitRequestFullscreen`, and the matching
  exit and `fullscreenchange` names. Two lines, and without them the button silently does nothing on a Mac.
- **iPhone has no element fullscreen at all.** iOS Safari implements fullscreen only for `<video>`; **iPadOS
  Safari does support it for elements**, so this is not "mobile does not have it". The button is therefore
  feature-gated on `document.fullscreenEnabled || document.webkitFullscreenEnabled` and **absent when false**
  — not disabled, not hidden, absent. A visible button that does nothing is worse than no button, and this
  costs an iPhone nothing real: the overlay already covers the viewport.
- **The button reflects state, icon and label both** (Lucide `maximize` / `minimize`, inline SVG per §7.6),
  because the user can leave fullscreen by pressing `Esc` or swiping without touching our button. That means
  listening to `fullscreenchange` rather than tracking a boolean ourselves.

The `Esc` interaction is the part most likely to be got wrong: **`Esc` while in fullscreen must exit
fullscreen without also closing the viewer.** The browser consumes that keypress for fullscreen, and the
`<dialog>`'s own `cancel` behaviour must not additionally fire — otherwise one press throws the reader all the
way back to the grid. Both orderings need checking on desktop Safari and Chrome, since they are not obliged to
agree.

Depends on task 402 for the action row to render into.

## Acceptance Criteria

- [ ] The button enters and leaves fullscreen on the overlay, with the `webkit`-prefixed request, exit and
      event names as fallbacks
- [ ] The control is **absent** — not disabled — when neither `fullscreenEnabled` nor
      `webkitFullscreenEnabled` is true, verified on an iPhone and contrasted with an iPad where it appears
- [ ] Icon and accessible label follow `fullscreenchange`, so leaving fullscreen by `Esc` or a swipe updates
      the button
- [ ] `Esc` in fullscreen exits fullscreen and leaves the viewer open; a second `Esc` closes the viewer
- [ ] The icons are inline SVG copied from Lucide (`maximize`, `minimize`), with no new icon set introduced

## Progress Log

- 2026-09-25 — Task created from PRD 023.
