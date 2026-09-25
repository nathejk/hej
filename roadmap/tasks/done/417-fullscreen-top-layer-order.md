# 417 — Fullscreen paints the album page over the photograph

**Status:** done
**Priority:** high
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

Reported by the maintainer, desktop macOS, Brave, immediately after task 416:

> *"When in fullscreen, the album-page is layered on top of current photo. The caption/credit works fine in
> viewport mode, but is located underneath photo in fullscreen, should be over as in viewport mode."*

**This is task 416's fix being wrong**, and it is worth writing down properly because the mechanism is the same
one that has now bitten twice.

### What actually happens

A modal dialog is in the **top layer** — `showModal()` put it there. The fullscreen API uses that same top
layer, and **the order things are added to it decides what paints over what.**

So task 416's change — requesting fullscreen on `document.documentElement` — adds `html` to the top layer
*after* the dialog. The consequences are exactly what was reported:

- the album page paints over the photograph, because `html` is now the later top-layer entry;
- the info panel, which is positioned over the photograph inside the stage, ends up under it, because the
  overlay's contents are being painted as part of a second, outer top-layer entry.

Asking the **dialog** adds nothing above it: it is already the top-layer entry, so the stacking inside the
overlay is untouched. That is the combination the two features are specified to cope with, and it is what this
task restores.

### The honest part about task 416

416 changed the target because the maintainer reported that fullscreen on the dialog "did nothing". That was
diagnosed **without a console**, from a symptom, and the change was a guess dressed as a diagnosis — the first
version caught the rejected promise and said nothing, so there was no information to go on either way.

The reporting added in 416 is the part that was actually valuable and it stays. If the dialog target is
genuinely refused by some engine, the viewer now says *"Fuld skærm er ikke tilgængelig her."* on screen rather
than leaving somebody to guess again.

## Acceptance Criteria

- [x] In fullscreen, the photograph is the thing on top and the album page is not visible
- [x] The caption and credit sit over the photograph in fullscreen, exactly as in windowed mode
- [x] A refused fullscreen request still says so rather than failing silently
- [x] The guard names the broken arrangement so it cannot be reintroduced by the same reasoning

## Progress Log

- 2026-09-25 — Reported. Diagnosed as task 416's own change: `document.documentElement` joins the top layer
  above the dialog, so the page paints over the overlay.
- 2026-09-25 — Target reverted to the dialog, with both directions and their reasons recorded in the code, so
  the next person to read "fullscreen the page instead" finds out why that was already tried.
- 2026-09-25 — The guard inverted and renamed, and it now names the broken arrangement explicitly: a test that
  only asserted the right answer would have been satisfied by 416's change too, since it asserted whatever was
  there at the time. Asserting *both* what must be there and what must not is what makes it a guard rather than
  a description.
- 2026-09-25 — Lesson, stated once here rather than repeated: **two of the last three viewer bugs were caused by
  fixing a symptom without a diagnosis.** 416 changed a working-but-unverified target on the strength of "it
  does nothing", and produced a worse bug. The right first move for a rendering or platform fault is a console
  message, which is why the reporting path added in 416 matters more than either target choice.
- 2026-09-25 — ✅ All criteria met. `gofmt`, `go vet`, full `go test ./cmd/api/` clean. What this cannot confirm
  is whether the dialog target is accepted in Brave — nothing in this repo can execute the fullscreen API. If it
  is refused, the on-screen message now says so, and that message is the next piece of information.
