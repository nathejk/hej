# 066 — Review the phpMyAdmin exposure now that real personal data lands

**Status:** open
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 008 §8. `docker-compose.yml` exposes phpMyAdmin through Traefik at
`sql.hej.local.nathejk.dk`, with `traefik.enable: "true"`. That was harmless
against an empty database.

Once tasks 050–055 land, that database holds minors' names, addresses, guardian
phone numbers and portrait references. A browsable, unauthenticated-by-default
admin UI in front of it deserves a second look **even in dev** — dev machines get
demoed, screen-shared and occasionally exposed.

## Acceptance Criteria

- [ ] Decide: keep as-is, require auth, bind to loopback only, or drop the service
- [ ] Decision and reasoning recorded in the compose file as a comment
- [ ] If kept, it is not reachable without a deliberate step
- [ ] Confirmed phpMyAdmin exists in no production manifest

## Progress Log

- 2026-08-25 — Task created from PRD 008.
