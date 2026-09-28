# 446 — The sort control in the album editor card

**Status:** done
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

PRD 024 §6 R1/R10 and §7. Needs 442 and 444.

A native `<select>` in the album editor card (`#albumeditor`, `albumeditor.js`) — where the title,
description, cover and publish state already are, because the sort is a property of the album. Saved on
change through the album PATCH the card already uses, then the grid reflects it.

Danish labels: *Manuel*, *Tid, ældste først*, *Tid, nyeste først*, *Filnavn A–Å*, *Filnavn Å–A*.

**This is the server-rendered admin tool, not the PWA** — Go `html/template` with vendored Pico, htmx and
Alpine, no build step. See the `go-server-rendered-pages` skill. A native `<select>` also means no keyboard
handling of our own.

Worth a line of help text under it: what a non-manual mode will do to photographs added later. The curator
choosing the mode is the person who needs to know that, at the moment they choose.

## Acceptance Criteria

- [ ] The control renders the album's current mode
- [ ] Changing it saves and the grid shows the new order without a manual page reload
- [ ] The five labels, in Danish
- [ ] A one-line explanation of what the mode does to photographs added later
- [ ] No Tailwind, no shadcn-vue, no new front-end dependency

## Progress Log

- 2026-09-28 — Task created from PRD 024 §6 R1/R10, §7.

## What changed

A native `<select id="sortmode">` in the editor card, saved **on change** rather than by the card's "Gem
ændringer" button: it takes effect immediately — the server re-sorts and the grid reorders — and a control whose
effect is visible while its value is unsaved is the combination nobody can reason about.

**The grid waits for the order, not for the photographs.** `ctx.settled` is the wrong tool here and reaching for
it is the obvious mistake: it waits for photographs to *exist*, and after a re-sort they already did. Their
presence says nothing about their positions, so a grid reloaded on that signal can still show the old order —
the complaint tasks 437/438 fixed for the uploader, in a new place. So `PATCH album` now answers with
`resortedOrder`, the order it published, and `ctx.settledOrder(albumId, order)` waits until the album read agrees
with it. Same bound, same backoff, same honesty on giving up.

## The copy

> **Billedernes rækkefølge** — Manuel · Tid, ældste først · Tid, nyeste først · Filnavn A–Å · Filnavn Å–A
>
> Billeder, du lægger i albummet, falder automatisk på plads i den rækkefølge du vælger her. Vælger du
> **Manuel**, bestemmer du selv: nye billeder lægges til sidst, og du flytter dem rundt ved at trække dem.
> Trækker du et billede i et album der sorterer sig selv, skifter albummet til Manuel — du bliver spurgt først.
>
> Tid er det tidspunkt kameraet har skrevet i filen. Mangler det, bruges tidspunktet billedet blev lagt op.

Two hints rather than one. The first is about the choice; the second is about the **data**, and it is there
because "sorted by time" invites a curator to trust a timestamp that a large minority of files do not carry. A
photograph landing somewhere unexpected is then explainable rather than a bug report.

The label is *"Billedernes rækkefølge"*, deliberately not *"Sortering"* — the field directly above it is
*"Rækkefølge på forsiden"*, which is `sortOrder`, the album's place among the albums. Two controls a few
millimetres apart, both about order, about different things: the labels have to do that work.

After a save: *"Albummet er nu sorteret efter tid, ældste først."*, or *"Albummet lå allerede i den
rækkefølge."*, or for manual *"Albummet er nu i manuel rækkefølge. Du bestemmer selv, og nye billeder lægges til
sidst."*

## Acceptance Criteria

- [x] The control renders the album's current mode
- [x] Changing it saves and the grid shows the new order without a manual page reload
- [x] The five labels, in Danish
- [x] A one-line explanation of what the mode does to photographs added later
- [x] No Tailwind, no shadcn-vue, no new front-end dependency

## Progress Log

- 2026-09-28 — Picked up with 445.
- 2026-09-28 — The labels exist twice — in the markup a curator chooses from, and in `ctx.sortModeName`, which is
  what the tool says back and what the drag warning names. They cannot share a definition: no build step, so a Go
  template cannot be called from JavaScript. `TestTheSortModeLabelsAgree` holds them equal instead, parsing the
  `<option>`s and checking each against the map — the one that would drift is the warning, which is where it
  matters most.
- 2026-09-28 — Added `ctx.settledOrder` rather than reusing `ctx.settled`, for the reason above. A third caller
  of the same idea, and the second shape of it: presence, and now order.
- 2026-09-28 — Found that the editor card is one template shared by the album **page** and the fragment, so
  `adminAlbumPageData` needed the field too. The failure was silent — a template that stops executing mid-card
  returns a truncated page with a 200 — and was caught by an existing view test rather than by mine.
- 2026-09-28 — `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...` clean.
