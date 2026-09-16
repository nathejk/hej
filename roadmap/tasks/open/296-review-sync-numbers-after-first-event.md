# 296 — Review the sync check's numbers after the first event

**Status:** open
**Priority:** medium
**Created:** 2026-09-16
**Picked up by:**
**Started:**
**Completed:**

## Description

Split out of task 293, which built the instrumentation. This is the half that needs an
event to have happened: reading the numbers and judging each dataset.

The BFF logs `sync check summary` and one `sync dataset summary` per dataset every five
minutes (`go/cmd/api/syncmetrics.go`). What to read, and how not to misread it:

**The aggregate.** `unchangedRatio` is the share of checks answered `304`, i.e. every
dataset that caller holds was current. PRD 017 §9 targets ≥ 95 %. Also read `meanMs` /
`maxMs`, which is the endpoint's own timing.

**Per dataset**, `churnRatio` is how often a witnessed version moved when it was
recomputed. Judge each of the six:

- **Plausible** — churn roughly matches how often that data really changes. Handouts and
  checkpoints should move during the race and be flat outside it; scans should move for
  patrols that are scanning; profile and contacts should be near zero except in the run-up;
  race area should be flat during an event entirely.
- **Too high** — a **wrongly-unstable** version: it changes when its data did not, so every
  device refetches that payload on every interval. `contacts.go` and `mapversion.go` both
  document the trap (nothing time-varying in a hash); a churn near 1 on a quiet dataset means
  something time-varying got in.
- **Suspiciously perfect** — a **wrongly-stable** version: zero churn across a whole event
  for data that demonstrably changed. This is the silent failure, and the instrumentation
  *cannot* prove it: it cannot tell "nothing changed" from "we failed to notice". Cross-check
  against whether the underlying data actually moved.

**Two ways to misread this, both easy:**

- **An early-race dip in the contacts ratio is expected**, not a fault. Photos are added
  heavily in the first hour, ~100–150 entries share one contacts version, and every new
  portrait moves it. PRD 017 §9 therefore measures the ≥ 95 % target *outside* the first
  hour. Judging the first hour by the steady-state number produces a wrong conclusion and,
  worse, a plausible one.
- **`churnRatio: -1` and `unchangedRatio: -1` mean unmeasured, not zero.** A dataset nobody
  held, or a server nobody called, reports -1 deliberately so it cannot be read as the
  alarming answer.

A non-zero `unavailable` on any dataset is a **server fault**, not an event state. It is
logged at error level per occurrence as well as counted.

## Acceptance Criteria

- [ ] Numbers from the first event recorded in PRD 017 §9, replacing the targets with
      measurements.
- [ ] Each of the six datasets explicitly judged: plausible, too high (wrongly-unstable), or
      suspiciously perfect (wrongly-stable).
- [ ] Contacts assessed with the first hour excluded, per PRD 017 §9.
- [ ] `unavailable` confirmed zero for the whole event, or the cause found.
- [ ] A recommendation on the served interval and debounce, given the numbers.
- [ ] A recommendation on whether photos need their own key (PRD 017 §11 says this is the
      first thing to revisit if the ratio disappoints).
- [ ] A note on whether the witness sample (32 keys per dataset) was sufficient to judge, or
      whether a wider sample is wanted next time.

## Progress Log

- 2026-09-16 19:00 — Task created, split out of task 293. The instrumentation shipped there;
  this is the reading of it, which needs an event and a human.
