# 053 — Connect to the shared JetStream broker (dev)

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

**Retitled and rescoped during implementation — the original premise was wrong.**

As created, this task said: "There is no broker in `hej`'s dev stack. Add a `nats`
service with JetStream enabled and a persistent volume." That would have been a
mistake. See the progress log.

What it actually needs to do: attach the `api` service to the **shared, org-level
`jetstream` network** and give it `JETSTREAM_DSN`, matching every other consumer
repo. `hej` runs no broker of its own.

## Acceptance Criteria

- [x] ~~`nats` service with JetStream enabled and a named volume~~ — **withdrawn**,
      see log. `hej` must not run its own broker.
- [x] `jetstream` declared as an **external** network and joined by the `api` service
- [x] `JETSTREAM_DSN` in the `api` environment with the org-standard dev value
- [x] Config plumbed into the Go `config` struct via the existing env pattern
- [x] Comment explaining who owns the broker and pointing at task 062 for production

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Picked up. Plan: mirror `hq`'s broker setup rather than invent one.
- 2026-08-25 — **Premise was wrong — stopped and rescoped.** `hq` has no `nats`
  service either. Checking the sibling repos: the **`nathejk` repo owns the broker**
  (`nats:2.10-alpine`, `command: -js`, volume, ports bound to loopback) on a
  `jetstream` network it declares with `external: false, name: jetstream`. Every
  consumer — `hq`, `tilmelding`, `skan` — declares that network `external: true` and
  connects with `JETSTREAM_DSN: nats://jetstream:4222`.
- 2026-08-25 — Why this matters rather than being a tidiness point: a private broker
  in `hej` would be a **split brain**. Events `hej` publishes (PRD 005's
  verification flag) would land in a broker `hq` never reads, so check-in would never
  see them — and the failure would be *silent*, because publishing succeeds and
  nothing consumes. That is the exact goal PRD 005 justifies itself with, so getting
  this wrong would have quietly invalidated a feature.
- 2026-08-25 — Implemented instead: `jetstream` as an external network, `api` joined
  to `local` + `jetstream`, `JETSTREAM_DSN: nats://jetstream:4222`, and a
  `jetstreamDSN` field in the `config` struct read via the existing `envStr` pattern.
  Env var name matches the siblings so operators do not learn a second one.
- 2026-08-25 — Empty/unreachable DSN is deliberately non-fatal, consistent with the
  database decision in task 050. Task 058 covers the full degraded-mode behaviour.
- 2026-08-25 — Updated PRD 008: the §6 requirement rewritten, a new §8 subsection
  "The broker is shared, not ours" added, and **§11 Q3 answered** (which broker `hej`
  talks to). What remains open there is production reachability, which is task 062.
- 2026-08-25 — Renamed the file to `053-jetstream-connection-dev.md` to match the
  corrected scope; ID unchanged per the board rules.
- 2026-08-25 — ✅ Criteria met (one withdrawn with reasoning). YAML parses with the
  expected resolved values; `go build` and `go vet` green.
- 2026-08-25 — Verification caveat: **Docker daemon not responding**, so the stack
  was not started. **Not** verified: that the external `jetstream` network exists on
  this machine and that `api` can actually reach `nats://jetstream:4222`. Anyone
  running the stack needs the `nathejk` repo's broker up first — worth noting in dev
  docs if it is not already.
- 2026-08-25 — Moving to done.
