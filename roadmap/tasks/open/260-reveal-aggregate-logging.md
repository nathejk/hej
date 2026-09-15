# 260 — Boot-time reveal aggregate and unattributed-scan count

**Status:** open
**Priority:** medium
**Created:** 2026-09-15

## Description

PRD 016 phase 2. This feature's characteristic failure is **silence**: a reveal rule that
resolves nothing, or a rota with no shifts recorded, produces an app that works perfectly
and shows an empty map. Nobody notices until a patrol phones in.

Two counters, in the spirit of `checkpoint.ReportPositionless` — which reports the
*aggregate* rather than each gap, because individual gaps are expected and only the
systematic case matters:

1. **At boot / after replay:** how many map sets, sheets and checkpoints the year has, how
   many sheets are in a `patrulje` set, how many checkpoints are reachable by any reveal
   rule at all. "428 sheets, 0 in a patrulje set" is the shape of the disaster we are
   guarding against.
2. **Unattributed scans:** how many scans could not be resolved to a checkpoint because no
   `checkpersonnel` shift covered the scanner at that moment (task 254). This is an
   *upstream* data gap — the postmandskab rota — that is invisible from inside this repo and
   that silently disables both the on-time verdict and reveal rule 3.

Both are logs, not endpoints. The point is that someone reading the service log during
setup can see the feature is wired to real data before the race starts.

## Acceptance Criteria

- [ ] Aggregate logged once after projections are built, not per request.
- [ ] Log line names the year and is greppable.
- [ ] Zero sheets in a `patrulje` set logs at a level that stands out.
- [ ] Unattributed-scan count exposed (log line or counter) and referenced in PRD 016 §9's
      ≤ 5 % target.
- [ ] No individual-checkpoint spam.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 2.
