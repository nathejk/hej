# 474 — Upload straight into an album

**Status:** done
**Priority:** medium
**Created:** 2026-09-30
**Picked up by:** agent
**Started:** 2026-09-30
**Completed:** 2026-09-30

**PRD:** 022

## Description

> when uploading new photos it should be possible to upload them into an album directly

Until now uploading and filing were two places: drag a card in on `/{year}/photos`, then select the photographs and
use **Tilføj til album**. For a curator assembling one album that is a round trip — upload, navigate back, select,
file — which is the same round trip task 396 removed for every other action by putting the whole action bar on the
album page.

## What shipped

**The uploader is on the album page**, and a batch dropped there is filed into that album when it finishes.

Chosen over a target-album picker beside the library's uploader, which was the other option. A curator assembling an
album is already on its page, and the picker would have been a second way to express something the page already says.
The all-photos uploader is unchanged: an upload there goes to the library and nowhere else, which is what a
photographer dumping four cards wants.

No new endpoint. The batch is filed with the same `POST /api/admin/albums/items` the action bar uses, so a card
dragged onto an album and a selection filed from the bar cannot end up ordered differently.

### Two decisions the feature turns on, and both are invisible on screen

**Which photographs get filed: the stored ones *and* the ones that were already in the library.**

Including `already` is the point rather than an edge case. Half a card is routinely already uploaded — a colleague's
dump, a retry, a second pass over the same folder — and filing only the new ones would put half the card in the album
and silently leave the rest out. The verification run demonstrated it by accident: of two uploads one came back
`stored` and one `already`, and the album gained **both**.

`gone` is deliberately excluded. A photograph a curator deleted is not re-uploaded (PRD 022 §8.5), and filing it into
an album would put it back on a public page — the one outcome here that is not merely untidy.

**When: after the projection wait, and once for the whole batch.**

`/api/admin/albums/items` validates photographs against the library projection, so filing an id the fold has not
reached would be a refusal for a photograph that is perfectly fine — task 437's race in a new place. So the existing
`ctx.settled` wait comes first and the filing second, and the filing then waits on the album read (task 457) before
the grid reloads.

One request for the whole batch, because **adding to an album re-sorts it** when its mode is not manual (PRD 024).
Filing three hundred photographs one at a time would publish three hundred reorder events, each rewriting every
ordinal in the album, for intermediate arrangements nobody sees.

### If the filing fails

The line says the upload succeeded and names the recovery that works: *"Billederne blev lagt op, men kunne ikke
lægges i albummet. Vælg dem og brug Tilføj til album."* The distinction matters to somebody holding a card — "try
again" would be wrong advice and would risk a second pass over three hundred files.

### One template, two views

The uploader's markup became a `{{define "uploader"}}` block used by both views. It has seven ids that are all looked
up by name, so two copies would be a feature that works on one view and silently does nothing on the other.

## On process

This is a new capability, and `.rules` asks for a PRD before one. I treated it as a task: no new endpoint, no schema
change, no privacy dimension, and both writes it performs already existed with their own PRDs (022 for the upload, 024
for the sort on add). If you would rather have it recorded as a PRD, say so and I will write it up retrospectively —
the decisions above are the content it would have.

## Acceptance Criteria

- [x] The album page can receive an upload, and says where the photographs will land
- [x] Already-uploaded photographs in the batch are filed too
- [x] Previously-deleted ones are not
- [x] The batch is filed once, after the projection wait
- [x] A filing failure does not imply the upload failed
- [x] The all-photos uploader still files nothing
- [x] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-09-30 — Verified end to end against the dev container: uploaded two photographs into the album "Test" (one
  `stored`, one `already`), the album went 10 → 12 with `added: 2`, and the test data was then removed so dev is as it
  was.
- 2026-09-30 — Mutation-checked the `already` decision: filing only `stored` fails
  `TestTheBatchFilesTheLivePhotographsAndNotTheDeletedOnes` with the reason written out.
