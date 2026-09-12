# 207 — Detection helpers honour the dev device profile

**Status:** done
**Priority:** high
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

PRD 014, phase 1. Make the app's two detection sites read the dev profile from
task 206, so a laptop can be treated as an installed phone **with the real gate
chain running**.

The whole design of this task is *where* the override enters. It goes into the
environment the helpers read, not into the helpers' logic:

- `vue/src/helpers/platform.ts` — change **`defaultEnv()` only**. The exported
  predicates `isMobileDevice()`, `isStandalone()` and `installPlatform()` keep their
  bodies, their signatures and their injectable `PlatformEnv` seam untouched.
- `vue/src/config/permissions.ts` — `detectPlatform()` consults the profile before
  sniffing.

Why that way round, and it is worth being stubborn about it:

1. **The existing tests stay valid as written.** `platform.spec.ts` calls every
   predicate with a hand-built `PlatformEnv`; if the override lived inside the
   predicates, every one of those cases would have to grow a "and no dev profile"
   precondition, and the file that documents the iPadOS and webview heuristics would
   become about dev tooling instead.
2. **The router guard picks up no new dependency.** `platform.ts`'s header states
   the constraint: synchronous, dependency-free, no store or config imports, because
   the guard decides during `beforeEach` on a cold start. A profile read is one
   synchronous `localStorage` get, which satisfies that; an import of anything
   heavier would not.
3. **`gates.spec.ts` already mocks `@/helpers/platform`**, so the gate tests are
   unaffected by construction — which is the check that this is the right seam.

The payoff is that the guard runs steps 2–4 **for real**: `?dev=iphone` lands on
`/welcome` through genuine onboarding gating, and `?dev=tab` lands on the install
wall. Neither is reachable today.

## The platform mapping

`installPlatform()` returns `'chromium' | 'ios-safari' | 'other' | 'webview'`, and
`detectPlatform()` returns `'ios' | 'android' | 'other'`. The profile's `platform`
field must map onto both without either helper learning about the other's
vocabulary. Note `ipad` and `iphone` both mean `ios-safari` / `ios` for these
purposes — every browser on iOS is WebKit and only Safari can add to the home
screen, which is exactly why `installPlatform()` collapses them today.

Getting this right is most of the point of the task: it is what makes the iOS
add-to-home-screen copy in `components/onboarding/InstallInstructions.vue` and the
per-platform blocked-permission text in `config/permissions.ts` inspectable from a
Mac, where neither can currently be seen at all.

## A comment that becomes wrong

`config/permissions.ts:19` currently says `detectPlatform` "is the ONLY user-agent
sniff in the app, and it is contained here so components never do their own". That
stays true, but it is no longer the only *input* — the comment must be updated to
say where the override enters, or the next reader will trust a stale invariant.
Same for the "nothing here reads a global at module level and nothing is cached at
import time" note in `platform.ts`: still true, and now load-bearing for a second
reason (the profile changes at runtime), so say so.

## Acceptance Criteria

- [x] `defaultEnv()` in `platform.ts` yields the simulated `navigator`
      (`userAgent`, `platform`, `maxTouchPoints`, `standalone`) and `matchMedia`
      answers derived from the profile
- [x] `isMobileDevice()`, `isStandalone()` and `installPlatform()` are **unchanged
      below their default argument** — no new branches in their bodies
- [x] `detectPlatform()` in `config/permissions.ts` honours the profile
- [~] `?dev=iphone` on a laptop: gate steps 2–4 run, the app lands on `/welcome`,
      and onboarding actually gates subsequent routes — **verified at the gate level**
      (`devPlatform.spec.ts` drives `deviceAndInstallGates` with the real platform
      helpers), not yet in a browser; see the log
- [~] `?dev=tab`: the install wall renders, with the **iOS** instructions when the
      profile says iOS and the Chromium ones when it says Chromium — the gate returns
      `{name:'install'}` and `installPlatform()` returns the right branch per profile,
      both tested; the rendering itself is unverified
- [x] `?dev=webview` reaches the webview branch of `InstallInstructions`
      (`installPlatform() === 'webview'`, tested)
- [x] `blockedGuidance()` renders the iOS and Android copy on demand — `detectPlatform()`
      returns `ios`/`android`/`other` per profile, tested
- [x] `?dev=off` / `?dev=desktop` restores real detection — a laptop is again sent
      to `/desktop.html` (tested: the gate returns `LEAVE_APP`)
- [x] `platform.spec.ts`, `gates.spec.ts`, `rolegate.spec.ts` and
      `navigation.spec.ts` pass **unmodified**
- [x] The stale comments in `permissions.ts` and `platform.ts` are updated
- [x] Overrides remain inert under `import.meta.env.PROD` (inherited from task 206,
      re-verified here at the helper level)

## Explicitly not in this task

- `router/gates.ts`, `router/index.ts` and `config/gates.ts` stay untouched. PRD 014
  §8 names this: if implementation edits gate logic, the design has drifted.
- No UI. Discovering the current simulated state still requires knowing what you
  typed — the always-visible readout is phase 2, and until it exists a stale profile
  is a real debugging trap (PRD 014 §5). Worth keeping phase 2 close behind.
- Role simulation. PRD 014 open question 4 argues against it: the BFF would still
  answer 403, so a simulated role would mislead rather than help. Log in as a seeded
  member of the role instead.

## Depends on

- **Task 206** — the profile this reads.
- **Task 116** — the platform helper being extended.
- **Task 137** — the guard whose steps 2–4 this makes reachable.

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 1.
- 2026-09-12 08:45 — Picked up.
- 2026-09-12 08:50 — Key decision: the simulated navigator carries **real user-agent strings**
  rather than tokens chosen to satisfy the checks. So `?dev=ipad` exercises the actual
  `MacIntel` + `maxTouchPoints > 1` path, and if that heuristic is ever wrong, simulating it
  reproduces the wrongness. A simulation that answered correctly by construction would hide
  exactly the class of bug task 139's device matrix exists to find.
- 2026-09-12 08:51 — `navigator.standalone` is set **only** for the Apple presets; Android
  reaches standalone through the `display-mode` query, as a real Android device does. Setting
  both would let the simulation satisfy `isStandalone()` by a route no device uses.
- 2026-09-12 08:52 — `simulatedMatchMedia` answers only the display-mode and pointer queries
  and **defers everything else to the real engine**. `matchMedia` is also used for
  `prefers-color-scheme` and orientation elsewhere; a stub answering `false` to all of it would
  quietly change unrelated behaviour while claiming only to simulate a phone.
- 2026-09-12 08:53 — Avoided the duplicated platform map: `platform.ts` exports `devNavigator()`
  and `detectPlatform()` sniffs *that*. So `permissions.ts` keeps its claim to be the app's only
  UA sniff, and there is one definition of "what an iPad looks like" rather than two that drift.
  Its stale comment now says where the override enters. Also relaxed its `navigator.maxTouchPoints`
  read to `?? 0`, since the injected navigator's field is optional.
- 2026-09-12 08:54 — `chromium` is a documented alias of `android`. It exists because
  `chromium` is the word `installPlatform()` returns, so it is the word a developer reaches for.
- 2026-09-12 08:57 — Added `helpers/devPlatform.spec.ts` (12 tests). Notably it drives
  **`deviceAndInstallGates` with the real platform helpers**, which nothing else in the suite
  does — `gates.spec.ts` mocks them, so a disagreement between the simulation and the gate would
  otherwise go unnoticed. That is what verifies the PRD's actual claim: a simulated iPhone with
  incomplete onboarding is redirected to `/welcome`, where `?nogate=1` would have returned
  `true` and skipped it.
- 2026-09-12 08:58 — One test needed `vi.stubGlobal('matchMedia', ...)`: the real `defaultEnv()`
  calls it and node has no such function. Not a new fragility — it is why `gates.spec.ts` mocks
  the helpers and why the router wraps the guard in try/catch (task 090).
- 2026-09-12 08:59 — ✅ Suite green: 444 tests across 35 files, `vue-tsc --noEmit` clean, and the
  four named existing specs pass unmodified.
- 2026-09-12 09:00 — **Two criteria marked `[~]` rather than `[x]`, deliberately.** Everything is
  verified at the decision level (gate return values, `installPlatform`, `detectPlatform`), but I
  cannot drive a browser from here, so "the wall *renders* with the iOS copy" is untested. The dev
  server also refused a container-internal request (Vite host check, 403), so not even a smoke
  fetch was possible. **Task 208 is the first point where a human will actually look at this in a
  browser — confirm the redirect and the wall copy then.**
