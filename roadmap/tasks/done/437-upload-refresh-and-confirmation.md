# 437 — After an upload the grid was stale, and nothing said the card had arrived

**Status:** done
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

Reported from use: after dragging a card in, you are left with a list of uploaded photographs you
cannot act on — no selection, no actions — and the contact sheet below does not have them until you
reload the page by hand. Asked for a confirmation along the lines of "XX billeder tilføjet".

**The refresh was already there**, which is what made this worth writing down. `upload.js` has called
`ctx.reloadSheet()` at the end of a batch since task 373, with a comment explaining why. Reading the
code, nothing looks wrong.

What is wrong is *when* it fires. An upload **publishes an event**; the grid reads a projection that a
jetstream consumer folds from it — `uploadAdminPhotoHandler` says so explicitly, and refuses to answer
200 on a failed publish for precisely this reason. So the reload went out in the same tick as the last
upload's response, and read a table that was under no obligation to have the rows yet. On a fast local
stack it usually won the race, which is exactly why this survived: the bug is invisible when reading the
code *and* intermittent when using it.

The missing confirmation is the second half of the same fault. With no line at the end, "nothing
appeared" and "nothing was uploaded" look identical.

## What changed

**The batch's end waits for the library to hold what was stored, then reloads once, then says so.**

- `libraryTotal()` reads `GET /api/admin/photos?limit=1` for `counts.total`. The counts travel with
  every library read and are about the **whole year rather than the page** (`adminlibrary.go`), which is
  what makes them usable as a progress signal here — one row is all the page needed. A failed read
  resolves to `null`: a count we could not get is not a count that disagrees.
- `waitForTotal(target)` polls with backoff (150 ms doubling to 1 s) until the total has grown by the
  batch's `stored`, or a **10-second deadline** passes. Bounded deliberately: a broken consumer must be
  reported, not waited out. If the deadline passes, the sheet is reloaded anyway and the line says
  "Kontaktarket kan være et øjeblik bagud — genindlæs siden, hvis nogle mangler."
- A second photographer uploading concurrently only makes the total *larger*, so the wait can end early
  but cannot hang on their account. Recorded rather than guarded against: ending early degrades to the
  old behaviour, which is the safe direction.

**The confirmation names every outcome, not just the successes.** `#uploadnote` gets, for example
"12 billeder tilføjet — 3 billeder var lagt op i forvejen." Each non-success is named because each means
something different to the person holding the card: a duplicate is nothing to do, a previously-deleted
photograph is a decision somebody made (task 372), and a failure is a file to drag in again. Silence
about any of them reads as "all of it went up". Counted where the server's answer is read, not derived
from the rows afterwards — the rows are presentation.

`#uploadnote` is its own line, weighted like `#actionnote` and `aria-live="polite"`. Not the progress
widget's label: that one is about files going up, this one is about the library having them.

## Acceptance Criteria

- [x] The grid shows the new photographs without a manual page reload
- [x] The reload happens after the projection has caught up, not in the same tick as the last response
- [x] The wait is bounded and says so if it gives up
- [x] A confirmation names what was added
- [x] Duplicates, refused re-uploads and failures are named rather than folded into the total
- [x] Pinned by test, mutation-checked

## Progress Log

- 2026-09-28 — Picked up. First reading suggested the feature was missing; it was not — `ctx.reloadSheet()`
  has been at the end of a batch since 373.
- 2026-09-28 — Found the actual fault: the reload races the projection. The write is a publish and the read
  is a fold, so "reload when the last response lands" is a read with no guarantee behind it. It wins often
  enough locally to look fine, which is why it lasted.
- 2026-09-28 — Considered polling for the uploaded **ids** instead of a count, and rejected it: under a
  filter the new photographs may legitimately not be in the grid at all, so "are they visible" is not a
  question with a stable answer. The year's total is filter-independent, which is what makes it the right
  signal.
- 2026-09-28 — ✅ `libraryTotal`, `waitForTotal`, per-batch tallies, `summary`, `#uploadnote`.
- 2026-09-28 — ✅ Guard `TestTheUploadWaitsForTheProjectionBeforeSayingItIsDone` in `adminupload_test.go`:
  the wait, its bound, the null-count case, the probe, the reload, the line, and all four tallies.
  Mutation-checked by short-circuiting the wait and by dropping one tally — both fail.
- 2026-09-28 — `gofmt`, `go vet`, `staticcheck` and `go test ./cmd/api/ -count=1` all clean.

## Worth knowing

**Every other write in the tool has the same race.** `albumaction`, `deleteaction`, `patrolaction`,
`positionaction`, `captionaction`, `creditaction`, `albumorder` and `vieweredit` all call
`ctx.reloadSheet()` immediately after their response. They are much less likely to be noticed — one row
folded rather than three hundred, and the visible change is an attribute on a cell that is already
there rather than cells that are absent — but the shape of the bug is identical. Fixed here only for the
upload, because that is where it was reported and where the fold is largest. If it shows up elsewhere,
the thing to reach for is not a copy of this polling loop in eight files: it is one `ctx.settled()` on
the context, with this as its first caller.

The deadline is 10 s. If a real card ever exceeds it, the consumer is the thing to look at, not this
number — a fold that takes longer than ten seconds for one batch is a problem the curator should be told
about, which is what the fallback sentence does.
