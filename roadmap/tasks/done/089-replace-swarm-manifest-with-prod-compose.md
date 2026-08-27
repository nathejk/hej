# 089 — Replace the unusable Swarm manifest with a prod compose stack

**Status:** done
**Priority:** high
**Created:** 2026-08-27
**Picked up by:** agent session (Zed)
**Started:** 2026-08-27
**Completed:** 2026-08-27

## Description

`docker stack deploy -c docker-swarm.yml hej` fails on the production manager with:

```
network "jetstream" is declared as external, but could not be found.
You need to create a swarm-scoped network before the stack is deployed
```

This is not a missing network, it is an orchestrator mismatch, and it means the
production deployment of this repo **has never succeeded**. That in turn
invalidates recent device testing: the phone has been running a stale image, so
task 036's bottom-inset fix and its `LayoutDebug` panel were never on the device.

## Acceptance Criteria

- [x] Root cause established from the actual production configuration, not assumed
- [x] A production manifest that works with the org's real deployment model
- [x] Secrets and the portrait bind mount fail loudly when unset
- [x] Every decision the Swarm file documented is preserved or explicitly superseded
- [x] Stale references to `docker-swarm.yml` in live files corrected
- [x] Manifest validated with `docker compose config`

## Progress Log

- 2026-08-27 — Task created after the maintainer reported the deploy failure.
- 2026-08-27 — **Root cause.** `docker stack deploy` only accepts swarm-scoped
  (overlay) networks. The shared networks on the production host are plain
  **bridge** networks created by standalone compose stacks:
  `nathejk/docker-compose.yml` runs Traefik and the NATS broker
  (`nats:2.10-alpine -js`) as ordinary containers and declares `jetstream:` with
  `external: false`, i.e. that stack *creates* it. So the network exists, but is
  invisible to `stack deploy`.
- 2026-08-27 — The bootstrap advice in `docker-swarm.yml`'s header
  (`docker network create --driver overlay jetstream`) could not have worked
  either: it would collide by name with the existing bridge network, and a
  standalone container cannot join an overlay unless it was created
  `--attachable`. Moving hej alone to Swarm was therefore impossible — it would
  have required migrating the broker, Traefik and the `nathejk`, `hq`,
  `tilmelding` and `skan` stacks in one operation, taking the shared broker down.
- 2026-08-27 — Confirmed there is no precedent to preserve: `docker-swarm.yml`
  was the **only** swarm/stack manifest in the entire org (checked all sibling
  repos under `~/Development/nathejk`). Tasks 061 and 062 recorded the same
  finding at the time and proceeded anyway; that was the wrong call, and it went
  unnoticed because neither task could test a deploy.
- 2026-08-27 — Maintainer supplied a **working** production stack (the `foto`
  service) as the model: plain compose, external `traefik` + `prod` networks,
  `prod-`-prefixed Traefik router names, `websecure` + `tls: "true"` +
  `certresolver: letsencrypt`, image pinned to `nathejk/foto:main.3`, state as a
  bind mount under `mounts/prod/<service>`.
- 2026-08-27 — **Decision: option 1, drop Swarm for this repo.** Added
  `docker-compose.prod.yml` modelled on that stack and deleted
  `docker-swarm.yml`. Rejected the alternative (migrate the whole org to Swarm):
  it is a multi-repo change that takes the broker offline, to buy scheduling
  features this stack explicitly refuses to use — it is pinned to one host by a
  bind mount and capped at one replica by its projections.
- 2026-08-27 — Translation of the Swarm-only keys, each preserving the reason the
  original documented:
  * `deploy.replicas: 1` + the long projection/PIN-store rationale → a
    "do not `--scale`" comment. The constraint is unchanged; only its enforcement
    mechanism is gone.
  * `update_config.order: stop-first` → dropped as redundant, with a note:
    `docker compose up -d` already recreates stop-first, which is what the
    two-instance argument requires. Swarm needed it stated because its default
    (`start-first`) would have opened exactly that window on every deploy.
  * `placement.constraints: node.labels.hej.storage == true` → dropped, with a
    note that "deploy hej" now means "deploy hej on that host". With no
    scheduler there is nothing to reschedule the task away from its data, so
    this is one fewer failure mode, not a lost safeguard.
  * `restart_policy` → `restart: unless-stopped` (not `always`: a deliberate
    operator stop should survive a daemon restart).
  * `deploy.resources` limits/reservations → kept as-is; compose honours them.
  * `internal` overlay → the `local` network name the sibling stacks use.
- 2026-08-27 — Changed the `certresolver` from `le` to `letsencrypt` and the
  router/service/middleware names from `hej*` to `prod-hej*`, matching the
  working stack. The old resolver name was invented here and would have produced
  a TLS failure on a deploy that otherwise came up — worth noting as the *second*
  bug this file had, and one that would have been blamed on ACME.
- 2026-08-27 — Made `BLOB_DIR` a required variable rather than defaulting it.
  Docker **silently creates a missing bind source**, so a relative default
  resolved from the wrong working directory would start cleanly against an empty
  portrait directory — indistinguishable from having lost every portrait, which
  is the one piece of state here that cannot be rebuilt from the event log
  (task 063). A hard `:?` failure is the safe behaviour.
- 2026-08-27 — Verified rather than assumed, since a silently-parsing YAML mistake is what
  made the old file look deployable: parsed the file with PyYAML and asserted on the
  **parsed output** (duplicate keys parse silently, last one wins), then ran
  `docker compose -f docker-compose.prod.yml config` three ways —
  with all variables set (valid; image, mount, resolver and limits as intended),
  without `BLOB_DIR` (fails: *required variable BLOB_DIR is missing a value*), and
  without the secrets (fails on `SESSION_SECRET`). `docker compose config` on the dev
  file still passes, so the comment fix there broke nothing.
- 2026-08-27 — Not verified, and cannot be from here: that the deploy itself succeeds. The
  external `prod` network, the broker's hostname on it, the `letsencrypt` resolver name
  and the blob path all live on the production host. This host is not even a swarm
  member (`docker info` → `Swarm: inactive`) and its `traefik`/`jetstream` networks are
  the local dev bridges. **The next deploy is the real test**, and it is the gate on
  task 036's remaining device checks.
- 2026-08-27 — Corrected task 036's log, which had recorded the `100dvh` fix as tried and
  failed. It was never deployed, so it is untested. Same for the missing `LayoutDebug`
  panel. Left the done tasks 061/062/066 untouched — their logs are history, and this
  entry supersedes them rather than rewriting them.
- 2026-08-27 — Open question for the maintainer, deliberately not guessed:
  **`JETSTREAM_DSN`**. It defaults to `nats://jetstream:4222` (the dev value) over the
  external `prod` network, but the broker may answer to a different name there. It is
  overridable per deploy, and an unreachable broker is survivable by design — reads come
  from SQL projections — so a wrong value degrades rather than breaks. It would,
  however, mean the app silently publishes nothing, which is worth getting right before
  PRD 005's check-in ships.
- 2026-08-27 — ✅ All criteria complete. Moving to done.
