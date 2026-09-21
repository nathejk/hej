# 349 — The distance, corrected: status instead of speed, and per-point outliers

**Status:** open
**Priority:** medium
**Created:** 2026-09-21
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 011 §0b.6 and §6. Task 339 shipped the estimated distance with a **7 km/h leg filter** described as
detecting vehicle transfers, and recorded a residual overstatement (one 2025 patrol reaching 103 km) as an
accepted cost. The maintainer has since corrected the premise this rested on:

> *"there are no such thing as a transfer section. if a user is no longer active, their track should not be
> taken into account, and then they might be transfered by car"*
>
> *"we are relying on a lot of different gps units, some more accurate than others, if we can identify an
> outlyer or something obvious wrong or just suspecious, then drop that single coordinate from
> calculation"*

**The signal is status, not speed.** A patrol in the race walks. A person who has left the race may be in a
car, and that is a fact we hold rather than something to infer from velocity. And the second source of
inflation is not transport at all: it is a fleet of mixed-quality GPS units, where one wild fix inflates the
two segments either side of it.

So two changes, and one demotion:

1. **Exclude points recorded after a member left the race.** `reunited` and `released` are the two endings
   that mean "no longer racing" (`stillInRace` in `cmd/api/contacts.go`, interim pending task 175).
   `finished` is **not** a withdrawal and must keep counting — a finisher walked the route.

   **The hard part is time, and it must not be skipped.** The `person` projection stores the *current*
   status with no record of when it changed, so "points after they left" is not answerable today. Excluding
   a withdrawn member's points *entirely* would discard the kilometres they genuinely walked before
   withdrawing, which is a real loss on the page of a patrol that had a rough night. Project the
   transition's timestamp (the status-changed event carries one) and exclude from that instant onward.

   Where the timestamp is unknown — an older event, a replay gap — exclude the member's points **entirely**
   rather than keeping them. That errs toward under-reporting, which is the direction *mindst* is allowed to
   be wrong in.

2. **Drop a suspicious coordinate, not the leg around it.** The unit of rejection is **one point**.
   `trackpoint.Point` already carries `Accuracy` (reported radius in metres, 0 = unknown) and
   `patroltrack.Point` currently drops it — that field is the most direct "obviously wrong" signal available
   and should reach the filter. Beyond accuracy, the classic phone failure is a **spike**: one fix far from
   its neighbours, where the track returns to where it was. A point whose implied speed *both in and out* is
   implausible is a spike; a point that is fast in and normal out is the patrol being driven, which rule 1
   now handles.

   **Thresholds are to be measured, not guessed** (see the pitfalls in 339's and 340's logs: several
   fixtures were wrong rather than the code). Measure on 2025 (scans only, no telemetry) and on 2026 dev
   telemetry, and record the distribution before choosing a number.

3. **The 7 km/h leg filter stays, demoted to a backstop.** It is no longer the primary mechanism and must
   stop describing itself as transfer detection — its comment is now wrong about *why* it works. Do not
   remove it in the same change: it is the only thing currently holding the 2025 maximum down, and removing
   it before the two rules above are measured would regress the figure while claiming to fix it.

**The label keeps the word *mindst*.** §0b.6 settles it: once a car is excluded by status and bad fixes are
dropped individually, the dominant remaining error is *missing* data, which is what "at least" describes
honestly. Do not change the wording in this task.

## Acceptance Criteria

- [ ] Points recorded **after** a member left the race (`reunited`, `released`) do not contribute. `finished`
      still contributes.
- [ ] The status **transition time** is available to the calculation, not just the current status; where it
      is unknown the member's points are excluded entirely and a comment says why that direction was chosen.
- [ ] `Accuracy` survives from `trackpoint.Point` into the merge and reaches the outlier filter.
- [ ] A single implausible coordinate is dropped **without** dropping the legs either side of it; a test
      demonstrates a spike costing one point rather than two segments.
- [ ] A point that is fast *in* and normal *out* is left to rule 1 rather than treated as a spike — asserted,
      because conflating the two is how a car becomes a GPS error.
- [ ] Thresholds are measured on real data and the distribution recorded in this task's log **before** the
      numbers are fixed in code.
- [ ] The 2025 figures are re-measured after the change: median, maximum, and what happened to the 103 km
      patrol from 339's log.
- [ ] The 7 km/h filter's comment no longer claims to detect transfers, and says what it is now for.
- [ ] The label still reads *"mindst ~N km"*; wording unchanged.
- [ ] Every rule lives in `internal/distance` (or the merge), not in a handler, and the reasoning for each
      threshold is at its definition.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-21 — Task created from PRD 011 §0b.6, the maintainer's correction of task 339's transfer premise.
  Depends on 339 and 340; related to 175 (the `stillInRace` predicate this needs belongs in shared-go).
