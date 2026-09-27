# 433 — Backfill the 800 px rendition onto existing photographs

**Status:** done
**Priority:** medium
**Created:** 2026-09-26
**Picked up by:** agent
**Started:** 2026-09-26
**Completed:** 2026-09-26

## Description

Task 409 added the 800 px rendition, and task 410 taught the viewer to ask for it — but only for
photographs uploaded **after** it shipped. Everything already in the library has `mediumRef = ""` and
serves the 1600 px display image instead, which is correct (PRD 023 §7.9's fallback) and misses the
point of the feature: the phone still downloads four times the pixels it can show.

Task 430's repair path cannot help, and the distinction is worth stating because the names sound
interchangeable. **Repair** fills a rendition the projection already *names*: the row holds a
`mediumRef` whose bytes have gone, so a serve that misses rebuilds them under that same ref.
**Backfill** is the other half: these rows hold no `mediumRef` at all, so there is nothing to
rebuild, and producing one has to change a row — which needs an event.

Answers PRD 023 §11 Q9.

## What was built

`POST /api/admin/photos/renditions`, behind the admin credential and `requireAdminYear`.

**Synchronous and batched**, 25 per call by default and 200 at most. A background job would need
somewhere to report to, and the thing it would report is already this response; batching makes it
resumable with no state at all — run it, read the counts, run it again until `examined` is 0. No
queue, no progress table, no job that fails silently at 03:00.

**Not a boot-time pass.** That would decode and re-encode the whole library on every deploy, inside
the window before the HTTP server starts listening — precisely the shape of task 431, where
housekeeping got in front of serving.

**`examined` is the documented loop condition, not `failed`.** A photograph whose source bytes are
gone can never gain a rendition, so a library with one unreadable photograph settles at
"examined 1, produced 0, failed 1" on every run. An operator waiting for `failed == 0` would loop for
ever.

**The rendition is stored with `PutCache`**, so it is derived, outside the backup scope (task 429) and
rebuildable on a miss (task 430). This is what makes the backfill cheap to justify: it adds nothing a
backup has to hold.

### A new event rather than a republished `Uploaded`

`photo.MediumAdded{PhotoID, Year, MediumRef, AddedAt}`, folded by an **UPDATE** that writes one
column.

Republishing `Uploaded` was the obvious move — it is exactly what re-dragging a folder does, the fold
upserts, and it leaves `caption`, `credit` and `deleted` alone. Rejected for two reasons:

1. **Blast radius.** `Uploaded`'s fold also writes `width`, `height`, `bytes`, the coordinate, the
   verdict and `uploadedAt`, so a backfill would have to echo six existing values back faithfully.
   Getting any one wrong silently corrupts a row, and a dropped coordinate takes a photograph off the
   public map with nothing to notice it. This event carries one field, so there is nothing to echo.
2. **The log should record why.** A second `uploaded` for a photograph uploaded weeks earlier says
   something untrue about what happened.

Also deliberately *not* a general `RenditionAdded{name, ref}`: a generic pair would need the fold to
map a name onto a column, which is a lookup table that has to agree with the schema. The next
rendition is a schema change anyway, so it can widen this deliberately.

## The bug this hit, and the guard that came out of it

The first real run reported `produced: 4` and the count of photographs needing a rendition **did not
move**. No error, no dead letter, no log line.

`MediumAdded` had an event type, a fold, a `Verb` constant, and `Subject` built the subject happily —
but the verb was missing from `consumer.Consumes()`, which is the **subscription filter**. The publish
succeeded, JetStream returned a PubAck, and the message was never delivered to anything. The rendition
was in the blob store and the event was on the log; only the column stayed empty.

`consumer.go` already warned about this in a comment: *"an unmatched subject is simply never delivered
to anything."*

There was even a test, `TestEverySubscribedVerbIsFolded` — and it **passed**. It checked a hardcoded
list of six verbs plus a count of six, so a verb added to the consts, the fold and `Subject` but not
to `Consumes()` is not mentioned by the list and does not move the count. Its one blind spot was the
bug it existed to prevent.

Replaced with `TestEveryVerbIsSubscribedAndDispatched`, which reads the verbs **out of the const
block** and checks both directions: every declared verb is subscribed *and* dispatched, and every
subscription corresponds to a declared verb. Removed rather than kept alongside, because a guard whose
green tick reads as coverage for the case it cannot see is worse than no guard.

## Acceptance Criteria

- [x] A photograph with no 800 px rendition gets one, at the right size, recorded on its row
- [x] The rendition is stored as **cache**, so the backup does not grow
- [x] Photographs that already have one are never examined, so re-running is free
- [x] A photograph whose source bytes are gone is counted, logged and skipped without stopping the pass
- [x] Bounded per call and resumable with no state
- [x] The fold writes one column and cannot touch a caption, a credit, a coordinate or `deleted`
- [x] Every verb is subscribed and dispatched, enforced from the const block

## Verified against the real dev database and volume

Not only in the suite, because the interesting failure was invisible to it:

| | |
|---|---|
| Photographs needing a rendition | 10 → **0** |
| Passes | `?limit=4` → `examined 4, produced 4`; then `examined 6, produced 6` |
| Second full pass | `examined 0, produced 0` — *"Alle billeder har en mellemstørrelse."* |
| `cache/` | 12 files, 902 KB |
| `original/` | 9.69 MB |
| **cache share of the volume** | **8.5%**, up from 1.7% before |
| A medium actually serves | `HTTP 200`, 105 KB (against ~920 KB for the display image) |

8.5% is the backup reduction now available on a portrait-heavy dev volume. On a real hand-in it will
be higher: PRD 022 §6 puts a photograph at ~3 MB of display rendition, against ~190 KB of medium and
~20 KB of thumbnail.

Mutation-checked: storing the rendition with `Put` instead of `PutCache` fails the class assertion;
adding `caption=""` to the fold fails the one-column guard; removing the verb from `Consumes()`
reproduces the silent delivery bug.

## Progress Log

- 2026-09-26 — Task created after the medium gap was demonstrated concretely: a re-upload of an
  existing photograph left `mediumRef` empty, because `storeAlbumImage` only runs for genuinely new
  uploads.
- 2026-09-26 — Scoped deliberately to the medium rendition and **not** to reclassifying already-migrated
  thumbnails out of `original/`. The measurement is the argument: a thumbnail is ~20 KB against the
  medium's ~190 KB, so moving thumbnails would buy roughly 2% while requiring a "is this ref an
  original anywhere?" check across every blob owner — the one question that, answered wrongly, drops a
  photograph out of the backup. Backfilled mediums land in `cache/` by construction, so the large half
  comes for free.
- 2026-09-26 — Chose a new event over a republished `Uploaded`, on blast radius: six echoed fields
  versus one.
- 2026-09-26 — Two existing structural guards caught the new handler before any test of mine did:
  `curatorboundary_test.go` required `adminbackfill.go` to be registered as admin-owned with a reason,
  and the OpenAPI walk required the 421 to be documented. Both were right.
- 2026-09-26 — Reordered the handler to validate `limit` **before** checking the library is available:
  a nonsense limit is wrong whatever the dependency's state, and a 503 would invite a retry that fails
  identically.
- 2026-09-26 — ✅ Ran it against the real dev database. `produced: 4`, count unchanged — found the
  missing `Consumes()` subscription. Fixed, and replaced the hardcoded guard that had passed.
- 2026-09-26 — ✅ Re-ran end to end: 10 → 0, second pass a no-op, `cache/` share 1.7% → 8.5%, and
  `?variant=medium` now serves 105 KB where it served 920 KB.
- 2026-09-26 — `gofmt`, `GOWORK=off go test -timeout 5m ./...` and `go tool staticcheck ./...` clean.

## Follow-up, deliberately not done

- **Reclassifying existing thumbnails** from `original/` into `cache/` — ~2% of the volume, and it
  needs a safe "this ref is not an original anywhere" check across every blob owner. Worth doing only
  if a production measurement says the share is materially larger there.
- **Running this on production.** The endpoint exists and is idempotent; somebody has to call it. Best
  done before the first real hand-in, while the library is small — the same argument PRD 023 §11 Q9
  makes.
