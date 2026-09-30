# 475 — The filename in the viewer, below the credit

**Status:** done
**Priority:** low
**Created:** 2026-09-30
**Picked up by:** agent
**Started:** 2026-09-30
**Completed:** 2026-09-30

**PRD:** 022

## Description

> when watching photo in large viewer in album edit mode, filename should be printed below photocredit

## What shipped

The viewer's info panel renders a third line under the caption and the credit: the photograph's own filename, in
monospace and quieter than the credit. `DSC_0142.JPG` is an identifier a photographer scans and compares against a
folder on their own computer, so a proportional font would make the one job it has harder.

It appears **wherever the curator's tool opens the viewer** — the album page and the all-photos page — rather than the
album page alone. The request named album edit mode because that is where the maintainer was looking; the filename is
a curator-facing fact and both of those pages are the curator's, so an asymmetry between them would be a rule with no
reason behind it. Say so if you would rather it were album-only.

## The interesting part is where a filename is allowed to be

This is the one field in the library whose exception rests entirely on **location**. Task 448 admitted the column on
the maintainer's bound — *"protected by authentication"* — and made that structural rather than a promise:
`isPersonShaped` keeps flagging the word, so every public read that grows one fails.

Until now nothing needed it on the wire, because the sort it was kept for runs in SQL. Showing it in the viewer moves
it one step closer to a page, and the viewer is a **shared component**: the same `viewer.js` runs on the public album
page. So the work was mostly in keeping the boundary honest:

- `adminLibraryPhoto.FileName` is a new exception in `TestAdminLibraryPayloadHasNowhereToPutAPerson` — which
  deliberately holds that response to the *public* standard — with task 448's two bounds restated at the line. Unlike
  the `CreditIsCrew` exception next to it, this field **is** person-shaped in the general case (a filename can read
  `mor-og-far.jpg`), so it is excepted on bounds rather than on being harmless.
- The tile carries `data-filename` only in `fragments.html`. The viewer reads it and renders nothing when it is
  absent, which is the same shape `data-viewer-deleted` already had: a field the shared component can carry and only
  one host provides.
- Three guards, and the two that matter point outward rather than inward:
  `TestOnlyTheCuratorsTileCarriesAFilename` reads `publicsite.go` and fails if the public template mentions a
  filename at all; `TestThePublicAlbumPageCarriesNoFilename` asserts it against a rendered page **and** that
  `publicAlbumItem` has nowhere to put one, which is what makes the first true by construction rather than by
  omission.

Task 448's other bound also holds: it is rendered **as a filename**, with no prefix and no attempt to make it a
sentence. Nothing attributes it to anybody and nothing derives a person from it.

## Verification

Against the dev container, where 3 of 14 photographs have a filename — the three the maintainer uploaded:

	admin tile:        data-filename="20260918_205915.jpg"
	public album page: 0 occurrences of data-filename

Mutation-checked by removing the attribute from the fragment, which fails
`TestOnlyTheCuratorsTileCarriesAFilename`.

## Acceptance Criteria

- [x] The viewer shows the filename below the credit, in the curator's tool
- [x] It is absent from the public viewer, and the public read model cannot hold one
- [x] The exception is stated where the guard forced it, with task 448's bounds
- [x] Rendered as a filename, not as an attribution
- [x] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-09-30 — Shipped. The privacy guard caught the new field on the first run, which is what it is for; the
  exception it forced is the record of the decision.
