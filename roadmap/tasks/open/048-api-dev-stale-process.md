# 048 — api-dev hot-reload leaves a stale server process

**Status:** open
**Priority:** medium
**Created:** 2026-08-24
**Picked up by:**
**Started:**
**Completed:**

## Description

Found while verifying task 044: after editing a `.go` file, the `api` container
appeared to reload (the gates re-ran and it logged `==> starting api`) but requests
still hit the **old binary** — a brand new route returned 404 until
`docker compose restart api`.

Likely cause: `docker/init/api-dev` starts the server with `go run` and records
`$!`, which is the PID of the `go run` **wrapper**. `go run` compiles to a temp
binary and execs it as a *child*; killing the wrapper does not necessarily kill
that child, so the old server keeps holding `:4000` while the new one fails to bind
(or binds and is never reached).

This is a dev-experience trap that costs real debugging time — the logs say the API
restarted, and it did not.

Suggested fixes (pick one):

- Start the process group and kill the group (`set -m`, then `kill -- -$PID`), or
- `go build -o /tmp/api` and run the binary directly so `$!` is the real server, or
- use `exec` semantics / a supervisor that tracks the child.

Option 2 is probably clearest and also makes startup marginally faster, since the
build already happens in the gates.

Not part of PRD 002 — recorded separately because it affects every future BFF task.

## Acceptance Criteria

- [ ] Editing a `.go` file results in requests hitting the **new** code with no
      manual `docker compose restart api`.
- [ ] No orphaned process is left holding `:4000` after a reload.
- [ ] The reload still runs the existing gates (test / vet / staticcheck / build)
      and still refuses to start on failure.
- [ ] `docker/init/api-dev` explains the chosen approach in a comment.

## Progress Log

- 2026-08-24 12:38 — Task created from a sub-agent's finding while implementing
  `GET /api/patrol/scans`: the endpoint 404'd against a stale binary and only a
  container restart fixed it.
