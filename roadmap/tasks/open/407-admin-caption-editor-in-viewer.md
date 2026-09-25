# 407 — The caption, editable in the viewer's info panel

**Status:** open
**Priority:** medium
**Created:** 2026-09-25
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] The caption is editable in the admin viewer's info panel and saves through `PATCH /api/admin/photos` with
      a single-element `photoIds`, with no new route and no Go change
- [ ] The panel carries the caption sheet's existing Danish sentence about the caption belonging to the
      photograph, character-for-character identical to the sheet's
- [ ] A failed save reports the failure in Danish and the typed text is still there afterwards
- [ ] A successful save updates the host item's `alt` and `data-caption` and triggers the sheet's **existing**
      post-action refresh, with no second refresh path added
- [ ] `→` after saving moves to the next photograph with the editor ready, so a run can be captioned without
      leaving the viewer
- [ ] Nothing renders the editor on the public album page, because that page declares no `caption` action

## Progress Log

- 2026-09-25 — Task created from PRD 023.
