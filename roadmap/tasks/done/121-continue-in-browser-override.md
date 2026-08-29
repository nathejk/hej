# 121 — Escape hatch: "Fortsæt i browseren"

**Status:** done
**Priority:** high
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

## Description

PRD 005 §6/§11. Add the persisted **"Fortsæt i browseren"** override to the install wall
(task 119): a low-prominence action that records `continueInBrowser` in the install store
(task 117) and lets normal login proceed in this browser, install wall bypassed, until the
user clears site data.

It is the **only** non-install affordance on the wall. PRD 005 §11 (2026-08-30) dropped the
earlier low-prominence link to the "desktop version": `/desktop` is a placeholder (task 123),
so pointing a stranded user at it helps nobody, and two adjacent escape links sitting next to
each other get confused with one another — the user picks the wrong one and ends up further
from the app than before. One escape, one meaning.

## Why this is load-bearing

This is not a courtesy for power users. PRD 005 §11 chose an aggressive detection tie-break —
**ambiguous devices classify as mobile** (task 116) — precisely *because* this hatch exists.
The reasoning runs in both directions: the tie-break is only safe while a misclassified
desktop user is one tap from getting on with it, and a false negative in detection must never
lock a participant out of what is, during the event, a **safety app**. PRD 005 §6
(Non-Functional) states it as a rule: *no lockout — every gate has a user-reachable escape
hatch.*

That gives the prominence question two hard edges, and the design has to sit between them:

- **Not so hidden that support cannot talk a user through it by phone.** "Scroll to the
  bottom, tap the grey text that says Fortsæt i browseren" has to be a sentence someone can
  say to a stranger in a noisy field. No hidden gesture, no tap-five-times, no dev console.
- **Not so prominent that it becomes the default path.** If it reads as an equal alternative
  to installing, a meaningful share of users will take it and the install wall stops working —
  PRD 005 §9 targets < 2% of authenticated sessions using it, and ≥ 95% originating from
  standalone.

In practice: a plain text-style `Button` (shadcn-vue `variant="link"` or `"ghost"`), placed
below the install content and visually subordinate to it, with one short line of Danish
explaining the trade-off — that some features work poorly or not at all in a browser tab
(notifications on iOS need a home-screen app at all; offline behaviour is degraded). The user
should be able to make the choice knowingly rather than discovering the consequences later.

## Scope boundaries

- The override is **per browser, per device**, persisted under `hej.install.*`. It is not a
  user setting and must not be synced to the BFF.
- It unblocks the **install** gate only. Auth, roles and onboarding gates still apply — the
  user lands in the normal login flow, not past it. The install gate is UX; the BFF continues
  to authorize every endpoint independently (PRD 005 §8).
- There must be a way back: once overridden, the wall no longer appears, so surface a way to
  reach it again (the natural home is PRD 003's profile page device section, which already
  holds per-device preference rows). At minimum, do not make the override irreversible.

**Rate-limiting or expiry is deliberately unresolved.** PRD 005 §12 Q6 asks whether the hatch
needs rate-limiting or an expiry (e.g. re-ask after 7 days) so it does not become the default
path. Do **not** invent a policy in this task — ship the plain persisted override, and store
enough alongside it (a timestamp) that an expiry can be added later without a migration or a
second key.

## Acceptance Criteria

- [x] The wall carries a single "Fortsæt i browseren" action that sets `continueInBrowser` and
      routes the user into the normal login flow
- [x] The override persists across reloads and app restarts under `hej.install.*`, with a
      timestamp stored alongside it so a future expiry (§12 Q6) needs no migration
- [ ] The router's install gate honours it; the auth, role and onboarding gates are unaffected
      — **not yet true: the gate is task 137.** See the log.
- [x] It is the **only** non-install affordance on the wall — no "desktop version" link (§11)
- [x] Visually subordinate to the install action (link/ghost button, below it), but reachable
      by plain description over the phone: no hidden gesture, no repeated taps, no console
- [x] One short Danish line states the trade-off (degraded notifications and offline
      behaviour in a browser tab)
- [x] The override is reversible — there is a documented way back to the wall, and it is not a
      one-way door
- [x] No rate-limiting or expiry implemented; a comment points at PRD 005 §12 Q6 as open
- [x] `npm run type-check` clean

## Depends on

- **Task 117** — `continueInBrowser` and its persistence.
- **Task 119** — the wall this lives on.
- The router guard task (PRD 005 §10) must read `continueInBrowser`, or the override changes
  nothing. If that task lands later, this one is not finished in practice — note it in the log
  rather than closing on the UI alone.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
- 2026-08-30 — Picked up.
- 2026-08-30 — **Hatch added to the wall**, as a shadcn-vue `variant="link"` button below the
  install content with one line of Danish stating what is lost. Three decisions:

  - **The timestamp *is* the stored value.** Task 117 wrote `'1'`; the key now holds
    `String(Date.now())` and presence-not-equality is what the store reads. Preferred over a
    second `…-at` key: with two keys a future expiry policy has to decide what an override with no
    timestamp means, which is exactly the migration this was meant to avoid. No expiry is
    implemented — the comment names PRD 005 §12 as the open question.
  - **The trade-off copy is specific rather than a generic warning.** "Du får ingen beskeder fra
    løbet på iPhone, og den virker ikke uden signal" — both are true consequences (iOS Web Push
    requires a home-screen app; the service worker is what makes the app work without coverage). A
    vague "nogle funktioner virker måske ikke" would be ignored, and the point is that the choice
    is made knowingly.
  - **The way back is a row on the profile page's "På denne enhed" section**, which PRD 003 had
    already left a comment reserving for exactly this. Without it the override is a one-way door:
    the wall never reappears, so the only remaining route to installing would be clearing site
    data — which also signs the member out. The row reads `isStandalone()` for its status rather
    than store state, since "is this an installed launch" is a fact about the current window, not
    something we should be recording.
- 2026-08-30 — ✅ UI and persistence done; `vue-tsc` clean, 27 tests still pass. **One criterion is
  deliberately left unchecked**: the router install gate does not exist yet (task 137), so today
  the hatch sets a flag nothing reads. Per this task's own instruction, that is recorded rather
  than closed over — the hatch is not functionally an escape until 137 lands, and 137 must read
  `install.continueInBrowser`.
