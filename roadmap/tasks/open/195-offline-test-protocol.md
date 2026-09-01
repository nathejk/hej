# 195 — Offline test protocol: radio off, and quota exhausted

**Status:** open
**Priority:** medium
**Created:** 2026-09-01

## Description

PRD 009 §6, §9. The PRD's claims are all of the form "this works when there is no network" and
"this data is never sacrificed". Neither is verifiable from a unit test alone, and neither is
something to discover during an event.

Two things to establish:

**1. Radio off, on real devices.** Every cached dataset opens and works with the connection
genuinely disabled — not DevTools' offline toggle alone, which does not reproduce iOS's
behaviour. Both platforms in the baseline: iOS/iPadOS Safari 16.4+ as an installed home-screen
app, and Android Chrome. Task 090's lesson is the specific thing to guard: the app must not
white-screen or look empty; a missing cache must produce an explanation.

**2. Quota exhausted.** This is the test that keeps task 186 honest, because with no registry
enforcing the priority order the order is otherwise just a document. Fill the origin, then
assert:

- unrecoverable data (the location track) survives;
- tiles are what got dropped, highest zoom first;
- no cache was emptied wholesale;
- the readiness view says what was dropped rather than showing a silent zero.

Where CI can do it, do it in CI; where it cannot, write the manual protocol down so it is
repeatable by someone who did not build it. A checklist a person can follow at a desk in ten
minutes is worth more than an elaborate harness nobody runs.

## Acceptance Criteria

- [ ] A written protocol in the repo, runnable by someone else, covering both platforms.
- [ ] Automated quota-exhaustion test asserting the four properties above.
- [ ] Automated "cache cleared" test: storage wiped, app launched, an explanation shown — never
      an empty-looking page.
- [ ] Verified against a **production build** with a service worker, not the dev server —
      several bugs in this area have only appeared in the generated `sw.js` (task 087's log).
- [ ] Results recorded, including anything that does not work, rather than only the passes.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
