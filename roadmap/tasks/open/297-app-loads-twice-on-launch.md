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

1. **Service worker taking control.** The most likely candidate, and the first to rule out.
   `vite.config.ts` uses `registerType: 'prompt'` and `helpers/pwa.ts` registers with
   `immediate: true`, so an *automatic* reload should not happen — `applyUpdate()` is the only
   caller of the plugin's reload, and it is wired to the user tapping `UpdatePrompt`. If a
   reload is happening anyway, that assumption is wrong somewhere: check whether the generated
   `sw.js` has `clientsClaim`/`skipWaiting` set, and whether a `controllerchange` handler
   reloads.
2. **iOS PWA launch behaviour.** Installed iOS apps launch at `start_url`; if something then
   navigates with a full page load rather than through the router, that is a second document.
   Check `router/gates.ts` — `LEAVE_APP` and `WEBSITE_PAGE` do real navigations, and the
   install/device gates run before anything else.
3. **A redirect that is not a router redirect.** Any `location.href = …` or
   `location.replace(…)` on the boot path.

Note the probe was opened at `/genoptag`, which is not `start_url`, so a launch-time
redirect to or from the start URL is worth eliminating early.

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
