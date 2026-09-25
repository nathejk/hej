# 408 — The credit line, editable in the viewer, and deliberately not the same control as the caption

**Status:** done
**Priority:** medium
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

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

- [x] The credit is editable in the admin viewer with its **own** field and its **own** save, so saving a
      caption cannot write the credit and vice versa
- [x] A separate "Fjern fotokredit" action clears it; saving an empty field is refused with the sheet's
      existing Danish line rather than treated as a removal
- [x] The field shows the line as it will publicly read ("Foto: …") and carries the sheet's existing warning
      that this is the one field publishing a person's name
- [x] It is prefilled from the photograph's current credit, and `hej.admin.lastCredit` is not read here —
      asserted by a source guard that strips comments before searching
- [x] Saving goes through `PATCH /api/admin/photos` with a single-element `photoIds`, so the credit is logged
      exactly as the sheet's is; no new route
- [x] A failed save reports it in Danish and keeps the typed text

## Progress Log

- 2026-09-25 — Task created from PRD 023.
- 2026-09-25 — Built as one file, `adminui/vieweredit.js`, registering **two** actions into the viewer's registry. Two controls and two requests rather than one form: a single form writing both fields would let a curator fixing a typo in a caption blank a credit by leaving it alone, and that is a loss nobody notices until a photographer asks why their name is gone.
- 2026-09-25 — **Registered from the admin side rather than built into `viewer.js`**, which is a deliberate departure from task 402's header. Two reasons, both structural: the write needs the working year (`ctx.fetch`, whose whole existence is to stop a silent wrong-year write) so `TestTheAdminScriptsFetchOnlyThroughTheYear` now covers this path for free; and `viewer.js` stays free of every admin concept, which matters because the strongest possible statement about a public editing control is that the code for it is not in the file the public page loads.
- 2026-09-25 — The viewer gained two generic events for this: `hv:show` (so an open editor follows the arrow keys — caption, right arrow, caption, which is the loop PRD 023 §4 was reopened for) and `hv:close`. Neither mentions captions or admin; the viewer only knows something might be listening.
- 2026-09-25 — The sheet is reloaded **once, on close**, not after each save. Swapping 120 thumbnails out from under somebody still looking at one, to update an attribute invisible on a cell, is the wrong trade — so the cell's attributes are updated in place immediately and the server reconciles when the overlay goes away. That still goes through `ctx.reloadSheet`, so there is no second path.
- 2026-09-25 — A failed save **keeps the typed text**. A caption is a sentence somebody composed and losing it to a dropped hotel connection is what makes a curator stop trusting the tool. Asserted as the absence of a reset on the error path, which is the only way to check it without executing it.
- 2026-09-25 — Two guards caught their own explanatory prose while being written: `TestTheCuratorsActionsOpenAsOverlays` matched a comment containing `panel.hidden = ` (fixed by renaming the variable to `editPanel` — the collision with the album sheet's name was accidental and the guard was right to be blunt), and the new test matched its own comment about `lastCredit`. That is the fifth and sixth time in this repo. The new test now strips comments through `withoutComments`.
- 2026-09-25 — ✅ Both guards break-tested: a combined `body.caption`/`body.credit` write fails, as it must.
- 2026-09-25 — Clearing a credit is its own button ("Fjern fotokredit"), styled as the destructive act it is, and only the credit has one. `creditaction.js` decided that removing an attribution should be deliberate rather than what a stray select-all-and-delete does on its way past, and the arrow keys make that more true here, not less.
- 2026-09-25 — The field shows the **public form while it is being typed** — "Offentligt: Foto: …" — updated on input rather than after saving. A curator putting a colleague's name on a public page should be looking at the public version of it as they write. The hint says so in Danish: this is the one field in the tool that names a person, and only a photographer who has agreed to it belongs there.
- 2026-09-25 — Prefilled from the **photograph**, never from `hej.admin.lastCredit`. That key exists because a batch has no single current value to show and retyping one line per card is real tedium; with one photograph in front of you the honest prefill is its own credit.
