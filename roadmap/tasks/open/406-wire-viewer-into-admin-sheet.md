# 406 — Wire the viewer into the admin contact sheet and album editor, without making a click ambiguous

**Status:** open
**Priority:** medium
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
