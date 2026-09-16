# 297 — The app loads twice on launch, one second apart

**Status:** done
**Priority:** medium
**Created:** 2026-09-16
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-16
**Completed:** 2026-09-17

## Outcome: working as designed, not a defect

**Confirmed 2026-09-17 00:47:** a launch with no pending update produces **one document** — a single
`mount`, `nav: navigate`, `load: m2ds`. The double load was the update prompt being accepted, exactly
as the leading explanation below predicted.

The distinguishing evidence, from one probe log:

| tid | event | load | nav | reading |
|---|---|---|---|---|
| 00.00.38 | `mount` | `b3ul` | **reload** | update accepted → the app reloaded itself |
| 00.47.54 | `mount` | `m2ds` | **navigate** | ordinary launch → one document, no second load |

So `nav` did what it was added to do: the two cases were indistinguishable in a table of events and
have entirely different causes. No code change; the service worker, the router and the boot path were
all innocent.

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

*(Superseded by the outcome above — kept because it is the reasoning that made this worth chasing.)*

Every launch pays for the app twice: two shell boots, two `/api/config` fetches, two
`/api/sync` checks, two session resolutions. That is small in isolation — and it lands
squarely on the cold-start path, which the same probe run suggested is common.

**One correction to that reasoning, from the final run:** an earlier note here claimed iOS
discards this app after roughly 51 seconds hidden. That was one observation over-generalised.
The final log shows the same document surviving **1947 s (32 minutes)** hidden and resuming
normally, then being gone after 55 s on a later cycle. So discard is **variable** — memory
pressure, not a timer — and "cold starts are common" is plausible but not established by this
data.

There is also a correctness edge: a reload one second in can interrupt whatever the first
document had started — an in-flight `/api/sync`, a session refresh, a store hydration. That
remains true of an *accepted update*, which is a deliberate reload, and is worth remembering if
somebody reports odd state right after updating.

## Leading explanation (needs one confirming launch)

**Measured 2026-09-17 00:00, installed iOS PWA:** one mount, `nav: reload`, `path: /genoptag`,
`sw: true`.

`nav: reload` means the document really was reloaded rather than freshly navigated to — so
candidate 1's paper reasoning was incomplete, but not in the way it looked. The missing piece is
not in the code, it is in the **circumstances of the test**: this whole probing session ran while
new builds were being pushed every few minutes. Every launch therefore found a waiting service
worker, showed *"En ny version er tilgængelig"*, and the tester — who was deliberately chasing the
newest build — tapped **Genindlæs**. That calls `applyUpdate()` → `messageSkipWaiting()` →
`controlling` with `isUpdate` → `location.reload()`, which is exactly the observed
`nav: reload`, and exactly what the update flow is designed to do.

It also explains the **two mounts a second apart**: mount #1 is the launch, and mount #2 is the
reload after tapping a banner that appears at the top of the screen immediately. One second is a
very plausible reaction time for somebody watching for that banner.

And it explains why *this* run shows only one mount: the log was cleared after the launch, so only
the reload's mount survived.

So the likely answer is **not a defect** — it is the update prompt working, amplified by a test
session that produced a new build every few minutes. An event does not look like that.

### The one measurement that settles it

Launch the app **with no new build pending and no banner shown**. Expected: exactly one `mount`,
`nav: navigate`. If that is what happens, this task closes as "working as designed".

**If a launch with no pending update still reports `nav: reload`, it is a real defect** and the
service-worker path is where to look, because nothing else in the app reloads.

### A related thing worth checking while there

With `registerType: 'prompt'`, tapping **Senere** leaves the worker waiting indefinitely — so the
banner returns on every subsequent launch until the user accepts it. That is arguably correct, but
it means "banner on every launch" is a state a participant can get stuck in, and it would look
exactly like this bug to anybody investigating later. Worth a note in `helpers/pwa.ts` either way.

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

- [x] The cause identified, and named in this task. — the update prompt being accepted.
- [x] Confirmed with the probe: a launch **with no pending update** produces **one** `load` id and
      `nav: navigate`.
- [x] If it is the service worker, the reason `registerType: 'prompt'` was not enough is
      written down. — it *was* enough. The reload came from the user accepting the prompt, which is
      the flow working; recorded here and in `helpers/pwa.ts`.
- [ ] Checked on Android Chrome too. **Dropped:** there is no defect to check for.
- [x] Verified no in-flight boot work is interrupted — an accepted update *does* reload mid-boot, by
      design. Noted in `helpers/pwa.ts` so a report of odd state right after updating has a lead.
- [x] Note in `helpers/pwa.ts` that "Senere" leaves the worker waiting, so the banner returns on
      every launch until accepted.

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
- 2026-09-17 00:15 — **Measured: `nav: reload`, `sw: true`, one mount.** So the document was
  reloaded — and re-reading the update path with that in hand, `applyUpdate()` has exactly one
  caller (`UpdatePrompt`'s **Genindlæs** button; no watcher, no auto-apply, verified in
  `UpdatePrompt.vue` and `app.store.ts`). The eliminations were right about the code; what they
  missed was the **test conditions**: builds were being pushed every few minutes throughout this
  session, so every launch found a waiting worker, showed the update banner, and got accepted.
  That produces precisely this `reload`, and the earlier 1-second double mounts are a launch
  followed by a tap on a banner that appears immediately.

  Recorded as a *leading explanation* rather than a conclusion, because the distinguishing
  measurement has not been taken: a launch with **no pending update**. If that shows `nav:
  navigate` and a single mount, this is the update flow working and the task closes; if it still
  shows `reload`, there is a real defect and the service worker is where it lives. Deliberately not
  marking this done on a plausible story — the whole reason this probe exists is that plausible
  stories about resume behaviour have been wrong twice already in this task's own history.
- 2026-09-17 01:45 — **Confirmed and closed.** A launch with no pending update gave one `mount`,
  `nav: navigate`, one `load` id. The story held. Two things worth keeping from it:
  1. **`nav` earned its place.** A reload and a fresh navigation are indistinguishable in a log of
     events, and I had spent a round of code-reading on the wrong one. The field is three lines and
     it ended the question.
  2. **I over-generalised the discard timing** and have corrected it above: the same document
     survived **32 minutes** hidden in this run, then was gone after 55 s later on. Discard is
     memory pressure, not a timer, so "cold starts are common during an event" is plausible but not
     something this data establishes. Task 291's load-test notes lean on it only as colour, not as a
     number.
