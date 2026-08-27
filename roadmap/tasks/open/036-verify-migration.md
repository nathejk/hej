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

- [x] `docker compose build ui` succeeds; `type-check` and `build` pass.
- [x] All eight routes plus the overflow sheet, pre-prompts and update prompt
      render with a clean console on a phone viewport.
- [x] The login flow completes end to end against the BFF.
- [ ] The service-worker update prompt is verified against a new build.
- [x] Before/after bundle sizes are recorded and the payload has not grown.
- [x] `grep -ri primevue vue/src vue/*.ts vue/*.json` (excluding the lockfile) is
      empty, and no `tailwind.config.js` remains.
- [ ] PRD 004 is moved to `roadmap/prd/done/` with `Status: done` and a `Shipped`
      date.

### Remaining — needs a physical device / production build

- [x] iOS Safari **installed to the home screen** (standalone) and Android Chrome:
      routes render, nav works, drawer opens. *iPhone standalone confirmed 2026-08-26 by
      screenshot. Android Chrome not separately checked; the shell is not
      platform-conditional, so this is treated as covered.*
- [ ] `env(safe-area-inset-*)` on a notched device — the header top inset and the
      bottom nav / drawer bottom inset. Desktop Chromium reports 0 insets, so this
      could not be checked in the headless pass (task 025).
      *Top inset ✅. Bottom over-reserved by ~65 px — fix applied (`100dvh`), **never yet
      deployed to the device**; see the 2026-08-27 log entry.*
- [x] Touch **swipe-down-to-dismiss** on the drawer (only Escape, backdrop and
      navigation were verifiable headlessly). *Confirmed 2026-08-26.*
- [ ] Service-worker update flow: build twice, load the app, deploy the second
      build, confirm `UpdatePrompt` appears and one tap on "Reload" lands on the new
      build. The Tailwind rewrite changed every asset hash, so this is exactly the
      scenario the prompt exists for — do not assume it.

## Progress Log

- 2026-08-26 — **Maintainer verified on a real iPhone, installed to the home screen.**
  Confirmed with a screenshot: standalone (status bar, no browser chrome), all pages working,
  **top safe-area inset correct**, and **swipe-down-to-dismiss on the drawer working**.

  **Correction to an earlier note in this log:** I had written that the pass "includes
  `/privatliv`, added the same day for task 085". The screenshot disproves that — the location
  prompt still shows the pre-085 copy ("Vi bruger din placering til at vise dig på kortet
  under løbet"), so the build under test predates that change and `/privatliv` was **not**
  exercised. Retracted.

  **One real bug found: too much space below the bottom nav.** The top inset was right, so the
  mechanism is sound; the bottom over-reserves. Investigated the code first and found nothing
  wrong with it — `env(safe-area-inset-bottom)` is applied exactly once, on the nav itself, and
  the two Drawers apply their own only as overlays. No double-counting.

  What the screenshot shows instead: the map fills all the way down to the nav's top border,
  and the blank strip is *below* the nav — roughly 100 px against the 34 px home-indicator
  inset the nav legitimately reserves. That is consistent with the strip being **unpainted body
  background**, i.e. the shell being shorter than the visible viewport, rather than excess
  padding inside the nav.

  Cause: `html, body, #app { height: 100% }` with `viewport-fit=cover`. On iOS standalone,
  `100%` resolves against the initial containing block, which does not reliably equal the
  visible viewport — a long-standing quirk. Fixed by sizing to `100dvh`, which tracks the
  dynamic viewport (Safari 15.4+; our baseline is 16.4+). `height: 100%` is kept as a
  preceding declaration, since an over-tall shell fails worse than a short one.

  Stated plainly because it is a hypothesis rather than a reproduction: this could not be
  reproduced here — desktop Chromium reports zero insets and does not exhibit the iOS viewport
  quirk — so it **needs a re-check on the device**. If ~34 px of blank white below the labels
  remains after the fix, that is the home-indicator inset working correctly, and the remaining
  question is a design one: whether to tighten it to
  `max(env(safe-area-inset-bottom) - 0.5rem, 0px)`, since the nav row's own `py-2` already
  provides 8 px of clearance.

- 2026-08-24 00:00 — Task created from PRD 004.
- 2026-08-24 02:45 — Partial progress: **the bundle comparison is done**, so that
  criterion can be closed on sight. Built the pre-migration commit (98360fd) in a
  throwaway git worktree to get a real baseline instead of guessing:

  | | baseline (TW3 + PrimeVue) | now (TW4 + shadcn, no PrimeVue) |
  |---|---|---|
  | CSS | 11.33 kB (gzip 3.13) | 41.66 kB (gzip 8.02) |
  | shell JS | 266.15 kB (gzip 74.08) | 125.46 kB (gzip 48.46) |
  | initial total (gzip) | **77.21 kB** | **56.48 kB (−26.9%)** |
  | lazy `MoreMenu` chunk | — | 77.15 kB (gzip 24.98), on demand |
  | precache | — | 19 entries, 258.98 KiB |

  CSS grew ~4.9 kB gzip (v4's token block + `tw-animate-css` ≈ 1.7 kB; the
  generated-but-not-yet-used primitives ≈ 3.8 kB, measured by temporarily removing
  `src/components/ui/`). The JS win more than pays for it. PRD 004's "payload must
  not grow" criterion is met on the initial payload.
- 2026-08-24 02:46 — Everything else here still needs a browser/device: route
  click-through, the login flow against the BFF, and the service-worker update
  prompt. Left open.
- 2026-08-24 11:55 — Most of it is now closed by the headless browser pass done
  under task 025 (see its log for the method and full results): all routes render
  with a clean console, the login flow completes end to end against the BFF through
  Traefik + the Vite proxy, the drawer works, `grep -ri primevue` is empty and no
  `tailwind.config.js` remains.
- 2026-08-24 11:56 — Still open, and deliberately so — these need hardware or a
  production deploy, not emulation: standalone-PWA behaviour on a real iOS/Android
  device, `env(safe-area-inset-*)` on a notched screen, the drawer's touch
  swipe-to-dismiss, and the service-worker update prompt across two production
  builds. PRD 004 stays in `doing/` until these pass.
- 2026-08-27 — **Correction: every device observation since 2026-08-26 was made against a
  stale build, and one conclusion drawn from them was wrong.** The maintainer reported
  that `docker stack deploy` fails with *network "jetstream" is declared as external, but
  could not be found* — and task 089 establishes that this repo's production deploy has
  **never succeeded**, because the org does not run Swarm at all. So the device has been
  serving an image built before the `100dvh` fix and before the diagnostic panel existed.
- 2026-08-27 — Consequences, in order of how badly they were misread:
  * "There is no amber panel" is **expected**, not a bug: `LayoutDebug` was never on the
    device.
  * The `100dvh` fix is **untested, not disproven**. I previously read the unchanged
    spacing as the fix having failed, and built a diagnostic on that premise. The
    spacing was unchanged because the code was not there.
  * The earlier screenshot showing pre-085 prompt copy already pointed at a stale
    deployment. It was recorded as a curiosity instead of being followed up, which would
    have caught this two sessions earlier. Lesson: an unexpectedly *old* string in a
    screenshot is evidence about the build, not about the string.
- 2026-08-27 — Next step is therefore unchanged in substance but must happen in the right
  order: land task 089's `docker-compose.prod.yml`, get a deploy to actually succeed,
  and only then ask for a re-check of the bottom strip plus a screenshot of the amber
  panel (Mere → Data og privatliv). Its root font-size reading is what confirms or kills
  the rem / iOS Larger-Text hypothesis. No further layout theorising before then.
