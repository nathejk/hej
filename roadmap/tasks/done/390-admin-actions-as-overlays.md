# 390 — The curator's actions open as overlays, not as cards stacked under the contact sheet

**Status:** done
**Priority:** medium
**Created:** 2026-09-23
**Picked up by:** agent
**Started:** 2026-09-23
**Completed:** 2026-09-23

## Description

Maintainer, using the tool:

> *"instead of cards underneath, use overlays/modals for functions"*

The four action panels — **Tilføj til album**, **Sæt position**, **Tag patrulje**, **Fjern eller slet** — were
`<div hidden role="dialog">` blocks that revealed inline *below* the contact sheet. With a selection made near
the bottom of a 120-thumbnail page the panel opened off-screen: the curator pressed a button and, as far as
they could tell, nothing happened.

## The constraint that survived

`adminpage.go` recorded it, and it is still true: *navigating away loses the selection (PRD 022 §7)*, so no
action may become a route. **An overlay satisfies that** — it is still one page and the selection is still in
memory. A card underneath was never the requirement, only the first reading of it, and the comment now says
which of the two it is.

## What landed

**One shell instead of four.** The old code had each opener hide its three siblings by hand — twelve
assignments, plus four more to forget when a fifth action arrives. Now `openSheet(el)` and `closeSheet()` are
the only things that show or hide a sheet, and a test fails if anything touches `.hidden` directly.

**One `.sheet` CSS rule instead of four near-identical id blocks.** `#panel`, `#pospanel`, `#tagpanel` and
`#delpanel` each repeated the same chrome — padding, border, radius, button colours, input styling — which is
four places for a fifth action to end up looking slightly different. Secondary buttons went from seven
id-specific overrides (`#panel #closepanel, #panel #createalbum { … }`) to one `.ghost` class. The CSS for
this feature is about 25 lines shorter and says more.

**Fixed positioning, and scrollable.** Centred in the viewport whatever the scroll position — that is the
reported bug — and `max-height: calc(100vh - 2rem)` with `overflow-y: auto`, because the position sheet holds
an 18 rem map and on a laptop in landscape it used to push its own buttons past the bottom of the screen.

**Real dialog semantics**, and this is not decoration. A curator tagging three hundred photographs works fast
and from the keyboard:

- `aria-modal="true"` on all four.
- Escape closes.
- Tab is trapped inside the open sheet, so it does not walk off into the contact sheet behind it.
- Focus moves into the sheet on open and **returns to the button that opened it** on close. Without that,
  closing a sheet drops focus to the top of the document and the next Tab starts from the page header.
- The body does not scroll behind an open sheet.
- Clicking the backdrop closes — **except the delete sheet.** Every other sheet costs nothing to dismiss; that
  one is showing a count of photographs it is about to delete permanently with no undo (task 379, §11 Q6), and
  a misplaced click next to it should not be how it goes away.

**The delete sheet's visual distinction survived** (task 379's whole point): the two choices are still
separated, the destructive one still has the red border and background, and its button is still the only red
one in the tool. Its buttons now default to secondary so that red button is the only thing that looks like an
action.

## The bug this change introduced, and caught

Leaflet computes its pixel size **when the map is created** and caches it. A map created inside a container the
browser has not laid out gets zero — and the symptom is a map that loads one tile in the corner and ignores
every drag.

That was safe while the panel was an inline card already in flow. The overlay made it unsafe. So
`drawPositionMap` now runs *after* `openSheet` has made the sheet visible, and `invalidateSize()` runs on the
**next frame**, once layout has settled — not in the same tick as unhiding.

Worth recording as the general shape: a presentation-only change is assumed not to cause breakage, and this one
would have broken the feature task 389 had just repaired, two commits earlier.

## Verified by breaking it

Five sabotages, each reverted, each failing:

| Sabotage | Caught by |
|---|---|
| a sheet shown with `tagPanel.hidden = false` again | the shell guard, naming it and the count |
| `position: fixed` removed (a card again) | the `.sheet` rule check |
| the delete sheet allowed to close on a backdrop click | its own guard |
| `invalidateSize()` dropped | the map-sizing guard |
| `sheetOpener` renamed, so focus is never returned | the keyboard guard |

## Verified live

Rebuilt and fetched `/admin`: one `#scrim`, four `class="sheet"` with `aria-modal="true"`, five `openSheet(`
call sites, the scroll lock, the `requestAnimationFrame`/`invalidateSize` pair, seven `.ghost` buttons, and the
delete sheet's `choice danger` block and red `#dodelete` intact. No stale `#panel {` / `#pospanel {` /
`#tagpanel {` positioning rules left behind.

## Acceptance Criteria

- [x] The four actions open as overlays, centred in the viewport, whatever the scroll position
- [x] One shell, used by all of them — adding a fifth action does not mean a fifth show/hide
- [x] Escape closes, focus is trapped while open and restored to the opening button on close
- [x] The body does not scroll behind an open overlay
- [x] The position map draws at the right size when its overlay opens, and on reopen
- [x] The selection survives opening and closing any overlay — the shell only touches visibility, and
      `syncActions` still closes the sheet when the selection empties
- [x] The destructive choice in the delete overlay is still visually distinct
- [x] No action becomes a route

## Notes

- **The backtick trap, twice in one task.** Both a CSS comment and a JS comment I wrote quoted identifiers in
  backticks, inside `adminpage.go`'s raw-string template — each one terminated the literal. The symptom is a
  syntax error pointing at a line of CSS: `unexpected literal \` rule rather than four near-identical…\``. That
  is now four times on this file. Anything written into that template must use plain quotes.
- Deliberately **not** `<dialog>`. It would give the backdrop, the focus trap and Escape for free — but it also
  brings its own top-layer stacking, its own `::backdrop` styling rules and a different close-event model, and
  the delete sheet needs to *refuse* a backdrop dismissal, which `<dialog>`'s built-in behaviour fights. Worth
  revisiting if a fifth or sixth sheet lands; not worth it for four.
- Next: task 391 (the album list) now has a shell to land in, which is why it was sequenced after this.
