# 339 — The estimated distance

**Status:** open
**Priority:** high
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] Distance computed by the three rules above, in one function, unit-tested including: no scans, one
      scan, scans without positions, unparseable coordinates, and a leg where track points exceed the
      straight line.
- [ ] The track can only raise a leg's distance, never lower it — asserted.
- [ ] Rendered as a floor with a tolerance in Danish, no decimals; wording and rounding defined in one
      place.
- [ ] An incomplete figure is labelled as incomplete rather than presented as whole.
- [ ] Verified against 2025 data for a sample of patrols, with the figures and the course's planned length
      recorded in this task's log.
- [ ] No sorting, ranking or comparison between patrols is introduced anywhere (PRD 011 §4).

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §10 (Phase 2).
