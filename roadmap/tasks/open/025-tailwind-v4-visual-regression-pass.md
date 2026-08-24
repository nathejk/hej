# 025 — Visual regression pass after the Tailwind v4 upgrade

**Status:** open
**Priority:** high
**Created:** 2026-08-24
**Picked up by:**
**Started:**
**Completed:**

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

## Acceptance Criteria

- [ ] Every route listed above renders with no console errors and no unintended
      visual change on a phone viewport.
- [ ] The `MoreMenu` bottom sheet, both permission pre-prompts and the update
      prompt are verified specifically (borders, shadows, rings, focus rings).
- [ ] Safe-area insets still behave on a notched viewport (`App.vue` header,
      `BottomNav.vue`).
- [ ] Any accepted visual difference is logged with a one-line rationale.
- [ ] Focus-visible styling is still present on all interactive elements.

### Added after tasks 029–032 landed

- [ ] **`MoreMenu` is now a shadcn `Drawer`, not hand-rolled markup** (task 032),
      and was implemented without a browser available. Verify explicitly: it opens
      from the "Mere" tab, closes on backdrop tap / escape / swipe-down / choosing a
      destination, traps focus while open, locks background scroll, and clears the
      iOS home indicator. If it misbehaves, the documented fallback is reverting to
      the previous hand-rolled sheet (see 032's log).
- [ ] The lazily loaded `MoreMenu` chunk fetches and renders on the **first** tap
      with no flash of empty overlay, and the close animation still plays.
- [ ] `PermissionPrompt`'s shadow still looks right after `shadow-sm` → `shadow-xs`
      (task 024).
- [ ] Nothing renders in a dark canvas now that `color-scheme` is `light` — check a
      phone with OS dark mode enabled, which is what the old `light dark` value
      mishandled (task 028).
- [ ] No webfont request to `fonts.googleapis.com` in the network panel, and
      typography is unchanged from the system stack (task 027).

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004.
- 2026-08-24 02:40 — Scope extended (see the added criteria): tasks 027–032 all
  landed without a browser available, so this task now covers the shadcn changes
  too, not just Tailwind's changed defaults. Everything verifiable without a
  browser has been done — type-check, build, `npm ci`, a dev-server fetch of the
  compiled CSS, and a source audit of every affected utility — but rendering and
  gestures genuinely need a device. Left **open** deliberately rather than
  claiming a pass I did not perform.
