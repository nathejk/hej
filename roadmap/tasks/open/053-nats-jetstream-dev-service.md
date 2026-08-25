# 053 — Add a JetStream-enabled nats service to dev compose

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 008 §6. There is no broker in `hej`'s dev stack. Add a `nats` service with
JetStream enabled and a persistent volume, on the internal `local` network, kept
off Traefik (`traefik.enable: "false"`) like `db` and `api`.

Add the connection URL to the `api` service environment. Which broker production
talks to is task 062 and PRD 008 §11 Q3.

## Acceptance Criteria

- [ ] `nats` service with JetStream enabled (`-js`) and a named volume for its
      store directory
- [ ] On the `local` network, `traefik.enable: "false"`
- [ ] `NATS_URL` (or equivalent) in the `api` environment with a dev default
- [ ] Volume declared in the top-level `volumes:` block
- [ ] Comment explaining dev-only scope and pointing at task 062 for production

## Progress Log

- 2026-08-25 — Task created from PRD 008.
