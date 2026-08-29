# 137 — Router guard: device, standalone and onboarding gates

**Status:** done
**Priority:** high
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

## Description

PRD 005 §6 and §8. Extend the global guard in `vue/src/router/index.ts` so the
device/install/onboarding gates run **before** the existing auth check, and add
`desktop?: boolean` to the `RouteMeta` declaration alongside `public`, `roles` and
`fullBleed`.

## Guard order

Fixed, and the order is the design:

1. **dev override** (task 139) — nothing else can be debugged if this is not first
2. **device class** — mobile vs desktop
3. **standalone** — installed vs browser tab
4. **onboarding complete**
5. **auth**
6. **roles**

Mobile + not standalone redirects every app route to `/install`. Desktop goes to
`/desktop` and must **never** reach `/welcome` or `/install` — a desktop visitor being
shown install instructions for a phone is worse than the placeholder.

## Why the device gate precedes auth

The device gate is **session-independent by design**: PRD 005 §11 (2026-08-25) decided
there is no desktop login for any role, so the gate needs no knowledge of the session.
That is what lets it short-circuit **before** `session.ensureReady()` and spare desktop
visitors a pointless `/api/me` round-trip.

Name the consequence the PRD names: because the gate precedes auth, **there is no
role-based bypass** if an organizer needs laptop access mid-event. The dev/QA override
would be the only way in, and it is not intended for that. If organizer desktop access is
ever wanted, it is a new PRD revisiting this gate — not a flag someone adds here.

## The install gate is UX, never security

The BFF continues to authorize every endpoint independently. Nothing in this guard
protects data; it shapes where a user lands. Do not let a later change lean on it.

Two existing constraints in this file carry over and must not be broken:

- **Nothing in the guard may reject** (task 090). A rejected navigation aborts, no route
  component mounts, and the user gets a blank white screen. `ensureReady()` is explicitly
  non-throwing; do not add an `await` that can reject. The new gates are synchronous and
  dependency-free anyway (PRD 005 §6 Non-Functional), which is also what keeps them from
  adding a redirect flash on cold start.
- The unauthenticated fallback currently returns `{ name: 'login' }`. It becomes
  `{ name: 'welcome' }`, or `{ name: 'install' }` when the install gate has not passed —
  the `login` route disappears with task 126.

**Offline.** `/install` and `/welcome` must be part of the precached shell, or the gate
redirects an offline user to a route the service worker cannot serve — turning a working
installed app into a blank page. This is a real failure mode, not a theoretical one: the
app is expected to run offline in the field.

## Acceptance Criteria

- [x] `desktop?: boolean` added to the `RouteMeta` module declaration
- [x] Guard order is exactly: dev override → device class → standalone → onboarding
      complete → auth → roles
- [x] The device gate runs **before** `session.ensureReady()`, so a desktop visit makes no
      `/api/me` request
- [x] Mobile + not standalone → `/install` from every app route
- [x] Desktop → `/desktop`, and `/welcome` and `/install` are unreachable on desktop
- [x] Mobile + standalone + onboarding incomplete → `/welcome`
- [x] Onboarding complete → the app behaves exactly as today
- [x] The unauthenticated fallback points at `welcome` (or `install` when the install gate
      has not passed), not `login`
- [x] No code path in the guard rejects or throws (task 090)
- [x] `/install` and `/welcome` are in the precache manifest, verified in the generated
      `sw.js` rather than assumed from config
- [x] The gates are synchronous and add no visible redirect flash on cold start
- [x] A comment records that the install gate is UX only and that the BFF authorizes
      independently

## Depends on

- **Task 116** — the platform helper (`isMobileDevice`, `isStandalone`).
- **Task 117** — the install store, for the `continueInBrowser` override.
- **Task 118** — the onboarding store, for the completion flag.
- **Task 123** — `DesktopView`, so `/desktop` has something to render.
- **Task 126** — removal of the `/login` route, which is what the fallback change follows.
- **Task 139** — the runtime flag and dev/QA override that this guard consults first.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
- 2026-08-30 — Picked up. This also closes the gap task 126 flagged: until now nothing sent a
  signed-in user with an incomplete onboarding flag into `/welcome`, so they saw no shell chrome.
- 2026-08-30 — **Gates implemented.** `desktop?: boolean` was already on `RouteMeta` (added by task
  123 for exactly this), and steps 2–4 are factored into `deviceAndInstallGates(to)` so the guard
  body reads as the six numbered steps instead of one long chain.

  Four decisions:

  - **Step 1 is a real seam, not a stub in the guard.** New `config/gates.ts` exports
    `gatesEnabled()`, honestly returning `true` today, documented as where task 139 puts the runtime
    flag and the dev/QA override. Inlining half an override here now is how two competing bypass
    mechanisms ship, and the undocumented one is the one found during an event.
  - **The gate block is wrapped in try/catch, falling through on any throw.** `isMobileDevice()`
    and `isStandalone()` read `navigator`/`matchMedia`; if either is ever absent, a throw here would
    abort the navigation and produce the blank white screen of task 090. Falling through degrades to
    an ungated app, which is the correct direction to fail for a gate that is explicitly UX rather
    than security — and the BFF still authorizes every endpoint.
  - **`/install` self-clears.** Once standalone (or overridden), a visit to `/install` redirects on
    to `welcome` or `maps` rather than rendering a wall telling an installed user to install. That
    matters for the profile row added in task 121, which navigates there deliberately: it works
    because it clears the override first.
  - **A mobile visitor hitting `/desktop` is sent to `/install`.** The reverse direction was
    specified; this one was not, and leaving it open would have let a phone user reach a page telling
    them to open the site on a phone.
- 2026-08-30 — **Precache verified in the generated worker**, not inferred from config:
  `dist/sw.js` contains `WelcomeView-Cz92mCLK.js` and `InstallView-BAvMssXh.js`. This matters more
  than it looks — the gate redirects offline users to those routes, so if either chunk were missing
  from the manifest an installed app with no signal would land on a blank page.
- 2026-08-30 — ✅ `vue-tsc`, `npm test` (27) and `npm run build` clean. Real redirect behaviour on
  hardware (including the iPad-classifies-as-mobile case) is task 139's matrix; there is no browser
  here, and the guard's inputs are exactly the heuristics that cannot be exercised on this machine.
