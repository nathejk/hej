# 420 — Fast clicks on the arrows select the photograph

**Status:** done
**Priority:** medium
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

> *"Fast consecutive clicks on next-button counts as a doubleclick which selects everything, in this case the
> shown photo. Make sure selection is disabled in the viewer."* — maintainer, desktop macOS, Brave

Two presses in quick succession are a double-click as far as the browser is concerned, and a double-click
selects. So walking quickly through an album — which is the normal way to use this — left a blue selection
highlight over the photograph.

Worth being precise about what was and was not broken: **the clicks always registered.** Both presses navigate,
and navigation was never the problem. The selection is the whole of it.

### The fix, and its one exception

`user-select: none` on the overlay, prefixed as well as unprefixed — Safari wanted `-webkit-user-select` until
well after the 16.4 floor in `.rules`.

The whole overlay rather than just the arrows, because the same thing happens on any of the controls and because
there is nothing in here a visitor would want to select. The caption is the only text, and it is already
`pointer-events: none` (task 416), so this takes away nothing that was reachable.

**The exception matters as much as the rule.** The caption/credit editor's field is text being *written*, so it
opts back in. A curator who could not select a word of their own caption to replace it would have a worse
problem than the one this fixes — and it is exactly the kind of thing a blanket rule breaks quietly.

## Acceptance Criteria

- [x] Fast consecutive presses on an arrow navigate without selecting anything
- [x] The editor's field is still selectable
- [x] Both are guarded, including the prefixed property

## Progress Log

- 2026-09-25 — Reported and fixed in one pass: `user-select: none` on `.hv`, `user-select: text` on
  `.hv-edit-field`, both with the `-webkit-` prefix.
- 2026-09-25 — Not fixed by suppressing the double-click, which was the other obvious route. The `dblclick`
  event is not what selects — the browser's default action on the second `mousedown` is — so cancelling the event
  would have been a change that looked relevant and did nothing. `user-select` is the property that actually
  describes the intent.
- 2026-09-25 — Noted for anybody adding text to the overlay later: it will not be selectable, and the fix is to
  scope the exception the way `.hv-edit-field` does rather than to loosen the rule on `.hv`.
- 2026-09-25 — ✅ All criteria met. `gofmt`, `go vet`, full `go test ./cmd/api/` clean.
