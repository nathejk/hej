# 265 — Server-side on-time verdict for all three schemes

**Status:** doing
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15

## Description

PRD 016 phase 4. Answer "were we on time?" for each checkpoint scan, and extend
`/api/patrol/scans` with it — additively (`checkpoint_id`, `on_time`, `delta_seconds`), so
the shipped client keeps working.

**Computed server-side**, so a device with a wrong clock cannot invent lateness.

Three schemes, three behaviours (PRD 016 §11.5):

- **`fixed`** — compare the scan time against `openFromUts` / `openUntilUts` directly.
- **`relative`** — the window opens at *this patrol's own scan* at the checkgroup named by
  `relativeCheckgroupId`, and lasts `openDuration` minutes. Resolvable because we hold that
  scan. If the anchoring scan does not exist yet there is no window, so **no verdict** — a
  wrong "for sent" is worse than a missing one.
- **`none`** — no window, no verdict.

**There is no grace period. The window is the window** (PRD 016 §11.6). The
`minusMinutes` / `plusMinutes` grace belongs to hq's superseded `controlpoint` model — the
one live copy of that query is commented out — and the current `checkpoint` table has no
grace columns. The live path (`scansByCheckgroup` in hq) is a plain inclusive
`uts BETWEEN openFromUts AND openUntilUts`. Match it exactly, so the app and the organizers'
own screens cannot disagree about who was on time.

Report **the delta** as well as the verdict: "12 min for sent" is something a patrol can act
on; a bare verdict is a judgement.

The `internal/scans` mock must be able to produce every verdict state, for dev simulation
(PRD 014, task 270).

## Acceptance Criteria

- [ ] Verdict computed in the BFF; nothing about it derived client-side.
- [ ] `fixed`, `relative` and `none` each tested, incl. exact boundary instants (inclusive).
- [ ] `relative` with no anchoring scan yields no verdict (test).
- [ ] No grace margin applied; a comment records why, with the hq reference.
- [ ] `delta_seconds` signed, so early and late are distinguishable.
- [ ] `/api/patrol/scans` change is additive; OpenAPI annotations updated.
- [ ] An unattributable scan is listed with no checkpoint and no verdict (task 254).

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 4.
- 2026-09-15 — Picked up. The scan projection already carries `scheme`, `relativeCheckgroupId` and the
  window on each row (tasks 252–254), so the verdict is a pure function over the patrol's scans. Computing
  it in `internal/scans` (which already assembles the registrations) rather than in SQL: the `relative`
  case needs the anchoring scan at *another* checkgroup, which is easiest over the in-memory set.
