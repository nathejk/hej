# 013 — Placeholder views + navigation config

**Status:** done
**Priority:** high
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Create the placeholder views and the declarative navigation config that drives
the bottom nav, per `roadmap/prd/001-hej-nathejk-event-app-skeleton.md`.

Known sections: **Maps, Contacts, Rulebook, Event Updates**. Add enough
additional role-gated stubs so that at least one role (spejder / bandit /
postmandskab / guide / samarit) exceeds 5 allowed destinations and exercises the
burger overflow. Per-role destination sets/order/icons are partly an open item —
use sensible placeholders that are easy to adjust.

Depends on: 001, 008 (roles available via session).

## Acceptance Criteria

- [x] `src/views/MapsView.vue`, `ContactsView.vue`, `RulebookView.vue`,
      `UpdatesView.vue` + extra stubs (`ScheduleView`, `SosView`, `FaqView`)
      exist and are lazy-loaded; all render a shared `PagePlaceholder`.
- [x] `src/config/navigation.ts` lists destinations with Lucide icon, label,
      route, and role rule; `visibleDestinations(role)` filters by role.
- [x] Routes are generated from the nav config in `src/router/index.ts`
      (single source for path/name/roles); `/` redirects to `maps`.
- [x] At least one role's set exceeds 5: spejder/bandit see 6 (maps, contacts,
      rulebook, updates, schedule, faq) and service roles see 7 (+ SOS) — both
      trigger the burger overflow.

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 19:20 — Implemented `config/navigation.ts` (7 destinations, Lucide icons, `visibleDestinations` role filter), a shared `components/PagePlaceholder.vue`, and 7 thin `*View.vue` files. Rewrote `router/index.ts` to generate destination routes from the nav config (`viewLoaders` map keyed by name), redirect `/`→maps, and pass `meta.roles` to the guard. Deleted the now-unused `HomeView.vue` (its healthcheck demo is no longer needed).
- 2026-07-30 19:21 — Decision: keep label/icon/roles in `navigation.ts` and component/path in the router (loaders map) — avoids Vite dynamic-import-with-variable pitfalls while keeping roles single-sourced.
- 2026-07-30 19:21 — ✅ Verified in `node:20-alpine`: `npm run build` (all views code-split) + `npm run type-check` clean.
- 2026-07-30 19:21 — Completed. Nav config + routes ready for the shell (010) and BottomNav (011).
