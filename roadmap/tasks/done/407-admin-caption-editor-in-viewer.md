# 407 — The caption, editable in the viewer's info panel

**Status:** done
**Priority:** medium
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

PRD 023 §6 ("Functional — admin") and §7.7. In the **admin** viewer the caption line in the info panel becomes
a plain `<textarea>` — one line, growing to three — with a save control, opened by the pencil in the action row.

This overrides PRD 023 §4's own first draft, and the reasoning it overrides is worth keeping because it still
holds for everything else: PRD 022 §5 spent its copy budget making "remove from album" and "delete from
library" impossible to confuse, and a destructive action in a dark overlay one icon from a share button is how
that work gets undone. A caption is different in kind — **non-destructive, reversible, and one of the two things
you can only judge while looking at the photograph large**, which is exactly the state the viewer puts you in.
Delete, album membership, position and patrol tag stay on the sheet's action bar.

### No new endpoint

It calls the existing `PATCH /api/admin/photos` with a **single-element `photoIds`**: `{photoIds:[id], caption}`.
That endpoint exists, is annotated, is behind `requireAdmin`, and is the same call the sheets make — so the
viewer inherits their behaviour for free. §8 is explicit that this is the point of choosing it: *"a second write
path for one field would be a second place to get 'belongs to the photograph, not the album' wrong."* **No Go
change is expected in this task.**

### It carries the sheet's own sentence, verbatim

`captionaction.js` and the caption sheet in `page.html` already make the point that the caption belongs to the
**photograph** and not to the album, so setting it here changes it in every album the photograph appears in. The
viewer must say the same thing **in the same Danish words, copied rather than re-worded** — today that is:

> Billedteksten hører til billedet, så den er den samme i alle album billedet ligger i.

Two phrasings of one fact is how a curator learns to distrust both.

### Failure and refresh

- **A failed save says so and keeps the typed text.** A caption is a sentence somebody composed; losing it to a
  dropped hotel connection is the failure mode that makes a curator stop trusting the tool.
- On success, update the host item's `alt` and `data-caption` so the sheet behind the overlay is not stale, then
  **reuse the sheet's existing post-action refresh**. Do not invent a second way for this to reach the sheet;
  the one that exists is the one that is already tested (§7.7).

The editor is **absent** on the public surface rather than hidden: the control is rendered because the admin
page declares the `caption` action, and the public page declares no such action (§7.7, task 402).

The payoff is the loop the arrow keys make possible: caption a photograph, press `→`, caption the next. §9 sets
the bar — if this is not faster than the sheet's caption panel for the one-at-a-time case, the editor is in the
wrong place.

Depends on tasks 402 and 406.

## Acceptance Criteria

- [x] The caption is editable in the admin viewer's info panel and saves through `PATCH /api/admin/photos` with
      a single-element `photoIds`, with no new route and no Go change
- [x] The panel carries the caption sheet's existing Danish sentence about the caption belonging to the
      photograph, character-for-character identical to the sheet's
- [x] A failed save reports the failure in Danish and the typed text is still there afterwards
- [x] A successful save updates the host item's `alt` and `data-caption` and triggers the sheet's **existing**
      post-action refresh, with no second refresh path added
- [x] `→` after saving moves to the next photograph with the editor ready, so a run can be captioned without
      leaving the viewer
- [x] Nothing renders the editor on the public album page, because that page declares no `caption` action

## Progress Log

- 2026-09-25 — Task created from PRD 023.
- 2026-09-25 — Built as one file, `adminui/vieweredit.js`, registering **two** actions into the viewer's registry. Two controls and two requests rather than one form: a single form writing both fields would let a curator fixing a typo in a caption blank a credit by leaving it alone, and that is a loss nobody notices until a photographer asks why their name is gone.
- 2026-09-25 — **Registered from the admin side rather than built into `viewer.js`**, which is a deliberate departure from task 402's header. Two reasons, both structural: the write needs the working year (`ctx.fetch`, whose whole existence is to stop a silent wrong-year write) so `TestTheAdminScriptsFetchOnlyThroughTheYear` now covers this path for free; and `viewer.js` stays free of every admin concept, which matters because the strongest possible statement about a public editing control is that the code for it is not in the file the public page loads.
- 2026-09-25 — The viewer gained two generic events for this: `hv:show` (so an open editor follows the arrow keys — caption, right arrow, caption, which is the loop PRD 023 §4 was reopened for) and `hv:close`. Neither mentions captions or admin; the viewer only knows something might be listening.
- 2026-09-25 — The sheet is reloaded **once, on close**, not after each save. Swapping 120 thumbnails out from under somebody still looking at one, to update an attribute invisible on a cell, is the wrong trade — so the cell's attributes are updated in place immediately and the server reconciles when the overlay goes away. That still goes through `ctx.reloadSheet`, so there is no second path.
- 2026-09-25 — A failed save **keeps the typed text**. A caption is a sentence somebody composed and losing it to a dropped hotel connection is what makes a curator stop trusting the tool. Asserted as the absence of a reset on the error path, which is the only way to check it without executing it.
- 2026-09-25 — Two guards caught their own explanatory prose while being written: `TestTheCuratorsActionsOpenAsOverlays` matched a comment containing `panel.hidden = ` (fixed by renaming the variable to `editPanel` — the collision with the album sheet's name was accidental and the guard was right to be blunt), and the new test matched its own comment about `lastCredit`. That is the fifth and sixth time in this repo. The new test now strips comments through `withoutComments`.
- 2026-09-25 — ✅ Both guards break-tested: a combined `body.caption`/`body.credit` write fails, as it must.
- 2026-09-25 — The caption's hint is `captionaction.js`'s own sentence about belonging to the photograph rather than to the album, copied and not re-worded: two phrasings of one fact is how a curator learns to distrust both. Prefilled from the photograph, and the caption also updates the cell's `alt`, which is where the sheet's existing caption panel reads it from.
