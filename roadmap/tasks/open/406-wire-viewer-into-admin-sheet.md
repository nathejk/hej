# 406 — Wire the viewer into the admin contact sheet and album editor, without making a click ambiguous

**Status:** open
**Priority:** medium (blocked: needs a decision, see the Description)
**Created:** 2026-09-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 023 §8 ("Functional — admin", and the Technical Considerations that follow). The curator's tool starts
using the same viewer file the public album page does, from the same two URLs — which is the point of task 402
and the thing PRD 023 §9 says it fails if a second copy appears.

The gap this closes (§2): the contact sheet exists to **sort**, so a click selects, which is right — and that
leaves no gesture meaning *"show me this one big enough to judge"*. A curator deciding whether a photograph is
worth publishing is currently doing it from a 150 px tile.

### The hard constraint

**A click on the cell must still select, and only select.** PRD 022 §7's selection model is the thing the whole
tool is built around, and PRD 023 §3 restates it: nothing here may make a click ambiguous. So the viewer opens
from a **small expand control in the cell's corner**, whose handler stops propagation, and the selection after
`Esc` is exactly what it was before. Shift-click ranges and keyboard navigation must be untouched.

> **BLOCKED on a decision, 2026-09-25.** The expand-control-in-the-corner design above cannot be built as
> written, and the alternatives all cost something a maintainer should choose between rather than an
> implementer.
>
> The cell **is** the button: `fragments.html` renders
> `<button type="button" class="cell…" role="option" aria-selected="false">`, inside a
> `<div id="sheet" role="listbox" aria-multiselectable="true">`. A control in its corner therefore means a
> `<button>` inside a `<button>`, which is invalid HTML and behaves differently in every browser — the inner
> one is not reliably clickable, and a screen reader is given a control it cannot describe. `stopPropagation`
> does not help, because the problem is the markup rather than the event.
>
> The four ways out, with what each costs:
>
> 1. **Wrap each cell in a container** carrying the control beside it. Breaks the listbox: an `option` must be
>    a child of its `listbox` (or owned via `aria-owns`), so this trades a keyboard-navigable, announced grid
>    for a visual one. The sheet's accessibility is not obviously less important than this feature.
> 2. **Make the wrapper the `option` and the cell a `div`.** Selection stops being a button, so
>    `contactsheet.js`'s click, shift-click-range and keyboard handling all move — the riskiest change in the
>    tool, for a viewer.
> 3. **Open from the action bar** — "Vis stort", enabled with exactly one photograph selected. Costs a second
>    click, gains discoverability, and needs no markup change at all: it is precisely how every other action in
>    this tool works, and the selection model is untouched by construction.
> 4. **Double-click the cell.** One gesture, familiar from Finder and Photos, no markup change; but invisible,
>    and it toggles the selection twice on the way (harmless, but it is a real thing to reason about).
>
> 3 and 4 are not exclusive, and 3 + 4 together is my recommendation: the action bar for discoverability and
> the double-click for the curator who has learned it. Both leave a click meaning exactly what it means today.
>
> This also affects tasks 407 and 408, which open editors *in* the viewer — they need the viewer to open at
> all, but nothing about them depends on which of these four is chosen.

Scope, per §8: `adminui/fragments.html`, `page.css` and `contactsheet.js` for the control and the two lines
that open the viewer; `page.html` gains the viewer's two asset tags and
`data-viewer-actions="caption,fullscreen"`. Admin cells carry the same `data-` attributes as the public tiles
(§7.4), pointing at `/api/admin/photos/{id}/media?year=…`. The album editor's sheet gets the same control, with
**the album's order as the viewer's order** — the editor is where order is the subject, so a viewer that walked
the library's order there would be showing a different sequence from the one on screen. A photograph already
marked `gone` stays visibly deleted in the viewer rather than being presented as live (§5).

### Two guards will fail, and both failures are the process working

- **`TestTheAdminToolAddsNothingToTheFrontend`** allowlists asset tags on the admin page by whole tag, each with
  the reason it is permitted. Add `/viewer/viewer.js` and `/viewer/viewer.css` **by name, with the reasoning**,
  exactly as its existing entries do — the reasoning being that this is the same file the public album page
  loads, which is the requirement rather than a coincidence. **Never by loosening the needle.**
- **`page.css` gains a new `PICO:` reconciliation** (§8 calls it the fourth entry). Pico puts native
  `<dialog>` at `z-index: 999` and styles it — flex, `backdrop-filter`, its own background — while the tool's
  local overlays deliberately sit at **1040/1050** above it, as that file's existing note records. The viewer is
  a real `<dialog>`, so it must be reset out of Pico's styling and placed above both, **scoped so the public
  site — which has no Pico — is unaffected**. Getting this wrong is a viewer that opens behind an action sheet,
  or a public viewer restyled to fix an admin-only problem.

## Acceptance Criteria

- [ ] Each contact-sheet cell has an expand control that opens the viewer; a click anywhere else on the cell
      still selects, and only selects
- [ ] The selection, shift-click ranges and keyboard navigation are unchanged after opening and closing the
      viewer — asserted, since it is PRD 022 §7's hard constraint
- [ ] The album editor's sheet has the same control and passes the album's order as the viewer's order
- [ ] A deleted photograph is still visibly deleted in the viewer
- [ ] `TestTheAdminToolAddsNothingToTheFrontend` allowlists the two `/viewer/...` tags by name with reasoning,
      and the needle itself is unchanged
- [ ] `page.css` carries a new `PICO:` note placing the viewer's `<dialog>` above 1040/1050 and resetting
      Pico's `dialog` styling, scoped so the public site is untouched

## Progress Log

- 2026-09-25 — Task created from PRD 023.
- 2026-09-25 — **Blocked before starting.** The expand-control-in-the-corner design needs a `<button>` inside a `<button>`, because the cell *is* the button and its parent is a `listbox`. Four ways out are written up in the Description with what each costs; 3 + 4 (an action-bar entry plus double-click) is the recommendation, and both leave a click meaning exactly what it means today. Not chosen unilaterally: options 1 and 2 trade away either the sheet's accessibility or its selection code, which is the tool's riskiest surface, and PRD 022 §7 treats the selection model as sacred.
