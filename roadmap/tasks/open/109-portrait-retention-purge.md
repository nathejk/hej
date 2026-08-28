# 109 — Portrait retention / purge job

**Status:** open
**Priority:** low
**Created:** 2026-08-28

## Description

Blocked by task 102 — the retention period is exactly what that task decides.

Once decided, portraits (full image, thumbnail and `portraitRef`) must actually
be removed when the period expires. Per the BFF conventions this belongs as a
consumer/worker inside the existing `cmd/api` binary, not a second binary.

## Acceptance Criteria

- [ ] Purge removes blob, thumbnail and the projection reference together — no
      dangling `portraitRef`.
- [ ] The retention period is configuration, read in `main.go`, not a literal
      deep in the call tree.
- [ ] Idempotent and safe to run repeatedly.
- [ ] Test with a clock fake.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
