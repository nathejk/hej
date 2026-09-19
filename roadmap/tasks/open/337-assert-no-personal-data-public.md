# 337 — Assert the public surface names no person

**Status:** open
**Priority:** high
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 011 §6, §8. The public frontpage's entire privacy claim is that **it names no human being**: a
patrol is a patrol — number, name, gruppe, korps — and its route is the patrol's route. That is what
makes publishing a merged track defensible (PRD 011 §0b.1), and it is why per-member tracks are
forbidden rather than deferred.

A claim like that is worth exactly as much as the test behind it, so this task is the test.

**Enforce it in the projection, not the template.** The public read model carries no name column, so a
careless template cannot leak one. This is not theoretical: the source row this page's header comes
from — shared-go's `tables/patrulje` — carries **`contactName`**, a personal name sitting directly
beside the `groupName` and `korps` we do want. One `SELECT *` is all it takes.

The fields that must never appear on this surface: a person's name, a phone number of any kind, a
portrait, and emphatically `phoneParent` — which `.rules` calls a hard rule rather than a per-feature
judgement, because the guardian it belongs to is not a user of this app and never agreed to be in it.

Prefer a test that is **hard to pass accidentally**: walk every public route, decode the response, and
fail on the presence of the forbidden field names *and* on known fixture values (a seeded person's name
appearing in HTML is the failure this catches that a schema assertion does not). Model it on
`glimtopenapi_test.go`'s route-walking approach so new public routes are covered by default rather than
when somebody remembers.

## Acceptance Criteria

- [ ] A test enumerates every public route (`/offentligt*`, `/api/public/*`) and asserts no response
      carries a person's name, a phone number, a portrait reference or `phoneParent`.
- [ ] The test fails if a **new** public route is added without being covered — enumeration, not a
      hand-maintained list.
- [ ] Seeded fixture values are used, so a name leaking into rendered HTML fails and not only a name
      leaking into JSON.
- [ ] The public patrol read model has no name column at all (cross-check with task 338), and
      `contactName` is demonstrably not projected.
- [ ] A test asserts no public response contains an individual member's track as an attributed series —
      only the merged, unattributed patrol track.
- [ ] Failure messages name the field and the route, so the next person can act on a red build without
      reading the test.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §8 / §10 (Phase 1).
