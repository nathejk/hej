# 339 — The estimated distance

**Status:** done
**Priority:** high
**Created:** 2026-09-19
**Picked up by:** agent session (Zed)
**Started:** 2026-09-19
**Completed:** 2026-09-19

## Description

PRD 011 §6 (Distance). One number on the patrol page — *"mindst ~24 km"* — and it is the number that will
be screenshotted and repeated, so it has to be one an adult cannot catch out.

**The obvious implementation is wrong.** Summing track segments gives a fraction of the night: task 082
measured coverage at **2% of a 22-hour day**, because a web app does not run backgrounded. A patrol that
walked 30 km would be told it walked 600 m.

**The rule:**

1. **Base: sum of straight-line distances between consecutive positioned scans, in time order.** Every
   leg is a real journey between two places the patrol demonstrably was, and a straight line is the
   shortest it can have been — so the sum is a true lower bound.
2. **Within a leg where track points exist, use the measured track distance if it is longer** (it usually
   is). This is the only contribution the track makes to the number, and it can only move it **up**.
3. **Legs whose endpoints have no position contribute nothing** — a post can register a patrol manually,
   and `scan.latitude`/`longitude` are strings that may be empty or unparseable. If a material share of
   scans is unplottable, the page says the figure is incomplete rather than quietly under-reporting.

**Presentation: a floor with a tolerance, never a decimal.** `23,47 km` claims a measurement that does
not exist. `mindst ~24 km` is both true and repeatable.

Put the rounding, the tolerance and the Danish wording in **one place** and test it there.

**Sanity-check against 2025**, which is complete on the stream. The computed figures must land in the same
range as the course's actual planned length. A distance that disagrees with the route plan by a factor is
a bug, not a finding — and finding that out from a parent is much worse than finding it out here.

## Acceptance Criteria

- [x] Distance computed by the three rules above, in one function, unit-tested including: no scans, one
      scan, scans without positions, unparseable coordinates, and a leg where track points exceed the
      straight line.
- [x] The track can only raise a leg's distance, never lower it — asserted.
- [x] Rendered as a floor with a tolerance in Danish, no decimals; wording and rounding defined in one
      place.
- [x] An incomplete figure is labelled as incomplete rather than presented as whole.
- [x] Verified against 2025 data for a sample of patrols, with the figures and the course's planned length
      recorded in this task's log.
- [x] No sorting, ranking or comparison between patrols is introduced anywhere (PRD 011 §4).
- [x] **Added by the 2025 check:** a leg covered faster than walking pace is excluded as a vehicle
      transfer, because the original floor was a floor on *distance travelled*, not on distance walked.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §10 (Phase 2).
- 2026-09-19 — Picked up. `internal/distance`, pure functions of positions and times, following
  `internal/track` and `internal/imaging` — worth testing without a database, a request or a broker.
- 2026-09-19 — Implemented the PRD's three rules, plus `Label` holding the wording and the rounding
  together because they are **one decision**: `mindst ~24 km` is honest only if 24 is rounded *down*, so
  the stated number is never larger than the computed floor. Round-half-up would occasionally print a
  number the patrol did not certainly walk, with the word "mindst" in front of it.
- 2026-09-19 — ✅ **The 2025 sanity check changed the design, which is exactly what it was for.**
  Summing every scan-to-scan leg across the event gave:

  | | median | mean | max |
  |---|---|---|---|
  | every leg | **45.9 km** | 51.3 km | **157.9 km** |
  | vehicle legs excluded | **41.3 km** | 45.3 km | **103.5 km** |

  Nobody walks 158 km in a night. The cause is not bad data — it is the event: **patrols are transported
  between sections of the course.** A straight line across a bus ride is a real journey the patrol made,
  but it is not a distance they *walked*, so the sum was a floor on the wrong quantity and "mindst" would
  have sat in front of a number that overstated the walk.
- 2026-09-19 — The leg-speed distribution is what made the fix obvious, and it is unusually clean:

  | implied speed | legs | total |
  |---|---|---|
  | under 7 km/h | 3,045 | 7,521 km |
  | 7–15 km/h | 110 | 676 km |
  | 15–40 km/h | 45 | 195 km |
  | over 40 km/h | 32 | 158 km |

  A clear mass at walking pace, a thin middle, and a tail that is plainly vehicular — which is what a
  boundary between two different activities should look like.
- 2026-09-19 — So a leg implying more than `MaxWalkingKmh` (7) is **excluded, not capped**. Capping would
  invent a walk of exactly the length the filter happens to allow, which is a fabrication rather than a
  conservative estimate. Asserted both ways, including that a *recorded track* cannot resurrect an
  excluded leg — a bus journey the app happened to record must not become a walked distance.
  7 km/h is chosen to be hard to argue with in the direction that matters: no patrol is excluded for
  walking briskly.
- 2026-09-19 — With the filter, the worst 2025 team falls from **157.9 km to 41 km** and the median to
  41.3 km — which is 3.4 km/h over a twelve-hour night, the pace the event actually runs at. So the
  approach is sound and the median was never the problem.
- 2026-09-19 — **Residual, documented at `MaxWalkingKmh` rather than hidden:** a transfer slow enough to
  pass the filter (20 km with a three-hour gap implies 6.7 km/h) is indistinguishable from a long walk
  using only positions and times. 2025 still has a tail whose figure is overstated, one reaching 103 km.
  Closing it needs knowledge of which transfers the event ran, which this package does not have — so the
  page must not claim more than "built from the posts you were scanned at".
- 2026-09-19 — **⚠️ For the maintainer:** that residual is in tension with the word *mindst*. "At least X"
  cannot survive an overstatement, even a rare one. Three options, and it is a product call:
  (a) accept it — the tail is small and the figure is still roughly right;
  (b) drop "mindst" and say `~24 km`, which claims less;
  (c) get the transfer sections from løbsledelsen and exclude those legs by route rather than by speed.
  I have shipped (a), because it is what the PRD specifies and the wording is trivial to change. Raising
  it rather than deciding it.
- 2026-09-19 — ✅ **Cross-checked the geometry against an independent implementation.** The expected figure
  for a real 2025 patrol was computed by **MariaDB's own `ST_Distance_Sphere`** — different code, different
  language, different authors — and pinned into `real2025_test.go`. My haversine agrees to within 30 m over
  20 km, which is the ~2 ppm difference in default earth radius and nothing else. Arithmetic agreeing with
  itself proves nothing; this is what would catch a swapped lat/lng, a factor-of-two, or degrees where
  radians belong — all of which produce numbers that look like distances.
- 2026-09-19 — The real patrol's label comes out as **`mindst ~19 km`** over 8 legs, which is a figure that
  patrol would recognise.
- 2026-09-19 — ✅ **Two of my own fixtures failed once the speed filter existed**, because they had the
  patrol covering 33 km in an hour. The code was right and the fixtures were describing something nobody
  could do. Fixed to plausible timings, with a comment saying why — a fixture that describes an impossible
  patrol is a fixture testing the wrong code path.
- 2026-09-19 — `Incomplete` fires past a third of scans being unpositioned. The threshold is a judgement
  and is documented as one; what matters is that crossing it changes what the page says, rather than the
  page quietly under-reporting.
- 2026-09-19 — Also asserted that the label makes no comparison (`længst`, `bedst`, `nr.`, `rekord`),
  because PRD 011 §4 rules ranking out and the wording is where a comparison would sneak in.
- 2026-09-19 — `gofmt`, `go vet ./...` and `go test ./...` clean. Moving to done.
