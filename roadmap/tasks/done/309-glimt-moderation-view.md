# 309 — GlimtModerationView — Team-section moderation UI

**Status:** done
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:** agent
**Started:** 2026-09-17
**Completed:** 2026-09-17

## Description

PRD 019 §7. Not a separate tool — Team-section members are in the app, on their phones, at the
event. `/glimt/moderation`, reusing the feed components with three differences: every scope is
included, each card shows its audience and report count, and the overflow carries *Skjul* /
*Vis igen*. Default sort puts reported-and-not-yet-reviewed first.

Visible only when the Team section is assigned — the route is gated, and the nav entry does
not appear otherwise. The client gate is convenience; the server check (task 300) is the
actual control.

## Acceptance Criteria

- [x] `/glimt/moderation` route, gated on the caller being in the Team section
- [x] `GlimtModerationView.vue` reuses `GlimtCard` rather than duplicating it
- [x] Each card shows audience and report count
- [x] Hide / unhide actions call the task 308 endpoints and update optimistically
- [x] Reported-first ordering (the BFF's, deliberately not re-sorted — see the log)
- [x] Not reachable or visible for a non-Team member
- [x] `npm run test:unit` and `npm run build` pass

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.

- 2026-09-17 — **A missing piece first: the client had no way to know it moderates.** Moderation is
  decided by `person.sectionSlug`, which is deliberately kept out of the session (task 300) and
  never sent to the client — so there was nothing to gate a route or draw a link on. The two
  options were a client-side probe of `/api/glimt/moderation` reading the 403, or a server-issued
  flag. The probe is the mistake the contacts prefetch already made, and it cost a 403 per
  foreground for every participant (see `hasContactsPane` in `vue/src/config/roles.ts`).

  So `moderates_glimt` was added to `identityResponse` (`go/cmd/api/auth.go`), computed from
  **`app.isGlimtModerator` — the same function the three moderation handlers gate on**, so the
  answer and the enforcement cannot drift. Set on `/api/me` *and* on both login paths
  (`/auth/verify`, `/auth/choose`): the client marks the session ready at login and does not
  re-ask, so without that a Team member would have had no moderation entry until their next
  foreground refresh. Not `omitempty` — absent and false must look the same to a client, and with
  non-moderators being almost everyone the field would otherwise be nearly always missing and easy
  to mistake for unimplemented. Four Go tests, including one asserting the field is **not** the
  permission: revoke the assignment and the very next request is 403 whatever the client was last
  told.

- 2026-09-17 — **`meta.moderator`, not `meta.roles`.** The route gate is a new `moderatorGate`
  (step 7 of the guard, after roles) rather than a role list, and this is the single most important
  decision in the task: **every Team member is `crew` as far as roles go — and so are the kitchen,
  PR, and every crew account whose section could not be classified.** `meta.roles: ['crew']` would
  have handed the widest read in the service — a queue with no visibility filter, containing every
  photograph in the event — to a large population of accounts, and would have looked correct doing
  it. `moderatorgate.spec.ts` asserts that no role satisfies the gate.

  The gate **inverts `roleGate`'s unknown-answer behaviour**, which is worth flagging because the
  two now sit next to each other and disagree on purpose. `roleGate` falls through on a null role,
  since bouncing a legitimate user mid-resolution is the blank-screen class of bug (task 090).
  Here `moderatesGlimt` is false both for a non-moderator and when we could not ask (offline cold
  start — the remembered identity carries no such flag), and falling through would render a queue
  that cannot load, because the view needs the network by definition. Refusing costs a moderator
  one reconnect and saves everyone else a screen they should not see.

- 2026-09-17 — **Reused `GlimtCard` by making it hold no opinion, rather than by adding a
  `moderating` flag.** It gained an `actions?: GlimtAction[]` prop and two slots (`meta`,
  `badges`). The view passes `moderationActions(entry)`, its own author line and its own report
  badge. A forked moderation card would have drifted from the feed's within a week; a `moderating`
  boolean would have put moderation logic inside the component every participant renders a hundred
  of.

  `moderationActions` is in `glimtPresentation.ts` with nine tests, because the decisions are not
  cosmetic:

  - **Exactly one direction, ever** — *Skjul* on a visible glimt, *Vis igen* on a hidden one, never
    both. A menu with both makes the moderator work out which is a no-op, at 03:00, on a
    photograph somebody has complained about.
  - **Hiding is not marked destructive**, though it looks like the destructive one. The row and
    media survive and *Vis igen* restores them; red styling would push a moderator towards
    hesitating, and PRD 019 §6 wants hiding cheap enough to be the immediate answer to an ambiguous
    report, because the public scope cannot afford waiting for certainty.
  - **No *Anmeld*** — reporting is how you ask a moderator to look, and one is already looking.
  - **No moderator delete.** Destroying another member's photograph is not a power this feature
    grants anyone: the author deletes, the Team hides, retention does the rest. `own` is still
    honoured, so a Team member's *own* glimt in the queue offers the author's *Slet*.

- 2026-09-17 — **Ordering: the BFF's, not re-sorted.** Task 308 already returns triage order
  (reported-and-not-hidden first, then by count, then newest). A client sort would be a second
  opinion for the first to disagree with, and it would actively hurt — after an optimistic *Skjul*
  the card must **stay put** so a misclick can be undone, where a live re-sort would make it jump
  out from under the moderator's thumb. Asserted, including across a hide.

  Hidden glimt also **stay listed** rather than leaving the queue: a reversal has to be possible,
  and a malicious report is only visible if the thing it was aimed at still is.

- 2026-09-17 — **Found and fixed a real hole while writing the privacy test.** The queue is the
  only payload in the app that names an author (deliberately — a report cannot be answered against
  an anonymous author), so it must never reach disk. It lives in its own `moderationQueue` field,
  which `persist()` does not write, and is dropped on unmount.

  But `ModerationGlimt extends Glimt`, so **TypeScript structurally accepts an array of queue
  entries — author names and all — anywhere a `Glimt[]` is wanted**, and `StoredPayload.glimt` is a
  `Glimt[]`. `JSON.stringify` writes what is on the object, not what its declared type admits. One
  assignment would have put author names into localStorage with nothing failing to say so, and my
  first draft of the test proved it by doing exactly that.

  Fixed at the write boundary with a `toStoredGlimt` whitelist in `persist()`, so the promise holds
  even for a widened object. The type system cannot express "no *more* than these fields", so the
  code does. This is the second time in this PRD that a structural guarantee turned out to depend
  on a client behaving; both are now tests.

  `glimtModerationNotCached.spec.ts` (16 tests) covers it behaviourally *and* structurally,
  following `profileNotCached.spec.ts`: the persisted payload contains no author name, the
  `StoredPayload` type and the persist literal mention no author or moderation field, and no
  service worker rule matches the endpoint.

- 2026-09-17 — Other behaviour worth knowing:

  - **A 403 is a state, not an error** (`moderationForbidden`), and it drops whatever the queue was
    holding — that content was fetched under an assignment the caller no longer has. Showing a
    retry button for something retrying cannot fix would be a lie.
  - **A failed hide reverts the flip** in both the queue and the cached feed copy. Telling a
    moderator a photograph is down while it is still up is the one lie this screen must not tell.
  - **A 404 mid-hide removes the row**: the author deleted it while the moderator was looking, which
    is not their failure and is effectively the outcome they wanted.
  - The view **states the Team section's reach on the screen that has it** — PRD 019 §6 requires
    that reach to be disclosed rather than discovered. The composer (317) and privacy page (321) say
    it to members; this says it to the moderator, along with the fact that hiding does not delete.
  - No nav slot: reached from a button on the Glimt feed, drawn only when `moderates_glimt` is
    true. Task 320's test asserts no `glimt-moderation` destination exists, so this cannot quietly
    acquire one.

  875 Vue tests + the Go suite passing; `gofmt`, `type-check` and `build` clean. Code-splitting
  works out: `GlimtCard` moved into a shared chunk and `GlimtView` shrank from 42 kB to 13 kB, so
  participants do not download the moderation view.

  **Not exercised on a device, and never against a real report.** The whole flow — a member
  reporting, the glimt auto-hiding, a Team member finding it in the queue and deciding — has only
  ever run against fixtures.
