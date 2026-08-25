# 061 — Decide and implement the production database route

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 008 §8 and §11 Q2. **Dev has a `db` service; production has nothing.**
`docker-swarm.yml` runs one stateless service with no database and no broker.
This is the decision that determines the backup story and the failure domain.

Options, in the PRD's order of preference:

1. **Its own MariaDB in the swarm stack**, with a volume and backups — mirrors
   dev, keeps `hej` independent, costs an operational component.
2. **A shared/managed MariaDB** owned by the org infra repo, with `hej` given its
   own schema — fewer moving parts, couples deployment to another repo.
3. **Read another service's database** — rejected: `hej` would depend on a schema
   it does not own, and `go-bff-layout` requires entities to own their schema
   slice.

**This needs a human decision** (it may depend on org infrastructure this repo
cannot see). Do not guess: if unanswered, log the blocker and stop.

## Acceptance Criteria

- [ ] Decision recorded in PRD 008 §11 Q2 with reasoning
- [ ] Implemented in `docker-swarm.yml` (coordinated with task 062)
- [ ] Credentials handled as secrets, never committed
- [ ] Dev and prod use the same driver, schema mechanism and connection code

## Progress Log

- 2026-08-25 — Task created from PRD 008.
