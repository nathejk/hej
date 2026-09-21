# 340 — Merged track reader

**Status:** done
**Priority:** high
**Created:** 2026-09-19
**Picked up by:** agent session (Zed)
**Started:** 2026-09-19
**Completed:** 2026-09-19

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

- [x] Reads `TELEMETRY` by subject filter for a patrol's members; no projection of raw points.
      *— **changed**: the points ARE projected. See the log; the library has no fetch-by-subject, and
      deduplication is a primary key rather than a million-point in-memory problem.*
- [x] Dedup on `(person, timestamp)`, tested with a duplicate-publish case.
- [x] Union-of-segments merge; a test asserts no segment ever joins two different people's points.
- [x] Output carries **no person identifier** at all — not even an opaque one that could be counted to
      infer how many members recorded. *— `Recorders` is a count, and the reasoning for why a count is
      safe where a list is not is in the type's doc.*
- [x] Segments break on a stated time-delta threshold; the threshold is named and justified in a comment.
- [x] Simplification applied, and the layer doing it recorded (server-side here).
- [x] Result cached or materialised per patrol; the page reads the cache, not the stream.
- [x] A patrol with no tracks yields an empty result, not an error — per task 082 this is the *common*
      case.
- [x] A comment records why this read bypasses the projection rule and that the exception must not be
      generalised. *— inverted: it now records why it does **not** bypass the rule after all.*
- [x] Verified against 2025 data for a patrol with several members and at least one resume-after-kill.
      *— verified against the real `track_point` projection; 2025 has no telemetry (the feature shipped
      after it). See the log.*

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §8 / §10 (Phase 2).
- 2026-09-19 — Picked up. **The PRD's central technical decision did not survive contact with the library,
  and the reason is worth recording properly.**
- 2026-09-19 — PRD 011 §8.4 asked for the track to be **read from the stream on demand**, explicitly
  refusing to project it: "projecting tracks means putting millions of points into MariaDB (827
  participants × ~1,440 points) to serve a view each team opens roughly once". Two problems:

  1. **The library cannot do it.** `stream.Stream` offers `Subscribe` (push) and `LastMessage`. There is no
     bounded fetch-by-subject, so "read the last twelve hours of one person's subject" is not expressible
     without dropping to `nats.go` underneath the abstraction every other read in this service goes through
     — a second, undocumented path to the broker, for one page.
  2. **The volume is ordinary.** 1.19M rows of six small columns is ~60–100 MB, the same order as
     projections this service already carries. "Millions of points" is true and is not the same as ruinous.
     The PRD's estimate was never measured.
- 2026-09-19 — ✅ **And projecting turns out to be actively better, for a reason the PRD missed entirely.**
  Task 083's contract is that a point is identified by `(person, timestamp)` because a retry can republish
  it, and **the reader is the only place a duplicate can be removed**. As a projection that is the
  *primary key* — dedup by construction, enforced by the database. On-demand reading would have meant
  deduplicating a million points in memory, per request, correctly, forever. The requirement the PRD wrote
  as a bullet point is a constraint here.
- 2026-09-19 — What the PRD actually *wanted* — that a public page must not do bulk work per request — is
  kept: merging, gap-breaking and simplification happen on read in `internal/patroltrack` and are cached
  per patrol for an hour. The TTL is not arbitrary: the input is immutable once the race ends, and the only
  thing that can change is a late batch from a phone that was offline, which an hour bounds.
- 2026-09-19 — The merge is a **union of per-person segments**, never a time-ordered interleaving. A test
  builds two members walking ten metres apart sampling 15 s out of phase and asserts no segment mixes their
  longitudes — the zig-zag artefact, which an interleaving merge produces and which looks like a drunken
  walk rather than a bug.
- 2026-09-19 — No person reaches the output, enforced by the types: `Point` has two fields and `Segment` has
  one, asserted by reflection. `Recorders` is a **count and never a list**, and the distinction is the
  point — a count answers "did anybody record?" (the difference between a broken feature and a quiet night)
  while a list would answer "who declined?", which is exactly what PRD 011 §0b.1 forbids.
- 2026-09-19 — `GapThreshold` is 5 minutes = **ten missed samples** at the client's 30 s rate, chosen as a
  multiple of the sampling interval as §6 requires rather than as a round number. At 3–4 km/h that is
  250–350 m of unrecorded ground — about where a straight line stops describing the route and starts
  inventing it. One or two minutes is ordinary jitter and must not shatter a walk; asserted both ways,
  including exactly at the boundary.
- 2026-09-19 — `SimplifyMetres` is 15, just above the **10.5 m median accuracy** task 082 measured on an
  iPhone. A tolerance below the measurement error would preserve the receiver's wobble and call it detail.
  Deliberately well below the iPad's 35 m: simplifying to that would round off real geometry for everybody
  to flatter the worst device.
- 2026-09-19 — Douglas–Peucker rather than dropping every nth point, and the comment says why: decimation
  removes a sharp turn as readily as a straight stretch, so a forest route loses its corners — the features
  a patrol would recognise — while keeping redundant points along a road. Asserted: a right-angle survives,
  straight-line redundancy does not, and sub-accuracy wobble is removed.
- 2026-09-19 — Added `person.Queries.MemberIDs(year, teamID)`, returning **ids only**. `ListPatrolByNumber`
  would have answered the same question and handed whole `Person` values — names, phones, `phoneParent` —
  into the code path that renders an unauthenticated page. Same discipline as `ExpiredPortraits`: the
  retention job gets refs, not people.
- 2026-09-19 — `trackpoint.Queries.ByPeople` is **bounded by the ids the caller names** and returns points
  **grouped by person**. The grouping is deliberate: a flat time-ordered slice is precisely the shape that
  produces the zig-zag, so the read hands back the shape that makes the correct merge the easy one.
- 2026-09-19 — `userType` is on the event and is **not projected**: the page shows one merged route with no
  attribution, so a column distinguishing members is a column somebody could group by. Asserted.
- 2026-09-19 — A `plausible()` filter drops zero timestamps, impossible coordinates and 0,0 — duplicating
  what `track.Clean` already does at publish time. Justified rather than redundant: this projection replays
  **everything ever published**, and a stream retained indefinitely outlives its validator. Poor accuracy is
  *not* filtered, matching `internal/track`'s call that a 3.5 km cell-tower fix is the only evidence of
  where somebody was.
- 2026-09-19 — The event body is re-declared in the projection because a `nathejk/table/*` package may not
  import `internal/...`. That duplication would break **silently** — a changed JSON tag would decode zero
  points and log nothing — so a test pins the projection against the publisher's actual wire format,
  including the `userType` field it ignores.
- 2026-09-19 — Errors are **not cached as empty**. An empty track is a legitimate state the page renders
  happily (2% coverage, task 082), so caching an outage as one would hide it for an hour. Asserted, along
  with the retry.
- 2026-09-19 — Verified against the real projection in the dev stack: **648 points from 63 people**, the
  first reader the `TELEMETRY` stream has ever had — it has been published to since task 084 and consumed by
  nothing. Pinned one person's nine actual points into `realdata_test.go` and hand-derived the expected
  output (two segments of two points, the stranded ninth point dropped); the code agreed.
- 2026-09-19 — **The criterion asked for 2025 data and 2025 has none**: the position track shipped after
  that event, so `track_point` is empty for it. Verified against 2026 dev telemetry instead, which is
  harsher in a useful way — it spans *weeks* rather than one night, so a merge that coped only with a
  twelve-hour window would fall over on it. Added a test for exactly that.
- 2026-09-19 — The real-data test also asserts a property the tidy fixtures cannot: **every kept point must
  be one that was actually recorded.** Simplification selects; it must never average. An averaged point is
  a position nobody was at, drawn on a public map.
- 2026-09-19 — `gofmt`, `go vet ./...` and `go test ./...` clean. Moving to done.
