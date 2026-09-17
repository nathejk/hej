# 310 — Glimt retention purge, configurable, 0 disables

**Status:** open
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §6, §8. Mirror `go/cmd/api/portraitpurge.go`, which already solves this shape.

Two env vars, both `time.Duration`, registered like `portraitRetention` in
`go/cmd/api/env.go` (`flag.DurationVar` + `envDuration`):

| var | controls |
|---|---|
| `GLIMT_RETENTION` | how long a glimt is kept after creation |
| `GLIMT_PUBLIC_RETENTION` | how long it stays on the public feed |

**`0` disables the purge entirely** — required for dev and testing, where a fixture posted
last month must still be there tomorrow. Guard with `<= 0` so a negative value also means
off, and **log it at startup** as `portraitpurge.go:44` does: "off for dev" and "off because
someone fat-fingered the env in prod" look identical otherwise, and this is the one config
value whose failure mode is keeping children's photos forever.

Measured **from creation**, not from a configured event end date — the same simplification
`portraitRetention` makes, and it fails in the safe direction.

The effective value is exposed on `/api/config` (which already carries
`contacts_poll_seconds`) so the composer and `PrivacyView` can state the real number instead
of a hard-coded one.

## Acceptance Criteria

- [ ] `glimtRetention` and `glimtPublicRetention` on `config`, via flag + env, documented in `env.go`
- [ ] Sweep purges glimt older than the window, publishing `.purged`
- [ ] `0` (and negative) disables the sweep and logs that it is disabled
- [ ] Public retention can be shorter than the internal one and is applied independently
- [ ] Effective retention days served on `/api/config`
- [ ] Tests: purges past the cutoff, keeps inside it, does nothing when disabled
- [ ] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
