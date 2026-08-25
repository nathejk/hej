# 064 — Decide and implement the replica strategy

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 008 §8 and §11 Q1. `docker-swarm.yml` reasons explicitly that HMAC-signed
sessions "would work across replicas". Adding projections invalidates that
reasoning.

If `hej` runs two replicas, both construct the same projectors and both consume
the same subjects against one database. Depending on durable-consumer naming and
whether a queue group is used, that is duplicated work, row contention, or events
split across instances — and the failure mode is a projection that is **subtly
wrong** rather than obviously broken.

Options:

1. Pin `hej` to one replica, documented as a constraint (cheapest correct answer)
2. Distinct durable consumer names per instance
3. Split the projector out of the API process

Deciding now is cheap; discovering it during an event is not.

## Acceptance Criteria

- [ ] Decision recorded in PRD 008 §11 Q1 with reasoning
- [ ] Implemented (replica count and/or durable naming) in `docker-swarm.yml`
- [ ] The stale "works across replicas" comment in `docker-swarm.yml` corrected
- [ ] Constraint documented where an operator will see it before scaling

## Progress Log

- 2026-08-25 — Task created from PRD 008.
