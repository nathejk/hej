# 062 — Production swarm changes: state, secrets, volumes

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
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
