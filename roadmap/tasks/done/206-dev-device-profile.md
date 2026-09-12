# 206 — Dev device profile (`config/devDevice.ts`)

**Status:** done
**Priority:** high
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

PRD 014, phase 1. The foundation of the whole dev simulation layer: a persisted,
development-only description of *what device the app should believe it is on*.

This is deliberately **not** another gate bypass. One already exists —
`?nogate=1` → `hej.gates.bypass` → `gatesEnabled()` in `vue/src/config/gates.ts`,
consulted once at `vue/src/router/index.ts:133` — and it is the wrong tool for
development because it switches guard steps 2, 3 **and 4** off together. Step 4 is
the onboarding redirect, so the only current way onto a laptop is also the way that
skips the flow most in need of testing. `gates.ts` says as much in its own comment
and declines to solve it ("that would be a new PRD"). PRD 014 is that PRD.

So this task adds a *profile*, and later tasks make the detection helpers report it.
The gates then run for real against a simulated phone.

## The shape

```ts
interface DevDevice {
  mobile: boolean
  standalone: boolean
  platform: 'ios' | 'ipad' | 'android' | 'chromium' | 'webview' | 'other'
}
```

Persisted at `localStorage['hej.dev.device']`. The `hej.dev.*` prefix is required
rather than cosmetic: it is what distinguishes dev overrides from the product keys
already in that origin (`hej.gates.bypass`, `hej.install-gate`, `hej.show-build-id`,
`hej.show-layout-debug`, `hej.dataforsyningen-token`, `hej.contacts-poll-seconds`)
and what makes "clear all dev overrides" a prefix scan rather than a hand-kept list.

## The URL form

`?dev=iphone|ipad|android|chromium|tab|desktop|off`, parsed once from `main.ts`
**before the router's first navigation** — next to the existing
`initGateOverride()` call, and for exactly the reason recorded there: the first gate
check must already see it, or the user watches a redirect flash.

Two presets need care:

- `tab` — mobile **but not** standalone. This is how the install wall gets tested,
  which is otherwise unreachable on a laptop.
- `off` / `desktop` — clears the profile. A dev override with no off switch is one
  that gets left on, which is the same argument `gates.ts` already makes for
  `?nogate=0`.

Note the entry point matters. A laptop with no profile is sent out of the SPA
entirely (`LEAVE_APP` → `/desktop.html`, `vue/src/router/gates.ts:59`), so `?dev=`
must work when typed onto the address bar of **that** page, not only onto the app.
Since it is a plain static file with no bundle, the parse cannot live in a component
— which is another reason it belongs in `main.ts` on the following load.

Both a query form and a stored form are needed, again for the reason already
recorded in `runtime.ts` and `gates.ts`: the manifest's `start_url` is `/`, so an
installed launch drops the query string.

## Production inertness

`import.meta.env.PROD` guards both read and write, so Vite compiles the branch out
and a production bundle cannot be talked into a simulated device by a URL. Same
technique as `gates.ts:23`. This is the task's most important property and it needs
a test, not just a guard — see the criteria.

## Acceptance Criteria

- [x] `vue/src/config/devDevice.ts` exports a read accessor, a writer, and an
      `initDevDevice(search?)` that parses `?dev=` and persists the result
- [x] All presets work: `iphone`, `ipad`, `android`, `chromium`, `tab`, `desktop`,
      `off` — plus `webview`, added during implementation (see log)
- [x] `tab` yields `mobile: true, standalone: false`
- [x] `desktop` and `off` clear the profile entirely
- [x] Blocked or unavailable `localStorage` degrades quietly (dev-only nuisance),
      matching the `try/catch` convention in `gates.ts` and `runtime.ts`
- [x] Nothing is cached at module level — the profile can change at runtime and the
      predicates that will consume it (task 207) are documented as uncached
- [x] `initDevDevice()` is called from `vue/src/main.ts` before the router's first
      navigation, adjacent to `initGateOverride()`
- [x] Reading and writing are both inert when `import.meta.env.PROD`
- [x] `vue/src/config/devDevice.spec.ts` covers every preset, the clear paths, the
      blocked-storage path, and **explicitly asserts inertness under `PROD`**
- [x] `config/gates.ts` is unchanged — `?nogate=` keeps its current meaning as the
      way to verify the `install_gate` kill switch

## Explicitly not in this task

- Making any detection helper honour the profile — that is task 207.
- Any UI. The dev panel is phase 2.
- Any change to `router/gates.ts`, `router/index.ts` or `config/gates.ts`. If this
  task edits gate logic, the design has drifted (PRD 014 §8).

## Depends on

- Nothing. This is the first task of PRD 014.

## Blocks

- **Task 207** — the helpers that consume the profile.

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 1.
- 2026-09-12 — Picked up. Plan: `config/devDevice.ts` with an injectable `{ storage, prod }`
  environment (vitest runs `environment: 'node'`, so there is no `localStorage` and no
  `PROD` to fake — injection is the only way to test either), then the `main.ts` wiring,
  then the spec.
- 2026-09-12 08:30 — `config/devDevice.ts` written. Environment is `{ storage, prod }`,
  injected the same way `platform.ts` injects `PlatformEnv` — forced by vitest's
  `environment: 'node'`, and it is what makes the `PROD` guard testable rather than merely
  asserted in a comment.
- 2026-09-12 08:32 — Decision: **added a `webview` preset** beyond the list in the task.
  `installPlatform()` has four branches and `webview` is one of them, so leaving it out
  would have left the in-app-browser instructions — the branch participants arriving from a
  Facebook group actually hit — as the one thing still unreachable on a laptop. It is pinned
  `standalone: false`, because a standalone webview is not a state any real device can be in
  and simulating one would test a branch no user can reach.
- 2026-09-12 08:34 — Decision: **`tab` preserves the platform already simulated** rather
  than carrying its own. `?dev=android` then `?dev=tab` otherwise showed the *iOS* install
  wall — wrong instructions, with nothing on screen to say the platform had changed
  underfoot. Falls back to iOS when there is no profile yet.
- 2026-09-12 08:36 — Decision: `applyPreset` returns `undefined` for an unknown name, not
  `null`. "You typed a name I do not know" and "simulation is off" must not collapse into one
  answer, or a typo looks like a working clear. `initDevDevice` turns the former into a
  `console.warn` listing the valid presets.
- 2026-09-12 08:38 — Stored values are validated field by field rather than trusted. A
  half-populated profile would put the app in a state no device can be in (`mobile:
  undefined`) and the resulting bug would look like one in the gates.
- 2026-09-12 08:39 — `clearDevOverrides` collects keys before deleting: removing during
  iteration reindexes `Storage` and silently skips entries. No-op against a stub that cannot
  enumerate.
- 2026-09-12 08:40 — Wired into `main.ts` directly below `initGateOverride()`, with a comment
  stating the contrast — the bypass switches the gates *off*, this changes what they *see*.
- 2026-09-12 08:41 — ✅ All criteria complete. 32 new tests pass; full suite 432/432 across
  34 files; `vue-tsc --noEmit` clean. `git diff --stat` confirms the only touched existing
  file is `main.ts` (2 lines + comment) — no gate logic edited, which was the design tripwire.
- 2026-09-12 08:41 — **Not verified in a browser, and cannot be yet:** nothing consumes the
  profile until task 207, so `?dev=iphone` currently persists a profile and changes no
  behaviour. End-to-end verification belongs to 207's criteria. Moving to done.
