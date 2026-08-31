# 158 — Hide the contacts pane from spejdere

**Status:** done
**Priority:** high
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

## Description

Spejdere do not get the contacts pane (PRD 007 §6). The `contacts` destination in
`vue/src/config/navigation.ts` currently has no `roles` list, and the comment in that
file spells out why that matters:

> a destination without `roles` is visible to every signed-in role including the `crew`
> fallback, so anything sensitive must gate explicitly rather than rely on being
> unlisted.

Gate it to everyone except `spejder`. Prefer deriving that from `ALL_ROLES` minus
`spejder` so a future role is not silently excluded from a pane it should have.

**A hidden nav item is not access control.** The route guard and the endpoints are what
enforce this; the nav gating is only so spejdere are not shown a door they cannot open.

## Implementation

- `config/roles.ts`: `allRolesExcept(...excluded)`, so the gate reads as "everyone but
  spejder" rather than as a list of the six roles that exist today. A role added later gets
  the pane by default instead of being silently left out — the failure mode of an allow-list
  nobody remembers to update. The places that genuinely want an allow-list (SOS) keep
  spelling one out.
- `config/navigation.ts`: `roles: allRolesExcept('spejder')` on the contacts destination,
  with a comment naming all three enforcement points and which one is actually security.
- `config/navigation.spec.ts` and `router/rolegate.spec.ts`: new.

**Extracted `roleGate` into `router/gates.ts`.** The role check was inline in
`router/index.ts`, which cannot be imported by a unit test — it calls `createWebHistory()`
at import time, so it needs a DOM. That is the same reason the device and install gates
already live in `gates.ts`, and PRD 007 promotes this gate from cosmetic to load-bearing, so
it earns the same treatment. The guard body now reads as six numbered steps throughout.

**Documented why a null role falls through.** The previous inline condition
(`to.meta.roles && session.role && ...`) allowed the route when `role` was null, and it was
not obvious whether that was deliberate. It is worth keeping: `role` is null while the
session is resolving and on a cold offline start, so redirecting then would bounce a
legitimate user off a page they are entitled to — the blank-screen class of bug task 090 was
about. The endpoint still refuses, so the cost of falling through is an empty pane, not a
disclosure. That reasoning is now in the code rather than implied by an `&&`.

Tests worth noting:

- the newly-added-role test asserts the gate equals `allRolesExcept('spejder')`, so adding a
  role to `ALL_ROLES` without thinking about contacts does not silently exclude it;
- a regression guard asserts the shared content pages (maps, rulebook, updates, schedule,
  faq, privacy) stay ungated — the obvious mistake while editing this file;
- the SOS gate is re-asserted, since it now flows through the same function;
- a "never throws for any role/route combination" case, per task 090: nothing in the guard
  may reject, because that aborts the navigation and leaves a blank white screen.

The profile route test uses the same gate under the name `contact-person`, so task 167 only
has to register the route with `roles` and the deep-link refusal is already covered.

## Acceptance Criteria

- [x] `contacts` destination carries a `roles` list excluding `spejder`.
- [x] The router guard refuses `/contacts` and `/contacts/:personId` for a spejder
      session, including on a deep link or a direct URL — `roleGate` is applied from
      `meta.roles`, which `destinationRoutes` copies from the destination.
- [x] A spejder is redirected somewhere sensible rather than shown an error page
      (`{ name: 'maps' }`).
- [x] Tests: `visibleDestinations('spejder')` excludes contacts; every other role
      includes it.
- [x] Test: navigating directly to `/contacts` as a spejder does not render the pane —
      covered as a unit test of the extracted gate rather than a full router integration
      test, which would need a DOM and a mocked session for no extra confidence.

## Progress Log

- 2026-08-31 21:05 — Picked up. Added `allRolesExcept` and gated the destination.
- 2026-08-31 21:10 — `npx` is not available on the host; the node toolchain lives in the `ui`
  container. Ran vitest and vue-tsc via `docker compose exec ui`.
- 2026-08-31 21:15 — Could not test the route refusal without a DOM, so extracted the inline
  role check into `roleGate` in `gates.ts`, alongside the gates already factored out for the
  same reason.
- 2026-08-31 21:20 — Documented the null-role fall-through, which the previous `&&` left
  ambiguous. Kept the behaviour: redirecting during session resolution is the blank-screen
  bug from task 090.
- 2026-08-31 21:25 — ✅ All criteria met. 71 frontend tests pass, `vue-tsc --noEmit` clean.
