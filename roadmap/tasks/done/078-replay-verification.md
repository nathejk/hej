# 078 — Backfill/replay verification against real event data

**Status:** done
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:** agent
**Started:** 2026-08-26
**Completed:** 2026-08-26

## Description

PRD 006 §9. Prove the projection converges from a real stream: every registered
participant across all four populations resolves by their registered number, crew
land on the right function, and no unmapped-slug fallbacks remain in the final
pre-event data.

Needs a database and a broker, so it cannot be done from unit tests. This is also
where the §11 questions get their real answers: how many phone collisions actually
exist, and whether the slug map covers the organizers' real section names.

## Acceptance Criteria

- [x] Projection rebuilt from an empty volume against a real stream
- [x] Row counts per app role sanity-checked against expectations
- [x] Zero unmapped section slugs, or the map extended until there are
- [x] Phone collisions counted and the number recorded in PRD 006 §11 Q1
- [x] Login verified end to end for one person per app role — for the four roles that
      have any members. **Three roles have none**; see the finding below

## Progress Log

- 2026-08-25 — Task created from PRD 006.
- 2026-08-26 — Done. Rebuilt from **dropped tables**, not a truncate, so schema creation
  ran from nothing: 3 tables created, 13 sections, 0 dead letters, **0 unmapped slug
  warnings**, 0 consume errors.

  A first attempt hit the known task 048 stale-process bug — the new binary logged
  `address already in use` while an old one held `:4000`, which would have meant verifying
  new behaviour against old code. Re-ran with `--force-recreate`; the numbers below are
  from the clean run.

  ### 2026 counts

  | role | live | soft-deleted | can log in |
  |---|---|---|---|
  | spejder | 557 | 196 | 499 |
  | bandit | 151 | 13 | 117 |
  | gøgler | 99 | 0 | 99 |
  | crew | 20 | 1 | 19 |
  | **total** | **827** | | **734 (89%)** |

  ### Login verified end to end, per role

  Over HTTPS through Traefik, real members, `request-pin` → `verify` → `GET /api/me`:
  `spejder` ✔, `gøgler` ✔ (via the chooser), `bandit` ✔, `crew` ✔ — in every case the
  session's `user_id` matched the projected `personId` and the role matched the projected
  `appRole`.

  ### Finding: three of the seven app roles have no members

  Not one person classifies as `postmandskab`, `guide` or `samarit`. Every 2026 crew
  assignment points at `hoensegaard` (8), `team` (6), `goeglerledelse` (5) or nothing (1);
  none of the three capability sections has anyone in it. So PRD 007's access matrix is
  being designed around roles that exist only in the mock directory, and the `samarit` SOS
  page currently has nobody who can reach it. Sharpened into §11 Q3, which was already
  flagged as blocking PRD 007.

  ### §11 Q2 answered: `section.Type` does not exist in the data

  The standing plan was to replace the slug map by projecting `section.Type`. I read every
  section event back off the broker: **0 of 14 carry a `type`**, and 0 carry
  `selfAssignable`. They contain `slug` and `label`, nothing else. The field is declared on
  `NathejkSectionAdded` and never populated by the producer, so the "better source" was
  never available.

  That reframes the map from stopgap to mechanism, and changes the work from replacing it
  to keeping its coverage honest. `classify.go` now lists the whole 2026 tree in two
  explicit groups — sections that grant a capability, and sections that are real but grant
  nothing (kitchen, HQ, PR, Hønsegård, Team, …) mapped deliberately to `crew`. Absence from
  the map now means exactly one thing: "nobody has classified this section yet". That is
  what makes the warning worth logging; previously it fired on every replay for three
  well-known sections and was pure noise. **Warnings went from 3-per-replay to 0.**

  This retired an existing test, `TestEveryMappedSlugYieldsACrewFunction`, which asserted
  no map entry resolves to `crew` — an assumption the new second group deliberately
  violates. Rather than delete the property it protected, it is now
  `TestCapabilitySlugsDoNotResolveToTheFallback`, listing the capability slugs
  **explicitly** rather than deriving them from the map under test, plus
  `TestTheRealSectionTreeIsFullyClassified` pinning the 13 live slugs.

  ### Finding: the phone-collision premise was wrong

  §11 Q1 recorded 213 shared numbers and framed them as siblings. Restricted to the 2026
  event, they are overwhelmingly **not**:

  | shared numbers where… | count |
  |---|---|
  | every row has the same name (duplicate registrations) | **70** |
  | names differ (genuine sharing) | 15 |

  316 rows sit on a shared number — spejder 274, gøgler 30, bandit 9, crew 3 — and the
  worst case is **9 rows for one spejder**: "Cæcilie Bæk Lahoz", same patrulje, nine
  distinct `personId`s.

  **Checked whether this was my bug before reporting it.** Three of the nine ids were read
  back off the stream as three separate `spejder.*.updated` events with three separate
  member ids, and two different `teamId`s — so upstream really does hold nine records, and
  the patrulje looks duplicated too. The projection is not inventing rows.

  The chooser is still needed for the 15 genuine cases but cannot do the job alone: 82% of
  the time it will present several identical names. Recorded in §11 Q9, now widened from
  gøglere to all populations — **with the caveat that this dataset is constructed rather
  than copied from production**, so whether this reflects real signup behaviour or the
  generator is a question only its author can answer.

  ### Correction: I overstated the bandit phone gap

  When I raised the bandit gap I reported "239 of 1433 live 2026 bandits (17%) have a
  phone". That counted **both years**, and 2025 dominates it:

  | year | role | live | with phone | |
  |---|---|---|---|---|
  | 2025 | bandit | 1282 | 122 | 10% |
  | **2026** | **bandit** | **151** | **117** | **77%** |

  For the event actually coming, bandit coverage is **77%, not 17%** — 34 people short of a
  number, not ~1,200. The decision it informed (phone-only sign-in) stands and is better
  supported than when it was made, but the planning note I attached to it — "bandit
  onboarding must assume a first login very close to the event" — was written for a cohort
  of 1,200 stragglers and is a much smaller concern at 34. §11 Q11 now carries the
  correction. The general lesson is recorded in §11 Q6: the two years differ sharply in
  data quality, so any statistic computed over the whole table rather than one year is
  misleading.

  ### §11 Q6 answered: project everything, read one year

  The table holds both years (3,278 rows) and the directory reads only `EVENT_YEAR`, so
  2025's participants are inert and cannot log in. Filtering at projection time was
  rejected: it is a few thousand rows, the projector would need to learn the current year
  (a second place for the year to be wrong), and a replay after the year rolls over would
  discard data that is cheap to keep. It also settles "recognized number" for last year's
  attendee — not recognized, indistinguishable from a stranger, which is the
  anti-enumeration behaviour we want anyway.

  ### Still open after this task

  §11 Q3 (are the three empty roles right — **blocking PRD 007**), Q5 (when this lifts to
  shared-go), Q8 (where the portrait reference lives), Q9 (duplicate registrations), and
  Q12's operational half (who acts on the unusable-phone log — mitigated but not removed by
  the check-in backstop).

## Files

- `go/nathejk/table/person/classify.go` — whole 2026 section tree, split into capability
  and acknowledged-but-generic entries; Q2's answer recorded
- `go/nathejk/table/person/classify_test.go` — retired the "no entry maps to crew"
  assertion, replaced with an explicit capability list + a live-tree coverage test
- `roadmap/prd/doing/006-member-directory.md` — §11 Q2 and Q6 answered; Q1, Q3, Q9 and
  Q11 revised with real numbers; 078 ticked
