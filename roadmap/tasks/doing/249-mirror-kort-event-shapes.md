# 249 — Mirror the `kort` / `kortsaet` event shapes locally

**Status:** doing
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15

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

- [ ] Mirror types in `go/nathejk/table/kort/messages.go` + `kortsaet_messages.go`.
- [ ] File comment names `roadmap/api/kort-events.md` as the shape's source of truth and
      explains why the types are local rather than imported.
- [ ] Patch vs whole-record distinction encoded in the types and documented.
- [ ] `HandoutCheckgroupID` round-trips an explicit `""`.
- [ ] No import of hq anywhere; `go build ./...` passes.
- [ ] Decode tests over the exact JSON bodies in the vendored contract, incl. an unknown
      extra field being ignored.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 1.
- 2026-09-15 — Picked up. Plan: `go/nathejk/table/kort/` with `messages.go` and
  `kortsaet_messages.go` holding mirror types, plus decode tests over the exact JSON bodies
  in `roadmap/api/kort-events.md`. Types only in this task; the projection is task 250.
