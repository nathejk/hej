# 061 — Decide and implement the production database route

**Status:** doing
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
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
- 2026-08-25 — Picked up and investigated. **Blocked: needs a human decision.** Not
  guessing, per the task's own instruction.
- 2026-08-25 — Evidence gathered, which narrows it usefully:
  - **No sibling repo has a swarm or stack manifest at all.** `hq`, `tilmelding`,
    `skan` and `nathejk` have only `docker-compose.yml`. So `hej` is the first repo
    here with a production Swarm deployment, and there is **no existing precedent to
    copy** — which is exactly why this cannot be inferred from the codebase.
  - In dev, every sibling runs its **own** MariaDB 10.8 container with its own
    database (`hq`, `tilmelding`, `hej`), i.e. one database per service, never a
    shared schema. Whatever production does, it should preserve that boundary — which
    rules out option 3 (reading another service's database) on the same grounds
    `go-bff-layout` gives: entities own their schema slice.
  - Sibling DSNs also revealed a missing flag in our dev DSN; fixed separately
    (`fix(051)`).
- 2026-08-25 — The open part is genuinely external to this repo: **how is production
  actually hosted?** If the org runs a managed/shared MariaDB, option 2 is nearly
  free and option 1 duplicates an operational component. If it does not, option 1 is
  the only real choice. Nothing in this repo, or in any sibling, reveals which.
- 2026-08-25 — Deliberately **not** implemented. Adding a MariaDB service plus a
  volume to `docker-swarm.yml` would look like progress while committing the project
  to running its own production database — a decision with backup, upgrade and
  data-loss consequences that outlast this task. A wrong guess here is expensive to
  reverse once it holds real data.
- 2026-08-25 — What was done instead: `docker-swarm.yml`'s stale comment (which
  claimed there was no `db` service because the API only served mock data) has been
  corrected to state that the API now *has* a persistent store, that production
  configures none of it, and that a task deployed from that file therefore runs
  degraded — with a pointer to this task. So the gap is documented where an operator
  will see it, rather than being silently wrong.
- 2026-08-25 — **Question for the maintainer:** does the org already run a
  managed/shared MariaDB that `hej` could be given a schema on, or should `hej` run
  its own MariaDB service in its swarm stack? Task 062 is blocked behind the answer.
