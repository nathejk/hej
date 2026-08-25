# 077 — Swap users.Directory to the projection

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 006 §6. Make the real projection satisfy `users.Directory` so login and
role-gated navigation read real data. The interface is the seam PRD 001 designed
for exactly this, so handlers must not change.

Keep `users.NewMockDirectory()` as the **test double** — it should not be deleted,
and it must not grow into a data source (PRD 003's task list was corrected on this
point).

Preserve the anti-enumeration contract: `found=false` must be indistinguishable
from `found=true` to the client, so verification simply never succeeds for an
unknown number.

## Acceptance Criteria

- [ ] An adapter satisfying `users.Directory` backed by the person querier
- [ ] Wired in `main.go`; mock retained for tests
- [ ] Falls back to the mock (or fails clearly) when there is no database
- [ ] No handler signature changes
- [ ] Anti-enumeration behaviour unchanged and tested
- [ ] Full suite green on both workspace and GOWORK=off paths

## Progress Log

- 2026-08-25 — Task created from PRD 006.
