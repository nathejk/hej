# 460 — Filters in the album view, and a caption filter to put in them

**Status:** done
**Priority:** high
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

Reported by the maintainer, with a screenshot of the album view:

> when in album context there are no filter options, but we do have a button for applying filters […] for being
> able to work with +100 photos in a single album, we need these filter options to see if any photos are missing a
> position or a caption - sometimes it's intended sometimes its an error

Two defects in one report:

1. **The album view had no filter row** — it was excluded when task 396 built the view, on the reasoning that the
   album *was* the filter. What that missed is that the album is where the work happens: at 100–300 photographs,
   "is anything missing a caption or a position" cannot be answered by looking, and the all-photos view cannot
   answer it either because it is not scoped to the album.
2. **The page still offered "Vælg alle der matcher filteret"** — a button naming a filter the page did not have.
   That is how the gap got noticed, and it is the more embarrassing half: the tool was describing a feature it had
   removed from that view.

And the filter the question actually needs did not exist anywhere: there was no way to ask for photographs
**without a caption**.

The maintainer's framing matters for the design — *"sometimes it's intended sometimes it's an error"*. This is not
a validation feature. A photograph that needs no caption is a decision, and the job of the filter is to make the
set **visible** so the decision can be made deliberately rather than by omission. So: no warnings, no badges
demanding attention, no "incomplete" count. A filter.

## What shipped

### A caption filter, end to end

`photo.Filter.HasCaption *bool` → `caption=yes|no` on `GET /api/admin/photos`, OpenAPI annotations updated.

Tested at the SQL level rather than only through the endpoint, for one specific reason: `caption` is
`TEXT NOT NULL` **with no default**, so an uncaptioned photograph holds `""`. An `IS NULL` test — the obvious way
to write this — would match nothing, and "uden billedtekst" would come back empty on a library full of them. An
empty grid reads as *"nothing to do here"*, which is the worst available way for this to be wrong.

`caption=yes|no` rather than `any|none`, deliberately inconsistent with `credit`: `credit` spells its sentinels as
words because it *also carries values* (`credit=Anne`), and `credit=no` would be ambiguous the day somebody is
credited as "no". A caption is prose and "which photographs say exactly this" is not a question anybody asks, so
`caption` follows `location`'s shape instead. `caption=any` and `caption=none` are **400s**, since they are the
plausible typo.

### One filter row, shown in both views

`adminFilters` gained `Uden billedtekst`, and `adminFiltersInAlbum` renders the same list minus the presets an
album cannot answer — which is exactly one, `album=none`. A flag on the preset rather than a second list, because
two lists drift, and *a filter a curator learns in one view and cannot find in the other is worse than no filter at
all*.

Only the negative caption preset is offered, following the credit preset's precedent (task 454): "which of these
has nobody written a line for" is the checklist question, and "med billedtekst" is the set you have finished with.

`album=none` is not merely hidden in the album view — it is **not honoured from that view's URL** either, which
matters because it would compose to `album=al-1&album=none` and the second wins in the parser: the unsorted pile
under an album's title.

### The composition, which is the part that could have gone badly

The album view's query is now `album={id}` **&** the preset, and the two halves are sent separately (`data-query`,
`data-base`) so the browser can recompose: a filter click replaces the preset and keeps the album, without a page
load — which is what makes the row feel like a filter rather than a navigation, and is what lets the selection
survive it (the re-paint in `contactsheet.js` is the load-bearing line there, and predates this).

The failure mode being guarded is not a wrong screen. `query` is what "Vælg alle der matcher filteret" pages and
what every action in the bar acts on, so a click that dropped `base` would mean **a bulk edit applied to
photographs the curator never saw**. Hence `compose()` in one place and
`TestTheFilterRowKeepsTheAlbumWhenItChangesThePreset`, which also fails if anyone writes `query = b.dataset.q`
again.

The URL gets the **preset only**: the album is already in the path by slug, and writing its id beside its slug
would be two answers to which album this is.

### Dragging is refused while the grid is filtered

This came with the row rather than being asked for, and it would have been a trap. A move says "these, before that
one" and the server rebuilds the album's **whole** order (`moveAdminAlbumItemsHandler`, because the browser may
hold 120 of 200). Drop a photograph before the twelfth *uncaptioned* one and it lands before the twelfth photograph
**of the album** — off screen — and the filtered grid can look unchanged afterwards, which reads as the drag having
failed. It also rewrites every ordinal in the album, so "looked like nothing happened" is not harmless.

So a drag is refused at the **start** of the gesture, with a sentence in the action line. Refused at the start,
unlike the sort-mode confirmation which deliberately waits for the end: there the curator has a decision to make
and may drop the photograph back where it came from, here there is no decision, and opening the gap would promise a
move that is not going to be sent.

`contactsheet.js` publishes `data-filtered` (it owns `query`, and a filter click does not reload the page, so the
server cannot say); `albumorder.js` reads it. Asserted as a pair, because the failure mode is a rename on one side —
the flag would simply never be set and dragging in a filtered album would silently come back.

### Also

`docs/billedarkiv-for-fotografer.md` §4 gained "Tjek albummet igennem, før du udgiver": the three filters as
questions, that leaving a photograph without a caption is allowed and the point is *seeing which*, that a filter is
an address you can send to a colleague, and that dragging needs **Alle** first.

## Rejected

- **A "mangler noget" badge or count per album.** It would turn a curator's decision into a defect list, and the
  maintainer's report is explicit that the absence is sometimes intended.
- **`Med billedtekst` as a button.** The finished set is not a working set; the negative is the checklist.
- **A second preset list for the album view.** Two lists that must agree, with nothing holding them equal.
- **Letting the drag through while filtered.** The move is well-defined server-side, so this was tempting — and it
  is unverifiable by the person making it, which is the same objection that made the 302-not-301 decision on the
  permalink.

## Acceptance Criteria

- [x] The album view has a filter row
- [x] Every preset in it narrows *within* the album
- [x] `caption=no` exists, and answers with the empty string rather than NULL
- [x] Presets that cannot apply in an album are neither shown nor honoured there
- [x] A filter is a shareable address, and survives a reload
- [x] "Vælg alle der matcher filteret" selects the composed set
- [x] Reordering cannot happen against a filtered grid, and says why
- [x] The photographers' guide explains it
- [x] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-09-28 — Shipped. Named the preset flag `HiddenInAlbum` after the privacy walk rejected `PhotosOnly`: the
  `photo` needle in `isPersonShaped` is deliberately blunt, and spending an allowlist entry on a layout flag would
  have blunted a real guard for nothing.
- 2026-09-28 — `TestTheAlbumViewIsTheEditorOverTheSharedSheet` asserted `id="filters"` was **absent**; that
  expectation is the defect, so it was inverted with the reasoning recorded there.
- 2026-09-28 — A needle of mine was wrong in an instructive way: in `hx-get="…?limit=120{{if .Query}}&{{.Query}}"`
  the literal `&` stays literal while the one *inside* `.Query` is escaped, so the attribute reads
  `…&album=al-1&amp;caption=no`. Asserted as actually written, with a note.
- 2026-09-28 — Mutation-checked by changing `query = compose(preset)` back to `query = preset`, which fails two
  assertions in the guard.
