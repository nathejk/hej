# PRD 008 — Persistence and event-stream infrastructure for `hej`

**Status:** doing
**Author:** agent session (Zed)
**Created:** 2026-08-25
**Last updated:** 2026-08-25
**Approved:** 2026-08-25
**Shipped:**
**Target users:** none directly — this is enabling infrastructure for PRDs 003, 005, 006 and 007

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 1. Summary

Make `hej` a stateful service: connect the Go binary to MariaDB, connect it to
NATS JetStream, and adopt the `jrgensen/cqrs` seam the other Nathejk services use,
so that projections and app-owned data can live here. Today `hej` is deliberately
stateless — every read is a mock and every write goes nowhere — and four queued
PRDs cannot proceed until that changes.

## 2. Problem & Motivation

- **What problem does this solve?** PRD 001 shipped `hej` as a skeleton with
  mocked reads and no persistence, explicitly deferring "domain aggregates,
  JetStream consumers, or event sourcing". That deferral has now been reached from
  four directions at once:

  | PRD | Needs |
  |---|---|
  | 003 profile page | a blob store + row for the portrait |
  | 005 onboarding | a per-person `verified_at` flag, written by the app |
  | 006 member directory | a projection consuming JetStream, read on the login path |
  | 007 portrait identification | thumbnails, plus a view audit log |

  Each was written as though persistence were incidental. It is not: it is the
  largest single piece of work under all four, and it is **shared** between them.

- **Why it deserves its own PRD rather than living inside PRD 006.** Three
  reasons:
  1. **It is not 006's private concern.** Three other PRDs need it, two of them
     (003, 005) for app-owned writes that have nothing to do with the directory.
     Burying shared infrastructure in one consumer's document means the next
     consumer cannot tell what it may rely on.
  2. **It is an architectural change, not a task.** `hej` goes from stateless to
     stateful. Production is *currently designed* stateless — `docker-swarm.yml`
     runs one service with no database and no broker, and its own comment notes
     that "session cookies are HMAC-signed and stateless, so they survive task
     restarts and would work across replicas". Adding state changes deployment,
     backup, secret handling and the replica story (§8). That decision deserves an
     explicit approval gate of its own.
  3. **Approving 006 would otherwise conflate two decisions** — "we want a member
     directory" and "we adopt stateful CQRS in `hej`". The first is obvious; the
     second is consequential and reversible only expensively.

- **What the repo actually has today** (verified 2026-08-25 — this corrects an
  earlier claim in PRD 006 §8 that MariaDB had to be introduced):
  - `docker-compose.yml` **already provisions MariaDB 10.8** as the `db` service,
    with database/user/password `hej`, a named `db` volume, and **phpMyAdmin**
    routed at `sql.hej.local.nathejk.dk`.
  - The `api` service has **no DSN environment variable**, and `cmd/api/main.go` /
    `config.go` contain **no `sql`, `db` or `nats` reference at all**. So the dev
    database is provisioned and entirely unused — template scaffolding.
  - There is **no NATS service** in dev compose.
  - `docker-swarm.yml` (production) has **neither a database nor a broker**.
  - `go.mod` has two direct dependencies: `httprouter` and `golang.org/x/crypto`.
    No driver, no `cqrs`, no `stream`, no `shared-go`.

  So the gap is smaller in dev than feared and **larger in production than
  anyone has written down**. That asymmetry is the main thing this PRD exists to
  surface.

## 3. Goals

- The Go binary reads and writes MariaDB in dev and in production.
- The binary consumes JetStream and can host projections, using the same
  `cqrs.Publisher` / `cqrs.Writer` / `cqrs.Reader` seam as `hq` and `tilmelding`.
- App-owned writes (verification flag, portrait, audit log) have a real home.
- Production is deployable, backed up, and safe to restart — including a clear
  answer on replicas.
- A developer can bring the stack up, get a populated database, and work offline
  from production data.
- The seam is boring and identical to the sibling repos, so knowledge transfers.

## 4. Non-Goals

- **Any user-facing feature.** No endpoints, no UI. Consumers are PRDs 003–007.
- **Designing the member directory projection.** PRD 006 owns its schema and
  classification rules; this PRD provides the machinery it plugs into.
- **Migrating `hej`'s existing state.** There is none: PIN records and sessions are
  ephemeral/stateless by design (PRD 001) and stay that way unless §11 decides
  otherwise.
- **Introducing a second binary or worker.** Per `go-bff-layout`, extra workers
  belong as JetStream consumers in the same process.
- **Changing the org's shared infrastructure.** The Traefik network and any shared
  broker are owned elsewhere; this PRD consumes them, it does not redefine them.
- **The portrait feature itself** — capture, upload validation, thumbnailing and
  the consent story are PRD 003's. This PRD provides the blob store and **owns the
  choice of object store vs mounted volume** (§11 Q4), since that is an
  infrastructure and backup decision, not a feature one.

## 5. User Stories & Scenarios

No end users. The stories are developer- and operator-facing:

- As a **developer**, I want `docker compose up` to give me an API that talks to a
  database and a broker, so I can build a projection without inventing plumbing.
- As a **developer**, I want realistic data locally, so a projection can be
  verified against something other than an empty table.
- As an **operator**, I want to deploy `hej` to production with its state, and know
  it survives a restart and a redeploy.
- As an **operator**, I want to know what happens if the broker is unreachable —
  ideally that people can still log in and read the app.

### Primary path

1. `docker compose up` starts `ui`, `api`, `db` and a new `nats` service.
2. `api` connects to MariaDB via a DSN and to JetStream, runs
   `CREATE TABLE IF NOT EXISTS` plus `cqrs.EnsureColumn`/`EnsureIndex` for each
   registered table, and subscribes its projectors through an `xstream.Mux`.
3. Events replay; projections converge; handlers read through `data.Models`.
4. The dev loop (`docker/init/api-dev`) restarts the binary on `.go`/`.sql` change
   without losing the database.

### Edge cases and failure modes

- **Broker down at startup.** Must not prevent the API from serving. Reads come
  from SQL projections, so a stale-but-present projection is far better than a
  dead app during an event. Startup must not block on JetStream.
- **Broker down during the event.** Projections go stale silently. Needs a health
  signal, and a documented answer to "how stale is too stale".
- **Database down.** Login stops working, since the directory read is on that path.
  This is a new single point of failure that `hej` does not have today, and it is
  the price of the design — worth stating plainly rather than discovering.
- **Replay from zero** on a fresh volume: must converge without manual steps, and
  must be fast enough to be routine.
- **Two replicas.** Both would run projectors against one database (§8) — the
  behaviour must be decided, not discovered.
- **Schema change with an existing volume.** `EnsureColumn`/`EnsureIndex` handle
  additive drift; anything destructive needs a documented procedure.
- **Deadletters.** A projection statement that cannot be applied must be visible,
  not swallowed.

## 6. Requirements

### Functional

- [ ] `DB_DSN` (or equivalent) configuration read in `main.go` via the existing
      `flag.StringVar(..., os.Getenv(...))` pattern, plumbed through the `config`
      struct — never read deeper in the call tree, per `go-bff-layout`.
- [ ] MariaDB driver added; connection pool configured with sane limits and a
      startup ping with bounded retry.
- [ ] `db` service added to the `api` service's environment and `depends_on` in
      dev compose (the service itself already exists).
- [ ] The `api` service joins the **shared, org-level `jetstream` network** and is
      given `JETSTREAM_DSN`. This repo does **not** run its own broker — see §8.
- [ ] The cqrs triple wired in `main.go`: `cqrs.Reader` (`*sql.DB`), `cqrs.Writer`
      (`deadletter` wrapping `sqlpersister`), `cqrs.Publisher` (`metatagger` over
      JetStream).
- [ ] `xstream.Mux` created, with the three-way registration pattern `hq` uses:
      constructor → `projections` slice → `data.NewModels(...)`.
- [ ] `shared-go` added to `go.mod`, with `go.work` resolving `../../shared-go` in
      dev, and a verified `GOWORK=off` build for CI/prod.
- [ ] `commands.Commands` becomes real (it is an empty struct today): `hej`
      **publishes** domain events, and no handler writes SQL directly.
- [ ] A blob store for portrait bytes (object store or mounted volume),
      content-addressed, with the reference carried on the stream (§8).
- [ ] Whichever position-telemetry mechanism PRD 002 settles on (§8) — most likely
      a separate, short-retention telemetry stream.
- [ ] Production: `docker-swarm.yml` gains whatever state the design requires — a
      database it can reach, broker credentials, and volumes/secrets to match.
- [ ] Health/readiness reflects database connectivity, and reports broker
      connectivity and projection lag **without** failing readiness on broker
      absence.
- [ ] Startup does not block on JetStream; the API serves reads if the broker is
      unavailable.
- [ ] Deadlettered projection statements are observable.
- [ ] A documented way to get realistic data locally (§11).
- [ ] `go test ./...`, `go vet`, `staticcheck` stay green in the dev loop, which
      means anything touching the DB must be testable without one — use
      `cqrs/cqrstest`'s in-memory `Writer`/`Publisher` fakes.

### Non-Functional

- **Availability.** A broker outage must not take the app down. During an event,
  degraded and serving beats correct and dead.
- **Login latency** stays acceptable with a database on the path.
- **Secrets.** DSN and broker credentials never committed; dev defaults in
  `docker-compose.yml`, real values in `docker-compose.override.yml` / swarm
  secrets, matching how `SESSION_SECRET` and the VAPID keys are already handled.
- **Backups.** The **blob store** is the backup scope: it is the only thing not
  rebuildable from a stream. Projections need not be backed up, and the schema
  should make that distinction obvious.
- **Parity.** Dev and prod use the same driver, schema mechanism and connection
  code, so "works in dev" means something.
- **Consistency with siblings.** Deviating from `hq`'s wiring needs a reason.

## 7. UX / UI Notes

N/A — no user-facing surface. The only user-visible consequence is indirect: an
app whose data is real, and a new class of outage in which login fails because the
database is unreachable.

## 8. Technical Considerations

### The replica question

`docker-swarm.yml` today is a stateless single service, and its comment reasons
explicitly about replicas: HMAC-signed sessions "would work across replicas".
Adding projections changes that reasoning, and it is the sharpest open issue here.

If `hej` ever runs two replicas, both would construct the same projectors and both
would consume the same subjects against one database. Depending on how the durable
consumer is named and whether a queue group is used, that is either duplicated
work, contention on the same rows, or events split across instances — and the
failure mode is a projection that is subtly wrong rather than obviously broken. It
needs a decision (§11): pin `hej` to one replica, use distinct durable names, or
separate the projector from the API process. Deciding it now is cheap; discovering
it during an event is not.

### Where the database comes from in production

Dev has a `db` service. Production has nothing. Options, in rough order of
preference:

1. **Its own MariaDB in the swarm stack**, with a volume and backups — mirrors dev,
   keeps `hej` independent, costs an operational component.
2. **A shared/managed MariaDB** owned by the org infra repo, with `hej` given its
   own schema — fewer moving parts, but couples deployment to something outside
   this repo.
3. **Read another service's database.** Convenient and a bad idea: it makes
   `hej` depend on a schema it does not own, and `go-bff-layout` is explicit that
   entities own their schema slice.

This is the main thing to settle before implementation, because it determines the
backup story and the failure domain.

### Alternative explicitly excluded

`hej` could in principle call an HTTP API on `tilmelding`/`hq` for directory data
and keep no projection and no broker. **This is forbidden by architecture rule
(2026-08-25): services may not contact other services' APIs, and `hej`'s api is
strictly a backend-for-frontend.** It is recorded here only so nobody rediscovers
it as a shortcut when the infrastructure work looks expensive.

The rule also removes the fallback this PRD previously leaned on. There is no
cheaper path to real data — the event stream is the only way in — so PRDs 003,
005, 006 **and** 007 are all gated on this PRD, not just 006. Plan accordingly.

### Write side: everything goes through the stream

Architecture rule (2026-08-25): **nothing writes directly to the database.** Every
state change is published as an event; SQL exists only as projections of the log.
Consequences this PRD must provide for:

- `hej` is a **publisher**, not only a consumer. `commands.Commands` (an empty
  struct today) becomes the write facade, and `cqrs.Publisher` is mandatory rather
  than incidental.
- Handlers never touch SQL: read through `data.Models`, write through
  `commands.Commands`, per `go-bff-layout`.
- Every table in `hej` is rebuildable from the log, with the two exceptions below.
- PRD 005's verification flag is a domain event — which is also what lets `hq` see
  it at check-in. That was 005's stated goal, and it now falls out of the
  architecture instead of needing a special case.

### The two things an event stream may not fit

**1. Portrait binaries.** The rule is settled for the metadata: **the existence of
a portrait and its metadata go on the stream**; the bytes are the open part. Large
image payloads in JetStream make replay expensive, inflate retention, and put an
opaque blob in a log optimised for small messages. Proposed shape:

- The event carries a **reference**, not bytes: person id, content type,
  dimensions, byte length, and a **content hash** naming the object.
- Bytes go to an object store (S3-compatible) or a mounted volume, keyed by that
  hash.
- A projection maps person → current portrait reference. Content addressing makes
  it idempotent, so a replay converges without re-uploading anything.
- Consequence: the blob store is the **only thing not rebuildable from the log**,
  and therefore the only thing that strictly must be backed up. A replay that finds
  a missing object must degrade to "no photo", never fail.

**2. The geolocation track.** As observed (2026-08-25): *a new coordinate does not
represent an event.* It is telemetry — high-frequency, individually meaningless,
useful only in aggregate — and modelling each fix as a domain event would swamp the
log with data nobody will replay for business meaning.

That collides with "nothing writes directly to the database", so one of the two must
give. Options, in order of preference:

1. **A separate telemetry stream**, own subjects, own retention limits (short,
   age- or size-capped), distinct from the domain log. Positions are still
   published, so the no-direct-writes rule survives intact, but they never pollute
   domain replay and they expire on their own. Batched uploads keep volume sane.
2. **An explicit carve-out** — a position sink that writes SQL directly, documented
   as the single exception. Simpler, but it puts a hole in the rule that the next
   feature will want to widen.
3. Coordinates as domain events. Not recommended, per above.

Recommendation: (1). It honours the rule, keeps domain replay clean, and gives
retention for free — which matters, because a position track of identifiable minors
is data we should want to expire quickly regardless.

**This decision belongs to PRD 002** (map with position and scan history), already
in `doing/`. It has to be settled there; this PRD provides whichever mechanism is
chosen.

### What goes in the database

Under the no-direct-writes rule, **everything in SQL is a projection** and is
rebuildable. The distinction that matters operationally is therefore not
"projection vs app-owned" but "in the log vs outside it":

| Kind | Examples | Source of truth | Rebuildable? | Backup? |
|---|---|---|---|---|
| Projection of the domain log | member directory (006), portrait metadata (003), `verified_at` (005), view audit (007) | JetStream | yes | no |
| Projection of a telemetry stream | position track (002) | telemetry stream, short retention | yes, while retained | no |
| **Blob store** | portrait bytes (003) | itself | **no** | **yes** |

The blob store is the whole backup story. Keep it clear of projection tables so a
rebuild cannot destroy it, and never join bytes into a row a replay would truncate.

### The broker is shared, not ours

*(Corrected 2026-08-25 during task 053, which had been written as "add a `nats`
service to dev compose".)*

The broker is an **org-level shared service**: the `nathejk` repo runs it
(`nats:2.10-alpine`, `command: -js`) on a `jetstream` network it owns, and every
consumer — `hq`, `tilmelding`, `skan` — declares that network `external: true` and
connects with `JETSTREAM_DSN: nats://jetstream:4222`. `hej` follows the same
pattern.

Running a private broker here would have been actively harmful, not merely
redundant: events `hej` publishes would land in a broker `hq` never reads, so PRD
005's verification flag would never reach check-in. The failure would be silent —
publishes succeed, nothing consumes them — which is the worst shape for this class
of bug. That also **answers §11 Q3**.

### Sequencing and verification

Infrastructure with no consumer is unverifiable, so this PRD should **not** be
declared done on "a connection exists". Its done condition is that PRD 006's first
projection persists, survives a restart, and rebuilds from an empty volume.

To be precise about the ordering, since "blocked by" in both directions reads as a
cycle (clarified 2026-08-25):

- **This PRD is blocked by nothing.** It can start immediately.
- **PRD 006's schema slice + first projection** is this PRD's acceptance test, and
  is therefore built as part of delivering it.
- **The rest of PRD 006** (the remaining populations, classification, the directory
  swap) is blocked by this PRD.

So the two are approved together, 008 sequenced first, with a thin slice of 006
riding along as the proof.

### Dependencies & risks

- **Risk: this is the schedule.** It is larger than any of the four PRDs waiting on
  it, and it is invisible to users, which makes it the easiest thing to
  underestimate and the worst thing to rush.
- **Risk: a new single point of failure on the login path.** Today login needs no
  database. Afterwards it does. Accept deliberately.
- **Risk: dev/prod divergence** if production takes a different database route
  than dev.
- **Risk: `go.work` convenience hiding a broken `GOWORK=off` build** — the
  `go-bff-layout` skill warns about exactly this; CI must prove it.
- **Risk: replay time grows** with event history, making a fresh volume slow.
- **Risk: phpMyAdmin is already exposed via Traefik** in dev
  (`sql.hej.local.nathejk.dk`) against an empty database. Once the database holds
  minors' names, addresses, guardian phone numbers and portraits, that surface
  deserves a second look even in dev.

## 9. Success Metrics

- `docker compose up` from a clean checkout yields an API connected to both
  MariaDB and JetStream, with no manual steps.
- A projection rebuilds from an empty volume without intervention.
- The API serves reads with the broker stopped.
- Production deploy, restart and redeploy preserve app-owned data; a restore from
  backup is tested at least once before the event.
- `GOWORK=off` builds green in CI.
- PRDs 003, 005, 006 and 007 proceed without further infrastructure work.

## 10. Rollout / Task Breakdown

Sequence: database first (it unblocks 003/005 on its own), then the broker and the
cqrs seam, then production. Keep the mock directory in place throughout so `hej`
stays runnable and the app is never half-migrated on `main`.

Proposed tasks for `roadmap/tasks/open/` (created 2026-08-25 as tasks **050–066**):

- [ ] 050 — `DB_DSN` config + MariaDB driver + pooled connection with startup ping
- [ ] 051 — wire `db` into the `api` service env/`depends_on` in dev compose
- [ ] 052 — add shared-go + cqrs + stream to `go.mod`/`go.work`; prove `GOWORK=off`
- [ ] 053 — JetStream-enabled `nats` service in dev compose with a volume
- [ ] 054 — construct the cqrs triple
- [ ] 055 — `xstream.Mux` + the projector registration pattern
- [ ] 056 — real `commands.Commands` write facade (publisher-backed)
- [ ] 057 — content-addressed blob store for portrait bytes
- [ ] 058 — non-blocking broker startup + degraded-mode reads
- [ ] 059 — health/readiness reporting DB, broker and projection lag
- [ ] 060 — deadletter observability
- [ ] 061 — decide + implement the production database route (§11 Q2)
- [ ] 062 — production swarm changes: state, secrets, volumes
- [ ] 063 — backup + restore for the blob store, tested once
- [ ] 064 — decide + implement the replica strategy (§11 Q1)
- [ ] 065 — local data seeding / replay procedure
- [ ] 066 — review the phpMyAdmin exposure

The position-telemetry mechanism is **not** a task here: PRD 002 owns that
decision (§8), and this PRD provides whichever mechanism it settles on.

## 11. Open Questions

1. ~~**Replicas.**~~ *Answered 2026-08-25 (task 064): pinned to a **single
   replica**, declared as `deploy.replicas: 1` in `docker-swarm.yml` with the
   reasoning inline.* The constraint is not the HTTP layer but the projections:
   `jrgensen/stream` subscriptions are ephemeral ordered consumers with **no queue
   group**, so every process receives every message. Two replicas would both apply
   every event to the same read model in the same database — fine for a strictly
   idempotent projection, silently wrong for an unconditional `UPDATE`, a
   non-deterministic insert, or a read-then-write. It also affects the write side,
   since a read-then-publish command has no compare-and-swap behind it. `hq`
   reached the same conclusion independently and documents it in its `main.go`.

   To replicate later, the clean route is to **split the projectors into their own
   single-instance process**, leaving the API stateless and freely replicable. The
   alternatives (distinct durable consumers plus a database each; or proving every
   projector and command idempotent under concurrent delivery, forever) are how
   projections quietly rot. The stale "would work across replicas" comment on
   `SESSION_SECRET` has been corrected.
2. **Production database:** own MariaDB service in the stack, or a shared/managed
   instance owned by the infra repo? (§8)
3. ~~**Which broker does `hej` connect to**~~ — *answered 2026-08-25 (task 053)*:
   the shared org broker on the external `jetstream` network, same as `hq`,
   `tilmelding` and `skan`, via `JETSTREAM_DSN`. `hej` runs no broker of its own.
   Still open in production: whether the swarm stack can reach that network, and
   whether credentials are needed (task 062).
4. **Portrait bytes: object store or mounted volume?** §8 settles that the bytes
   stay off the stream and that the reference goes on it; where they land is open.
   An S3-compatible store is easier to back up and share between replicas; a volume
   is one less component. Does the org already run object storage? **This PRD owns
   the decision** (it is infrastructure); PRD 003 links here rather than deciding it
   too.
5. **Position telemetry — separate stream or documented carve-out?** §8 recommends a
   separate short-retention stream. **Owned by PRD 002**, which is in `doing/` and
   was written before this rule existed; it needs revisiting there.
6. **What retention** for the telemetry stream, and does it satisfy the privacy
   expectation for a position track of minors?
7. **How do developers get realistic data?** Replay from a real stream, an
   anonymised dump, or seeded fixtures? Given the data is minors' personal details,
   an anonymised path is likely the only acceptable one.
8. **Should PIN records and sessions move into the database** now that one exists?
   They are deliberately ephemeral today. Sessions are stateless by design and
   should probably stay so; PIN records are a weaker case. Note the no-direct-writes
   rule makes both awkward as SQL — a login PIN is not a domain event either, which
   is an argument for leaving them exactly where they are.
9. **How stale is too stale** for a projection during an event, and what should the
   app do about it — a banner, a log, an alert?
10. **Does the deadletter table live here or in shared infrastructure**, and who
    watches it?
11. **Is retention/purge of the blob store and audit log** (PRDs 003/007) an
    infrastructure job here or a feature job there? A scheduled purge needs a place
    to run.
