# 453 — The picker in the "Fotokredit" sheet

**Status:** open
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 025 §6 R2 and §7. Needs tasks 449 and 450.

The existing sheet gains a list of crew names — `pr` first, with a way to see all of the year's crew — above the
text field, which stays (PRD 025 §11 Q2: the typed path is a peer, for a guest photographer or one outside
`pr`).

**The copy has to distinguish the two paths.** Picking a colleague from a roster and typing a guest's name are
the same outcome by different routes, and only one of them can be misspelled. The sheet's existing sentence —
which tells the curator they are putting a name on a public page — stays; a curator picking from a list should
know that just as clearly as one typing.

The `localStorage` default (task 393) matters less now but must keep working for the typed path.

The server-rendered admin tool: Pico, htmx, Alpine, no build step. See the `go-server-rendered-pages` skill.

## Acceptance Criteria

- [ ] The list is offered, `pr` by default, all-crew available
- [ ] Picking sets the id; typing sets the text; each clears the other
- [ ] Copy that makes both paths and their consequence clear, in Danish
- [ ] Keyboard-reachable, like the tool's other sheets
- [ ] The typed path and its remembered default still work

## Progress Log

- 2026-09-28 — Task created from PRD 025 §6 R2, §7.
