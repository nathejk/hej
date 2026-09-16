# 267 — `GET /api/patrol/handouts`

**Status:** done
**Priority:** high
**Created:** 2026-09-15
**Completed:** 2026-09-16

## Description

PRD 016 phase 4. Serve the patrol's map sheets, from the `maphandout` projection (task 251)
joined to `kort` for the sheet's name and format, plus synthesised entries (task 257).

- Behind `requireAuth`. **Empty list with 200** for users with no patrol, matching
  `/api/patrol/scans`.
- Scoped to sheets in the **patrol map set(s)**: `kortsaet.teamType == "patrulje"` —
  **never** matched on the set's name, which an organizer may rename mid-season
  ("Patruljer", "Patruljekort", "Patruljer nord"). Collect **all** sets carrying the value:
  it is a filter yielding candidate sheets, not a key, and a year may legitimately split its
  patrol maps across two sets.
- Per entry: QR sticker number (absent for a synthesised handout), sheet name, format,
  handed-out time, and whether the patrol still holds it.
- `mapId == ""` renders as **"Ukendt kort"** — the same wording hq's patrol page uses, so a
  patrol and an organizer on the phone read the same words. Empty means *unknown sheet*, not
  *no sheet*.
- Times are **unix seconds** upstream; convert deliberately.
- **The successor team must not appear.** hq's organizer view shows "Flyttet til {team}";
  ours says only that the sheet is no longer held. The columns are not selected at all
  (task 251), so there is nothing here to strip — but assert it anyway.
- **OpenAPI annotations mandatory** (repo rule), stating that other teams are never named.

## Acceptance Criteria

- [x] Handler in `go/cmd/api/handouts.go` + route registered.
- [x] OpenAPI annotations incl. the no-other-teams guarantee.
- [x] Filter is on `teamType == "patrulje"`, across all matching sets (test with two sets).
- [x] A set renamed does not change the result (test) — guards the name-matching mistake.
- [x] `mapId == ""` yields a listed entry (test).
- [x] 200 + empty list for a user with no patrol (test).
- [x] Response carries no successor-team field (test).
- [x] 503 with no projection, via the nil-interface trap test.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 4.
- 2026-09-16 — `GET /api/patrol/handouts` added. The work was mostly wiring: `reveal.Handouts` (task 257)
  and `kort.PatrolSheets` (which already filters on `teamType == patrulje` across all matching sets, never
  on name) do the real work, and `data.MapReads.Handouts` already exposes it. Handler shapes the response:
  `name`/`format`/`qr_id`/`handed_out`/`still_held`, unix seconds converted to a timestamp (matching
  `scanned_at`), and an empty sheet id named "Ukendt kort" server-side — the same server-side Danish
  fallback pattern as the scans endpoint's "Registrering", and HQ's own wording. Route registered beside
  `/patrol/scans`; 200+`[]`, 401, 503 (nil-interface trap via `mapReadsOrNil`), 500-not-empty branches all
  mirror the checkpoints handler. To pin the rename guard, extracted `PatrolSheets`' query to a
  package-level const `patrolSheetsQuery` (as scan does with `byTeamQuery`) and asserted it filters on
  teamType and never on a set name. Tests: `cmd/api/handouts_test.go` + `kort/querier_test.go`. All four Go
  gates green.
