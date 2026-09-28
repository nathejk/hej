# 438 — Wait for the ids the upload returned, not for the library's total

**Status:** done
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

Task 437 made the uploader wait for the projection before reloading the grid, and waited by polling the
year's photograph **count** until it had grown by the number stored. The maintainer named the better
design on reading it:

> the api should answer back with an OK and a newly generated photoId, then we should be able to wait
> until we get this id back from the server, signifying that the photo have been on the stream and have
> been projected

That is exact where the count was a proxy, and the response already carries the id — `adminUploadResponse`
has returned `photoId` since task 372, because the id *is* the content hash of the stored rendition.

The count had three faults, and the first is the one that matters:

- **It could not say which photographs had landed.** A batch of three hundred either satisfied the
  arithmetic or did not; when it did not, the tool had no idea whether that was one file or all of them.
- **It needed a reading from before the batch**, held as a promise through the whole upload, and had no
  answer at all when that reading failed.
- **It could be satisfied by somebody else's upload.** Two photographers working at once is the ordinary
  case the day after the event, and a colleague's card could end the wait early. Documented as
  "degrades to the old behaviour, which is the safe direction" — true, and still a race written into a
  mechanism whose entire job was to remove one.

Getting an id back from a **read** is not a proxy for anything. It is the condition itself: the event has
been through the stream and the consumer has folded it.

## What changed

**A presence filter on the library read.** `photo.Filter.PhotoIDs` renders `p.photoId IN (?, …)`, one
placeholder per id, bound like every other value there — `TestTheIDFilterBindsRatherThanSplices` holds
that, because this is the first filter value that arrives as a *list* from a query string. It composes
with the rest, which means it inherits `deleted = 0`: asking after a photograph a curator removed
correctly returns nothing.

`GET /api/admin/photos?ids=a,b,c`, at most `maxAdminLibraryIDs` = 200. **Refused rather than clamped**,
like every other filter value in `adminLibraryFilter` and for a sharper reason than usual here: a
silently shortened list answers a different question, and the caller would read the missing ids as "not
there yet" and wait for something that is never coming. Whitespace is trimmed and empty elements dropped,
so a trailing comma from a client that built the list in a loop is not an id that can never be found.

**The uploader waits for its own ids.** `batch.stored` is now the array of ids the server generated, not
a number. `waitForPhotos` asks in chunks of 100, drops the ids that came back, and re-asks only for the
rest — so a card of three hundred converges rather than re-asking after the ones that arrived first. Same
10-second bound, same backoff, same fallback sentence when it gives up. The before-reading and its
promise are gone.

**A response without a `photoId` is now an error rather than a success.** It is the endpoint's contract
and the wait is built on it, so `applyOutcome` refuses to claim a file went up when it cannot tell which
photograph it became.

The guard in `adminupload_test.go` also now checks that the client's chunk size is one the endpoint will
accept: a client asking about more than 200 at a time would get a 400 on every poll, which presents
exactly as "the photographs never arrived".

## Acceptance Criteria

- [x] The wait is for the ids the upload returned
- [x] Ids are bound, never spliced, and compose with the year and the live-only default
- [x] An over-long or unusable id list is refused, not clamped
- [x] A batch larger than one request's worth converges
- [x] The wait is still bounded and still says so if it gives up
- [x] A success without an id is not reported as a success
- [x] Pinned by test, mutation-checked

## Progress Log

- 2026-09-28 — Picked up from the maintainer's reading of 437. The count was my choice and it was the
  weaker one; the id was already in the response.
- 2026-09-28 — Re-examined why I had rejected id-polling in 437. The note there says "under a filter the
  new photographs may legitimately not be in the grid at all, so 'are they visible' is not a question
  with a stable answer" — which is true of the *grid*, and I let that decide the shape of a **presence**
  read, which has nothing to do with the grid's filter. That conflation is the whole mistake.
- 2026-09-28 — Added `Filter.PhotoIDs` and the `ids` parameter. Unit-tested the SQL directly
  (`nathejk/table/photo/curatorfilter_test.go`) rather than only through the endpoint: the filter answers
  a question about timing, and one that quietly matched nothing or everything would produce either a wait
  that never ends or a reload that fires too early — both indistinguishable from the bug 437 fixed.
- 2026-09-28 — Chose 200 as the ceiling and 100 as the client's chunk. The ceiling exists because the
  statement's width is the caller's to keep sane; the chunk is half of it so the limit is not one request
  away.
- 2026-09-28 — ✅ Rewrote `waitForPhotos`, dropped `libraryTotal`/`waitForTotal` and the before-promise.
- 2026-09-28 — ✅ Guards: `TestAdminLibraryFiltersByID`, `TestAdminLibraryRefusesAnUnusableIDList` (both
  ends of the boundary), three SQL-level tests, and the updated JS guard including the chunk-size check.
  Mutation-checked by short-circuiting the wait and by raising the chunk above the endpoint's limit.
- 2026-09-28 — `gofmt`, `go vet`, `go test ./cmd/api/ ./nathejk/table/photo/ -count=1` clean.

## Worth knowing

**`ids=` is a presence read and should stay one.** It is tempting as a general "fetch these photographs"
call — the viewer or a future reconcile could use it — and that is fine, but the reason it is bounded at
200 and refuses rather than clamps is that its current caller treats an absent id as *"not yet"*. Any new
caller that treats an absent id as *"does not exist"* is drawing a different conclusion from the same
answer, and the two readings only agree once the projection has settled.

The note in task 437 still stands: every other write in the tool reloads the sheet in the same tick as
its response and has the same race. With the mechanism now being "wait for these ids", the shape of the
shared helper is clearer than it was — `ctx.settled(ids)` on the context, with this as its first caller.
