# 421 — Closing the viewer leaves ?foto= in the address, so a reload reopens the photograph

**Status:** done
**Priority:** high
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

> *"When I close the album the URL stays `https://hej.local.nathejk.dk/2026/album/test?foto=9#foto-9`, but then
> if I reload the overview, the previously selected photo opens — not expected."* — maintainer, desktop macOS,
> Brave

Closing wound the history back, and only that:

```js
if (ui.pushed && window.history) { ui.pushed = false; window.history.back(); }
```

That is right when the viewer pushed the entry it is sitting on, and wrong in two different ways when it did
not:

- opened from a `?foto=` link **in a fresh tab** there is nothing behind it, so `back()` does nothing at all —
  the address keeps the parameter and a reload reopens the photograph, which is exactly what was reported;
- and where there *is* something behind it, `back()` leaves the album altogether. That second failure had not
  been noticed yet and would have read as "closing the viewer navigated me away".

### One flag was standing for two different facts

- **`reflected`** — the address already names the current photograph, so the next move should `replaceState`
  rather than push.
- **`pushed`** — *we* added a history entry, so closing may wind it back.

A viewer opened from a link is the first and **not** the second. `openFromURL` set `pushed = true` to get the
replace-rather-than-push behaviour, and that one line bought both meanings at once.

They are now two flags, and closing has two paths: `back()` when we pushed — which restores the address and
leaves no leftover entry that looks like somewhere you could return to — and `replaceState` with the parameter
and the fragment removed when we did not.

The fragment goes with the parameter. Leaving it would be harmless in itself, since a hash scrolls and does not
open anything, but half a cleaned address is the kind of thing that gets reported a second time.

## Acceptance Criteria

- [x] Closing the viewer leaves the album's own address, with no `foto` parameter and no fragment
- [x] A reload after closing shows the album, not the viewer
- [x] Closing a viewer opened from a shared link does not navigate away from the album
- [x] Back still closes an open viewer, and does not walk back through every photograph swiped past
- [x] Guarded by test, including the `openFromURL` line the bug was on

## Progress Log

- 2026-09-25 — Reported. Root cause found on the second reading: `openFromURL` setting `pushed = true` with a
  comment explaining it as "there is nothing to push", which was true of the *intent* and the opposite of what
  the flag then did.
- 2026-09-25 — Split into `pushed` and `reflected`, with the distinction written where both are set. The comment
  that described the old behaviour correctly while the code did something else is worth remembering: a flag whose
  name states one fact and whose use implies another will eventually be read as whichever is convenient.
- 2026-09-25 — `unreflectURL()` added for the not-pushed path: `replaceState` in place, never `back()`, because
  there may be nothing behind and whatever *is* behind belongs to the sender of the link rather than to this
  album.
- 2026-09-25 — The `popstate` path was already correct and stays: it clears `pushed` before closing, so the close
  takes the in-place branch, finds the address already clean, and does nothing.
- 2026-09-25 — ✅ All criteria met. `gofmt`, `go vet`, full `go test ./cmd/api/` clean.
