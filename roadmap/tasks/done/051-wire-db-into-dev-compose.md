# 051 — Wire the existing db service into the api service (dev compose)

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

PRD 008 §2. `docker-compose.yml` already provisions MariaDB 10.8 as the `db`
service (database/user/password `hej`, named `db` volume) plus phpMyAdmin — but
the `api` service has no DSN and nothing uses it. Wire them together.

This is dev only; production is task 061/062.

## Acceptance Criteria

- [x] `DB_DSN` added to the `api` service environment with a working dev default
      pointing at the `db` service
- [x] `api` gains `depends_on: [db]`
- [x] Comment explains the value follows the same override convention as
      `SESSION_SECRET` (real values in `docker-compose.override.yml`)
- [x] `docker compose config` parses (or, if the Docker daemon is unavailable,
      the YAML is validated and the limitation logged)

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Picked up. Plan: add `DB_DSN` + `depends_on` to the `api` service,
  matching the credentials already on the `db` service.
- 2026-08-25 — DSN is `hej:hej@tcp(db:3306)/hej?parseTime=true`. Credentials read
  from the existing `MARIADB_USER`/`PASSWORD`/`DATABASE` values rather than invented.
  `parseTime=true` included deliberately: without it `DATETIME` columns scan into
  `[]byte` instead of `time.Time`, which every projection with a timestamp would
  then have to work around.
- 2026-08-25 — Note on `depends_on`: it waits for the *container*, not for MariaDB
  inside it to accept connections. That gap is what task 050's retrying ping covers,
  so no healthcheck condition is added here.
- 2026-08-25 — ✅ All criteria complete.
- 2026-08-25 — Verification caveat: the **Docker daemon is not responding on this
  machine**, so `docker compose config` could not be run. Validated the file by
  parsing it with PyYAML instead and asserting the resolved values: `DB_DSN` present
  on `api`, `depends_on: ['db']`, credentials matching the `db` service, and the
  `db` volume still declared. **Not** verified: that the API actually connects when
  the stack comes up.
- 2026-08-25 — Moving to done.
