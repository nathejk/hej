# 499 — Viewer: video playback, sound rules, loop and keyboard

**Status:** done
**Priority:** high
**Created:** 2026-10-03
**Picked up by:** claude (agent session)
**Started:** 2026-10-04
**Completed:** 2026-10-04

## Description

In the PRD 023 viewer: autoplay with sound when opened by a click or tap (fall back to muted if rejected); muted when arrived at by swipe, with an unmute button; unmute persists for the session. Clips ≤ 15 s loop. Space toggles pause, ←/→ seek ±10 s, M toggles mute (ignored in form fields). Only the visible item plays; leaving pauses and releases the source. Swipes on the scrubber seek rather than change item.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [x] Sound and mute rules as described — verified in headless desktop Chrome; **iOS Safari and Android Chrome are task 504**
- [x] Loop ≤ 15 s
- [x] Keyboard shortcuts work and Space does not scroll — **seek is Shift+←/→ and J/L, not bare ←/→** (see log)
- [x] Leaving an item stops it

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
- 2026-10-04 — Picked up.
- 2026-10-04 — `viewer/viewer.js`: one `<video controls playsinline preload=metadata>` beside the `<img>`, pointed at the current video and emptied (`removeAttribute('src')` + `load()`) on every move and on close, so only the visible item plays or downloads. `pictureFor` returns the poster for a video, so prefetch and the filmstrip never fetch an MP4. Clips ≤ 15 s loop. Downloads of a video are named `.mp4`.
- 2026-10-04 — Sound: `open(container, index, byGesture)`; a tile click (and the admin's 'Vis stort', which calls `open` without the argument) plays with sound, falling back to muted if `play()` is refused; a `?foto=` link and any swipe/arrow arrival play muted with a 'Slå lyd til' button. An unmute, by our button or the native control, holds for the rest of the viewing; M toggles it.
- 2026-10-04 — **Deviation from PRD 029 §6:** bare ←/→ still move between items, because otherwise arrowing through an album would get stuck in the first video; seeking ±10 s is Shift+←/→ and J/L. Space is intercepted while a video is shown, since focus starts on Close and Space would otherwise close the viewer. Keys are ignored in text fields (the curator's caption editor). A touch drag in the bottom 64 px of the video is the scrubber, not a swipe.
- 2026-10-04 — Smoke-tested by driving headless Chrome over CDP against a page with a photo, a short and a long video: 19/19 checks (gesture → sound, swipe → muted + button, unmute persists, Space/M/Shift+←/L, loop, release on leave/close, no neighbour MP4 requested). That run caught a real bug: the unmute button hid itself while focused, dropping focus out of the dialog and killing the keyboard; focus now returns to Close.
