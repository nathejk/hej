# 445 — Dragging switches the album to manual, after a confirmation

**Status:** done
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

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

## What changed

**The server refuses it.** `PATCH …/move` answers **409** unless the album is manual, naming the mode — because
"switch it to manual" is only actionable if you know what the album is currently doing. 409 rather than 400: the
request is well-formed and would be correct a moment later, which is what that status is for. The tool asking
first is politeness; this is the rule, and the endpoint is reachable with `curl` and the shared credential.

**The tool asks at the end of the gesture.** The gap and the carried stack behave exactly as in a manual album;
the confirmation replaces the request that would otherwise have gone out on release. Interrupting a pointer drag
with a dialog is how a drag gets abandoned by accident — and the curator may drop the photograph back where it
started, in which case there was nothing to ask about.

**The mode is switched before the move, and a failed switch abandons the move.** The other order would leave an
arrangement in an album that still claims to sort itself: work with a timer on it, destroyed by the next
addition, after the curator was told the move succeeded.

The mode is read off `#albumeditor[data-sort-mode]` **at the end of each gesture** rather than cached, because
the curator may have changed it in the editor card since the last drag — and the card writes that attribute when
they do, along with the `<select>`, so the next drag asks the right question or none.

## The copy

Drafted rather than generated, per PRD 022 §5. The sheet has to say three things before the curator agrees, not
after: what the album does now, what will change, and what that means for photographs added later.

> **Skift til manuel rækkefølge?**
>
> Albummet er sorteret efter **tid, ældste først**.
>
> Flytter du et billede med hånden, skifter albummet til **manuel rækkefølge**. Så er det dig der bestemmer, og
> billeder du lægger i albummet bagefter bliver lagt til sidst i stedet for at falde på plads af sig selv.
>
> Du kan altid vælge sorteringen igen i albummets oplysninger — men så bliver den rækkefølge du har lavet med
> hånden lagt om.
>
> [Flyt og skift til manuel] [Annuller]

The third paragraph is the one worth keeping: it answers "can I undo this", and the honest answer is "yes, and it
costs you the arrangement". A curator who finds that out afterwards has lost an afternoon.

On cancel: *"Rækkefølgen blev ikke ændret."* On a failed switch: *"Kunne ikke skifte til manuel rækkefølge.
Billederne blev ikke flyttet."* — the second clause matters more than the first.

## Acceptance Criteria

- [x] `PATCH …/move` answers 409 on a non-manual album, and publishes nothing
- [x] The tool confirms, then switches to manual, then moves — and abandons all three on cancel
- [x] A failed switch does not move: the arrangement must not outlive the mode that protects it
- [x] No confirmation when the album is already manual
- [x] Escape and the keyboard reach the confirmation (the shell's own behaviour, unchanged)
- [x] The sheet and the editor card both show the new mode afterwards

## Progress Log

- 2026-09-28 — Picked up with 446, since they share the editor card and the mode label.
- 2026-09-28 — Found a real trap while wiring the 409: spelling the rule `a.SortMode != SortModeManual` makes
  `""` mean "sorts itself", so an album with **nothing stated** would refuse a hand move and try to re-sort
  itself on every addition. The safest input producing the least safe behaviour. Added `album.SortModeOr`, which
  every reader now goes through, and a test whose failure message says which direction the bug goes.
- 2026-09-28 — ✅ Behavioural tests for the 409 across all four automatic modes, and source guards for the two
  orderings that are invisible in a diff: confirm-at-the-end, and switch-before-move. Mutation-checked.
- 2026-09-28 — `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...` clean.
