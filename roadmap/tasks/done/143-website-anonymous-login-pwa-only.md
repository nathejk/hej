# 143 — The website is anonymous: login only in the installed app

**Status:** done
**Priority:** high
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

## Description

Reported by the maintainer, 2026-08-30, after task 142 fixed the detection bug: in Safari the
install wall now appears correctly — but tapping the wall's secondary action ("Fortsæt i
browseren") led to `/welcome` and the login flow. That is wrong in kind:

> the website is anonymous, no login — login is only for pwa. When on website, mobile users
> should have a call-to-action box in top saying something like "install as app", this should
> link to /install

So the browser is not a degraded way to *be a user*; it is a **public website**. The app —
login included — exists only once installed.

## Changes

- **The gate has no override.** Mobile + not standalone → `/install`, unconditionally. Asking
  for `/welcome` by hand in a browser lands on the wall too.
- **`install.store`'s `continueInBrowser` and its `localStorage` key are deleted**, not
  relabelled. Nothing was left for it to unblock.
- **The wall's secondary action is now "Gå til hjemmesiden"**, a plain `<a>` to the website
  (a separate document, so not a router link).
- **The website's call to action is a box, not a strip of text** — "Installér som app" →
  `/install` — shown only on devices that qualify for the PWA, since it is the one action a
  phone visitor has on an otherwise anonymous page.
- **The profile page's install row is removed** (task 121 added it as the way back from the
  override). That page is only reachable inside the installed app, so the row could only ever
  have said "Installeret".

## What this gives up, deliberately

PRD 005 §6 required **"No lockout — every gate has a user-reachable escape hatch. This is a
safety app."** That is no longer true in the sense it was written: a phone that genuinely
cannot install a PWA now has no way to sign in.

Accepted, and the reasoning is worth keeping because it is the one part of this change that
could bite:

- Such a device has no Web Push, no reliable service worker and no background sync either, so
  the app's core features were already beyond it — the browser baseline in `.rules` says as
  much.
- The common way to arrive unable to install is a Facebook/Instagram in-app browser, and that
  is recoverable: the webview instructions (task 120) tell the user to reopen the link in
  Safari or Chrome.
- Every gate still has a reachable **exit** — the website — even though it no longer has an
  entrance.

The consequence is that the install instructions' clarity *is* now the mitigation for
drop-off, where previously the hatch was. That is recorded in PRD 005 §8's risk list.

Also worth noting: the deleted flag was a live suspect while diagnosing task 142, precisely
because it was invisible — a device that had ever tapped it behaved differently forever with
nothing in the UI to say so. Removing it removes that class of report.

## Acceptance Criteria

- [x] A browser tab on a phone/tablet always lands on `/install`, including when asking for
      `/welcome` directly
- [x] No persisted "continue in browser" override exists anywhere in the client
- [x] The wall links to the anonymous website, and the link does not promise the app
- [x] The website shows an "Installér som app" **box** linking to `/install`, only on devices
      that qualify for the PWA
- [x] The profile page's install row is gone
- [x] PRD 005 updated where it stated the opposite: §1, §3, §5 edge cases, §6 (requirement and
      the no-lockout non-functional), §8, §9 metrics, §10, §11 (new decision + the two it
      supersedes), §12 Q6
- [x] Task 121's log records that it is superseded and why
- [x] `vue-tsc`, `npm test` and `npm run build` clean

## Progress Log

- 2026-08-30 — Task created from the maintainer's clarification.
- 2026-08-30 — Implemented. The gate simplification is the substantive part: with the override
  gone, step 3 is a single unconditional rule, which is also why the termination test lost a
  dimension and gained a stronger assertion — "no state and no URL gets a browser tab to the
  login flow", checked across four entry points and both session states.
- 2026-08-30 — Replaced the old escape-hatch test rather than deleting it, so the file still
  says something about that path: it now asserts the opposite of what it used to, which is the
  honest record of a reversed decision.
- 2026-08-30 — ✅ `vue-tsc`, 37 tests and `npm run build` clean.

  Not verified from here (no browser): that the website's CTA box renders correctly on a phone
  and stays hidden on a laptop, and that the wall → website → wall round trip works. Both are on
  task 139's matrix.
