# 428 — `docker build` fails: a test reads the app's tree, which the build stage does not have

**Status:** done
**Priority:** high
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

> *"docker build fails"*

Reproduced with `docker build -f docker/Dockerfile --target build .`:

```
--- FAIL: TestThePublicSiteUsesTheAppsFavicon (0.00s)
    publicsite_test.go:921: reading the app's index.html: open ../../../vue/index.html: no such file or directory
```

Self-inflicted, in task 426. That guard reads `vue/index.html` to check the public site declares the *same*
favicon the app does — a good property, enforced the right way, and the only test in the Go suite that reads a
**sibling tree**.

The Dockerfile's `build` stage is `FROM base`, and `base` copies `go/mod`, `go/sum` and `go/` — nothing else. That
split is the point of the multistage layout: the Go image has no reason to carry the frontend. So `go test ./...`
there found no `vue/` and failed the whole build over a favicon.

Worth noting what did *not* catch it: `go test ./...`, `go vet ./...` and `gofmt` all pass on a developer's
checkout, because there the sibling tree is right where the test expects. Nothing local can see this — only the
build context can, which is why the maintainer found it and I did not.

## The fix, and why it is not "skip if the file is missing"

The obvious repair is to skip when the file is absent, and that would have thrown the guard away: it would also
pass on the day somebody moved or deleted the app's shell, which is precisely what it exists to notice.

So the distinction is **the directory, not the file**:

| Condition | Behaviour |
|---|---|
| no `vue/` at all | skip — a Go-only build context, nothing to compare against |
| `vue/` present, no `index.html` in it | **fail** — somebody moved or deleted the app's shell |

Both branches were verified: the build stage now passes, and renaming `vue/index.html` on a local checkout fails
with *"the app's tree is here but its index.html is not"*.

## Acceptance Criteria

- [x] `docker build --target build` passes
- [x] `docker build --target prod` passes end to end
- [x] The guard still fails when the app's favicon moves
- [x] The guard still fails when the app's `index.html` is deleted from a real checkout
- [x] It skips only where the sibling tree genuinely is not present

## Progress Log

- 2026-09-25 — Reproduced in Docker rather than guessed at, which named the failing test on the first run.
- 2026-09-25 — Fixed as above. Both `--target build` and `--target prod` now build; the prod image was built end
  to end, including `npm ci`, the frontend build and `check-no-dev-layer.sh`.
- 2026-09-25 — Checked whether anything else reads outside the Go tree: nothing does. The other `../../` hits in
  tests are path-traversal *inputs* to ref validators, not file reads. So the convention this broke is real and
  worth stating: **a Go test may read the Go tree — `adminSource` and `viewerAsset` do — but not a sibling tree,
  unless it handles the tree being absent.**
- 2026-09-25 — The wider lesson, which is mine: a guard that reaches outside its own module buys a property the
  module cannot otherwise check, and pays for it with a context where it cannot run. That trade can be worth
  making — this one is — but the absent case has to be designed at the same time, not discovered by a failed
  release build.
- 2026-09-25 — ✅ All criteria met. `gofmt`, `go vet`, `go test ./...` and both Docker targets clean.
