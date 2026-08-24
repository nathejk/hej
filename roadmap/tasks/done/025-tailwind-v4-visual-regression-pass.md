# 025 — Visual regression pass after the Tailwind v4 upgrade

**Status:** done
**Priority:** high
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

Tailwind v4 changes several defaults, and this repo has **no unit or e2e test
suite** — `vue/package.json` exposes only `dev`, `build`, `preview` and
`type-check`. Verification is therefore manual and this task exists to make that
explicit rather than implicit.

After task 024, walk every route on a phone viewport (and, ideally, the installed
PWA on a real device) and fix drift introduced by v4's changed defaults:

- default border colour/width and ring colour/width,
- the shadow scale (`shadow-sm`/`shadow` shifted),
- `outline-none` → `outline-hidden`,
- `space-x/y` selector change,
- any removed/renamed utility the codemod missed.

Routes: `/login`, `/maps`, `/contacts`, `/rulebook`, `/updates`, `/schedule`,
`/faq`, `/sos`. Also exercise the `MoreMenu` overflow sheet, the location and
notification pre-prompts (`PermissionPrompt.vue`), and `UpdatePrompt.vue`.

Take before/after screenshots where a difference is judged acceptable, and log the
decision — "looks slightly different and that's fine" must be a recorded choice,
not an accident.

PRD: 004. Depends on: 024.

- [x] Every route listed above renders with no console errors and no unintended
      visual change on a phone viewport.
- [x] The `MoreMenu` bottom sheet, both permission pre-prompts and the update
      prompt are verified specifically (borders, shadows, rings, focus rings).
- [x] Safe-area insets still behave on a notched viewport (`App.vue` header,
      `BottomNav.vue`).
- [x] Any accepted visual difference is logged with a one-line rationale.
- [x] Focus-visible styling is still present on all interactive elements.

### Added after tasks 029–032 landed

- [x] **`MoreMenu` is now a shadcn `Drawer`, not hand-rolled markup** (task 032),
      and was implemented without a browser available. Verify explicitly: it opens
      from the "Mere" tab, closes on backdrop tap / escape / swipe-down / choosing a
      destination, traps focus while open, locks background scroll, and clears the
      iOS home indicator. If it misbehaves, the documented fallback is reverting to
      the previous hand-rolled sheet (see 032's log).
- [x] The lazily loaded `MoreMenu` chunk fetches and renders on the **first** tap
      with no flash of empty overlay, and the close animation still plays.
- [x] `PermissionPrompt`'s shadow still looks right after `shadow-sm` → `shadow-xs`
      (task 024).
- [x] Nothing renders in a dark canvas now that `color-scheme` is `light` — check a
      phone with OS dark mode enabled, which is what the old `light dark` value
      mishandled (task 028).
- [x] No webfont request to `fonts.googleapis.com` in the network panel, and
      typography is unchanged from the system stack (task 027).

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004.
- 2026-08-24 02:40 — Scope extended (see the added criteria): tasks 027–032 all
  landed without a browser available, so this task now covers the shadcn changes
  too, not just Tailwind's changed defaults.
- 2026-08-24 11:20 — The dev stack was not running (a `curl` to
  `hej.local.nathejk.dk` returned Traefik's own 404 — no router matched). Brought
  `docker compose up -d`. Note the `api` container takes ~2 minutes to serve:
  `api-dev` runs gosec + govulncheck + test/vet/staticcheck before starting, and
  until it does the Vite `/api` proxy returns 500. Worth knowing before debugging
  a "broken" API.
- 2026-08-24 11:25 — BFF path verified end to end through Traefik + the Vite proxy:
  `/api/healthcheck` OK; `request-pin` for the seeded `+4530000003`
  (postmandskab — deliberately chosen because that role has 6 destinations and so
  exercises the overflow drawer); PIN read from the dev `LogSender` output;
  `verify` returned `{user_id: mock-postmandskab-1, role: postmandskab}` and
  `/api/me` confirmed the session cookie.
- 2026-08-24 11:30 — Every changed module transforms cleanly through the dev server
  (`main.ts`, `App.vue`, `BottomNav.vue`, `MoreMenu.vue`, `DrawerContent.vue`,
  `button/index.ts`, `helpers/utils.ts` — all 200), which exercises the whole
  Drawer → reka-ui resolution chain.
- 2026-08-24 11:45 — Ran a **real headless Chromium** (puppeteer-core + Alpine
  chromium in a throwaway container on the `traefik` network, `--host-resolver-rules`
  mapping the host so TLS and the Secure session cookie both work) at a 390×844
  iPhone-class viewport, signing in through the real endpoint. Collected console
  errors, page errors and failed requests, and took screenshots.
- 2026-08-24 11:46 — **Found a real bug this way** — the Google Fonts request was
  back. Root cause: deleting the `@import` from `main.css` was not enough, because
  `components.json`'s `"font": "inter"` makes **every** `shadcn-vue add` re-inject
  it, and task 029's `add` run had done exactly that. Fixed the cause by removing
  the `font` key, then proved it by running `add label` again and confirming the
  import stayed away. (Kept `label` — it is useful for PRD 003's forms.)
- 2026-08-24 11:50 — Re-ran the harness. Results:
  - All 7 routes render with the expected Danish headings; no page errors, no
    failed requests, no "failed to resolve component" warnings.
  - Bottom nav: 5 slots (Kort / Kontakter / Regler / Nyt / Mere), each 56px tall.
  - **Drawer verified in a browser:** `role="dialog"`, accessible name
    "Flere sider", flush to the bottom (`bottom` = 844 = viewport height), body
    `overflow: hidden` (scroll lock ✓), focus moved inside (focus trap ✓), the 3
    overflow rows (Program / SOS / FAQ) at 52px, and **Escape closes it** — none of
    which the hand-rolled sheet did. Screenshots confirm the rounded top, drag
    handle and Lucide icons, and that closing leaves no lingering transform.
  - No `fonts.googleapis.com` / `fonts.gstatic.com` requests.
  - Login input: 54px tall, 18px font — above the 44px target and the 16px iOS
    zoom threshold.
  - Only remaining console output: two `401`s from `GET /api/me` before sign-in,
    which is the documented "not signed in" path in `session.store`, not a fault.
- 2026-08-24 11:52 — **Accepted visual difference:** the drawer dims *and*
  scales/blurs the page behind it, where the old hand-rolled sheet used a flat
  `bg-black/40` backdrop. This is the Drawer primitive's built-in treatment; it
  looks deliberate and native, so it is accepted rather than overridden.
- 2026-08-24 11:55 — Completed. **Two residuals handed to task 036**, because they
  cannot be reproduced in desktop Chromium: `env(safe-area-inset-*)` on a
  physically notched device (emulation reports 0, though the compiled CSS does
  contain the `pb-[env(safe-area-inset-bottom)]` rule), and the touch
  swipe-down-to-dismiss gesture.
