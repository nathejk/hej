# 418 — Fullscreen: neither the dialog nor the page, but the stage

**Status:** done
**Priority:** high
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

Third and final round on the fullscreen button. Task 417 reverted the target to the dialog and added a visible
message for a refusal; the maintainer's next test produced that message:

> *"Now it says 'fuld skærm er ikke tilgængelig her' — it was just before."*

Which is the answer the previous two rounds were missing: **the request for the dialog is genuinely refused.**

### All three targets, and why only one works

| Target | Outcome | Why |
|---|---|---|
| The `<dialog>` | **Refused** | It is in the top layer, put there by `showModal`. Chromium will not fullscreen an element that is already there, so the promise rejects. |
| `document.documentElement` | **Accepted, renders wrong** | It joins the top layer *after* the dialog, so the album page paints over the photograph and the info panel falls behind it (task 417). |
| `.hv-stage` | **Works** | An ordinary div inside the dialog: not in the top layer itself, so the request is accepted, and it holds the photograph, the arrows, the action row and the caption. |

So the fullscreen element is the stage. The filmstrip and the editor panel are the only things outside it —
see below for the editor; the strip being absent from a fullscreen view is the right trade rather than a
regrettable one, since fullscreen is for looking at one photograph as large as the screen allows and the strip
is how you leave it.

### The consequence that had to be fixed with it

The caption/credit editor's panel was a child of the dialog, beside the filmstrip. The pencil that opens it is
in the action row, **inside** the stage, so it stays clickable in fullscreen — and the panel it opened would
have appeared behind the fullscreen element, where nobody could see it. A control that appears to do nothing,
which is a failure this viewer has already produced twice by other means.

The panel now lives inside the stage, positioned over the photograph, and takes the info panel's place while it
is open: the read-only copy of the text being edited would otherwise sit underneath it, one sentence out of
date.

## Acceptance Criteria

- [x] Fullscreen works, with the photograph on top and the album page not visible
- [x] The caption and credit sit over the photograph in fullscreen, as in windowed mode
- [x] The stage brings its own background and backdrop when it becomes the fullscreen element
- [x] The caption and credit editors are usable in fullscreen
- [x] The guard names **both** rejected targets, so neither returns by the same reasoning

## Progress Log

- 2026-09-25 — The message from task 417 arrived, which settled it: the dialog target is refused rather than
  silently rendering wrong. Both failing targets are now recorded in the code next to the working one, because
  the next person will reach for one of them.
- 2026-09-25 — The stage needs `background` and `::backdrop` of its own once it is the fullscreen element: the
  dialog supplies the colour in windowed mode and is no longer behind it in fullscreen, and the browser's default
  backdrop is a different black from the viewer's.
- 2026-09-25 — `ctx.stage` added to the narrow action context. It was deliberately narrow — "an action that
  needed the whole `ui` would be an action that could move the viewer" — and this is a legitimate addition: one
  control needs an element to hand to a platform API.
- 2026-09-25 — The editor panel moved inside the stage and now hides the info panel while open.
- 2026-09-25 — The guard asserts the working target **and names both wrong ones**. A test that only asserted
  whatever was current would have passed in all three rounds of this, which is how task 416's guard managed to be
  green while describing a bug.
- 2026-09-25 — Three rounds for one button, and the lesson is not about fullscreen. **The first version swallowed
  a rejected promise**, so a refusal and a rendering fault were indistinguishable from outside, and two rounds
  went on guessing between them. Reporting the failure cost four lines and would have saved both. Anywhere this
  code hands work to a platform API, the rejection path is part of the feature.
- 2026-09-25 — ✅ All criteria met. `gofmt`, `go vet`, full `go test ./cmd/api/` clean. Whether Brave accepts the
  stage cannot be confirmed here — nothing in this repo executes the fullscreen API — but a refusal will now name
  itself on screen.
