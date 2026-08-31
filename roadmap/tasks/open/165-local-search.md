# 165 — Local search across the directory

**Status:** open
**Priority:** medium
**Created:** 2026-08-31

## Description

A sticky search field at the top of the pane, spanning **every person the caller may
list** (PRD 007 §6). Matches on name, group (klan / section) and arm number.

**It runs locally against the synced index** — a search that needs the network is not a
search, and this pane's whole premise is working at 03:00 with no signal.

Two hard constraints:

- **Spejdere are never in the index being searched.** They are not listable; they are
  reachable only through the patrol lookup (task 168), which is a separate, explicit
  action with its own input. Do not merge the two into one "smart" field — that would
  make patrol numbers browsable by accident.
- Results are ranked with **favourites first**, then whatever ordering is sensible;
  the common case should need no typing at all.

Performance target: feels instant at full event size (~250 listable records for the
largest role) on a mid-range Android. That is small enough that no index structure is
needed — but the matching should still avoid re-scanning on every keystroke if it is
cheap to memoise.

## Acceptance Criteria

- [ ] Search field sticky at the top, reachable one-handed while scrolling.
- [ ] Matches name, group and arm number, case- and diacritic-insensitively (Danish
      names: æ/ø/å must behave).
- [ ] Runs entirely offline against the synced index.
- [ ] No spejder can ever appear in results — asserted by test.
- [ ] Favourites ranked first.
- [ ] Empty state distinguishes "no matches" from "nothing synced yet".
- [ ] Results update without losing input focus.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §6 / §7.
