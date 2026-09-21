# 349 — The distance, corrected: status instead of speed, and per-point outliers

**Status:** done
**Priority:** medium
**Created:** 2026-09-21
**Picked up by:** agent
**Started:** 2026-09-21
**Completed:** 2026-09-21

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

- [x] Points recorded **after** a member left the race (`reunited`, `released`) do not contribute. `finished`
      still contributes.
- [x] The status **transition time** is available to the calculation, not just the current status; where it
      is unknown the member's points are excluded entirely and a comment says why that direction was chosen.
      **⚠ The mechanism works; the data does not exist yet — see the log and task 350.**
- [x] `Accuracy` survives from `trackpoint.Point` into the merge and reaches the outlier filter.
- [x] A single implausible coordinate is dropped **without** dropping the legs either side of it; a test
      demonstrates a spike costing one point rather than two segments.
- [x] A point that is fast *in* and normal *out* is left to rule 1 rather than treated as a spike — asserted,
      because conflating the two is how a car becomes a GPS error.
- [x] Thresholds are measured on real data and the distribution recorded in this task's log **before** the
      numbers are fixed in code.
- [x] The 2025 figures are re-measured after the change: median, maximum, and what happened to the 103 km
      patrol from 339's log.
- [x] The 7 km/h filter's comment no longer claims to detect transfers, and says what it is now for.
- [x] The label still reads *"mindst ~N km"*; wording unchanged.
- [x] Every rule lives in `internal/distance` (or the merge), not in a handler, and the reasoning for each
      threshold is at its definition. **One exception, argued rather than excused:** the race-status cutoff
      lives in `cmd/api/patroltrack.go`, because `trackpoint.ByPeople` deliberately discards the person id
      — see the log.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-21 — Task created from PRD 011 §0b.6, the maintainer's correction of task 339's transfer premise.
  Depends on 339 and 340; related to 175 (the `stillInRace` predicate this needs belongs in shared-go).
- 2026-09-21 — **Done, and the measurement changed the design.** I set out to build the two rules the PRD
  asked for and found a **third** that matters more than either. In order:

  ### 1. What the data actually said

  **Accuracy (2026 dev telemetry, 648 points, 63 people).** A clean gap, which is what a threshold wants:

  | accuracy | points |
  |---|---|
  | ≤ 10 m | 316 |
  | ≤ 20 m | 226 |
  | ≤ 35 m | 46 |
  | ≤ 100 m | 53 |
  | 100–500 m | **1** |
  | > 500 m | 6 (worst: **11.8 km**) |

  So `MaxAccuracyMetres = 250` sits in an empty band rather than on a judgement. Accuracy 0 means *unknown*,
  not perfect, and is kept — discarding it would remove whole patrols whose phones report nothing.

  **Leg durations (2025 scans, 168 patrols, 3,232 legs).** This is the finding:

  | leg duration | legs |
  |---|---|
  | ≤ 1 h | 2,043 |
  | ≤ 2 h | 663 |
  | ≤ 4 h | 302 |
  | ≤ 12 h | 200 |
  | > 12 h | 24 |

  The worst patrol (103.5 km) was built from a **38.7 km leg spanning 13.6 hours** and a **10.4 km leg
  spanning 143 hours** — six days. Neither is a journey; they are two scans that do not belong to the same
  night. **A speed filter waves them through by construction**, because dividing by a huge number gives a
  walking pace. That is the hole task 339 recorded as "a slow transfer is indistinguishable from a walk", and
  it turns out not to be about transfers at all.

  ### 2. So there are three rules, not two

  **A third: a leg longer than 4 h *and* further than 10 km is not one walk** (`MaxLegHours`/`MaxLegKm`).
  Both conditions are needed, and that is the part worth reading twice: a long gap **alone is normal**,
  because patrols rest — 2025 has six-hour legs covering two kilometres, which is a patrol that slept and
  then walked to the next post. Rejecting those on duration would discard real walking for nothing, since a
  leg that is long in time and short in distance contributes almost nothing anyway.

  Measured alternatives, on 2025, all on top of the 7 km/h filter:

  | rule | mean | max | patrols > 60 km |
  |---|---|---|---|
  | none (as shipped by 339) | 44.8 km | **103.5 km** | 26 |
  | duration only, 12 h | 41.6 | 90.8 | 13 |
  | duration only, 8 h | 37.8 | 73.4 | 2 |
  | duration only, 6 h | 36.1 | 63.6 | 1 |
  | duration only, 4 h | 32.5 | 57.4 | 0 |
  | **4 h + 10 km (chosen)** | **38.1** | **63.6** | **1** |
  | 4 h + 8 km | 35.8 | 57.4 | 0 |
  | 4 h + 15 km | 39.4 | 64.8 | 2 |

  The compound rule was chosen over the plain 6 h cut because they reach the same maximum while the compound
  one keeps ~2 km per patrol of genuine walking. **The 103 km patrol is gone**; the event's worst case is now
  63.6 km and one patrol sits above 60.

  ### 3. The two rules the PRD asked for

  **Race status.** `person` gained `memberStatusAt`, projected from the event's own stream time — never
  `NOW()`, because these tables replay from sequence zero and a wall clock would restamp every withdrawal in
  the event's history with the time of the last deploy. `cmd/api/patroltrack.go` trims each withdrawn
  member's points at that instant; where the instant is unknown it drops them entirely, which errs low as
  *mindst* permits. `finished` is **not** a withdrawal — asserted, because a rule that treated it as one
  would zero out the distance of exactly the patrols this page celebrates.

  **Per-point outliers.** In `patroltrack.Merge`, *before* the gaps are found — order matters: a bad fix left
  in place breaks the run around itself, and one removed afterwards leaves two segments with a hole where the
  bad point was. Two rules: accuracy over 250 m, and a **spike** — implausibly fast away from the last kept
  point where the track then *comes back* within a few fixes. The "comes back" half is what separates an
  error from a journey: fast away and staying away is a phone in a car, which is rule 1's business. Judged
  against the last *kept* point, so two consecutive bad fixes cannot shelter each other — which they did in
  the first version, and a test caught it.

  ### 4. ⚠ The status cutoff cannot fire today, and that is a measurement, not a guess

  Verified in the dev stack after deploying: `racing` (from `patrulje…started`) has a time on 73 of 85 rows,
  while **every** `waiting`/`transit`/`sheltered`/`reunited`/`released` row has NULL. The member lifecycle
  events arrive with a zero `msg.Time()`, and their bodies carry no timestamp field at all — unlike
  `qr.scanned`, whose projection depends on `msg.Time()` and works.

  So in practice the rule degrades to its fallback: **a withdrawn member contributes nothing**. Shipped that
  way deliberately — it is the correct reading of "we do not know when" and it errs low — and raised as
  **task 350**, which asks upstream to carry the transition time in the event body. The column and the trim
  logic stay: they are right, and they start working the day the data exists.

  ### 5. One rule is not in `internal/distance`, and why

  The race-status cutoff lives in `cmd/api/patroltrack.go`. Not laziness: `trackpoint.ByPeople` returns point
  groups with **the person id thrown away**, precisely so the merged track cannot be attributed to anybody
  (PRD 011 §0b.1). A per-person rule therefore cannot be applied after that call, so it is applied before it
  — which also means members needing a cutoff are read individually (one batch query plus one per withdrawn
  member, asserted in a test, and the whole result is cached per patrol for an hour).

  ### 6. Also changed

  `person.MemberIDs` → `person.TrackMembers`, returning id + status + status time. One read rather than two,
  because two would be two chances for the ids and the statuses to disagree about who is in the patrol — and
  still three columns, so a name cannot reach the unauthenticated handler. Four test doubles updated.

  `Estimate.VehicleLegs` keeps its name but its doc no longer claims to know a car when it sees one: it
  counts legs the backstop rejected, which may be a car, a mis-stamped scan or a mis-attributed one.

  `gofmt`, `go vet`, `go test ./...` clean; dev stack restarted, schema drift applied, the patrol page and
  its map endpoint still render.
