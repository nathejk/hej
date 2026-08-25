# 062 — Production swarm changes: state, secrets, volumes

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

PRD 008 §6. `docker-swarm.yml` currently deploys one stateless `hej` service on
the Traefik network. Making the service stateful means it needs a reachable
database (task 061), broker connectivity (PRD 008 §11 Q3), the blob store path or
bucket (task 057), and volumes/secrets to match.

Note the existing comment in that file reasons that HMAC-signed sessions "would
work across replicas" — adding projections changes that reasoning, which is task
064. Coordinate the two.

## Acceptance Criteria

- [x] `DB_DSN`, broker URL and blob-store config present in the swarm service
- [x] Secrets via swarm secrets or `:?`-guarded env vars, following the
      `SESSION_SECRET` precedent — nothing committed
- [x] Volumes declared for any filesystem-backed state
- [x] Replica count consistent with task 064's decision
- [x] Deployment documented well enough to be repeated by someone else

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Picked up. Blocked on task 061 and on whether the Swarm could reach the
  shared broker.
- 2026-08-25 — **Answers received:** `hej` runs its own database (061), the production
  Swarm **can** reach the production JetStream, and portrait storage is a **bind
  mount** rather than object storage. All three unblocked; PRD 008 §11 Q4 is answered
  by the third.
- 2026-08-25 — Implemented on the `hej` service: `DB_DSN` pointing at the new `db`
  service over the stack-private `internal` network, `JETSTREAM_DSN` defaulting to
  `nats://jetstream:4222` over the **external `jetstream`** overlay network (same host
  the siblings use, so one broker and one event log), and `BLOB_PATH=/blobs` backed by
  a bind mount from `/srv/hej/blobs`.
- 2026-08-25 — `jetstream` is declared `external: true`, like `traefik`: the broker is
  an org-level service this stack attaches to and must not redefine (task 053).
- 2026-08-25 — Bind mount chosen per the decision, and the reasoning recorded: the
  bytes are visible on the host for backup (task 063) without going through a
  container. The cost is that it is **node-local**, so the API task is now pinned with
  `node.labels.hej.storage == true` — the same constraint as the database.
- 2026-08-25 — This is the part worth flagging rather than burying in YAML: an
  unpinned task rescheduling onto another node would find an **empty**
  `/srv/hej/blobs` and carry on serving. Projections would rebuild from the log, which
  *looks* like a clean recovery, while every portrait would be silently and
  permanently gone. Portraits are the one thing here that cannot be rebuilt. Stated in
  the file header, not just at the constraint.
- 2026-08-25 — Secrets follow the existing precedent exactly: `${DB_PASSWORD:?...}`
  and `${DB_ROOT_PASSWORD:?...}` with no defaults, so a missing value fails
  `stack deploy` instead of silently deploying something guessable. Nothing committed.
- 2026-08-25 — Replica count unchanged at 1 and consistent with task 064; the database
  is likewise pinned to 1 (two tasks on one volume would be two divergent databases
  behind one name).
- 2026-08-25 — Header rewritten so a deployment can be repeated: the two overlay
  networks to create, the node label to set, the `install -d -m 700` for the blob
  directory, and the full list of variables to export. The `0700` mode matches what
  task 057 enforces in code — these are photographs of minors, and a bind mount means
  the host's permissions are the ones that matter.
- 2026-08-25 — ✅ All criteria complete. Validated by interpolating variables and
  parsing the YAML: both services on the right networks, one replica each, both pinned,
  `stop-first`, DB reachable only internally.
- 2026-08-25 — Verification caveat: **Docker daemon not responding**, so no
  `stack deploy` was attempted and the bootstrap steps are unrehearsed. The bind mount
  source directory does not exist on any node yet — the first deploy will fail if the
  header's `install -d` step is skipped, which is deliberate: failing is better than
  Docker helpfully creating a root-owned empty directory.
- 2026-08-25 — Moving to done.

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Picked up. **Blocked on task 061** (production database route) for the
  `DB_DSN` half, and on an infrastructure question for the broker half.
- 2026-08-25 — Partially resolvable, partially not:
  - **`DB_DSN`** — blocked: the value depends entirely on 061's answer (own service
    vs managed instance).
  - **Broker** — blocked on a question outside this repo. Task 053 established that
    the broker is the shared org service on an **external `jetstream` network**
    created by the `nathejk` repo's compose stack. Whether that network exists in the
    Swarm cluster, and whether reaching it needs an overlay network or credentials,
    cannot be determined from this repo. No sibling has a swarm manifest to copy.
  - **Blob store** — this part *could* be done now (`BLOB_PATH` plus a volume), but
    it is not worth a deploy on its own, and PRD 008 §11 Q4 leaves the production
    choice (volume vs object storage) open anyway. Held with the rest so production
    config lands as one coherent change rather than three partial ones.
  - **Secrets pattern** — already settled and needs no decision: follow the existing
    `${VAR:?msg}` convention in the file, which fails `stack deploy` loudly on an
    unset value. Whatever 061 decides, the credential is injected that way.
- 2026-08-25 — Done as groundwork rather than left implicit: `docker-swarm.yml` now
  documents that the API has a persistent store, that none of it is configured there,
  and that a task deployed from the file runs in its degraded mode (mock read models,
  no broker, in-memory blobs). Task 064 also landed the replica constraint and fixed
  `update_config.order` to `stop-first` in the same file.
- 2026-08-25 — **Questions for the maintainer:** (a) 061's answer; (b) does the Swarm
  cluster have access to the shared `jetstream` network, and does it need credentials?
  (c) volume or object storage for portraits in production (PRD 008 §11 Q4)?
