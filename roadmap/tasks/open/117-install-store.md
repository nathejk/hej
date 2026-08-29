# 117 — Install store: beforeinstallprompt capture and browser override

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 005 §8 (Frontend). Add `vue/src/stores/install.store.ts` (Pinia, alongside the existing
`location.store.ts` / `notifications.store.ts`), holding everything the app knows about its
own installability. It backs the install wall (task 119) and the escape hatch (task 121).

Surface:

- `canPrompt` — a `beforeinstallprompt` event was captured and has not been consumed.
- `promptInstall()` — calls `prompt()` on the captured event, awaits `userChoice`, and clears
  it. The event is **single-use**: once prompted it cannot be re-prompted, so the store must
  drop its reference and flip `canPrompt` to false regardless of the outcome, or the button
  becomes a no-op that looks like a broken app.
- `installed` — set from the `appinstalled` event.
- `continueInBrowser` — the persisted override, with a setter.

## The timing risk is the whole difficulty

PRD 005 §8 (risks): **`beforeinstallprompt` fires once, and early.** If the listener is not
registered before the Vue app mounts, the event is gone and one-tap install silently
degrades to manual add-to-home-screen instructions — on the platform that is the *only* one
where one-tap works. Silently is the problem: nothing errors, the wall just quietly shows the
worse variant, and it will not reproduce reliably in dev.

So registration goes in `vue/src/main.ts` **before `app.mount('#app')`**, not in a store's
`onMounted` and not in the wall's `setup()`. Note that `main.ts` already has an ordering
comment for `initSafeArea()` for a similar reason — follow that shape, and say in the comment
*why* the position matters so a future tidy-up does not move it below the mount.

The listener must `preventDefault()` the event (otherwise Chromium may show its own mini
infobar, competing with our wall) and stash it somewhere the store can pick up. Because
Pinia is available before mount (`app.use(createPinia())` runs first), the cleanest form is a
small `initInstallPrompt()` exported from the store module and called from `main.ts`; avoid a
free-floating module-level variable that the store then has to reach into.

## The override

`continueInBrowser` is per-browser device state, persisted in `localStorage` under the
`hej.install.*` namespace (PRD 005 §8, Data/storage). Follow the existing pattern in
`config/runtime.ts`: every `localStorage` access wrapped in `try/catch`, because Safari
throws on access in some privacy modes and an exception here would break the router gate and
white-screen the app (see task 090's lesson).

Per-user state is explicitly **not** stored here — profile confirmation comes from the BFF.
This store is device-scoped only.

## Acceptance Criteria

- [ ] `vue/src/stores/install.store.ts` exposes `canPrompt`, `promptInstall()`, `installed`
      and `continueInBrowser` (+ setter)
- [ ] The `beforeinstallprompt` listener is registered from `main.ts` **before**
      `app.mount('#app')`, with a comment explaining why the position is load-bearing
- [ ] The event is `preventDefault()`ed so Chromium's own mini infobar does not compete with
      the wall
- [ ] `promptInstall()` awaits `userChoice`, then clears the stored event and `canPrompt`
      whether the user accepted or dismissed — the event cannot be reused
- [ ] `installed` is set from the `appinstalled` event
- [ ] `continueInBrowser` persists under a `hej.install.*` key and survives a reload
- [ ] Every `localStorage` read/write is wrapped so a throwing/blocked storage cannot break
      the router gate
- [ ] No per-user state in this store
- [ ] `npm run type-check` clean (`BeforeInstallPromptEvent` is not in the DOM lib — declare a
      local type rather than casting to `any`)

## Depends on

- **Task 116** — `isStandalone()`/`installPlatform()`, which the store's consumers pair with
  `canPrompt` to decide what the wall shows. Not a hard import dependency, but building the
  store before the helper means guessing at its shape.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
