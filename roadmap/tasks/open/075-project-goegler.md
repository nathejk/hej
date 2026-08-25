# 075 — Project gøgler

**Status:** open
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 006 §8. Consume `NATHEJK.*.gøgler.*.signedup` / `.updated` /
`.status.changed`.

Gøgler people do **not** exist in shared-go: they live in `hq`'s local
`personnel` table. Projecting them here is therefore a second projection of the
same population in a second repo, which can disagree with hq's — PRD 006 §11 Q4
asks whether hq's slice should be promoted to shared-go instead. Proceed, but
record the duplication.

No guardian phone.

## Acceptance Criteria

- [ ] Gøgler subjects consumed and classified as the `gøgler` app role
- [ ] Guardian phone explicitly "not applicable"
- [ ] Idempotent, tested with `cqrstest` fakes
- [ ] The duplication with hq's `personnel` noted in the package docs and §11 Q4

## Progress Log

- 2026-08-25 — Task created from PRD 006.
