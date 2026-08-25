# 066 — Review the phpMyAdmin exposure now that real personal data lands

**Status:** done
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

PRD 008 §8. `docker-compose.yml` exposes phpMyAdmin through Traefik at
`sql.hej.local.nathejk.dk`, with `traefik.enable: "true"`. That was harmless
against an empty database.

Once tasks 050–055 land, that database holds minors' names, addresses, guardian
phone numbers and portrait references. A browsable, unauthenticated-by-default
admin UI in front of it deserves a second look **even in dev** — dev machines get
demoed, screen-shared and occasionally exposed.

## Acceptance Criteria

- [x] Decide: keep as-is, require auth, bind to loopback only, or drop the service
- [x] Decision and reasoning recorded in the compose file as a comment
- [x] If kept, it is not reachable without a deliberate step
- [x] Confirmed phpMyAdmin exists in no production manifest

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Picked up. Previous state: `traefik.enable: "true"` with
  `Host(sql.hej.local.nathejk.dk)`, on the shared external `traefik` network.
- 2026-08-25 — **Decision: keep the tool, remove the ambient reachability.** Published
  on loopback only (`127.0.0.1:8081:80`), `traefik.enable: "false"`, off the
  `traefik` network. Rejected the alternatives: dropping it entirely costs a genuinely
  useful dev tool for no extra safety over loopback, and "require auth" means
  inventing and storing a credential for a dev convenience.
- 2026-08-25 — Reasoning recorded in the compose file, in terms of what actually
  changed: this was harmless against an empty database, but from PRD 008 onwards the
  database holds members' names, addresses, guardian phone numbers and portrait
  references — personal data about minors — with an unauthenticated
  browse-everything UI in front of it. A shared Traefik with `exposedByDefault`, a
  demo, a screen-share, or a laptop on a conference network is then one hostname away
  from disclosing all of it.
- 2026-08-25 — Checked the production claim rather than asserting it: `grep` found
  `phpmyadmin` in `docker-swarm.yml`, but only inside a comment — there is no such
  service. Claim holds, and the comment now says so precisely.
- 2026-08-25 — Found a related stale comment while verifying: `docker-swarm.yml` said
  "There is no `db` service on purpose: the API currently serves mock data … nothing
  behind them yet". That became false with tasks 050-057. Rewrote it to state the
  real situation — the API now has a persistent store, none of it is configured in
  production, so a task deployed from that file runs degraded — and to point at the
  two open decisions (tasks 061, 062) that block fixing it. Left the fix itself to
  those tasks rather than widening this one.
- 2026-08-25 — ✅ All criteria complete. YAML parsed and asserted: loopback-only port,
  `local` network only, `traefik.enable: "false"`, and no `sql.hej` route remaining.
- 2026-08-25 — Verification caveat: **Docker daemon not responding**, so it was not
  confirmed by actually trying to reach the old hostname and failing.
- 2026-08-25 — Moving to done.
