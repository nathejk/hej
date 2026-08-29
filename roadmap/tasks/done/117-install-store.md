# 117 — Install store: beforeinstallprompt capture and browser override

**Status:** done
**Priority:** high
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

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

- [x] `vue/src/stores/install.store.ts` exposes `canPrompt`, `promptInstall()`, `installed`
      and `continueInBrowser` (+ setter)
- [x] The `beforeinstallprompt` listener is registered from `main.ts` **before**
      `app.mount('#app')`, with a comment explaining why the position is load-bearing
- [x] The event is `preventDefault()`ed so Chromium's own mini infobar does not compete with
      the wall
- [x] `promptInstall()` awaits `userChoice`, then clears the stored event and `canPrompt`
      whether the user accepted or dismissed — the event cannot be reused
- [x] `installed` is set from the `appinstalled` event
- [x] `continueInBrowser` persists under a `hej.install.*` key and survives a reload
- [x] Every `localStorage` read/write is wrapped so a throwing/blocked storage cannot break
      the router gate
- [x] No per-user state in this store
- [x] `npm run type-check` clean (`BeforeInstallPromptEvent` is not in the DOM lib — declare a
      local type rather than casting to `any`)

## Depends on

- **Task 116** — `isStandalone()`/`installPlatform()`, which the store's consumers pair with
  `canPrompt` to decide what the wall shows. Not a hard import dependency, but building the
  store before the helper means guessing at its shape.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
- 2026-08-30 — Picked up.
- 2026-08-30 — **Store written**, plus `initInstallPrompt()` called from `main.ts` immediately
  before `app.mount()`, following the shape of the existing `initSafeArea()` ordering comment and
  saying explicitly why it must not be moved below the mount.

  Three decisions:

  - **The captured event is held in a module-level variable, not in Pinia state.** It is a live
    DOM event whose `prompt()` must be called on the original object; putting it in state would
    have Vue wrap it in a reactive proxy and would drag it into devtools serialisation. The store
    keeps a boolean mirror (`canPrompt`), which is all any consumer needs. The task warned against
    a "free-floating module-level variable that the store then has to reach into" — the variable
    stays, but it is private to the module and only the exported `initInstallPrompt()` and the
    store's own action touch it, so nothing reaches in from outside.
  - **`canPrompt` is cleared in a `finally`.** The event is single-use: Chromium refuses a second
    `prompt()` and only issues a fresh event on a later page load. Clearing it on the happy path
    alone would leave a live-looking button that does nothing after a throw — which reads as a
    broken app rather than a declined prompt.
  - **`continueInBrowser` is read eagerly in `state()`,** because the router guard needs it
    synchronously on the very first navigation. Both accesses are wrapped: Safari throws on
    `localStorage` in some privacy modes, and an exception on that path would white-screen the app
    before anything renders — the same failure shape as task 090.

  Also noted in the code: `appinstalled` only fires in the tab that did the installing, so it is
  not an answer to "is this app installed?" — which is exactly why the wall still needs "jeg har
  allerede installeret appen" (task 119).
- 2026-08-30 — ✅ All criteria met. `vue-tsc --noEmit` clean; `npm test` still green (17).

  Not verifiable from here: that the event is actually captured before mount on real Chromium.
  There is no browser in this environment and the behaviour is timing-dependent, so it belongs to
  task 139's device matrix — where the check is simply that Android Chrome shows the one-tap
  button rather than the manual instructions.
