# 457 — Seven admin writes still reload in the same tick as their response

**Status:** open
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

## Description

Task 437 fixed this for **upload**, and task 439 pulled the fix onto the context so the rest of the tool could
use it. The rest of the tool never did. `ctx.settled` has exactly one caller, and `ctx.settledOrder` exactly one;
the other seven writes still do what the uploader was reported for:

| File | Write | What it reloads |
|---|---|---|
| `adminui/albumaction.js` | add/remove photographs to an album | `ctx.reloadSheet()` |
| `adminui/deleteaction.js` | delete, and undelete | `ctx.reloadSheet()` (twice) |
| `adminui/patrolaction.js` | tag/untag a patrol | `ctx.reloadSheet()` |
| `adminui/positionaction.js` | set/clear a coordinate | `ctx.reloadSheet()` |
| `adminui/captionaction.js` | set/clear a caption | `ctx.reloadSheet()` |
| `adminui/creditaction.js` | set/clear a credit | `ctx.reloadSheet()` |
| `adminui/vieweredit.js` | caption + credit from the viewer | `ctx.reloadSheet()` |

Every one of these **publishes an event**; the sheet reads a projection a jetstream consumer folds from it. The
two are milliseconds apart in a healthy system and **not ordered at all in principle**, so a reload in the same
tick as the response is a read with nothing behind it. This is not a theory — it is the report behind task 437:
a curator was left looking at a list of things they had just done, above a grid that did not have them.

It is less visible here than it was for upload only because the batches are smaller. A curator who captions one
photograph and sees the old caption come back assumes they mis-clicked and does it again; a curator who removes
forty photographs from an album and sees thirty-nine go assumes the fortieth failed. Both are the same race, and
both look like a different bug.

## This is not one fix applied seven times

The reason `ctx.settled(ids)` cannot simply be dropped in, recorded at the function in `adminui/main.js`:

> Deliberately **not** given to a caller that has no ids to wait for: `settled([])` is honest about a batch that
> stored nothing, but a caller that cannot name what it wrote needs its own answer to *"which ids prove this
> landed"* — and for the removals that answer is an id **disappearing**, which is a different read.

So each write owes a predicate, and they are genuinely different:

- **`captionaction`, `creditaction`, `positionaction`, `patrolaction`, `vieweredit`** — the ids already exist and
  come back regardless, so presence proves nothing. What settles is a **field's value**, which means either a read
  that returns the field (the library read does return captions and credits) or a per-field variant of the wait.
  Note the empty case is the hard one: "wait until the caption is gone" and "wait until the read has caught up"
  are indistinguishable to a poller that only checks presence of a value.
- **`albumaction`** — adding is presence *in the album read*, not in the library; removing is absence from it.
  `ctx.settledOrder` is next door and is the wrong tool (it waits on positions), but its shape is the right one.
- **`deleteaction`** — deleting settles when an id **stops** coming back from the default live-only library read;
  undeleting settles when it starts again. This is the case the comment above singles out, and it is the one that
  cannot reuse `settled` at all.

A plausible shape is `ctx.settledWhen(ids, predicate)` with `settled` and the absence case expressed through it,
but that is a design decision and should be taken with at least two of the seven in hand rather than one — the
generalisation that fits `deleteaction` and `captionaction` is probably the right one, and the one that fits only
`captionaction` probably is not.

## What must be preserved from tasks 437/439

- **Bounded.** A broken consumer is reported, not waited out (`SETTLE_MS`, backoff).
- **A failed read is not "not yet".** `seen` returns null and the wait gives up rather than polling a dead endpoint.
- **A boolean, and honesty on false** — refresh anyway and admit the view may be behind.
- **Chunking** against `maxAdminLibraryIDs` for anything that can cover a whole card.
- **The guard pattern.** `TestTheUploadWaitsForTheProjectionBeforeSayingItIsDone` asserts needles **against the
  file that owes them**, precisely so the wait cannot be dropped from one asset while another still shows it.
  Whatever lands here needs the same per-file discipline, and these are source-read guards because there is no
  JavaScript runtime in the suite — so keep the reads they poll covered by real Go tests, as
  `TestAdminLibraryFiltersByID` covers the `ids=` filter.
- Copy in Danish, as everywhere in this tool.

## Acceptance Criteria

- [ ] Each of the seven writes waits for a predicate it can actually prove, or records why it cannot
- [ ] Removals/deletes wait on an id **disappearing**, not on presence
- [ ] No second copy of the polling loop — whatever generalisation is chosen lives in `main.js`
- [ ] Bounded, gives up on a failed read, and tells the curator when it gave up
- [ ] Guards asserted against the file that owes each needle
- [ ] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-09-28 — Created. The gap has been known since task 439 put the wait on the context for exactly these
  callers; recorded now so it stops being a paragraph in a comment. Not scoped to a single mechanism on purpose:
  the shared shape should be chosen with the delete case and a field case in hand together.
