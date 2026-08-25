# 061 — Decide and implement the production database route

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

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

- [x] Decision recorded in PRD 008 §11 Q2 with reasoning
- [x] Implemented in `docker-swarm.yml` (coordinated with task 062)
- [x] Credentials handled as secrets, never committed
- [x] Dev and prod use the same driver, schema mechanism and connection code

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Picked up and investigated. Blocked pending a human decision; evidence
  recorded (no sibling repo has a swarm manifest, so no precedent existed to copy).
- 2026-08-25 — **Decision received from the maintainer: `hej` runs its own database.**
  Option 1 of the three. Unblocked.
- 2026-08-25 — Implemented a `db` service in `docker-swarm.yml`: `mariadb:10.8`
  (matching dev, so "works in dev" means something), database/user `hej`, credentials
  from `${DB_PASSWORD}` / `${DB_ROOT_PASSWORD}` with no defaults so `stack deploy`
  fails loudly rather than deploying something guessable.
- 2026-08-25 — Root and application passwords are kept **distinct**. The API only ever
  uses the `hej` user; a leak of the application credential should not also hand over
  root on the server.
- 2026-08-25 — The database is on a new **stack-private `internal` overlay network**
  only — no `traefik` network, no published ports. Nothing outside the stack has any
  business reaching it, and phpMyAdmin is deliberately not replicated here (task 066).
- 2026-08-25 — The consequence that needed thinking about, not just configuring:
  MariaDB's data lives in a **node-local volume**, so the task is pinned with
  `node.labels.hej.storage == true`. Without a constraint Swarm may reschedule onto
  another node and find an empty volume. For this service that is recoverable — every
  table is a projection and rebuilds from the log — but it would mean a full replay at
  the worst possible moment, and it is unrecoverable for the portrait bind mount on
  the same node (task 062). A node label rather than `node.role == manager` so the
  data node can be moved without editing the file.
- 2026-08-25 — `update_config.order: stop-first` here too, and more strictly than for
  the API: two MariaDB tasks briefly sharing one volume risks corruption, not just
  contention.
- 2026-08-25 — Added a `mariadb-admin ping` healthcheck. It goes green when the server
  accepts connections, which is exactly what the API's retrying startup ping (task
  050) is waiting for.
- 2026-08-25 — Same driver, same schema mechanism, same connection code as dev: the
  DSN differs only in host and credentials, and carries the same `parseTime` and
  `multiStatements` flags (see `fix(051)`).
- 2026-08-25 — ✅ All criteria complete. Validated by interpolating the variables and
  parsing the YAML, asserting on the resolved values.
- 2026-08-25 — Verification caveat: **Docker daemon not responding**, so no
  `stack deploy` was attempted. The bootstrap steps this now requires (create the
  `jetstream` overlay network if absent, label the storage node, create
  `/srv/hej/blobs`) are documented in the file header but **unrehearsed**.
- 2026-08-25 — Moving to done.

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
