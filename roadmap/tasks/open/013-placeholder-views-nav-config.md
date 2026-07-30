# 013 — Placeholder views + navigation config

**Status:** open
**Priority:** high
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `src/views/MapsView.vue`, `ContactsView.vue`, `RulebookView.vue`,
      `UpdatesView.vue` + extra role-gated stubs exist and are lazy-loaded
      (Home/first may be eager).
- [ ] `src/config/navigation.ts` lists destinations with icon, label, route, and
      role/visibility rule. Icons are **Lucide** (`lucide-vue-next`) components
      per `.rules` — no PrimeIcons/other sets.
- [ ] Routes registered in `src/router/index.ts` with `props: true` where
      relevant.
- [ ] At least one role's allowed set exceeds 5 to demonstrate the burger.

## Progress Log

- 2026-07-30 13:12 — Task created.
