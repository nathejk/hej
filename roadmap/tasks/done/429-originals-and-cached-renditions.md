# 429 — Originals and cached renditions are distinguishable on the volume

**Status:** done
**Priority:** high
**Created:** 2026-09-26
**Picked up by:** agent
**Started:** 2026-09-26
**Completed:** 2026-09-26

## Description

The blob store held everything in one flat, content-addressed tree: `<root>/<ab>/<ref>`. A photograph
and a thumbnail of that photograph were indistinguishable — both just bytes under a hash.

That was fine while the store held portraits, where PRD 008 §8's "the blob store is the whole backup
scope" was both true and cheap. It stopped being the useful statement once the store grew glimt media
and the photograph library, because most of what is now in there is **derived**: a resize produced at
upload by `internal/imaging` from bytes the store already holds.

The two classes have completely different operational requirements, and no way to act on the
difference:

- An **original** is the only copy of some pixels. Losing it loses the photograph. Must be backed up.
- A **cached rendition** can be produced again. Losing it costs bandwidth and a regeneration.

This matters now rather than later because of PRD 022's arithmetic. §6 puts one hand-in at order 1 GB,
§11 Q2 resolved that those photographs are **never purged**, so the store grows monotonically, one
event per year, forever. Task 384's outstanding criterion is a backup target confirmed to hold that
trajectory *with a restore spot-checked* — and every derived byte in the backup makes that target
larger and that restore slower for no recoverable value.

## Decisions

**The distinction is on-disk layout, not a database column.** This is the whole design, so it is worth
stating why. The consumer of the distinction is not this service — it is an operator, or a cron line,
pointing `restic`/`rsync` at a directory. That consumer has no database connection and should not need
one. Anything requiring a query, a manifest or a filename pattern is something a backup script can get
wrong, or can silently stop doing when the schema moves. An exclude that is one directory stays
correct.

```
<root>/original/<ab>/<ref>   must be backed up
<root>/cache/<ab>/<ref>      need not be; may be emptied to reclaim space
```

**Original is the default; cache is opt-in.** `Put` means original, `PutCache` means derived. The two
mistakes are not symmetric: classifying a rendition as an original wastes disk an operator can see,
while classifying a sole copy as cache drops it from the backup silently — and that is discovered
during a restore, which is the worst possible moment to discover anything. So the safe direction is
what you get by not thinking about it.

**Reads stay class-agnostic.** `Get`/`Exists`/`Delete` still take a bare `Ref` and search both
subtrees. A ref travels through projections and URLs and says nothing about how the bytes were made;
teaching every projection a second column to save one `stat` against a local filesystem would be a bad
trade. Only the writer, which knows where the bytes came from, picks a class.

**One store, not two configured stores.** The classes must share one dedup domain and one `statfs` —
the disk floor from task 384 measures *the volume*, and identical bytes must not be written twice
because they arrived through a different call.

**Dedup spans classes, but asymmetrically.** `PutCache` finding an existing original is ideal: present
*and* backed up, so nothing is written. `Put` finding only a cache copy **promotes** — it writes the
original anyway. Otherwise the sole copy of a photograph would sit in the subtree a backup skips. The
case is narrow (a full rendition and its thumbnail do not hash alike) but reachable with an image small
enough that resizing is a no-op, and it fails silently, which is the combination worth code.

**Delete removes both copies.** The purge paths have already established nothing references the ref; a
copy left in the other subtree would be a deleted photograph still on disk, and still in the backup.

**The migration treats all pre-existing objects as originals.** Which class the old flat objects belong
to is genuinely unrecoverable. Over-counting the backup by the thumbnails already on the volume costs
disk; guessing the other way drops real photographs out of the backup scope. It runs on boot, is a
no-op once the top level is empty, moves a bucket with one rename where it can, and merges file-by-file
when an interrupted run left a bucket in both places.

## Classification, as applied

The rule: **cache iff the service could rebuild these bytes from something it still holds.** Not "is it
small", and not "is it a thumbnail".

| Call site | Class | Why |
|---|---|---|
| `portrait.go` full image | original | served, and the only copy when `PORTRAIT_KEEP_ORIGINAL` is off |
| `portrait.go` thumbnails | **cache** | resize of the above; serving falls back to the full image when a rendition is absent |
| `portrait.go` original (task 111) | original | the point of it is to be the thing renditions are rebuilt from |
| `glimtmedia.go` full rendition | original | glimt keep **no** original, so these bytes are the member's only copy — downscaled to `maxGlimtEdge` on the way in, and still an original |
| `glimtmedia.go` thumbnail | **cache** | the client already falls back to the full item (task 302) |
| `albummedia.go` full rendition | original | an admin upload keeps no separate original (PRD 022 §8.5), never purged (§11 Q2) |
| `albummedia.go` thumbnail | **cache** | the album page falls back to the full image |
| `photobytes.go` (diploma) | original | left on `Put` deliberately: these bytes become the library's canonical ref for an album item, so they are a photograph, not a cache of one |

Every surface that stores a cached rendition already degrades to serving the full image when the
rendition is missing. That is not a coincidence, it is the precondition that makes excluding the cache
from a backup safe — a restore without it is soft grid tiles, not broken pages.

## Acceptance Criteria

- [x] Originals and derived renditions live in separate, documented subtrees of the blob root
- [x] The backup scope is a single path, exported in code (`FileStore.OriginalDir`) rather than only in prose
- [x] `Put` defaults to original; deriving is an explicit `PutCache`
- [x] Readers and purges need no knowledge of the class
- [x] Objects written under the old flat layout migrate into `original/`, idempotently
- [x] Cached objects keep the same 0600/0700 privacy as originals
- [x] Each call site classified, with the reason recorded at the call site
- [x] Tests assert the on-disk outcome, not just a return value
- [x] A restore that omits `cache/` provably loses no original

## Progress Log

- 2026-09-26 — Task created, after the observation that the store cannot tell a photograph from a
  thumbnail of it and therefore the backup cannot either.
- 2026-09-26 — Considered and rejected recording the class in the projections that hold the refs
  (`photo`, `album_item`, `person`). It would have needed no storage change, but it puts the answer
  somewhere a backup command cannot see it, which is the only place the answer is needed. Chose
  subtrees.
- 2026-09-26 — Considered and rejected two configured stores (a durable root and a cache root). Cleaner
  interface, but it splits the dedup domain and the `statfs` that task 384's disk floor depends on.
- 2026-09-26 — `blob` package: documented the two classes, added `PutCache` to `Store`, reworked
  `FileStore` onto the subtree layout with `locate`, class-spanning dedup with promotion, both-class
  delete, and the boot-time legacy migration. `MemoryStore` records the class so the *call sites* are
  testable.
- 2026-09-26 — Classified all call sites. Kept `internal/photobytes` on `Put`: its bytes become an
  album item's canonical ref, so they are an original despite arriving over HTTP from foto.
- 2026-09-26 — ✅ `internal/blob/class_test.go`: layout, class-agnostic reads, both-class delete, dedup
  in both directions, migration (including the partial-run merge, guarded so it cannot silently stop
  exercising the merge path), and privacy of cached objects. The load-bearing one is
  `TestExcludingTheCacheDirectoryLosesNoOriginal` — it deletes `cache/` and asserts the original reads
  back, which is a restore rehearsal rather than a claim about an interface.
- 2026-09-26 — ✅ `cmd/api/blobclass_test.go`: the classification decision itself, per call site, plus a
  source-level guard that every `PutCache` carries a nearby comment justifying it. Mutation-checked —
  flipping the album thumbnail back to `Put` fails the test with the right message, so it is not
  passing vacuously.
- 2026-09-26 — `go vet ./...` and `go test ./...` clean. Appended the operational consequence to task
  384, whose backup criterion this exists to make cheap. Opened task 430 for regeneration, which is
  the one thing this does **not** deliver: the layout says the cache is expendable, and nothing yet
  rebuilds it.
