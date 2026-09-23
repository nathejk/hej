# 390 — The curator's actions open as overlays, not as cards stacked under the contact sheet

**Status:** open
**Priority:** medium
**Created:** 2026-09-23
**Picked up by:**
**Started:**
**Completed:**

## Description

Maintainer, using the tool:

> *"instead of cards underneath, use overlays/modals for functions"*

The four action panels — **Tilføj til album**, **Sæt position**, **Tag patrulje**, **Fjern eller slet** — are
`<div hidden role="dialog">` blocks that reveal inline *below* the contact sheet. With a selection made near
the bottom of a 120-thumbnail page, the panel opens off-screen: the curator presses a button and nothing
appears to happen.

They are already marked up as dialogs. This is a presentation change, not a behaviour one.

## The constraint that must survive

`adminpage.go` records it: *"The panel opens inline rather than on its own page, because navigating away loses
the selection (PRD 022 §7)."* An overlay satisfies that — it is still one page, and the selection is still in
memory. **Do not turn any of these into a route.**

## What this needs

- One overlay shell rather than four, so the four panels cannot drift in behaviour or styling. They currently
  repeat their own show/hide.
- Real dialog semantics: `<dialog>` if it carries its own weight, otherwise the existing `role="dialog"` plus
  a focus trap, `aria-modal`, Escape to close, a backdrop, and focus returned to the button that opened it.
  Keyboard reachability is not optional here — a curator tagging three hundred photographs works fast.
- Scroll lock on the body while one is open, or the backdrop scrolls the sheet underneath.
- **The map panel needs care.** Leaflet computes its size on creation, so a map created inside a hidden or
  animating container renders at the wrong size. `drawPositionMap` already calls `invalidateSize()` on reopen;
  it will need to be called *after* the overlay is visible and settled (task 389 fixed what it draws, not when).
- The delete panel's visual distinction must survive (task 379): the destructive choice stays the one that
  looks different.

## Sequencing

Do this **before** task 391, so the album list lands as an overlay rather than as a fifth card that then has
to be converted.

## Acceptance Criteria

- [ ] The four actions open as overlays, centred in the viewport, whatever the scroll position
- [ ] One shell, used by all of them — adding a fifth action does not mean a fifth show/hide
- [ ] Escape closes, focus is trapped while open and restored to the opening button on close
- [ ] The body does not scroll behind an open overlay
- [ ] The position map draws at the right size when its overlay opens, and on reopen
- [ ] The selection survives opening and closing any overlay — asserted, since that is the one hard constraint
- [ ] The destructive choice in the delete overlay is still visually distinct
- [ ] No action becomes a route
