# 221 — `?dev=` lost the race against the desktop redirect

**Status:** done
**Priority:** high
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

Reported immediately after PRD 014's tasks were finished: on a laptop, opening the app
with `?dev=iphone` still redirected to `/desktop.html`. The simulation layer appeared to
do nothing, every time.

Two separate faults, and the second is what made the first inescapable.

## 1. The ordering was wrong (the bug)

`main.ts` registered the dev simulation **after** `app.use(router)`:

```ts
app.use(router)          // ← triggers vue-router's initial navigation
initSafeArea()
initGateOverride()
if (import.meta.env.DEV) {
  const { initDevSimulation } = await import('@/dev/bootstrap')   // ← too late
  initDevSimulation()
}
```

Installing the router runs its first navigation, which runs the device gate, which answers
a desktop with a **synchronous** `window.location.replace('/desktop.html')`. The dynamic
import is still in flight at that moment, so:

- the profile is never written,
- the page navigates away, and
- `?dev=iphone` leaves with the URL.

The comment above the block claimed it ran "before the router's first navigation" and it
was wrong — it ran before `app.mount()`, which is a different and insufficient thing.
Nothing caught this: the unit tests call `initDevDevice`/the provider directly, so they
verify the mechanism and are silent about *when* it runs.

**Fix:** move the dev block above `createApp`/`app.use(router)`. Top-level `await` makes
the ordering deterministic rather than merely likely, by suspending the rest of the module
until registration is done.

Noted while there: `initGateOverride()` is late by the same argument and survives on a
technicality — `?nogate=` only matters on a device the gate does not eject, so there is no
synchronous `location.replace` for it to lose to. That is now written down next to it,
because it is not obvious and the next person to reorder this file needs it.

## 2. The documentation sent the user in a circle

`README.md`, `devDevice.ts` and `bootstrap.ts` all claimed `?dev=` could be typed onto
`/desktop.html`, "because the SPA parses it on the next load". **False.**
`public/desktop.html` is a plain static file with no bundle — nothing on it reads the
query, and reloading it just loads it again.

So the documented recovery path was a no-op, and the only working path (`/?dev=iphone`)
was broken by fault 1. Between them there was no way in at all.

**Fix:** all three now say the parameter must be given on an app URL, and the README tells
you what to do if you are looking at the placeholder.

## Acceptance Criteria

- [x] `?dev=iphone` on the app root loads the app on a desktop browser
- [x] `?dev=tab` reaches the install wall
- [x] No parameter, and `?dev=off`, still eject to the placeholder
- [x] The ordering constraint is recorded where it can be violated (`main.ts`,
      `bootstrap.ts`, `devDevice.ts`)
- [x] The false `/desktop.html?dev=` claim is corrected in all three places
- [x] The production bundle still contains no dev layer, and no top-level await from it

## Progress Log

- 2026-09-12 10:24 — Reported. Reproduced by reading the order in `main.ts`: `app.use(router)` is
  line 18, the dev bootstrap line 44. vue-router's `install()` starts the first navigation, so the
  gate had already ejected the page before the import resolved.
- 2026-09-12 10:26 — Moved the dev block to the top of `main.ts`, above `createApp`. Recorded the
  reason in a comment that names the failure rather than the rule, since the previous comment stated
  the rule correctly and still allowed the bug.
- 2026-09-12 10:28 — Found the second fault while fixing the first: the `/desktop.html?dev=` advice
  was invented and wrong. Corrected in `README.md`, `dev/devDevice.ts` and `dev/bootstrap.ts`. The
  comment in `devDevice.ts` now says explicitly that the earlier claim was wrong, so nobody
  reinstates it.
- 2026-09-12 10:32 — ✅ **Verified in a real browser** with headless Chrome against the dev stack
  (the same technique as `vue/scripts/check-install-gate.sh`), each run on a throwaway profile so
  no leftover localStorage could fake a pass:
  * `/?dev=iphone` → the **app**: "Log ind" and "Telefonnummer" present, `more to come` absent, and
    the dev panel's `DEV` handle rendered.
  * `/?dev=tab` → the **install wall**: add-to-home-screen wording present, no login form (task 143).
  * `/` with a fresh profile → the placeholder.
  * `/?dev=off` → the placeholder.
- 2026-09-12 10:33 — ✅ Suite 472/472, `vue-tsc` clean, production build re-scanned: no dev-layer
  strings, no `src/dev/` chunk, and the only `await import` occurrences are the pre-existing
  `tileBulk` and `workbox-window` ones.
- 2026-09-12 10:33 — This clears most of PRD 014 §9's outstanding item 1. Still unlooked-at by a
  human: the panel's *appearance*, the faked insets moving the shell, and the fake position drawing
  on the map.
