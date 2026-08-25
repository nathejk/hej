# 064 — Decide and implement the replica strategy

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

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

- [x] Decision recorded in PRD 008 §11 Q1 with reasoning
- [x] Implemented (replica count and/or durable naming) in `docker-swarm.yml`
- [x] The stale "works across replicas" comment in `docker-swarm.yml` corrected
- [x] Constraint documented where an operator will see it before scaling

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Picked up. Decision: **stay at a single replica**, option 1 of the
  three listed.
- 2026-08-25 — The evidence is concrete rather than theoretical. `hq`'s `main.go`
  documents that `jrgensen/stream` subscriptions are "ephemeral ordered consumers
  with no queue group, so every process receives every message rather than sharing
  them out", and concludes "hq must not run two replicas" for its own saga. The same
  mechanics apply here: two replicas would both apply every event to the same read
  model in the same database — fine for a strictly idempotent projection, silently
  wrong for an unconditional `UPDATE`, a non-deterministic insert, or a
  read-then-write.
- 2026-08-25 — It also affects the **write** side, which the task description did not
  anticipate: PRD 005's verification command is read-then-publish with no
  compare-and-swap, so two instances could both read the pre-state and both publish.
- 2026-08-25 — Found on inspection that `replicas: 1` was **already** set, for a
  different reason (PRD 001: the PIN and push stores are per-process in-memory maps).
  So the useful output of this task is not the number but the *reasoning*: making
  those stores shared is now **necessary but not sufficient** to replicate. Without
  that written down, someone fixing the PIN store would reasonably conclude the
  constraint was lifted.
- 2026-08-25 — **Found a real bug while verifying:** `update_config.order` was
  `start-first`, which boots the new task *before* stopping the old one — precisely
  the two-instance window the replica constraint forbids. Every deploy would have had
  both processes consuming the same events and writing the same read model. Changed
  to `stop-first`. The trade is a brief serving gap instead of an overlap, which for
  an in-event app is the right way round: a few seconds of 502 is recoverable, a
  corrupted projection during the race is not.
- 2026-08-25 — Also fixed a mistake of my own: I first added a second `deploy:` key
  rather than extending the existing one. YAML silently keeps the last duplicate, so
  it parsed and looked fine while dropping half the config. Caught by asserting on
  the parsed output rather than trusting the diff — merged into the single block and
  re-verified there is exactly one.
- 2026-08-25 — Corrected the stale `SESSION_SECRET` comment, which said sessions
  "would work across replicas": true about sessions, but it was the only thing in the
  file reasoning about replication and it pointed the wrong way. It now defers to the
  `deploy.replicas` constraint.
- 2026-08-25 — ✅ All criteria complete. PRD 008 §11 Q1 marked answered with the
  reasoning and the route to replicating later (split the projectors into their own
  process).
- 2026-08-25 — Verification caveat: **Docker daemon not responding**, so this is
  YAML-verified only — no `stack deploy` was attempted, and the `stop-first`
  behaviour was not observed.
- 2026-08-25 — Moving to done.
