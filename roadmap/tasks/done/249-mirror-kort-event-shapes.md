# 249 — Mirror the `kort` / `kortsaet` event shapes locally

**Status:** done
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

## Description

PRD 016 phase 1. This repo consumes hq's `kort` (map sheet) and `kortsaet` (map set)
events off JetStream, but their Go types live in the hq repo and are **not** being lifted
to `shared-go` — they have not stabilised, and a shape lifted with one consumer gets
lifted wrong.

So we decode the JSON ourselves against local mirror types. **Nothing may import hq**
(PRD 016 §8): no Go import, no shared module, no path to another checkout. The contract is
vendored at `roadmap/api/kort-events.md`.

Types needed (see the vendored contract's "Payloads" section):

- `Created`, `Updated`, `Deleted`, `Sorted` for sheets
- `SetCreated`, `SetUpdated`, `SetDeleted`, `SetsSorted` for sets
- `Format` (`a4` | `a3` | `skitse` | `andet`), `Extent` (northWest/southEast pair)

Three semantics the types themselves must encode, because getting them wrong is silent:

1. A sheet's `Updated` is a **patch** — every field a pointer, absent means unchanged.
   `CheckpointIDs` and `Extents` are pointers *to slices*, so "now empty" is expressible
   and distinct from "untouched".
2. `HandoutCheckgroupID` must carry an explicit `""` — that is a real value meaning "the
   QR rule", not an absence.
3. A set's `SetCreated`/`SetUpdated` is a **whole record** — an absent `TeamType` means
   the set has none, *not* unchanged.

Decoding must tolerate unknown fields (an additive upstream field must not break a
consumer mid-event) but should log a body it cannot make sense of — that is our signal the
vendored copy has gone stale (PRD 016 §11.11).

## Acceptance Criteria

- [x] Mirror types in `go/nathejk/table/kort/messages.go` + `kortsaet_messages.go`.
- [x] File comment names `roadmap/api/kort-events.md` as the shape's source of truth and
      explains why the types are local rather than imported.
- [x] Patch vs whole-record distinction encoded in the types and documented.
- [x] `HandoutCheckgroupID` round-trips an explicit `""`.
- [x] No import of hq anywhere; `go build ./...` passes.
- [x] Decode tests over the exact JSON bodies in the vendored contract, incl. an unknown
      extra field being ignored.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 1.
- 2026-09-15 — Picked up. Plan: `go/nathejk/table/kort/` with `messages.go` and
  `kortsaet_messages.go` holding mirror types, plus decode tests over the exact JSON bodies
  in `roadmap/api/kort-events.md`. Types only in this task; the projection is task 250.
- 2026-09-15 — `messages.go` written: `KortID`/`KortsaetID`, `Format` (incl. why `skitse` is
  the awkward one), `Extent`, `Created`, `Updated`, `Deleted`, `Sorted`. Package doc carries
  the no-import-hq rule, the vendored-contract pointer, and why tolerant decoding is a
  requirement rather than laziness.
- 2026-09-15 — Decision: dropped two speculative helpers I had drafted (an
  `UnknownFieldsAreIgnored` const and a `time.Time` alias). Neither had a caller; the
  reasoning belongs in the package doc, not in fake code that exists to be commented.
- 2026-09-15 — `kortsaet_messages.go` written. Added `PatrolTeamType = types.TeamTypePatrulje`
  as a named constant so the "spejder" mistake has exactly one place it could be made, with
  the filter-not-a-key consequences documented on it.
- 2026-09-15 — ✅ All criteria complete. `go build ./...`, `go vet` and `gofmt -l` clean;
  `go test ./nathejk/table/kort/` passes (13 tests). Grepped the package for any hq path: none.
- 2026-09-15 — Test coverage note: the patch-vs-snapshot distinction has three separate tests
  (absent stays nil, explicit empty array is an edit, explicit `""` is a value) because each is
  a distinct silent failure. Also pinned `PatrolTeamType != "spejder"` in a test — a filter on
  a value HQ refuses to store would reveal nothing while looking correct.
- 2026-09-15 — Done. Moving to done/; task 250 (the projection) is unblocked.
