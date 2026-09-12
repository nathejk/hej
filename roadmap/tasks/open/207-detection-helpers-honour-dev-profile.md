# 207 — Detection helpers honour the dev device profile

**Status:** open
**Priority:** high
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `defaultEnv()` in `platform.ts` yields the simulated `navigator`
      (`userAgent`, `platform`, `maxTouchPoints`, `standalone`) and `matchMedia`
      answers derived from the profile
- [ ] `isMobileDevice()`, `isStandalone()` and `installPlatform()` are **unchanged
      below their default argument** — no new branches in their bodies
- [ ] `detectPlatform()` in `config/permissions.ts` honours the profile
- [ ] `?dev=iphone` on a laptop: gate steps 2–4 run, the app lands on `/welcome`,
      and onboarding actually gates subsequent routes
- [ ] `?dev=tab`: the install wall renders, with the **iOS** instructions when the
      profile says iOS and the Chromium ones when it says Chromium
- [ ] `?dev=webview` reaches the webview branch of `InstallInstructions`
- [ ] `blockedGuidance()` renders the iOS and Android copy on demand
- [ ] `?dev=off` / `?dev=desktop` restores real detection — a laptop is again sent
      to `/desktop.html`
- [ ] `platform.spec.ts`, `gates.spec.ts`, `rolegate.spec.ts` and
      `navigation.spec.ts` pass **unmodified**
- [ ] The stale comments in `permissions.ts` and `platform.ts` are updated
- [ ] Overrides remain inert under `import.meta.env.PROD` (inherited from task 206,
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
