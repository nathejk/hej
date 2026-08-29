# 137 — Router guard: device, standalone and onboarding gates

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `desktop?: boolean` added to the `RouteMeta` module declaration
- [ ] Guard order is exactly: dev override → device class → standalone → onboarding
      complete → auth → roles
- [ ] The device gate runs **before** `session.ensureReady()`, so a desktop visit makes no
      `/api/me` request
- [ ] Mobile + not standalone → `/install` from every app route
- [ ] Desktop → `/desktop`, and `/welcome` and `/install` are unreachable on desktop
- [ ] Mobile + standalone + onboarding incomplete → `/welcome`
- [ ] Onboarding complete → the app behaves exactly as today
- [ ] The unauthenticated fallback points at `welcome` (or `install` when the install gate
      has not passed), not `login`
- [ ] No code path in the guard rejects or throws (task 090)
- [ ] `/install` and `/welcome` are in the precache manifest, verified in the generated
      `sw.js` rather than assumed from config
- [ ] The gates are synchronous and add no visible redirect flash on cold start
- [ ] A comment records that the install gate is UX only and that the BFF authorizes
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
