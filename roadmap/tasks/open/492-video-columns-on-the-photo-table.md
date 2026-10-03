# 492 — Video columns on the photo table, and delete walks that know them

**Status:** open
**Priority:** high
**Created:** 2026-10-03
**Picked up by:**
**Started:**
**Completed:**

## Description

Add `kind`, `status`, `durationMs`, `videoRef`, `videoSdRef` and `failReason` to the `photo` table (PRD 029 §8, option A). Existing rows default to `kind=photo`, `status=ready`. Every shared-blob delete walk (`RefsInUse` et al.) must claim the new refs.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [ ] Columns added; existing photographs unaffected
- [ ] Delete walks claim `videoRef`/`videoSdRef`, with tests in the style of `medium_test.go`

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
