# 036 — Final verification of the shadcn-vue / Tailwind v4 migration

**Status:** open
**Priority:** high
**Created:** 2026-08-24
**Picked up by:**
**Started:**
**Completed:**

## Description

Closing verification for PRD 004. Because this repo has no automated test suite,
this task is the safety net for a change that touched the styling foundation and
the component library at once.

Steps:

1. `docker compose build ui` (lockfile churned across 024/027/030) and bring the
   stack up.
2. `npm run type-check` and `npm run build` in the `ui` container.
3. Click through every route on a phone viewport: `/login`, `/maps`, `/contacts`,
   `/rulebook`, `/updates`, `/schedule`, `/faq`, `/sos` — plus the `MoreMenu`
   overflow sheet, both permission pre-prompts and the update prompt. Console must
   be clean, including of "failed to resolve component" warnings.
4. Sign in end to end against the BFF (phone → PIN → session) to confirm the
   login flow still works.
5. **Verify the service-worker update flow.** The Tailwind rewrite changes every
   asset hash, so the first deploy after this lands is exactly the scenario
   `UpdatePrompt.vue` exists for: an open session must detect the new build and
   one tap on "Reload" must land on it. Do not assume this — exercise it.
6. Record before/after bundle sizes (JS + CSS) from the build output in the
   progress log. Removing PrimeVue should make it smaller; PRD 004 requires it not
   to grow.
7. Confirm the docs/code gap is closed: `.rules` and `vue3-pwa-layout` now
   describe the code as it actually is.

When this is done, all PRD 004 tasks are complete and the PRD moves
`doing/` → `done/`.

PRD: 004. Depends on: 024, 025, 026, 027, 028, 029, 030, 031, 032.

## Acceptance Criteria

- [ ] `docker compose build ui` succeeds; `type-check` and `build` pass.
- [ ] All eight routes plus the overflow sheet, pre-prompts and update prompt
      render with a clean console on a phone viewport.
- [ ] The login flow completes end to end against the BFF.
- [ ] The service-worker update prompt is verified against a new build.
- [ ] Before/after bundle sizes are recorded and the payload has not grown.
- [ ] `grep -ri primevue vue/src vue/*.ts vue/*.json` (excluding the lockfile) is
      empty, and no `tailwind.config.js` remains.
- [ ] PRD 004 is moved to `roadmap/prd/done/` with `Status: done` and a `Shipped`
      date.

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004.
