# 254 — `scan` + `checkpersonnel` projections; retire the scans mock

**Status:** doing
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15

## Description

PRD 016 phase 1. Make the patrol's registrations real. `internal/scans` currently serves a
seeded mock behind a `Source` interface introduced for exactly this substitution.

Two projections:

- **`scan`** from `NATHEJK.*.qr.*.scanned` (`NathejkQrScanned`): team, scanner, time,
  position. Note `qr.scanned` is a *post visit*; `qr.registered` is a *handout* and
  belongs to task 251.
- **`checkpersonnel`** from `NathejkCheckpersonnelAdded` / `…Removed` /
  `…TimeSpecified`: which user is on shift at which checkpoint, and when.

**A scan carries no checkpoint.** The only link is who scanned it and when, so a scan
counts for a post if the scanner was on a registered shift there at that moment. hq's live
implementation is the reference (`scansByCheckgroup` in `cmd/api/checkgroupteams.go`):

```sql
FROM scan s
JOIN checkpersonnel cpn ON s.scannerId = cpn.userId
                       AND s.uts >= cpn.startUts AND s.uts <= cpn.endUts
JOIN checkpoint    cpt ON cpn.checkpointId = cpt.id
```

**The postmandskab rota is therefore load-bearing** — hq's own comment says so: with no
shifts recorded, no scan can be attributed and every team reads as missing. That means an
upstream data gap shows up here as a silently empty feature, which is why task 260 counts
unattributed scans.

An unattributable scan must still be **listed** as a registration (a scan the patrol
genuinely made), just without a checkpoint or a verdict. Losing it entirely would be worse
than showing it plainly.

## Acceptance Criteria

- [ ] `go/nathejk/table/scan/` and `go/nathejk/table/checkpersonnel/` projections.
- [ ] Scan → checkpoint resolution by scanner id within the shift window, matching hq's
      predicate exactly (inclusive bounds).
- [ ] An unattributable scan is returned without a checkpoint rather than dropped (test).
- [ ] A scanner on two shifts at different posts resolves to the shift covering the scan's
      timestamp (test).
- [ ] `internal/scans.Source` backed by the projection; the mock stays for dev/tests only.
- [ ] `/api/patrol/scans` still answers an empty list for users with no patrol.
- [ ] Idempotent under replay (test).

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 1.
- 2026-09-15 — Picked up. Plan: one package `go/nathejk/table/scan/` holding **both** tables —
  `scan` and `checkpersonnel` — following the precedent of `person`, which owns a second `section`
  table in the same package. They are not two independent entities here: `checkpersonnel` exists only
  to attribute a scan to a checkpoint, the join is the whole point, and splitting them would put the
  one query that matters in neither package.
