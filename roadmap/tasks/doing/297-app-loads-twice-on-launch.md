# 297 — The app loads twice on launch, one second apart

**Status:** open
**Priority:** medium
**Created:** 2026-09-16
**Picked up by:**
**Started:**
**Completed:**

## Description

Found by the resume probe (task 280) on an installed iOS 18.7 home-screen PWA. Launching
the app produces **two document loads about a second apart**:

| tid | event | visibility | load |
|---|---|---|---|
| 23.36.59 | `mount` | visible | `5lp5` |
| 23.37.00 | `mount` | visible | `snh3` |

The `load` column is a random id minted once per document, added precisely to answer this.
**Two different ids means two documents** — so this is not a view mounting twice, and in
particular `useSyncLoop` is not being registered twice (PRD 017 §6's "exactly one app-level
loop" is intact). It is an ordinary full reload, one second after launch.

Reproduced on two separate launches (23.36.59/23.37.00 and, in the previous run,
23.24.25/23.24.26), so it is systematic rather than a one-off.

## Why it is worth fixing

Every launch pays for the app twice: two shell boots, two `/api/config` fetches, two
`/api/sync` checks, two session resolutions. That is small in isolation — and it lands
squarely on the path we now know is common, because the same probe run showed **iOS
discarding this app after roughly 51 seconds hidden**. During an event, a patrol's phone
cold-starts often, on a mobile link, with the shell already precached. Paying twice for
each of those is the opposite of what PRD 017 was written to achieve.

There is also a correctness edge: a reload one second in can interrupt whatever the first
document had started — an in-flight `/api/sync`, a session refresh, a store hydration — and
that is a race nobody designed.

## Candidate causes, in the order worth checking

**All in-app causes are now eliminated by reading the code and the built output (2026-09-16):**

1. ~~**Service worker taking control.**~~ The generated `sw.js` calls `self.skipWaiting()` **only** on
   an explicit `SKIP_WAITING` message, and has no `clients.claim()`. The reload in the
   `virtual:pwa-register` module is guarded: `wb.addEventListener('controlling', e => e.isUpdate &&
   location.reload())`, and that listener is only registered once `waiting` has fired. So a reload
   needs (a) an update waiting **and** (b) `messageSkipWaiting()`, whose only caller is
   `applyUpdate()` — wired to the user tapping `UpdatePrompt`. On paper this cannot fire
   unattended. *If the measurement says `reload`, this paper is wrong and that is the finding.*
2. ~~**A full-document redirect on the boot path.**~~ `window.location.replace` appears exactly once
   in `vue/src` (`router/index.ts:154`), on the `LEAVE_APP` branch — desktop visitors only, and it
   goes to `/desktop.html`, not back to the app. The dev bootstrap's `replace` is stripped from
   production (`scripts/check-no-dev-layer.sh`). Every other gate outcome is a *router* redirect,
   which is same-document.
3. ~~**`index.html`.**~~ No script, no meta refresh; the only markup is the static boot fallback.
4. ~~**Navigation fallback.**~~ Ordinary SPA config, with a denylist for `/desktop.html` only.

What remains is **platform behaviour on iOS launch**, which cannot be established by reading this
repo. The probe now records the three facts that decide it, on every `mount`:

| field | meaning |
|---|---|
| `nav` | `navigate` \| `reload` \| `back_forward`, from Navigation Timing |
| `path` | which URL that document loaded |
| `sw` | whether a service worker controlled the document at mount |

### What each outcome means

- **`nav: reload` on the second document** — something in the page reloaded it, so candidate 1's
  reasoning above is wrong somewhere. Most likely an update was applied without the user tapping;
  check whether `onNeedRefresh` → `UpdatePrompt` is auto-accepting, and whether an SW activated
  between the two loads (`sw` should read `false` then `true`).
- **`nav: navigate` on both** — two independent navigations, i.e. iOS launching the app twice. Then
  compare `path`: two different paths means a launch at `start_url` followed by a restore of the
  last URL, and the fix is to make that a router navigation rather than a document load. The same
  path twice is pure platform behaviour and may be unfixable — in which case the honest outcome is
  to record it here and stop, rather than keep hunting.
- **`nav: back_forward`** — a history traversal, which on iOS accompanies some app resumes.
- **`sw: false` then `true`** — the first load was uncontrolled and the second was not, which is the
  signature of a worker activating in between.

## Acceptance Criteria

- [ ] The cause identified, and named in this task.
- [ ] Confirmed with the probe: a launch produces **one** `load` id, not two.
- [ ] If it is the service worker, the reason `registerType: 'prompt'` was not enough is
      written down — that assumption is stated in `helpers/pwa.ts` and would otherwise mislead
      the next reader.
- [ ] Checked on Android Chrome too, so the fix is not iOS-specific guesswork.
- [ ] Verified no in-flight boot work is interrupted (session resolve, `/api/sync`, hydration).

## Progress Log

- 2026-09-16 23:45 — Task created from task 280's probe data. The double *mount* was originally
  suspected to be a double-registered sync loop, which would have broken PRD 017 §6; the `load`
  ids ruled that out and turned it into this narrower, still-real problem.
- 2026-09-16 23:55 — Picked up. Read the built `sw.js`, the `virtual:pwa-register` output in the
  bundle, `index.html`, the workbox config and every `location.reload`/`location.replace` in
  `vue/src`. **All four in-app candidates eliminated** — see above, each with the specific reason
  rather than "checked, fine", since the next person deserves to know what was ruled out and how.
- 2026-09-17 00:05 — Rather than keep guessing at platform behaviour, extended the probe to record
  `nav` (Navigation Timing type), `path` and `sw` (whether a worker controlled the document) on
  every mount, and to show them live in a **"Denne indlæsning"** panel so the question can be read
  off the screen without pasting anything. Added the decision table above: each possible `nav`
  value now maps to a conclusion and a next step, so the measurement settles this rather than
  producing more data to interpret.
- 2026-09-17 00:10 — Blocked on one launch of the app on a device. Note the eliminations above are
  claims about *this* repo at this commit; if `nav` comes back `reload`, candidate 1 is where the
  mistake is.
