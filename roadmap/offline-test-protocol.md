# Offline test protocol

PRD 009 §9, task 195. **Device access is task 172's prerequisite — read that first.**

Everything this app promises about working offline is a claim about a specific phone in a specific
state — installed, full, evicted, or three weeks unopened — and none of those states exist on a
development machine. The automated tests in `vue/src/helpers/offline/quota.spec.ts` cover the
decisions; this covers the device.

**Run this before an event, not after a release.** The failure modes here are seasonal: they show up
on a phone that has been installed for three weeks and used for four hours, which is exactly the
configuration nobody tests by accident.

---

## Two walls to get over before step 1

**Wall 1 is down as of 2026-09-01: use `https://hej.nathejk.dk`.** The app is deployed to a public
server (not production yet), which solves device access outright and better than the tunnel and
tailnet options task 172 was weighing — it is a real host with a real certificate serving a real
build, so what a phone sees there is what a phone will see in production. Prefer it over anything
local for every step below.

Wall 2 still stands, and both are documented in **task 172**, the companion to this file (it covers
PRD 007's pane specifically; this covers the shared offline layer):

1. ~~**A phone cannot reach the dev stack.**~~ Still true of the *dev* stack — `hej.local.nathejk.dk`
   resolves to `127.0.0.1`, which from a phone is the phone, and `http://<LAN-IP>` gives no secure
   context so there is no service worker and no install. Use the deployed host instead of solving it.
2. **An installed app keeps its old bundle.** With `registerType: 'prompt'` the device runs the build
   it installed until someone accepts the update prompt. So every result below must state *which build
   the device is actually running*, or the test silently exercised an older one and passed for nothing.
   This one gets *more* important with a deployed host, not less: a phone that has had the app on its
   home screen for a week is exactly the device that is quietly a week behind.

## Before you start

Test against a **production build with a service worker**, never the dev server:

```sh
docker compose exec ui npm run build
docker compose exec ui npm run preview
```

This is not pedantry. Several bugs in this area have only ever existed in the generated `sw.js` —
task 087 shipped a Workbox route that built cleanly, type-checked, and threw `ReferenceError` on
every tile request, because `generateSW` stringifies config callbacks instead of bundling them. If
you did not load the app from a build, you did not test the thing that ships.

Install to the home screen on iOS, and use the installed app. An installed PWA and a Safari tab have
**different storage rules** — Safari's seven-day inactivity eviction does not apply to installed
apps, and `navigator.storage.persist()` is granted on install-related heuristics — so a result from a
browser tab tells you very little.

---

## 1. Radio off, everything present

The baseline claim: what is cached works with no network at all.

1. Open the app with signal. Visit `Kort` and pan over the race area. Open `Kontakter`. Open your
   profile so the readiness section has measured everything.
2. Note what the readiness section says: sizes, counts, "Klar".
3. **Enable aeroplane mode.** Not DevTools' offline toggle — that does not reproduce what iOS does to
   fetches, and it lies about `navigator.onLine`.
4. Force-quit the app and reopen it.

| check | expected |
|---|---|
| the app opens at all | shell renders, no white screen (task 090) |
| the shell notice | one line, "Ingen forbindelse — se hvad du har hentet", linking to the profile |
| `Kort` | tiles you browsed are drawn; areas you did not are blank with a notice, not grey silence |
| `Kontakter` | the directory lists, search works, portraits show |
| profile readiness | sizes and last-synced times, not "Ikke tjekket" |

Anything that renders an empty state without saying why is a failure, even if nothing crashed.

## 2. Names without faces

The index and the binaries are separate datasets precisely so this works. Someone at 03:00 needs the
phone number more than the photograph.

1. Online, with the directory synced, delete **only** the portrait cache:
   `caches.delete('nathejk-portraits-v1')` in the console (Safari: Develop → Web Inspector).
2. Aeroplane mode, reopen.

Expected: every row still lists, search still works, portraits fall back to initials. If names
disappear with the faces, the separation has regressed and PRD 007's offline premise is gone.

## 3. The OS took everything

The most likely real-world failure: iOS clears caches for a web app it has not seen recently, and PRD
005 deliberately pushes people to install *early* — so "installed three weeks ago, never reopened,
arrives empty" is a normal path, not an edge case.

1. Clear all site data for the origin (Settings → Safari → Advanced → Website Data, or the Web
   Inspector's Storage tab).
2. Reopen the installed app, offline.

Expected: the app explains itself. The readiness section says what is missing, the panes say they
have nothing rather than showing nothing. It must never look like an app with no data in it — that is
the difference between "the phone cleaned up" and "this app is broken", and only one of those gets
reported honestly.

Then go online and confirm it recovers without being asked twice.

## 4. Persistence was granted

1. Online, installed, open the profile.
2. Expected: **no** "Telefonen kan slette det hentede" warning.

If the warning shows on an installed iOS app, `navigator.storage.persist()` was refused, and
everything above became much more fragile. Worth chasing rather than accepting: WebKit is documented
to grant it for home-screen web apps, so a refusal means something about the install is not what we
think it is.

## 5. The phone is full

Automated as far as it can be (`quota.spec.ts`), but the numbers there are fake and iOS's are not.

**Pick the device for this one deliberately.** The budget is planned against the **iOS 16.4–16.7**
quota of ~1 GB, because `.rules` puts the baseline there. Later iOS grants a share of total disk
instead — documented as 60% when this was written, which predates iOS 26 and has not been rechecked —
so on a recent iPhone with free space the origin will not fill and this scenario cannot be reached at
all. A pass on a 14 Pro on iOS 26.6 says nothing about the constraint the budget was designed for.

**Before anything else, read the real number off the device.** Open the profile: the readiness section
renders usage as a percentage of the quota the browser actually reports, and `offline.store` holds
`quotaBytes` from `navigator.storage.estimate()`. One look replaces a policy document of uncertain
vintage with a fact about the phone in your hand — and if it turns out modern iOS grants this origin
several gigabytes, that is worth knowing before anyone plans a bigger tile set.

Then either find a 16.x device, or fill the disk first, and record which.

1. Fill the origin — pan the map across a large area at high zoom until writes start failing.
2. Watch for: the map keeps working with what it has; tiles stop being *added* rather than the cache
   emptying; the readiness section reports something was dropped.
3. Check `Din rute` on the profile still shows its points. **The track must never be what gets
   sacrificed** — it is the only data on the device that exists nowhere else.

## 6. Recording survives, offline, for hours

Not strictly a cache test, but it shares the storage and it is the thing that cannot be re-fetched.

1. Aeroplane mode, walk with the app open for 30+ minutes.
2. Check `Din rute`: points recorded, a pending upload count.
3. Go online. The backlog ships within a couple of minutes without being prompted.

---

## 7. The patrol lookup still refuses (crew only)

Belongs to task 172 but shares this pass, and it is the one case where working offline would be the
**bug**: the patrol lookup is deliberately live and stores nothing (PRD 007, task 157), because a
cached copy would turn it into the browsable index of minors' faces the design exists to avoid.

Offline, it must say it needs a connection and point at the radio — never an empty patrol, never a
stale one from an earlier lookup.

---

## Recording results

Write what happened into the task or PRD that prompted the run, **including what did not work**. A
protocol whose recorded outcome is always "all good" is a protocol nobody ran. Note the device, the
OS version, and whether the app was installed — a result without those three is not reproducible.

## What this protocol cannot tell you

- **Whether iOS will evict you in the field.** Nothing can. Mitigation is `persist()`, install-first
  onboarding, detection and re-sync — not prevention (PRD 009 §8).
- **Whether a dormant device purges its data.** A phone that never reopens the app runs nothing of
  ours. The server-issued deadline (task 193) is checked on the next launch, whenever that is, and
  that is genuinely the strongest promise available.
- **What a full event's worth of devices does to the BFF.** The freshness poll is the app's only
  continuous during-race traffic; its interval is served from `/api/config` so it can be widened
  during an event (task 190).
