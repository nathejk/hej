# 457 — Seven admin writes still reload in the same tick as their response

**Status:** done
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

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

- 2026-09-28 — Picked up.

## What shipped

**Nine writes, three questions.** The shape of the answer was not "apply the fix seven times" and not one
generalisation either — it was noticing that the tool asks one of three things, and which one follows from what the
write changed:

| Question | Primitive | Callers |
|---|---|---|
| Did these photographs **appear**? | `ctx.settled(ids)` | upload — and nothing else, because every other write acts on photographs that already existed |
| Does a **filtered read** agree? | `ctx.settledFilter(ids, filter, want)` | album add (`&album=`), album remove (`&album=`, **want false**), credit (`&credit=`), patrol (`&tagged=yes`), position (`&location=`+`&verdict=`) |
| Does the **row** say what we wrote? | `ctx.settledRows(ids, ok)` | caption, delete (`row.deleted === true`), viewer-edit captions |
| Does the **order** agree? | `ctx.settledOrder` (existed) | sort mode (PRD 024), and now the hand move |

All of them are the same bounded, backing-off loop over the same read: `waitFor` in `main.js`, with `askLibrary` /
`rowsFor` under it. `ctx.settled` is now three words long — `settledFilter(ids, '', true)` — and the loop that used
to be inside it is shared by nine callers instead of one.

### The key realisation: the library's own filters are the proof

`Filter.where` ANDs every condition, so `ids=` **composes** with every other filter. That turns `&album=x` from
"what is in x" into "which of these ids are in x", and it means the settle condition can be expressed in the
projection's own terms rather than the browser's. This matters most for the credit: a crew credit is stored as an
id and rendered as a name, so a client comparing strings would be guessing at the server's formatting —
`&credit=<crew id>` asks the question exactly, and the same expression covers a typed line and `none` for a
clearing.

### The removal was the case the design had to bend around

`main.js` had already written down that a removal's proof is an id *disappearing*, and that is why `settledFilter`
takes `want` rather than assuming presence. Two things fell out of implementing it:

- **The delete does not use absence.** Absence from the live-only read would also be true of a photograph the
  library cannot see at all — a wrong year, a mistyped id — which would read as a successful delete. So the delete
  reads with `deleted=1` and waits for the row to *say* `deleted: true`. `settledRows` asks with `deleted=1` for
  exactly this reason, and `TestTheUploadersWaitDoesNotAskForDeletedPhotographs` holds the uploader on the
  live-only read, because a re-uploaded deleted photograph publishes the same id and must not read as present
  (PRD 022 §8.5).
- **Removing and deleting live in one file and must not swap proofs.** Getting them the wrong way round would be
  invisible: every removal would wait on a flag no event sets, time out, and warn the curator for nothing.
  `TestTheRemovalAndTheDeleteWaitForDifferentThings` pins both directions.

### A server change was needed for the hand move (an eighth write, not in the original seven)

`PATCH /admin/albums/{albumId}/move` answered **204**, and the client cannot compute the order to wait for:
`moveAlbumOrder` builds it server-side from the projection precisely because the browser may hold only the first
page of a long album. Re-implementing that ordering rule in JavaScript would have been two copies of one rule on a
surface with no build step to share them — and the wait would then be checking the browser's arithmetic rather
than the server's answer.

So the move now answers **200 with `{"order": [...]}`**, exactly as the sort-mode change has carried
`resortedOrder` since PRD 024, and `albumorder.js` waits with `ctx.settledOrder`. OpenAPI annotations updated;
three tests moved from 204 to 200, and one now asserts the order comes back.

### Honest limits, both documented at the code

- **Patrol tags** can only be proved as far as `tagged=yes`. The library exposes no tag ids, deliberately — there
  is no patrol list anywhere in the service (PRD 022 §8.6) and a tag-id read would be one — so tagging an
  already-tagged photograph gets a wait that can end early.
- **Re-positioning** a photograph that already had a coordinate with the same verdict can settle early, since the
  filter it must satisfy was already true. Distinguishing it needs a per-photograph revision the read does not
  carry. Clearing a coordinate, which is the case where being wrong would leave a photograph on the public map
  after a curator took it off, is exact.

Both are ceilings of the reads that exist rather than things to work around in the client, and both leave a stale
mark rather than wrong text.

### Copy

One sentence for the caveat, `ctx.behindNote`, because seven writes now say it and seven wordings is how a curator
learns to ignore all of them. `albumeditor.js` was folded onto it; the uploader keeps its more specific tail
("hvis nogle mangler"), which is the one place the reader can actually check.

### Guards (`adminsettle_test.go`)

- `TestEveryAdminScriptThatReloadsTheSheetWaitsFirst` — the blanket rule, driven off `adminPageScripts`, so a
  ninth action gets it for free. This is the one that will still be working in a year.
- `TestEachAdminWriteWaitsForWhatItActuallyChanged` — because the blanket test is satisfied by the *wrong* wait,
  which is the mistake this tool has already made twice.
- plus the removal/delete pair, the live-only rule, and the one-wording rule.

Mutation-checked three ways: stubbing caption's wait to `true` fails both the blanket and the specific guard;
flipping the removal's `want` to `true` fails two tests; the earlier 302/301 lesson was applied and the container
gates re-run after restoring each mutation.

## Acceptance Criteria

- [x] Each of the seven writes waits for a predicate it can actually prove, or records why it cannot — plus the
      hand move, which turned out to be an eighth
- [x] Removals/deletes wait on an id **disappearing** (removal) and on the row's `deleted` flag (delete)
- [x] No second copy of the polling loop — `waitFor` in `main.js` is the only one, and `ctx.settled` now sits on it
- [x] Bounded, gives up on a failed read, and tells the curator when it gave up
- [x] Guards asserted against the file that owes each needle
- [x] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-09-28 — Picked up.
- 2026-09-28 — Three primitives rather than one, because the filters compose with `ids=`. Move endpoint changed
  from 204 to 200 with the published order; every other write is client-side only.
- 2026-09-28 — Full gate clean. `TestAMorningAfterBurstCostsOneTrackRead` flaked twice with connection resets
  under a parallel full-suite run and passes alone at `-count=3`; unrelated to this change, worth a task if it
  recurs.
