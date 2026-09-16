# 292 — Retire /api/contacts/version

**Status:** open
**Priority:** low
**Created:** 2026-09-16
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 017 phase 2 (cleanup). `GET /api/contacts/version` was the reference implementation
for the freshness convention and is superseded by `/api/sync`, which returns the same
contacts version among five others.

**It stays for one release after the client stops calling it**, then goes. An installed
PWA can be running an old service-worker-cached bundle, and removing the endpoint the
moment the new client ships would break freshness for anyone who has not updated — the
users least likely to notice and most likely to be mid-event.

Its comment documents itself as the shape other panes should copy. That documentation
duty passes to `/api/sync` (task 285 does the client side), so removing it must not
delete the reasoning — the two traps (wrongly-stable is silent, wrongly-unstable is
expensive) live on in `contacts.go` and `mapversion.go` and must survive.

`contactsVersionFor` itself **stays** — `/api/sync` depends on it. This task removes the
HTTP route and handler only.

## Acceptance Criteria

- [ ] Confirmed no client code path calls `/api/contacts/version`.
- [ ] At least one release has shipped with the sync loop before removal.
- [ ] Route and handler removed from `routes.go` / `contacts.go`.
- [ ] `contactsVersionFor` and its `versionCache` retained and still tested.
- [ ] OpenAPI annotations for the removed endpoint deleted.
- [ ] `/api/config`'s `contacts_poll_seconds` removed with it, along with the now-dead
      `contactsPollSeconds` in `vue/src/config/runtime.ts` and its remembered-value key —
      nothing has read the client value since task 288, and the two halves belong together
      (noted in task 289).
- [ ] `contactsversion_test.go` reduced to the derivation's own tests.
- [ ] The convention's reasoning still documented somewhere it will be found.
- [ ] All four Go gates green.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 2.
