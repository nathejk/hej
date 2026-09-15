# 260 — Boot-time reveal aggregate and unattributed-scan count

**Status:** done
**Priority:** medium
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

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

- [x] Aggregate logged once after projections are built, not per request.
- [x] Log line names the year and is greppable.
- [x] Zero sheets in a `patrulje` set logs at a level that stands out.
- [x] Unattributed-scan count exposed (log line) and referenced in PRD 016 §9's ≤ 5 % target.
- [x] No individual-checkpoint spam.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 2.
- 2026-09-15 — Landed as `go/cmd/api/mapreadiness.go`, reported from the projections-running callback rather
  than at construction: the numbers only mean anything once the stream has replayed, and that callback is the
  first moment we know it has.
- 2026-09-15 — Decision: **levels are chosen by what an operator has to do**, not by severity in the
  abstract. No sheets at all is `INFO` — early in the season that is simply the truth, and a warning that
  fires for months trains people to ignore the channel (the same reasoning `watchDeadletters` uses for
  staying quiet at zero). Sheets that exist but sit in no `patrulje` set is `WARN`, because somebody has to
  go and mark one, and until they do **every patrol sees an empty map while every other number looks
  healthy**. That is the disaster this task exists for.
- 2026-09-15 — The warning names the expected team type (`patrulje`) in the message. Without it an operator
  is left guessing which of four values is meant, which is how they end up marking the wrong one.
- 2026-09-15 — Decision: unattributed scans are reported as a **ratio**, not a count, and the threshold is a
  tenth. "40 unattributed" is a rounding error in a healthy event and a catastrophe in a quiet one, so the
  count alone is not actionable. A tenth sits well above PRD 016 §9's ≤ 5 % target and still catches a rota
  with a few posts missing rather than only a wholly empty one.
- 2026-09-15 — Refactor prompted by the tests: split `reportAttributionCounts` out of the read, so the
  *judgement* is testable without a database. The judgement is the part that can be wrong and the part an
  operator acts on; the query around it is not interesting.
- 2026-09-15 — Test note: the logger writes **JSON** and the assertions read the fields, not the rendered
  message. The numbers are the whole point of these lines, and a substring match would happily pass on a
  line carrying the wrong count.
- 2026-09-15 — Silence before the race is deliberate and tested: with no scans there is nothing to say, and a
  startup "everything is fine" trains people to ignore the channel just as effectively as a periodic one.
- 2026-09-15 — ✅ All criteria complete. 7 tests; build, vet, gofmt and the full suite clean.
- 2026-09-15 — Done. **Phase 2 of PRD 016 is complete**: the reveal rule, its read-time resolutions, the
  handout list, the endpoint, the regression test and the diagnostics against silent failure. Phase 3 is
  frontend work.
