# 267 — `GET /api/patrol/handouts`

**Status:** open
**Priority:** high
**Created:** 2026-09-15

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

- [ ] Handler in `go/cmd/api/handouts.go` + route registered.
- [ ] OpenAPI annotations incl. the no-other-teams guarantee.
- [ ] Filter is on `teamType == "patrulje"`, across all matching sets (test with two sets).
- [ ] A set renamed does not change the result (test) — guards the name-matching mistake.
- [ ] `mapId == ""` yields a listed entry (test).
- [ ] 200 + empty list for a user with no patrol (test).
- [ ] Response carries no successor-team field (test).
- [ ] 503 with no projection, via the nil-interface trap test.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 4.
