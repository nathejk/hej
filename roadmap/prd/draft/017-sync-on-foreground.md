# PRD 017 — One sync check on foreground, for every dataset the device holds

**Status:** draft
**Author:** agent session (Zed / Claude)
**Created:** 2026-09-15
**Last updated:** 2026-09-15
**Approved:**
**Shipped:**
**Target users:** participant (all roles — patrol members, crew, personnel)

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 1. Summary

Every time the app comes to the foreground, it asks the server once whether
anything it holds has changed — map sheets, revealed checkpoints, scans, contact
phone numbers, portraits, the user's own profile — and refetches only the
datasets whose answer differs.

Today this exists for **contacts alone**. This PRD generalises it to every cached
dataset, in **one** request rather than one per dataset.

## 2. Problem & Motivation

- **What problem does this solve?**
  - **Only contacts stay current.** `useFreshnessLoop` is already this repo's
    convention (PRD 009 §6, task 190), and its own header addresses "the next
    dataset that needs to stay current during an event" — but the only followers
    are the contacts pane loop and the app-level quiet prefetch, both checking the
    same dataset. Scans are fetched once in `MapsView`'s `onMounted` and then
    never again for as long as the view is mounted. A patrol that scans a post and
    looks at the map sees the old list.
  - **The datasets that change most are the ones that never refresh.** PRD 016
    adds map handouts and revealed checkpoints, which change *during* the race and
    are the whole basis of what the map may draw. A patrol handed a new sheet at a
    post must see its checkpoints appear. A reveal that arrives on the next cold
    start arrives after it mattered.
  - **Corrected data stays invisible.** Phone numbers are corrected during the
    event — that is why the contacts loop was built in the first place. People add
    portraits in the run-up and on the night. A profile is confirmed on another
    device. None of it reaches a device already holding the wrong copy, unless it
    happens to be a contact row.
  - **Following the convention N times is the wrong shape at N > 1.** Six datasets
    each with its own version endpoint is six requests per foreground, from a few
    hundred devices, on a mobile link where round-trip count dominates payload
    size. The convention was written when there was one dataset; honouring its
    *intent* at six means multiplexing the check.

- **Why now?** PRD 016 is the tipping point: it adds two datasets whose freshness
  is not a nicety but the mechanism by which the map becomes useful at all. Doing
  it as two more bespoke loops would leave four half-followed conventions to
  unify later, mid-season.

- **Evidence.**
  - `vue/src/composables/useFreshnessLoop.ts` — the convention, written to be
    reused, reused for one dataset.
  - `go/cmd/api/contacts.go` — the version endpoint's own comment: any other pane
    needing during-event freshness "should copy the shape".
  - `vue/src/views/MapsView.vue` — `void scans.fetch()` on mount, and nothing
    after it.
  - `vue/src/helpers/offline/prefetch.ts` — app-level freshness already exists and
    already argues for itself; it is simply scoped to contacts.

## 3. Goals

- Anything the device holds becomes current within one foreground, or within one
  interval tick while the app is open.
- One request answers "what changed?" for every dataset the user has.
- A user with nothing changed costs the server approximately nothing.
- Operators can reduce this traffic during an event without shipping a release.
- Datasets stay independently refetchable: one dataset's failure does not stale
  the others.

## 4. Non-Goals

- **Not** push-based invalidation. iOS 16.4+ requires every web push to raise a
  user-visible notification (`public/push-sw.js` always calls
  `showNotification`), so there is no silent data push on our baseline — using
  one would buzz every phone over a corrected phone number, or get the permission
  revoked. Settled, not open.
- **Not** deltas or a `?since=` cursor. These datasets are small enough for one
  payload each, and PRD 009 §6 already decided: **replace, do not merge**, because
  a delta needs explicit tombstones or the server's purge is decorative.
- **Not** a websocket or SSE connection. A few hundred devices holding open
  connections to the same BFF that takes position reports, for updates that are
  fine arriving on foreground, is cost without a user-visible gain.
- **Not** background sync. A phone in a pocket has nobody reading anything; the
  existing loop stops entirely when hidden and should keep doing so.
- **Not** bulk portrait prefetching. Faces still arrive with the rows that display
  them (PRD 009 §7 reasoning stands).
- **Not** changing what any dataset contains or who may see it. This PRD moves
  bytes at better moments; it grants nothing.

## 5. User Stories & Scenarios

- As a **patrol member**, I want the map to show a sheet's checkpoints as soon as
  we are handed it, so the app keeps up with the race.
- As a **patrol member**, I want our newest scan in the drawer when I next look at
  my phone, so the list matches what we just did.
- As a **crew member**, I want a corrected phone number without restarting the
  app, so I do not call a number that has already been fixed.
- As **any user**, I want a photo somebody added to show up, so the directory
  looks maintained rather than abandoned.

**Happy path.** A patrol scans in at Post 4 and is handed Kort 3. The scanner's
event reaches the BFF. Two minutes later a patrol member unlocks the phone; the
app foregrounds and issues one `GET /api/sync`. The response says `scans`,
`handouts` and `checkpoints` have new versions and `contacts` does not. Three
payload requests go out, the drawer gains a row marked "på tid", two new flag
markers appear, and the contacts manifest — the largest thing on the device — is
not touched.

**Edge cases.**

- **Nothing changed** (the common case): one small request, one unchanged answer,
  no payload traffic.
- **Rapid foregrounding** — checking the map, locking, unlocking twice while
  walking. Must be debounced: a check within a few seconds of the previous one is
  skipped. The current loop guards *overlapping* checks but not repeated ones, and
  this is the map's normal rhythm rather than an edge case.
- **Offline foreground** — no request; the cached copy stays and the offline
  notice already explains the situation. The reconnect trigger picks it up.
- **One dataset fails** — the others still apply. The failed one keeps its cached
  copy and records staleness rather than blanking the UI.
- **A dataset the user has no business holding** — a spejder has no contacts pane.
  It is absent from the response by role, so the device never learns a version for
  something it may not fetch, and never asks.
- **The user has no patrol** (personnel): scans, handouts and checkpoints are
  absent, exactly as their endpoints already return empty.
- **Session expired** — a `401` stops the loop rather than retrying on every
  foreground; the existing auth handling owns the redirect.
- **A dataset shrinks server-side** (a sheet deleted, a contact who left the race)
  — replace-not-merge means it stops existing on the device. A requirement, not a
  side effect.
- **Clock skew** — versions are opaque and server-issued, so nothing here depends
  on the device clock. The manifest's `expiresAt` stays server-issued for the same
  reason.
- **Return from bfcache or the iOS app switcher** — see §11.1. `visibilitychange`
  is not reliably the only signal, and the current loop listens to nothing else.

## 6. Requirements

### Functional

- [ ] A single endpoint returns, for the calling user, a small opaque version per
      dataset that user holds.
- [ ] The client checks it on **foreground**, on **reconnect**, and on an
      **interval while visible** — the three trigger points and no more.
- [ ] Mounting counts as foregrounding (existing behaviour).
- [ ] The client compares each version with what it holds and refetches only the
      datasets that differ.
- [ ] Datasets covered at ship: contacts manifest (incl. portrait versions), own
      profile, patrol scans, patrol map handouts, revealed checkpoints, race area.
- [ ] A dataset absent from the response is one the user may not hold; the client
      must not request it.
- [ ] Refetched datasets **replace** the cached copy wholesale.
- [ ] Metadata propagates ahead of images: a corrected number may arrive before
      the new portrait; never the reverse.
- [ ] Checks are debounced — a check within N seconds of the previous one is
      skipped, N served rather than compiled in.
- [ ] The interval is served; zero disables the **interval only**, leaving
      foreground and reconnect checks running.
- [ ] A per-dataset failure is isolated and recorded; the loop continues.
- [ ] Exactly one app-level loop. Per-pane loops for these datasets are removed
      rather than left running alongside it.

### Non-Functional

- **Load is the primary constraint.** This is the app's only continuous
  during-race traffic besides position reporting, and it lands on the same BFF.
  The unchanged case must be cheap enough that a few hundred devices checking on
  every foreground plus an interval is not measurable next to position reports.
  Every version must be answerable from a projection read or a short-lived cache
  — `contactsVersionFor` is the reference — never by building the payload and
  hashing it.
- **The response must stay tiny.** A handful of short strings. If it grows,
  something belongs in a payload instead.
- **Battery.** No traffic while hidden. No timer survives a hidden document.
- **The version travels in the JSON body**, not only in a header: `fetchWrapper`
  does not expose response headers, so a header-only version would force every
  consumer to bypass it. `ETag` stays on the response for the browser's own
  conditional requests.
- **Privacy.** Versions are scoped to the caller's permitted set, so a version
  cannot signal the existence of data the caller may not see. Guardian phone
  numbers stay out of every payload (repo rule) — nothing here changes what is
  sent, and no version may be derived from a field that must not leave.
- **Offline-first.** A failed check is a non-event: keep the copy, record
  staleness (PRD 009).
- **Testability.** The loop's browser surface stays injectable
  (`FreshnessTarget`), so trigger-point decisions remain testable without a DOM.
- **Browser baseline.** iOS/iPadOS Safari 16.4+, Chrome 111+.

## 7. UX / UI Notes

Mostly invisible, and that is the point — this should feel like the app being
right, not like syncing.

- **No spinner on a foreground check.** A check is expected to find nothing; a
  progress indicator on every unlock would be noise. Panes may keep their existing
  subtle refreshing state while a *payload* is refetched.
- **No toast when something changes.** The screen updating is the feedback. The
  one case worth considering is a new scan arriving while the drawer is open
  (§11.5).
- **Staleness stays where it already is.** `OfflineNotice` and the panes' "last
  synced" affordances (PRD 009) are the honest place to say a copy is old. Do not
  add a second vocabulary for the same fact.
- **Nothing shifts under a reading finger.** A list being scrolled must not
  reorder mid-refetch; apply on next idle or preserve scroll anchoring.

## 8. Technical Considerations

### The shape: one multiplexed check, not N

`useFreshnessLoop.ts` §1 says "a separate, cheap version endpoint". That was
written with one dataset, and it remains right about *what* a version endpoint is;
it is wrong about *how many requests* to make once there are six. This PRD keeps
every property the convention argued for — small, opaque, per-caller, answered
from a projection read, ETag-able, version in the body — and changes only the
multiplicity:

```
GET /api/sync
{ "versions": { "contacts": "a1b2", "profile": "c3d4", "scans": "e5f6",
                "handouts": "…", "checkpoints": "…", "race_area": "…" },
  "interval_seconds": 60 }
```

- **Absence is meaningful.** A dataset the caller may not hold is simply not a
  key. That is how a spejder device learns not to ask for a directory — better
  than today's arrangement, where the client's own role table decides and a
  disagreement with the BFF produces a 403 per foreground until `forbidden`
  sticks.
- **`interval_seconds` is served in the response**, not only in `/api/config`.
  The reason is the 02:00 lever: an operator shedding load wants it to take effect
  on the next check, on every device, without a config refetch. Zero keeps meaning
  "drop the timer, keep the event-driven checks" — the distinction an operator
  would get wrong, and therefore the one with its own test.
- **The per-dataset interval knob is lost**, and that is the honest cost of
  multiplexing: `contactsPollSeconds` could be widened for contacts alone.
  Accepted, because one endpoint has one cost to tune, and a dataset that
  genuinely needs its own cadence can be given its own key later.

### Frontend (Vue 3 / TS)

- `useFreshnessLoop` keeps its role and gains **debounce** (a minimum gap between
  checks). It currently guards overlap but not repetition, and
  unlock-check-lock-unlock is how the map is actually used.
- New `composables/useSyncLoop.ts`: one app-level loop whose `check` fetches
  `/api/sync` and dispatches per-dataset refreshes. Registered once in `App.vue`,
  beside the existing app-level concerns.
- Each store gains a uniform `refreshIfVersionDiffers(version)`. The contacts
  store's `refreshIfStale` is the model, minus its private version request — the
  sync response now provides that.
- `useContactsFreshness` and `useQuietPrefetch` **collapse into the sync loop**.
  Two loops checking the same dataset on the same triggers is exactly the
  duplication this PRD exists to prevent; leaving them would double contacts
  traffic.
- `MapsView`'s mount-time scan fetch is removed in favour of the loop.
- Stores must tolerate being refreshed while a view is rendering them.

### BFF (Go)

- `GET /api/sync` in a new `sync.go`, behind `requireAuth`, with **OpenAPI
  annotations** (repo rule).
- It composes existing version derivations and must not become a place where
  payloads get built. Each contributing dataset exposes a `Version(viewer)` that
  is a projection read or a cached hash — `contactsVersionFor` plus its
  `versionCache` is the pattern to follow, including keying the cache by
  *permitted set* rather than by user, so devices sharing a role share the work.
- Datasets are assembled by role and patrol, so the response is the authoritative
  answer to "what may this caller hold".
- `/api/contacts/version` stays for one release while the client transitions, then
  goes. It is documented as the reference implementation, so removing it needs a
  note pointing at `/api/sync`.
- Log the unchanged/changed ratio and the endpoint's own timing. If the unchanged
  case is not overwhelmingly dominant, some version is being derived wrongly, and
  an event is the wrong time to discover that.

### Dependencies & risks

- **Load, and it is the whole risk.** Six versions computed per call, per
  foreground, per device. Mitigated by cached versions keyed on permitted set, a
  tiny response, ETag, the debounce, and the served interval as the live lever. A
  load test at expected device count belongs in the rollout, not after it.
- **A wrongly-stable version is silent.** The failure mode is not an error but a
  device that quietly never updates — the hardest kind to notice during an event.
  Every version needs a test that it *changes* when its data changes, not merely
  that it is stable.
- **A wrongly-unstable version is expensive**: one that changes on every request
  makes every device refetch every payload every minute. `contacts.go` already
  documents this trap (the expiry is deliberately not part of the hash); the same
  discipline applies to each new version.
- **Coupling to PRD 016.** Three of the six datasets do not exist yet. Sequenced
  in §10 so the mechanism ships with what exists and gains keys as PRD 016 lands.
- **Ordering.** Independent refetches can apply out of order relative to one
  another; nothing may assume that, say, checkpoints and handouts are mutually
  consistent within a single check.

## 9. Success Metrics

- Median staleness of a change reaching a foregrounded device: under one interval
  tick; under 5 s for a device foregrounding after the change.
- ≥ 95 % of `/api/sync` calls report nothing changed, at a small fraction of a
  position report's cost.
- `/api/sync` p95 well inside the BFF's other read endpoints, at expected device
  count.
- Zero reports of a device holding a stale copy for a whole event — the silent
  failure this PRD's tests exist to prevent.
- No measurable battery regression from the interval on the baseline device.

## 10. Rollout / Task Breakdown

Phase 1 ships value with today's datasets and no dependency on PRD 016; later
phases add keys.

**Phase 1 — the mechanism**
- [ ] Task: debounce in `useFreshnessLoop` (minimum gap between checks)
- [ ] Task: `GET /api/sync` composing contacts, profile, scans and race-area
      versions + OpenAPI annotations
- [ ] Task: `Version(viewer)` for profile, scans and race area, each with a
      changes-when-data-changes test
- [ ] Task: cached version derivation keyed by permitted set (follow
      `versionCache`)
- [ ] Task: `useSyncLoop` at app level, dispatching per-dataset refreshes
- [ ] Task: uniform `refreshIfVersionDiffers` across the stores
- [ ] Task: collapse `useContactsFreshness` and `useQuietPrefetch` into the sync
      loop; remove `MapsView`'s mount-time scan fetch
- [ ] Task: served `interval_seconds`, incl. the zero-disables-the-interval test
- [ ] Task: load test at expected device count; record the numbers in this PRD

**Phase 2 — PRD 016's datasets**
- [ ] Task: add `handouts` and `checkpoints` keys once PRD 016 phase 2 lands
- [ ] Task: verify on device that a reveal appears within one foreground

**Phase 3 — cleanup**
- [ ] Task: retire `/api/contacts/version` and rewrite the convention comment in
      `useFreshnessLoop.ts` to describe the multiplexed check
- [ ] Task: instrument the unchanged/changed ratio and review after the first
      event

No feature flag. The mechanism is a strict improvement over "fetch once on
mount", and the lever that matters — the interval — is served, so load can be
shed without a release.

## 11. Open Questions

1. **Is `visibilitychange` enough on our baseline?** "In focus" is what was asked
   for, and the current loop listens only to `visibilitychange` and `online`.
   Returning to a home-screen PWA from the iOS app switcher, and a bfcache
   restore, do not reliably present the same way. Candidates to add: `pageshow`
   (with `persisted`), `focus`, and `resume`. Needs a device check on iOS/iPadOS
   before phase 1 is called done — this is the difference between the feature
   working and appearing to work.
2. **What is the debounce, and the interval?** Proposal: debounce 5 s, interval
   60 s (matching today's contacts poll). Both served; both deserve a number
   somebody has thought about rather than inherited.
3. **One key per dataset, or a single app-wide version?** A single version is one
   comparison, but it refetches everything when anything changes — including the
   largest payload on the device. This PRD assumes per-dataset keys for that
   reason; worth confirming, since a single version is markedly simpler.
4. **Should the response carry more than versions?** A tiny "event state" — race
   started/ended, an active SOS, a new update post — would let one request drive
   more of the app. Attractive, and exactly how a small endpoint becomes a large
   one. Deliberately excluded here.
5. **Does a new scan arriving while the drawer is open deserve a notice?** The
   list simply growing may be enough; a patrol that just scanned in might
   appreciate the confirmation. A UX call, not a technical one.
6. **What about the user's own writes?** A profile confirmation or a photo upload
   already knows it changed something. Should a write force the next check, or
   update its store directly and let the loop confirm? The latter is simpler and
   avoids a write racing its own refetch.
7. **Do photos need their own key?** Portrait versions ride inside the contacts
   manifest today, so a new photo changes the contacts version and refetches the
   whole directory to learn one thumbnail changed. Acceptable at current size, and
   the first thing to revisit if the unchanged ratio disappoints.
8. **`/api/config` overlap.** Runtime config is already fetched on load and is now
   partly duplicated by `interval_seconds`. Does config fold into the sync
   response, or stay separate because it is public and unauthenticated? Leaning
   strongly to separate — `/api/config`'s contract says everything it carries is
   public by definition, while `/api/sync` is per-caller — but the duplication
   should be a decision rather than an accident.
