# 285 — Rewrite the freshness convention comment for the multiplexed check

**Status:** open
**Priority:** medium
**Created:** 2026-09-16
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 017 phase 1 — deliberately *in* phase 1, not cleanup. `useFreshnessLoop.ts` opens
with a numbered convention for "the next dataset that needs to stay current", and three
of its points become actively wrong the moment `useSyncLoop` lands:

- **§1** points at `GET /api/contacts/version` as the reference and describes one
  version endpoint per dataset. The reference is now `GET /api/sync`, and the whole
  point of PRD 017 is that six datasets get *one* request.
- **§3** says "Three trigger points, and no more". There are now four — foreground,
  interval, `online`, and an explicit user request (the manual refresh, task 282) — plus
  whatever task 280's device check adds for iOS resume paths.
- **§4** says to "add another value rather than reusing this one" for a second
  dataset's interval. That advice produced exactly the N-loops shape the PRD replaces.
  The interval is now served in the sync response itself, because the 02:00 load lever
  must take effect on the next check without a config refetch.

A stale convention comment is worse than none: it is the thing the next person copies.
What stays true and must survive the rewrite: opaque per-caller versions from a
projection read, version in the body not the header, no traffic while hidden, zero
disables the interval only, metadata ahead of images, push is not an option for
invalidation, and replace-do-not-merge.

Also document the debounce (task 281) and `unavailable` (task 284) — both are decisions
the next reader will otherwise re-lit­igate.

## Acceptance Criteria

- [ ] §1 describes the multiplexed check and points at `/api/sync`.
- [ ] §3 lists the real trigger points, including the manual refresh and task 280's
      findings.
- [ ] §4 describes the served interval and drops the add-another-value advice.
- [ ] Debounce and its manual-refresh exemption documented.
- [ ] `unavailable` vs absent documented on the client side.
- [ ] The still-true points survive, with their reasoning intact.
- [ ] No stale references to `/api/contacts/version` as *the* reference anywhere in
      `vue/src` comments.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1. Do this after tasks 280–284, so
  the comment describes what shipped rather than what was planned.
