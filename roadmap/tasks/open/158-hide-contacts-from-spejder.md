# 158 — Hide the contacts pane from spejdere

**Status:** open
**Priority:** high
**Created:** 2026-08-31

## Description

Spejdere do not get the contacts pane (PRD 007 §6). The `contacts` destination in
`vue/src/config/navigation.ts` currently has no `roles` list, and the comment in that
file spells out why that matters:

> a destination without `roles` is visible to every signed-in role including the `crew`
> fallback, so anything sensitive must gate explicitly rather than rely on being
> unlisted.

Gate it to `['bandit', 'gøgler', 'postmandskab', 'guide', 'samarit', 'crew']` — i.e.
everyone except `spejder`. Prefer deriving that from `ALL_ROLES` minus `spejder` so a
future role is not silently excluded from a pane it should have.

**A hidden nav item is not access control.** The route guard and the endpoints are what
enforce this; the nav gating is only so spejdere are not shown a door they cannot open.

## Acceptance Criteria

- [ ] `contacts` destination carries a `roles` list excluding `spejder`.
- [ ] The router guard refuses `/contacts` and `/contacts/:personId` for a spejder
      session, including on a deep link or a direct URL.
- [ ] A spejder is redirected somewhere sensible rather than shown an error page.
- [ ] Tests: `visibleDestinations('spejder')` excludes contacts; every other role
      includes it.
- [ ] Test: navigating directly to `/contacts` as a spejder does not render the pane.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §6 / §8.
