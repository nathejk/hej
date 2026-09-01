# 191 — Delta shape that can express removal

**Status:** open
**Priority:** high
**Created:** 2026-09-01

## Description

PRD 009 §6. The one part of the freshness contract that was **not** built with tasks 155/162,
and the reason task 171 stays open.

A delta that carries only present fields cannot say "this value is gone". So a member who
withdraws has their phone number purged on the server (PRD 007 §6, `portraitpurge.go`) while
every device that synced earlier keeps showing it — and the purge becomes a server-side
gesture. Row-level removal has the same problem in a different shape: a person who leaves the
caller's permitted set must *disappear* from the cached index, not merely stop being updated.

So the delta needs to express three things, not one:

- a **row added or changed**;
- a **field unset** on a row that otherwise remains (a withdrawn member keeps their name and
  status until the end of the race, but loses their number — PRD 007);
- a **row removed** entirely (permission narrowed, or the person left the permitted set).

**The server decides, always.** A client that filters a broader payload is not enforcing a rule
it evaluated — PRD 009 §6, and the same principle that keeps `phoneParent` out of responses
rather than merely unrendered.

## Acceptance Criteria

- [ ] The delta shape supports add/change, **field-level unset**, and row removal, with
      OpenAPI annotations on the endpoint (`.rules`).
- [ ] Applying a delta on the client can clear a field to absent — not to `""`, which renders
      as a callable-looking empty row.
- [ ] **The test that makes this real:** sync, withdraw a member server-side, re-sync, and
      assert the number is gone from device storage. PRD 007 names the decorative purge as a
      risk; this is what closes it.
- [ ] A narrowed permission removes rows on the next sync, verified the same way.
- [ ] Task 171 closed on merge.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval; carries the one unbuilt point from task 171.
