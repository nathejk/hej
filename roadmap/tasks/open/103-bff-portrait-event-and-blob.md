# 103 — BFF: portrait event, projection column and blob write path

**Status:** open
**Priority:** medium
**Created:** 2026-08-28

## Description

Unblocked 2026-08-28 (task 102: consent held from sign-up, safety basis, purged
after the event).

Write side for the portrait: a domain event published through
`internal/commands`, consumed by the `person` projection to set `portraitRef`
(the column already exists on `person.Person`), with the bytes stored via
`internal/blob` content-addressed. No direct SQL write from a handler.

The read side is exposed through `internal/data.Models`, per the BFF
conventions.

## Acceptance Criteria

- [ ] Portrait command + event; nothing writes `person` outside its consumer.
- [ ] `person` consumer sets `portraitRef` from the event; replay-safe
      (idempotent).
- [ ] Blob written content-addressed through `internal/blob`; stored objects are
      not publicly enumerable.
- [ ] Consumer test covering set, replace and replay.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
