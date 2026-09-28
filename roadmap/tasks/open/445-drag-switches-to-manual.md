# 445 — Dragging switches the album to manual, after a confirmation

**Status:** open
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 024 §6 R8 and §7. Needs 442.

Dragging a photograph in a non-manual album is a contradiction: the arrangement would be recomputed away
the next time anything was added. So a hand move **switches the album to manual**, and because that
changes how the album will behave from then on, it is confirmed first.

- **The server enforces it**, not the client: `PATCH /api/admin/albums/{albumId}/move` answers **409** on a
  non-manual album. The confirmation is the tool being polite; the 409 is what makes the rule true.
- **The confirmation appears at the end of the drag, not the start.** Interrupting a pointer gesture with a
  dialog is how a drag gets abandoned by accident, and the curator may well drop where they started. The
  gap and the floating stack (tasks 435, 436) behave exactly as now; the confirmation replaces the request
  that would otherwise have gone out on release.
- **Declining changes nothing** — the mode is untouched and the grid is as it was. Not applied-then-reverted.
- The tool's own overlay (`sheetshell.js`), not `window.confirm`, and Escape closes it.

**The wording is the feature, not decoration** (PRD 022 §5). It has to say three things: this album is
sorted by *X*; moving a photograph by hand switches it to manual; new photographs will then be added at the
end. Buttons: *"Flyt og skift til manuel"* / *"Annuller"*. Write these deliberately; do not generate them.

## Acceptance Criteria

- [ ] `PATCH …/move` answers 409 on a non-manual album, and publishes nothing
- [ ] The tool confirms, then switches to manual, then moves — and abandons all three on cancel
- [ ] A failed switch does not move: the arrangement must not outlive the mode that protects it
- [ ] No confirmation when the album is already manual
- [ ] Escape and the keyboard reach the confirmation
- [ ] The sheet and the editor card both show the new mode afterwards

## Progress Log

- 2026-09-28 — Task created from PRD 024 §6 R8, §7.
