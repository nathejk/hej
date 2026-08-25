# 062 — Production swarm changes: state, secrets, volumes

**Status:** doing
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:**

## Description

PRD 008 §6. `docker-swarm.yml` currently deploys one stateless `hej` service on
the Traefik network. Making the service stateful means it needs a reachable
database (task 061), broker connectivity (PRD 008 §11 Q3), the blob store path or
bucket (task 057), and volumes/secrets to match.

Note the existing comment in that file reasons that HMAC-signed sessions "would
work across replicas" — adding projections changes that reasoning, which is task
064. Coordinate the two.

## Acceptance Criteria

- [ ] `DB_DSN`, broker URL and blob-store config present in the swarm service
- [ ] Secrets via swarm secrets or `:?`-guarded env vars, following the
      `SESSION_SECRET` precedent — nothing committed
- [ ] Volumes declared for any filesystem-backed state
- [ ] Replica count consistent with task 064's decision
- [ ] Deployment documented well enough to be repeated by someone else

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
