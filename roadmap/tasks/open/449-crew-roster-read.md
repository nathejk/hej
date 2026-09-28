# 449 — The crew roster read behind the credit picker

**Status:** open
**Priority:** high
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 025 §6 R2. An admin-only read of a year's crew, so the "Fotokredit" sheet can offer a list to pick from
instead of a field to type into.

**Names only.** Not a phone number, not an email, not a portrait, not a birthday — the person row holds all of
them and this read must carry exactly the id and the name it needs, because a read that returns "the person" is
one somebody later renders more of. The library's privacy guards do not walk the person package, so this one is
on the reviewer and on the test.

Defaults to `sectionSlug = "pr"`, where most photographers are, and can return all of the year's crew — `pr` is
the default filter, not the boundary. `KEY year_section (year, sectionSlug)` already exists.

Only people who classify as `person.RoleCrew` (see `classify.go`). Not because a participant would otherwise
appear in a photographer picker by accident, but because the same rule has to hold in the resolver (task 451)
and two places deciding "who is crew" is how they come to disagree.

## Acceptance Criteria

- [ ] `GET /api/admin/crew` (or similar) — the year's crew, `pr` by default, all-crew on request
- [ ] The response carries id and name, and **nothing else**
- [ ] Only `RoleCrew`; a spejder or bandit never appears
- [ ] Behind the admin credential, `no-store` like the rest of `/admin`
- [ ] OpenAPI annotations
- [ ] A test that the response type cannot grow a phone number without failing

## Progress Log

- 2026-09-28 — Task created from PRD 025 §6 R2.
