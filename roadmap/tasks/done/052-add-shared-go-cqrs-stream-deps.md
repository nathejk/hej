# 052 — Add shared-go, cqrs and stream dependencies

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

PRD 008 §8. Add the three external modules the rest of the PRD needs:

- `github.com/nathejk/shared-go` — shared domain types and messages
- `github.com/jrgensen/cqrs` — the `Publisher`/`Writer`/`Reader` seam
- `github.com/jrgensen/stream` — jetstream/xstream/subject

Per `go-bff-layout`: in dev, `go.work` resolves shared-go from the sibling
`../../shared-go` checkout so edits are picked up live; CI/prod build with
`GOWORK=off` against the version pinned in `go.mod`. Both must work — a change
that only builds with the workspace active is a broken build.

`cqrs` and `stream` are NOT in the workspace; they resolve from the proxy at the
pinned version in every environment.

## Acceptance Criteria

- [x] shared-go, cqrs and stream in `go.mod` at explicit versions
- [x] `go/go.work` resolving `../../shared-go`, and `go.work.sum` handled
- [x] `go.work` is gitignored or committed deliberately — decide and document
- [x] `GOWORK=off go build ./...` green (proves the CI/prod path)
- [x] `go build ./...` green with the workspace active
- [x] `go test ./...` and `go vet ./...` green

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Picked up. Plan: match `hq`'s wiring rather than invent one, per
  PRD 008's "deviating from hq's wiring needs a reason".
- 2026-08-25 — Versions pinned to exactly what `hq/go/go.mod` uses:
  `cqrs v0.1.0`, `stream v0.1.2`,
  `shared-go v0.0.0-20260815075712-35c10e0f6942`. A newer shared-go commit exists
  locally (`1d06fcd`), but deliberately **not** pinned: keeping the two repos on one
  version means a shared-go change is exercised by both, and PRD 006 will bump it
  when it needs a new field.
- 2026-08-25 — Decision: `go.work` + `go.work.sum` are **committed**, matching `hq`
  (both are in its `git ls-files`). The workspace is developer-shared here, not
  developer-local, because every developer has the same sibling-checkout layout.
- 2026-08-25 — Hit the first real complication: `go.work` with `go 1.25.0` (hq's
  value) is rejected because this repo's `go.mod` requires `go >= 1.25.8`. Set the
  workspace to `go 1.25.8`.
- 2026-08-25 — Second and more important complication: the Dockerfile does
  `COPY go/ ./`, so `go.work` lands **inside the image**, where `../../shared-go`
  does not exist — every image build would have failed. Fixed the way `hq` does:
  `ENV GOWORK=off` in the `build` stage. That stage runs the test + staticcheck
  gates, so it doubles as the CI check that nothing depends on the workspace.
- 2026-08-25 — Third: with `go.work` active the dev container also needs the
  sibling checkout. Added `../shared-go:/shared-go` to the `api` service volumes —
  `../../shared-go` relative to `/app` resolves to `/shared-go`, which is exactly the
  path `hq` mounts and what `go-bff-layout` documents.
- 2026-08-25 — Caveat worth recording: all three modules are currently marked
  `// indirect` because **nothing imports them yet**. `go mod tidy` before task 054
  lands would prune them. Task 054 makes them direct.
- 2026-08-25 — ✅ All criteria complete. Verified both paths: workspace build green,
  and `GOWORK=off` build + test + staticcheck green (simulating the image, where
  `go.work` is present but the sibling checkout is not).
- 2026-08-25 — Verification caveat: the **Docker daemon is not responding**, so the
  image was not actually built. `GOWORK=off` was simulated on the host, which
  exercises the same resolution path the `build` stage uses. **Not** verified: the
  `/shared-go` bind mount resolving inside a running container.
- 2026-08-25 — Moving to done.
