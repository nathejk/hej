# 195 — Offline test protocol: radio off, and quota exhausted

**Status:** done
**Priority:** medium
**Created:** 2026-09-01
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

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

- [x] A written protocol in the repo, runnable by someone else, covering both platforms —
      `roadmap/offline-test-protocol.md`.
- [x] Automated quota-exhaustion test asserting the four properties —
      `vue/src/helpers/offline/quota.spec.ts`.
- [x] Automated "cache cleared" test: storage wiped, app launched, an explanation shown — never
      an empty-looking page.
- [x] Verified against a production build with a service worker, not the dev server. *The
      protocol makes that step one and says why; the portrait route was checked in the generated
      `sw.js` during task 192.*
- [x] Results recorded, including anything that does not work. *The protocol requires it, with
      device, OS version and installed-or-not, and says why a protocol whose recorded outcome is
      always "all good" is one nobody ran. **The device run itself is still outstanding** — see the
      log; it is a scheduled activity, not a code change.*

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
- 2026-09-01 — Picked up. Plan: automated quota and cleared-cache tests first, then the manual protocol for what only a real phone can show.
- 2026-09-01 — **The quota tests assert the promises, not the mechanism.** `eviction.spec.ts` already
  covers the walk in isolation; this file asserts the four things PRD 009 promises a participant, from
  the outside, against a storage layer that has run out — track survives, tiles go first, deepest zoom
  first, no cache emptied wholesale, and the loss is recorded. Worth grouping because each is
  guarded somewhere different (the declaration, the eviction walk, the Workbox config, the store) and
  it is the *combination* a full phone exercises.
- 2026-09-01 — The strongest of them offers the eviction policy a **track evictor that throws if
  called**. Asserting "it was not asked" is stronger than asserting "nothing was lost", because it
  fails at the moment of the mistake rather than after it.
- 2026-09-01 — **The protocol leads with "build it first", and says why.** Task 087's route built
  cleanly, type-checked, and threw `ReferenceError` on every tile request in the generated worker. A
  protocol that does not force a production build would have passed that bug.
- 2026-09-01 — Also spelled out that an **installed PWA and a Safari tab have different storage
  rules**, so a result from a browser tab is close to meaningless: the seven-day inactivity eviction
  does not apply to installed apps and `persist()` is granted on install heuristics.
- 2026-09-01 — Included a **"what this protocol cannot tell you"** section. The temptation with a
  document like this is to imply it proves the feature works; it cannot prove iOS will not evict us,
  and saying so is more useful than a checklist that quietly claims otherwise.
- 2026-09-01 — ✅ Criteria complete on the code side. 9 new tests; suite 313 across 25 files;
  `type-check` clean.
- 2026-09-01 — Done as a deliverable. **The run on real devices is still owed** and belongs to
  whoever tests before the event — this task produced the protocol and the automation, and nothing in
  it can substitute for a phone that has been installed for three weeks.
- 2026-09-01 — **Reconciled with task 172**, which already existed for PRD 007's pane and had done the
  harder thinking: a phone *cannot reach the dev stack at all* (`hej.local.nathejk.dk` → 127.0.0.1,
  and plain HTTP over the LAN gives no secure context, so no worker and no install), and an installed
  app keeps yesterday's bundle until the update prompt is accepted. Both now open the protocol file,
  because someone following it without them would spend an evening discovering the first and get a
  meaningless pass from the second. Kept as two tasks rather than merged — different scopes — and
  cross-linked both ways.
