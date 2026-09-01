# 171 — Raise the freshness/invalidation contract against PRD 009

**Status:** open
**Priority:** high
**Created:** 2026-08-31

## Description

PRD 007 requires directory changes to reach devices during the event — immediately on
foreground, within ~60 s while open. The mechanism (a cheap version check, pulled at
foreground / reconnect / interval) is **generic**: "tell me if this dataset changed" is
not portrait-specific.

**Superseded in part, 2026-09-01.** Two things happened after this task was written. The
mechanism was **built** for this one dataset — task 155 (`GET /api/contacts/version`) and
task 162 (`useContactsFreshness.ts`, served interval) are both done — and **PRD 009 was
rescoped** to carry freshness as a first-class requirement rather than framing sync as a
pre-event readiness problem only. So the question is no longer "does 009 cover this", it is
"is the shipped pattern written down as the convention before a second consumer needs it".

Status of the four points originally put to 009:

- **version endpoint convention + who polls** — covered. PRD 009 §6 and §8 now name tasks
  155/162 as the reference implementation: body `version`, a separate cheap endpoint, the
  three trigger points, and the interval served from `/api/config` where 0 disables the
  interval but not the foreground/reconnect checks.
- **field-level removal in a delta** — **still open**, and now a PRD 009 §6 requirement with
  its own task in 009 §10. This is the real remainder: a withdrawn member's phone number
  must *disappear* from a device that already synced it (task 160), which "fetch what
  changed" alone does not express.
- **sync-class split** — covered. PRD 009 §6 now states two classes explicitly: bulk
  transfers are pre-race and user-initiated, while metadata deltas are *permitted* during
  the race on mobile data. It also records that "wifi-only" is not implementable on iOS,
  since `navigator.connection` is unavailable in Safari.
- **where the interval lives** — covered: served per dataset from `/api/config`, hard-coded
  in neither place.

## Acceptance Criteria

- [x] The four points above raised in PRD 009 (as open questions or requirements).
- [x] A decision recorded: **009 owns freshness**, by generalising the pattern PRD 007
      already shipped rather than defining a new one. Recorded in PRD 009 §2, §6 and §8, and
      **approved 2026-09-01** — so this is settled, not pending.
- [x] Tasks 155 and 162 — shipped ahead of 009 by necessity; 009 now documents them as the
      convention rather than asking them to consume a different one.
- [ ] PRD 007 §8 updated with the outcome.
- [ ] Field-level removal carried into 009's delta-shape task, and 007 §8's freshness
      section pointed at it. *Carried — **task 191**, created on 009's approval. This task
      closes when 191 does.*

## Progress Log

- 2026-08-31 — Task created from PRD 007 §8 and its dependencies section.
- 2026-09-01 — Largely resolved by a PRD 009 review and rescope. Three of the four points are
  now 009 requirements citing the shipped implementation; only field-level removal is
  unbuilt. Reduced to a documentation follow-up on PRD 007 §8.
- 2026-09-01 — PRD 009 approved. Field-level removal is now task 191, and the convention
  write-up is task 190. This task is a stub pending those two.
