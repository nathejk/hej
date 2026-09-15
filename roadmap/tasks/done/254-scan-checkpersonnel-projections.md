# 254 — `scan` + `checkpersonnel` projections; retire the scans mock

**Status:** done
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

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

- [x] `go/nathejk/table/scan/` and `go/nathejk/table/checkpersonnel/` projections.
      (Both tables landed in one package — see the log for why.)
- [x] Scan → checkpoint resolution by scanner id within the shift window, matching hq's
      predicate exactly (inclusive bounds).
- [x] An unattributable scan is returned without a checkpoint rather than dropped (test).
- [x] A scanner on two shifts at different posts resolves to the shift covering the scan's
      timestamp (covered by the join predicate; asserted on the query text).
- [x] `internal/scans.Source` backed by the projection; the mock stays for dev/tests only.
- [x] `/api/patrol/scans` still answers an empty list for users with no patrol.
- [x] Idempotent under replay (test).

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 1.
- 2026-09-15 — Picked up. Plan: one package `go/nathejk/table/scan/` holding **both** tables —
  `scan` and `checkpersonnel` — following the precedent of `person`, which owns a second `section`
  table in the same package. They are not two independent entities here: `checkpersonnel` exists only
  to attribute a scan to a checkpoint, the join is the whole point, and splitting them would put the
  one query that matters in neither.
- 2026-09-15 — Attribution query written as a **triple LEFT JOIN** (shift → checkpoint → checkgroup),
  carrying the checkpoint's window and the group's scheme along with each row so task 265 needs no
  second round trip. LEFT rather than INNER is the load-bearing choice: an INNER JOIN would silently
  drop an unattributable scan, turning an upstream rota gap into "the app lost our scan". Two tests
  guard it — one on behaviour, one on the query text, because a fixture can satisfy the first while
  the real query still drops rows.
- 2026-09-15 — Decision: a shift added **without** hours is stored with a zero window rather than
  skipped. A zero window matches no scan, so the shift attributes nothing — the safe direction.
  Guessing a window would attribute scans to a post on no evidence, and a wrong post name plus a wrong
  on-time verdict is worse for a patrol than a missing one.
- 2026-09-15 — Decision: `checkpersonnel.added` is an **upsert**, not an INSERT (hq uses an INSERT).
  `timespecified` can arrive before `added` on a replay, and an INSERT would then fail or lose the
  window that had already been set.
- 2026-09-15 — Decision: shift removal is a **hard DELETE**, unlike the soft deletes elsewhere in this
  codebase. A removed shift must stop attributing scans immediately; a flag would mean every
  attribution query had to remember to filter on it, and the one that forgot would attribute scans to a
  post nobody was standing at.
- 2026-09-15 — Caught the `SUM()`-over-empty-table trap in `UnattributedCount`: it returns NULL, not 0,
  so a fresh year would have failed the diagnostic query rather than reporting zero. Scanned into a
  `sql.NullInt64`, with a test.
- 2026-09-15 — Wired into `main.go` alongside the other projections, constructed on a database rather
  than on the broker — same reasoning as `person` and `checkpoint`: a patrol must be able to see its
  own map page during a broker outage, which is exactly when it would be reaching for it.
- 2026-09-15 — Decision on the mock: it survives as the **no-database fallback**, chosen by the
  *absence of a projection* rather than by an environment flag. That way a misconfigured production
  deployment gets an empty list and a log line, instead of silently serving a plausible invented evening
  of scans to real patrols. Fixture data reaching a race would be worse than no data, because it looks
  right. See `cmd/api/scansource.go`.
- 2026-09-15 — **Known gap, recorded rather than guessed: bandit catches.** `KindBandit` is part of the
  API contract and the drawer renders a red skull for it, but nothing available says a scan was a catch
  — `qr.scanned` carries the scanner, and the two cases are physically identical. Labelling a post visit
  "Bandit taget" in front of a patrol that was not caught would be worse than labelling it plainly, and
  it would look like data rather than a bug. Every real scan is therefore a checkpoint scan for now.
  Raised as **task 271**, with a pointer to it in the code.
- 2026-09-15 — An unattributed scan is labelled "Registrering" — not blank (which reads as a rendering
  bug) and not a guessed post name (which is a lie about where the patrol was).
- 2026-09-15 — ✅ All criteria complete. 20 tests across `nathejk/table/scan` and `internal/scans`;
  build, vet, gofmt and the full suite clean.
- 2026-09-15 — Done. **Phase 1 of PRD 016 is complete** — every upstream fact this feature needs is now
  projected locally. Phase 2 (the reveal rule) is unblocked.
