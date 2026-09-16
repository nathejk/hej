# 293 — Instrument the unchanged/changed ratio and review after the first event

**Status:** open
**Priority:** low
**Created:** 2026-09-16
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 017 phase 2. The endpoint's whole economy rests on one assumption: almost every
check finds nothing. If it does not, some version is being derived wrongly — and PRD 017
§8 is explicit that an event is the wrong time to discover that.

Log, per `/api/sync` call: which datasets were unchanged, which changed, which were
`unavailable`, and the endpoint's own timing. Aggregate the unchanged ratio per dataset,
not just overall — an overall figure hides a single wrongly-unstable version behind five
well-behaved ones, which is exactly the failure this instrumentation exists to catch.

**Two things the review must not misread:**

- **An early-race dip in the contacts ratio is expected**, not a fault. Photos are added
  heavily in the first hour, ~100–150 entries share one contacts version, and every new
  portrait moves it. PRD 017 §9 therefore measures the ≥ 95 % target *outside* the first
  hour. Judging the first hour by the steady-state number would produce a wrong
  conclusion and, worse, a plausible one.
- **A dataset at exactly 100 % unchanged for a whole event is a suspect, not a
  success.** That is the signature of a wrongly-stable version — the silent failure where
  a device quietly never updates. Cross-check it against whether the underlying data
  actually changed.

A non-empty `unavailable` is a server fault and should be alertable, not merely logged.

## Acceptance Criteria

- [ ] Per-call structured log: unchanged / changed / unavailable per dataset, plus
      duration.
- [ ] Per-dataset unchanged ratio aggregated, not only an overall figure.
- [ ] `unavailable` surfaced as a fault rather than an ordinary log line.
- [ ] Reviewed after the first event, with the numbers written into PRD 017 §9.
- [ ] Each dataset explicitly judged: ratio plausible, too low (wrongly-unstable), or
      suspiciously perfect (wrongly-stable).
- [ ] Contacts assessed with the first hour excluded, per PRD 017 §9.
- [ ] A recommendation on the interval and on whether photos need their own key
      (PRD 017 §11 says this is the first thing to revisit if the ratio disappoints).

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 2.
