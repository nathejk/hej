# 171 — Raise the freshness/invalidation contract against PRD 009

**Status:** open
**Priority:** high
**Created:** 2026-08-31

## Description

PRD 007 requires directory changes to reach devices during the event — immediately on
foreground, within ~60 s while open. The mechanism (a cheap version check, pulled at
foreground / reconnect / interval) is **generic**: "tell me if this dataset changed" is
not portrait-specific.

**PRD 009's draft does not cover it.** It frames sync as a *pre-event readiness*
problem — download everything before the race, report progress, manage a storage budget —
not a *during-event freshness* one.

So either 009 gains an invalidation/freshness contract, or PRD 007 builds a private one
and we get exactly the duplication 009 exists to prevent. This task is to raise it and
get a decision, not to build anything.

Points to put to 009:

- a per-dataset version/etag endpoint convention, and who owns the polling loop;
- **field-level removal** in a delta — needed so a withdrawn member's phone number
  disappears from a device that already synced it (task 160), which "fetch what changed"
  alone does not express;
- the **sync-class split** PRD 007 §6 now requires: bulk image sync stays wifi-only and
  pre-race, while small metadata deltas are allowed during the race on mobile data. 009's
  current framing would forbid the latter;
- whether the freshness loop's interval belongs in 009's config or each consumer's.

## Acceptance Criteria

- [ ] The four points above raised in PRD 009 (as open questions or requirements).
- [ ] A decision recorded: 009 owns freshness, or PRD 007 builds it locally.
- [ ] If 009 owns it, tasks 155 and 162 are updated to consume 009's contract rather
      than defining their own.
- [ ] PRD 007 §8 updated with the outcome.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §8 and its dependencies section.
