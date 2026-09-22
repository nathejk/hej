# 363 — The `photo` projection: the year's photo library, its events and its fold

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent session (Zed)
**Started:** 2026-09-22
**Completed:** 2026-09-22

## Description

PRD 022 §8.3 makes photographs first-class: a **year-scoped library** that albums reference, rather
than rows that only exist inside an album. This task builds the bottom of that — a new projection
package `go/nathejk/table/photo/` with `table.sql`, `events.go`, `consumer.go`, `querier.go` and
`table.go`, modelled directly on `go/nathejk/table/album/`.

`photo` owns everything that is a fact about the photograph: `blobRef`, `thumbRef`, `width`,
`height`, `bytes`, `caption`, `latitude`, `longitude`, `boundsVerdict`, `uploadedAt`, `deleted`. The
events go on `NATHEJK.<year>.photo.<photoId>.*` per PRD 022 §8.7 — `uploaded`, `updated`,
`locationcleared`, `deleted` — with the subject validating its tokens the way `album.Subject` does,
one event per fact, refs never bytes, and **pointer fields on `updated`** so "not mentioned" and "set
to empty" are different messages (the reason `album.Updated` gives).

The reason a photograph gets its own row rather than staying an album item is in PRD 022 §8.3 and it
is not tidiness: the brief requires a photograph that is in **no** album (the bulk), and a photograph
in **two** albums. Under today's model the second case is two rows with two coordinates, two captions
and two verdicts, free to disagree — a curator who corrects a location in one album and not the other
has made a bug they cannot see. The rejected alternative was keeping the media columns on
`album_item` and adding a nullable "orphan album" for the bulk, which buys nothing and keeps the
divergence.

The fold must follow `album`'s `created` rule exactly: the `uploaded` upsert **does not touch
`deleted`**. PRD 022 §8.5 — a re-upload must not resurrect a photograph a curator removed, because
that removal may have honoured somebody's objection. The photo id is the content ref
(`blob.ComputeRef` of the stored full rendition), so the upsert is how idempotency is achieved at all.

No person-shaped field goes in this table: no uploader, no curator, no name, no phone, and
emphatically no `phoneParent` (PRD 022 §6 Non-Functional, `.rules`). The credential is shared so the
projection *cannot* honestly name who wrote a row (§8.2) and must not pretend to.

## What landed

The package, its schema, all six event shapes, the fold, a minimal querier, and the app wiring.

Three deviations from the task as written, each deliberate:

**The tag verbs came too.** `patroltagged` / `patroluntagged` and the `photo_patrol` table were
scheduled for task 367, and the fold could not honestly be written without them. The album consumer
established the rule this follows: subscribe to every verb the entity will ever publish, *including*
the ones whose handlers arrive later, because "a projection that ignores an event it was not yet
taught about would silently keep showing a photograph somebody took down". Subscribing to a verb whose
table does not exist would fail at the first message, so the schema had to land with the subscription.
367 keeps its reads, its renumbering test and its no-public-read assertion.

**A `Location` struct instead of three optional fields.** `Lat`, `Lng` and `BoundsVerdict` travel as
one value on both `uploaded` and `updated`. The reason is the bug the obvious shape allows: a message
that moved a coordinate and forgot to restate the verdict would leave a photograph plotted at its *old*
judgement — a point moved out of the race area still on the public map, because the field that governs
plotting was not mentioned. A single value makes that unsayable. It also made two rules expressible in
one place (`locationColumns`): an unrecognised verdict becomes `unknown` and never `inside`, and a
verdict of `none` beside a real coordinate becomes `unknown` rather than silently plotting.

**`caption` is excluded from the upsert clause as well as `deleted`.** The task only named `deleted`.
But the upload event carries no caption — there is nothing to caption a file with at upload time — so
leaving the column in the clause would write an empty string over an evening's editing every time a
photographer re-dragged a folder. Same class of bug as the `deleted` one, same fix.

The verdict constants are duplicated from `album` rather than shared. `photo` is now their canonical
home, because after PRD 022 §8.3 a verdict is a fact about a photograph; `album`'s copies are legacy,
kept only so its fold can still read them off events published before the library existed (task 364).
A shared constant would outlive that need and leave a permanent dependency between two projections to
express agreement that is only temporarily required.

## Acceptance Criteria

- [x] `nathejk/table/photo/` exists with the five files, and `table.sql` carries a header explaining
      the library, the coordinate column and the blob-sharing hazard as `album/table.sql` does
- [x] Events `uploaded`, `updated`, `locationcleared`, `deleted` are defined, subject-validated, and
      carry refs rather than bytes; `updated` uses pointers for every optional field
- [x] The `uploaded` fold upserts and **never clears `deleted`** — asserted against the generated
      update clause, which is where the rule actually lives
- [x] Replaying the whole stream twice converges on identical rows — every write is an upsert or a
      flag set; no statement appends
- [x] Indexes support the reads that follow: `year_uploaded`, `year_plottable`, `ref_lookup`,
      `thumb_lookup`
- [x] No column, struct field or SQL statement in the package can carry an uploader, a curator or any
      person-shaped value
- [x] `consumer_test.go` covers a malformed event being dropped rather than erroring the replay — an
      unknown verb is ignored, a missing year is refused, an invalid ref is refused, a bad thumbnail is
      blanked rather than fatal
- [x] Wired into `data.Models.Photos` and `main.go`'s projection list, behind the typed-nil guard
      `photoQueriesOrNil` — because `RefsInUse` is called from the takedown paths, where a panic is
      worst
- [x] `go build ./...`, `go vet ./...` and the full `go test ./...` pass
