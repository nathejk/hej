# 048 — api-dev hot-reload leaves a stale server process

**Status:** done
**Priority:** medium
**Created:** 2026-08-24
**Picked up by:** agent
**Started:** 2026-08-26
**Completed:** 2026-08-26

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

- [x] Editing a `.go` file results in requests hitting the **new** code with no
      manual `docker compose restart api`.
- [x] No orphaned process is left holding `:4000` after a reload.
- [x] The reload still runs the existing gates (test / vet / staticcheck / build)
      and still refuses to start on failure.
- [x] `docker/init/api-dev` explains the chosen approach in a comment.

## Progress Log

- 2026-08-24 12:38 — Task created from a sub-agent's finding while implementing
  `GET /api/patrol/scans`: the endpoint 404'd against a stale binary and only a
  container restart fixed it.
- 2026-08-26 — Done. Option 2, as suggested. The original diagnosis was exactly right
  and is now confirmed from the live container:

  ```
  PID  PPID COMMAND
    1     0 /bin/sh /usr/local/bin/api-dev
 1174     1 go run nathejk.dk/cmd/api          <- $! pointed here
 1208  1174 /go/.cache/go-build/.../api        <- the actual server
  ```

  `$!` was the wrapper (1174); the server was its child (1208). Killing the wrapper
  orphaned the server, which kept `:4000`, so the next process could not bind and
  requests kept reaching the old code while the log said `==> starting api`.

  The loop now runs `go build -o /tmp/api-dev-server nathejk.dk/cmd/api` and execs that
  directly, so `$!` *is* the server and a plain `kill` works. The tree is flat:

  ```
    1     0 /bin/sh /usr/local/bin/api-dev
 1810     1 /tmp/api-dev-server
  ```

  `GO_BUILD_FLAGS` (e.g. `-race`) moved onto the build, where it belongs.

  **A detail that would have broken the safety net.** I added a `port_held()` check as a
  belt-and-braces guard, reading `/proc/net` because this image has no `ss`, `netstat`,
  `lsof` or `fuser`. My first version read only `/proc/net/tcp` — and found nothing, while
  the server was demonstrably running. Go's `net.Listen("tcp", ":4000")` binds a
  **dual-stack IPv6** socket, so the listener appears in `/proc/net/tcp6` and not in
  `tcp`. A check that only looked at `tcp` would have reported "port free" every time:
  the same silent-success failure this task exists to remove. Both files are now read,
  and the awk expression was verified positively (server up → held) and negatively (port
  9999 → free) before being relied on.

  `start()` now also **refuses to start when the port is held**, printing an explicit
  error instead of launching a process that cannot bind. That turns the original
  expensive symptom — a reload that silently did nothing — into a message that names the
  problem.

  ### Verified end to end

  | check | result |
  |---|---|
  | new route added, **no** manual restart | `hot-reload-ok`, 200 |
  | route removed again | 404 |
  | orphan processes after two reloads | none; one server, PID replaced each time |
  | `go run` wrapper present | no |
  | deliberate syntax error | `==> build failed; waiting for changes`, port **not** held |
  | after fixing the error | healthcheck 200, one server process |

  **One behaviour change worth knowing about.** When the gates fail, the API is now
  genuinely *down* (Traefik returns 500) rather than answering from a stale binary. That
  is the same `stop`-then-`start` sequence as before — the difference is that `stop` now
  works. It is the better failure: previously a compile error left old code serving,
  which is indistinguishable from success until something subtle is wrong.

  This bug interfered with verification twice during PRD 006 (tasks 074 and 078), in task
  078's case risking checking new behaviour against old code. That is what moved it up the
  list.

  `docker/init/ui-dev` was checked for the same flaw and does not have it: it runs
  `npm run dev` in the foreground and leaves reloading to Vite's own HMR, with no PID
  tracking to get wrong.

  Note the entrypoint is `COPY`'d into the image rather than mounted, so this fix needs
  `docker compose build api` — a plain `up -d` will keep running the old script.

## Files

- `docker/init/api-dev` — build-and-exec instead of `go run`; `port_held()` guard over
  both `/proc/net/tcp` and `tcp6`; refuse to start on a held port; the reasoning recorded
  in a header comment
