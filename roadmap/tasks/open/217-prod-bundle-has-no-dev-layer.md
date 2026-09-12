# 217 — CI check: no dev-layer strings in a production bundle

**Status:** open
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] A script (committed, runnable locally) that builds and scans `vue/dist`
- [ ] Fails on any dev-layer string; passes on the current tree
- [ ] Wired into CI alongside the existing build
- [ ] A deliberate temporary regression makes it fail (verified once, then reverted)
- [ ] Also asserts that **no chunk** is emitted for `src/dev/`
- [ ] The Go side's equivalent guarantee is already covered by task 215's
      route-absence test — reference it rather than duplicating it

## Depends on

- All of phases 1–4, since it scans for their strings.

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 5.
