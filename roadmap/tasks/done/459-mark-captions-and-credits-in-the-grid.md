# 459 — Mark a thumbnail when it has a caption, and when it has a credit

**Status:** done
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

Asked for by the maintainer:

> in album admin view the thumbnails should be marked with an icon if credit and caption exists - 2 different icons.

The cells already carry four marks — the position verdict, how many albums, whether a patrol is tagged, whether it
is deleted — and captions and credits were the two facts a curator had no way to see from the grid. Both are work
that gets done photograph by photograph over several sittings, so "which of these still needs one" is the question
the sheet is being scanned for, and answering it meant opening cells one at a time.

## What shipped

Two marks in the cell's `.marks` row, from the fields the fragment already had (`adminLibraryPhoto.Caption`,
`.Credit`) — no read, no model and no endpoint changed:

- **caption** → Lucide `pencil`
- **credit** → Lucide `user`

The same two icons the viewer's edit buttons use for those fields (`vieweredit.js`), so the icon a curator clicks
to write a caption is the icon that tells them one exists.

It lands on **both** views, not only the album one. The contact sheet is a single fragment shared by the album page
and the all-photos page (`readAdminLibraryPage` is shared for the same reason — task 395), and "what still needs a
caption" is at least as much an all-photos question. Splitting them would have meant two cell templates that must
otherwise stay identical.

### Icons for these two, words for the others

Deliberate, and the asymmetry is the point. The position/album/patrol marks are read **one cell at a time**, when a
curator has already stopped on a photograph — words are clearer there and "uden for området" cannot be an icon.
Caption and credit are scanned **a screenful at a time**, and two more text pills in a row that already holds up to
four would have made that slower rather than faster.

### The credit mark says that there *is* one, never who

A credit is a person's name — the single PRD 011 §0b.1 exception this tool carries (task 393) — and a grid of 120
thumbnails is the wrong place to print a dozen of them at 11px. The name still reaches the cell in `data-credit`,
because the viewer reads it from there; what it must not do is get rendered.
`TestTheCaptionAndCreditMarksNameNobody` holds that, including the caption: the cell's `alt` already carries the
text, and a mark repeating it would be noise.

`role="img"` plus a label on each mark, because the cell is a `role="option"` whose accessible name is computed
from its contents — an unlabelled icon would simply be silent, and these two labels now join the other marks'
words.

### A sprite, not inline SVG per cell

The symbols are defined once in `page.html` and referenced with `<use>`. Two reasons, and the second is the one
that would have bitten later:

- a grid is up to 120 cells, so inlining two Lucide paths per cell repeats ~25 kB of identical markup on a surface
  served `no-store` (task 371) — re-fetched on every filter change and after every action;
- the definitions cannot live in the **fragment**, because the fragment is swapped on a filter change and
  *appended to* by "Hent flere" — a `<symbol>` inside it would be replaced on one path and duplicated on the other.

`TestTheContactSheetIconsAreDefinedOncePerPage` pins both halves: exactly one definition in the page, none in the
fragment.

## Acceptance Criteria

- [x] A photograph with a caption is marked; one without is not
- [x] A photograph with a credit is marked; one without is not
- [x] Two visibly different icons, from Lucide, inline (no build step on this surface)
- [x] Neither mark renders the caption text or the credit name
- [x] Both marks are labelled for a screen reader
- [x] Defined once per page rather than per cell
- [x] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-09-28 — Shipped. Mutation-checked by rendering `{{.Credit}}` in place of the icon, which fails the
  "names nobody" guard as well as the icon guard — the check that matters, since that is the plausible future edit.
- 2026-09-28 — Files: `adminui/page.html` (sprite), `adminui/fragments.html` (the two marks),
  `adminui/page.css` (`.mark.meta`, `.sprite`), `adminfragments_test.go` (three tests).
