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

## Acceptance Criteria

- [ ] A script (committed, runnable locally) that builds and scans `vue/dist`
- [ ] Fails on any dev-layer string; passes on the current tree
- [ ] Wired into CI alongside the existing build
- [ ] A deliberate temporary regression makes it fail (verified once, then reverted)
- [ ] The Go side's equivalent guarantee is already covered by task 215's
      route-absence test — reference it rather than duplicating it

## Depends on

- All of phases 1–4, since it scans for their strings.

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 5.
