# 119 — InstallView: the install wall

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 005 §6/§7. Add `vue/src/views/InstallView.vue`, routed at `/install` with `meta: { public: true }`
(alongside the existing `/login` entry in `vue/src/router/index.ts`). This is the wall a mobile
visitor hits when the app is not running standalone: the app is not usable in a browser tab, so
the tab's only job is to get the app installed.

Two states, chosen by `install.store.canPrompt`:

- **Captured `beforeinstallprompt`** → a single primary "Installér app" button calling
  `promptInstall()`. After the prompt resolves, `canPrompt` is false; the view must fall back
  to the manual instructions rather than leaving a dead button (a user who dismissed the
  native prompt still needs a route forward).
- **No prompt available** (iOS Safari, Android non-Chrome, in-app webview) → the
  platform-specific manual instructions from task 120.

Plus, in both states, **"Jeg har allerede installeret appen — åbn den"**.

## Why the "already installed" affordance is mandatory

Installation **cannot be reliably detected from a browser tab.** `getInstalledRelatedApps()`
is Chromium-only and, despite the name, reports related *native* apps — it is not a general
"is my PWA installed" query. And on iOS there is no install-accepted event at all (task 120).
So the honest situation is: a user may have installed the app a minute ago and this tab has no
way to know.

Without that affordance the wall's failure mode is a user who has done exactly what was asked,
staring at the same screen telling them to do it again. The action itself can only be an
instruction plus a nudge — "luk denne fane og åbn Hej Nathejk fra hjemmeskærmen" — since a tab
cannot launch its own standalone instance. Say that plainly rather than wiring a button that
appears to do something and does not.

## Shell and chrome

PRD 005 §7: the top bar **and** `BottomNav` are hidden on `/install`. That is the App.vue
`showShell` change (a separate task from §10's list) — this view must not attempt to hide them
itself, and must not use `fullBleed`, which only suppresses the header *inside* `showShell`
and would be a no-op here. Build the view assuming it owns the whole viewport.

`UpdatePrompt` must not overlay the wall (PRD 005 §8) — if it does, note it here and fix it in
the App.vue task rather than working around it in this view.

`/install` is part of the precached shell (PRD 005 §6, Offline). Verify it is reachable
offline once built; the route is lazily imported like every other view, so its chunk needs to
be in the precache manifest.

## UI

shadcn-vue `Card` and `Button` (both already generated in `vue/src/components/ui/`), Lucide
icons (`Download` for the primary action). The page headline uses `font-nathejk` per `.rules`,
matching `LoginView.vue`'s `<h1 class="font-nathejk …">`. All copy in Danish. Do not
hand-roll a button or card here — the only hand-rolled element PRD 005 §7 sanctions is the
platform install illustration, and that lives in task 120.

## Acceptance Criteria

- [ ] `vue/src/views/InstallView.vue` exists and `/install` is registered with
      `meta: { public: true }` and a lazy import
- [ ] With `canPrompt` true: one primary "Installér app" button calling `promptInstall()`
- [ ] After the native prompt resolves (accepted **or** dismissed) the view shows the manual
      instructions instead of a dead button
- [ ] With `canPrompt` false: `InstallInstructions` for the detected platform
- [ ] **"Jeg har allerede installeret appen — åbn den"** is present in both states, and is
      honest about what it can do — it explains how to open the installed app rather than
      pretending to launch it
- [ ] No `getInstalledRelatedApps()`-based detection; the code comments why
- [ ] The view assumes no shell chrome and does not use `fullBleed`
- [ ] shadcn-vue `Card`/`Button`, Lucide icons, `font-nathejk` headline, all copy Danish
- [ ] Reachable offline from the precached shell
- [ ] `npm run type-check` clean

## Depends on

- **Task 116** — `installPlatform()` / `isStandalone()`.
- **Task 117** — `canPrompt` / `promptInstall()`.
- **Task 120** — the instructions component (the fallback state is empty without it).
- **Task 121** — the "Fortsæt i browseren" escape hatch, which lives on this view.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
