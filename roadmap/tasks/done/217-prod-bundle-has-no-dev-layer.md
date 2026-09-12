# 217 — CI check: no dev-layer strings in a production bundle

**Status:** done
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

PRD 014, phase 5. Every part of the dev layer is guarded by
`import.meta.env.PROD`, which Vite compiles out — but that is a *claim about the
build*, and the whole safety argument for PRD 014 rests on it. So assert it against
the real artefact.

The layer can fabricate a "mobile, installed" identity. If a future refactor moves a
guard behind a runtime condition Vite cannot fold away, nothing else in the repo
would notice, and the failure would be silent in exactly the direction that matters.

## Approach

Build, then scan `vue/dist` for the dev layer's fingerprints: `hej.dev.`, the
`?dev=` preset names as a set, the panel's component name, the dev PIN path. Fail the
build if any appear. Keep the string list in one place so adding a dev feature
without extending the check is hard to do by accident.

Add it to the existing GitHub Actions build (`.github/workflows/`), so it runs where
production images are actually produced.

## This is not belt-and-braces — the regression already happened

Updated 2026-09-12, during task 208. The first implementation of the dev layer guarded every
*use* with `import.meta.env`, which is what the PRD asked for, and the production bundle still
contained the simulation's fake user-agent strings and the panel's copy — found by grepping
`dist/assets/index-*.js`. `import.meta.env` guards make code unreachable; they do not remove
code that something still statically imports.

That was fixed by moving the layer to `src/dev/` behind a dynamic import, but the fix is a
*convention* ("nothing in `src/dev/` may be imported statically from product code") and
conventions decay silently. One careless `import { readDevDevice } from '@/dev/devDevice'` in a
product module restores the leak with no test failure and no visible symptom. This scan is the
only thing that would catch it.

## Acceptance Criteria

- [x] A script (committed, runnable locally) that builds and scans `vue/dist`
      — `vue/scripts/check-no-dev-layer.sh`, following the convention of the existing
      `check-install-gate.sh`
- [x] Fails on any dev-layer string; passes on the current tree
- [x] Wired into the **image build** (`docker/Dockerfile`, `ui-builder` stage) rather than a
      separate CI step — see the log for why that is the better place
- [x] A deliberate temporary regression makes it fail (verified, then reverted)
- [x] Also asserts that **no chunk** is emitted for `src/dev/`
- [x] The Go side's equivalent guarantee is referenced, not duplicated

## Depends on

- All of phases 1–4, since it scans for their strings.

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 5.
- 2026-09-12 10:05 — Picked up. Script written in the style of `check-install-gate.sh`: fingerprint
  list in one place, `ok`/`FAIL` per line, non-zero exit with a pointer to the invariant.
- 2026-09-12 10:06 — Wired into **`docker/Dockerfile`'s `ui-builder` stage**, not a GitHub Actions
  step. The workflow only builds and pushes an image, so the Dockerfile *is* where production
  artefacts are produced — putting the gate there means it also fires for a local `docker build`
  and cannot be bypassed by someone building the image outside CI.
- 2026-09-12 10:07 — 🐛 Fixed a bug in my own first version: `for pattern in $(echo "$FINGERPRINTS")`
  word-split the multi-word fingerprints, so the check "passed" while grepping fragments like
  `Android` and `14;`. Rewritten as `while IFS= read -r` over a heredoc. A check that passes for the
  wrong reason is worse than no check, and this one nearly shipped that way.
- 2026-09-12 10:09 — ✅ **Negative test done properly, and the first attempt was too weak.** I added
  an unused `import { DEV_DEVICE_KEY }` to `config/gates.ts` and the check still passed — correctly,
  because Rollup tree-shakes an unused import, so nothing leaked. Only a *used* import leaks. Redid
  it with the value actually referenced inside `initGateOverride`, and the check failed as intended:
  `FAIL present in the bundle: hej.dev.` naming `dist/assets/index-*.js`, exit 1. Reverted.
  Worth recording, because it means the realistic leak is precisely the dangerous one.
- 2026-09-12 10:11 — ✅ `docker build --target ui-builder` runs the gate and passes on the current
  tree.
