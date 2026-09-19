# 340 — Merged track reader

**Status:** open
**Priority:** high
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 011 §6, §8. Read a patrol's position tracks from the `TELEMETRY` stream and produce **one** track for
the patrol. Nothing reads this stream today (task 081 publishes to it; it has had no reader at all).

Subject filter: `TELEMETRY.<year>.track.<personId>.reported`, per member of the patrol.

**Merged and unattributed is a requirement, not a presentation choice.** The public surface's whole
privacy claim is that it names no person (PRD 011 §0b.1). An attributed track breaks it, and would also
disclose which member declined location (visibly absent from a track the others are on) and which member
was elsewhere (visibly apart). Neither was ever agreed to.

**Merge as a union of segments, not an interleaving of points.** Sorting every member's points into one
timestamp-ordered polyline draws a line that jumps between members walking ten metres apart — a zig-zag
that is pure artefact. Simplify and break each source track independently, then emit one *multi-segment*
polyline in one colour.

**Dedup on `(person, timestamp)`.** This is the contract task 083 established: a retry after a timeout can
legitimately publish the same point twice, and the reader is the only place it can be removed.

**Gaps are rendered as gaps.** Break a segment on a time delta above a stated multiple of the sampling
interval. Joining two points either side of a two-hour hole draws a confident line through terrain nobody
walked. Per task 082 the gaps match backgroundings almost to the second — they are information, and
smoothing them away is the one presentation choice this feature must not make.

**A deliberate departure from "reads come from projections"** (PRD 008 §8), and the reasoning must travel
with the code: projecting tracks would mean millions of points in MariaDB (827 participants × ~1,440
points) to serve a page opened once per patrol. The exception holds because the read is **bulk, cold and
non-critical**.

**But this is a public URL**, openable repeatedly by strangers — so the merged, simplified result must be
**cached or materialised per patrol**, and the cache, not the stream, is what the page reads. Do not
generalise the exception to anything else.

Scale reference: one patrol of six at 30 s sampling is roughly 8,600 points across ~2,160 stream messages.

## Acceptance Criteria

- [ ] Reads `TELEMETRY` by subject filter for a patrol's members; no projection of raw points.
- [ ] Dedup on `(person, timestamp)`, tested with a duplicate-publish case.
- [ ] Union-of-segments merge; a test asserts no segment ever joins two different people's points.
- [ ] Output carries **no person identifier** at all — not even an opaque one that could be counted to
      infer how many members recorded.
- [ ] Segments break on a stated time-delta threshold; the threshold is named and justified in a comment.
- [ ] Simplification applied, and the layer doing it recorded (server-side here, or client-side in 342 —
      state which).
- [ ] Result cached or materialised per patrol; the page reads the cache, not the stream.
- [ ] A patrol with no tracks yields an empty result, not an error — per task 082 this is the *common*
      case.
- [ ] A comment records why this read bypasses the projection rule and that the exception must not be
      generalised.
- [ ] Verified against 2025 data for a patrol with several members and at least one resume-after-kill.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §8 / §10 (Phase 2).
