# 408 — The credit line, editable in the viewer, and deliberately not the same control as the caption

**Status:** open
**Priority:** medium
**Created:** 2026-09-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 023 §6 ("Functional — admin") and §7.7, answering §11's question 5 with a yes. The photographer's credit
line becomes editable in the admin viewer's info panel, under the caption editor from task 407 — and it is
**its own control**, not a second field in the caption's form. Three differences, each with a reason already
established in this codebase:

- **Its own save.** One form writing both fields means a curator fixing a caption typo can blank a credit by
  leaving it alone in a form that submits everything. §6 states the rule as *"two separate fields with two
  separate saves, never one form that writes both"*.
- **Its own "remove" action.** `creditaction.js` already decided that clearing an attribution is a deliberate
  act rather than what a stray select-all-and-delete does on the way past — the sheet has a separate **"Fjern
  fotokredit"** button and refuses an empty save with *"Skriv en fotokredit, eller brug 'Fjern fotokredit'."*
  That reasoning does not weaken in a viewer; with the arrow keys in play it gets stronger.
- **Its own warning.** This is **the one field in the whole tool that records a person's name**, and the only
  one on the public site that does — task 393, PRD 011's single documented exception. The field shows the line
  as it will publicly read, *"Foto: Jens Hansen"*, so a curator typing a colleague's name knows that is what
  they are doing. The credit sheet says this plainly today; **the viewer must not become the quiet way to do
  the same thing.** Carry the sheet's existing Danish over rather than writing new copy.

### Prefill from the photograph, not from `localStorage`

The sheet prefills from `hej.admin.lastCredit` because a **batch** has no single current value to show and
retyping one line per memory card is real tedium (task 393). Here there is exactly one photograph in front of
you, so the honest prefill is **its own credit**. Reading `hej.admin.lastCredit` in the viewer would silently
propose overwriting a correct attribution with the last one somebody typed on this laptop.

### Same endpoint, same inheritance

`PATCH /api/admin/photos` with `{photoIds:[id], credit}` — the existing, annotated, `requireAdmin`-protected
route, as with the caption. No new Go route. Note that the credit **is** logged where the caption is not (task
393): *"who was credited on which photographs, and when"* is the question somebody may have to answer later,
and with a shared credential the log is the only record there is. Going through the existing endpoint is what
keeps that true here.

On save: update the host item's `data-credit`, then reuse the sheet's existing post-action refresh. A failed
save says so and keeps the typed text, for the same reason as the caption.

The real curator loop this unlocks (§7.7): credit a whole memory card one photograph at a time while actually
looking at them. That is what made it worth overriding §4 for.

Depends on tasks 402, 406 and 407.

## Acceptance Criteria

- [ ] The credit is editable in the admin viewer with its **own** field and its **own** save, so saving a
      caption cannot write the credit and vice versa
- [ ] A separate "Fjern fotokredit" action clears it; saving an empty field is refused with the sheet's
      existing Danish line rather than treated as a removal
- [ ] The field shows the line as it will publicly read ("Foto: …") and carries the sheet's existing warning
      that this is the one field publishing a person's name
- [ ] It is prefilled from the photograph's current credit, and `hej.admin.lastCredit` is not read here —
      asserted by a source guard that strips comments before searching
- [ ] Saving goes through `PATCH /api/admin/photos` with a single-element `photoIds`, so the credit is logged
      exactly as the sheet's is; no new route
- [ ] A failed save reports it in Danish and keeps the typed text

## Progress Log

- 2026-09-25 — Task created from PRD 023.
