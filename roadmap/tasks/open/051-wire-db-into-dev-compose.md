# 051 — Wire the existing db service into the api service (dev compose)

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 008 §2. `docker-compose.yml` already provisions MariaDB 10.8 as the `db`
service (database/user/password `hej`, named `db` volume) plus phpMyAdmin — but
the `api` service has no DSN and nothing uses it. Wire them together.

This is dev only; production is task 061/062.

## Acceptance Criteria

- [ ] `DB_DSN` added to the `api` service environment with a working dev default
      pointing at the `db` service
- [ ] `api` gains `depends_on: [db]`
- [ ] Comment explains the value follows the same override convention as
      `SESSION_SECRET` (real values in `docker-compose.override.yml`)
- [ ] `docker compose config` parses (or, if the Docker daemon is unavailable,
      the YAML is validated and the limitation logged)

## Progress Log

- 2026-08-25 — Task created from PRD 008.
