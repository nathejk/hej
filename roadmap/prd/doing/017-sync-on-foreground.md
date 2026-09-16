# PRD 017 — One sync check on foreground, for every dataset the device holds

**Status:** doing
**Author:** agent session (Zed / Claude)
**Created:** 2026-09-15
**Last updated:** 2026-09-16
**Approved:** 2026-09-16
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

**State of play.** Half the mechanism is already in the repo: PRD 016 has shipped,
and task 269 landed `go/cmd/api/mapversion.go` — cached, per-patrol version
derivations for scans, handouts and revealed checkpoints, each with a
changes-when-data-changes test. What is missing is the endpoint that multiplexes
them, versions for profile and race area, and the client loop that consumes them.

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
    shipped map handouts and revealed checkpoints, which change *during* the race
    and are the whole basis of what the map may draw. A patrol handed a new sheet at
    a post must see its checkpoints appear. A reveal that arrives on the next cold
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

- **Why now?** PRD 016 was the tipping point and it has shipped. It brought two
  datasets whose freshness is the mechanism by which the map becomes useful at all,
  and it brought their version derivations (task 269) *for this PRD* — cheap,
  cached, tested, and currently served from no endpoint. Everything below is now
  the last mile: leaving those versions unconsumed means three datasets that change
  during the race and refresh on cold start only.

- **Evidence.**
  - `vue/src/composables/useFreshnessLoop.ts` — the convention, written to be
    reused, reused for one dataset.
  - `go/cmd/api/mapversion.go` — three finished `*VersionFor` derivations whose own
    header says they feed "PRD 017's foreground-sync check", with nothing calling
    them.
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
- **Not** every cached thing on the device — only server-owned datasets the user
  *reads*. Vehicles (PRD 010) are excluded because they are a form the user
  themselves writes, and a background replace mid-registration would fight the
  user; favourites and notification state are device-local and have no server
  version to compare against. Either can be given a key later without changing the
  shape.
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
- As **any user**, I want a way to force a refresh when I think the app is behind,
  so I do not have to guess whether what I am looking at is current.

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
  notice already explains the situation. The reconnect trigger picks it up. A manual
  refresh while offline must say so rather than appear to succeed.
- **One dataset fails** — two distinct failures. A *payload* refetch failing leaves
  the cached copy and records staleness rather than blanking the UI. A *version
  derivation* failing on the server omits its version and marks the dataset
  unavailable, so the check still answers for the other five and the device retries
  next foreground. Neither may fail the whole check.
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
- **Return from bfcache or the iOS app switcher** — see §11 *Decided*.
  `visibilitychange`
  is not reliably the only signal, and the current loop listens to nothing else.

## 6. Requirements

### Functional

- [ ] A single endpoint returns, for the calling user, a small opaque version per
      dataset that user holds.
- [ ] The client checks it on **foreground**, on **reconnect**, on an **interval
      while visible**, and on an explicit **user request** — four trigger points and
      no more.
- [ ] Mounting counts as foregrounding (existing behaviour).
- [ ] A **manual refresh** control is available on the panes that hold synced data.
      It bypasses the debounce (the user has asked, and a control that visibly does
      nothing is worse than the request), but not the overlap guard.
- [ ] The client compares each version with what it holds and refetches only the
      datasets that differ.
- [x] Datasets covered at ship: contacts manifest (incl. portrait versions), own
      profile, patrol scans, patrol map handouts, revealed checkpoints, race area.
      **Five refresh a cached copy; `race_area` has none** — it is fetched on demand when a
      bulk tile download starts, so a change in it is surfaced as "more map available to
      download" instead (task 294).
- [ ] A dataset absent from the response is one the user may not hold; the client
      must not request it.
- [ ] A dataset whose version could not be derived is reported as **unavailable**,
      distinctly from absent. The client keeps its cached copy, does not refetch, and
      keeps asking on the next check — a transient projection error must never be
      read as "you may not hold this", or a device stops asking for something it is
      entitled to and nothing ever tells it otherwise.
- [ ] Refetched datasets **replace** the cached copy wholesale.
- [ ] Metadata propagates ahead of images: a corrected number may arrive before
      the new portrait; never the reverse.
- [ ] Checks are debounced — a check within N seconds of the previous one is
      skipped, N served rather than compiled in. A manual refresh is exempt.
- [ ] The interval is served; zero disables the **interval only**, leaving
      foreground, reconnect and manual checks running.
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
- **A manual refresh is the exception, and it must show something.** The whole
  value of the control is that a user who suspects the app is behind can settle the
  question, so it acknowledges the tap even when the answer is "nothing changed" —
  otherwise it reads as broken and gets tapped repeatedly. Prefer a standard
  shadcn-vue control; pull-to-refresh alone is not enough, because it collides with
  a scrolling list and does not exist on the map.
- **No toast when something changes.** The screen updating is the feedback. A new
  scan arriving while the drawer is open just grows the list — no notice.
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
  "unavailable": [],
  "interval_seconds": 60 }
```

- **Absence is meaningful.** A dataset the caller may not hold is simply not a
  key. That is how a spejder device learns not to ask for a directory — better
  than today's arrangement, where the client's own role table decides and a
  disagreement with the BFF produces a 403 per foreground until `forbidden`
  sticks.
- **`unavailable` exists so absence can stay meaningful.** Each `*VersionFor`
  returns an error, and one failing projection read must not 500 the check for the
  other five. But dropping its key would say "you may not hold this", which is a
  lie the client cannot recover from — it would stop asking. So a failed derivation
  is named, and the client treats it as "unchanged, ask again". Normally empty; if
  it is not, that is a server fault worth logging as one.
- **`interval_seconds` is served in the response**, not only in `/api/config`.
  The reason is the 02:00 lever: an operator shedding load wants it to take effect
  on the next check, on every device, without a config refetch. Zero keeps meaning
  "drop the timer, keep the event-driven checks" — the distinction an operator
  would get wrong, and therefore the one with its own test.
- **One interval serves every dataset's cadence, which is a property of
  multiplexing rather than a compromise.** The strictest requirement sets it — scans
  should surface inside a minute — and contacts, portraits and the rest get
  their looser "within a few minutes" for free, because the check is one request for
  all of them and costs the same whether one dataset changed or none did. Hence 60 s
  (§11).
- **The per-dataset interval knob is lost**, and that is the honest cost of
  multiplexing: `contactsPollSeconds` could be widened for contacts alone.
  Accepted, because one endpoint has one cost to tune, and a dataset that
  genuinely needs its own cadence can be given its own key later.

### Frontend (Vue 3 / TS)

- `useFreshnessLoop` keeps its role and gains **debounce** (a minimum gap between
  checks) with an override for a user-requested check. It currently guards overlap
  but not repetition, and unlock-check-lock-unlock is how the map is actually used.
- The loop's returned `check` is what the manual refresh calls, so the control is a
  button wired to the existing seam rather than a second path to the same endpoint.
- New `composables/useSyncLoop.ts`: one app-level loop whose `check` fetches
  `/api/sync` and dispatches per-dataset refreshes. Registered once in `App.vue`,
  beside the existing app-level concerns.
- Each store gains a uniform `refreshIfVersionDiffers(version)`. The contacts
  store's `refreshIfStale` is the model, minus its private version request — the
  sync response now provides that. All six stores exist today
  (`contacts`, `profile`, `scans`, `handouts`, `checkpoints`, plus the race-area
  cache), so this is a uniform addition rather than new state.
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
  payloads get built. Three of the six already exist: `checkpointsVersionFor`,
  `handoutsVersionFor` and `scansVersionFor` in `mapversion.go` (task 269), each
  cached per patrol on a 5 s TTL, alongside `contactsVersionFor`. Only **profile**
  and **race area** still need one, and `mapversion.go`'s header documents the rules
  they must follow — projection read or cached hash, keyed by *permitted set* rather
  than by user, nothing time-varying in the hash.
- Composition is failure-tolerant: a derivation returning an error contributes to
  `unavailable` rather than failing the response (§6, and the shape above).
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
  that it is stable. Done for the three map datasets; owed for profile and race
  area.
- **A wrongly-unstable version is expensive**: one that changes on every request
  makes every device refetch every payload every minute. `contacts.go` already
  documents this trap (the expiry is deliberately not part of the hash); the same
  discipline applies to each new version.
- **The version caches' 5 s TTL is part of the latency budget.** A version may be
  served up to 5 s stale, and the debounce adds its own gap on top, so a change can
  take ~10 s to reach a device that foregrounds at the wrong moment. Accounted for
  in §9 rather than treated as a bug.
- **Ordering.** Independent refetches can apply out of order relative to one
  another; nothing may assume that, say, checkpoints and handouts are mutually
  consistent within a single check.

## 9. Success Metrics

- Median staleness of a change reaching a foregrounded device: scans inside one
  minute; contacts, portraits and profile inside a few minutes. Under 10 s for a
  device foregrounding after the change — the version caches' 5 s TTL plus the
  debounce, not a round-trip budget.
- ≥ 95 % of `/api/sync` calls report nothing changed, at a small fraction of a
  position report's cost. **Measured outside the first hour of the race**: photos are
  added heavily early on, every one of them moves the contacts version, and a
  depressed ratio then is the system working rather than a fault (§11).
  *Instrumented in task 293: the BFF logs `unchangedRatio` from the `304` rate, plus
  per-dataset `churnRatio`, every five minutes. Task 296 reads them after the first event.*
- `/api/sync` p95 well inside the BFF's other read endpoints, at expected device
  count.
- Zero reports of a device holding a stale copy for a whole event — the silent
  failure this PRD's tests exist to prevent.
- No measurable battery regression from the interval on the baseline device.

## 10. Rollout / Task Breakdown

PRD 016 has shipped and task 269 landed the map datasets' version derivations, so
there is no longer a phase gated on another PRD. What remains is one coherent piece
of work plus cleanup.

**Phase 1 — the mechanism**
- [ ] Task 280: **device check on iOS/iPadOS home-screen PWA and bfcache** — which
      events actually fire on return (§11 *Decided*). A blocker, not a question: it decides
      what the loop listens to, and every other task assumes an answer. **Needs a device.**
- [x] Task 281: debounce in `useFreshnessLoop`, with the manual-refresh override
- [x] Task 282: manual refresh control, wired to the loop's `check`
- [x] Task 283: `Version(viewer)` for **profile** and **race area**
- [x] Task 284: `GET /api/sync` composing all six versions, `unavailable` handling, OpenAPI
- [x] Task 285: rewrite the convention comment in `useFreshnessLoop.ts`
- [x] Task 286: `useSyncLoop` at app level, dispatching per-dataset refreshes
- [x] Task 287: uniform `refreshIfVersionDiffers` across the stores
- [x] Task 288: collapse `useContactsFreshness` and `useQuietPrefetch` into the sync loop;
      remove `MapsView`'s mount-time fetches
- [x] Task 289: served `interval_seconds`, incl. the zero-disables-the-interval test
- [ ] Task 290: verify on device that a reveal appears within one foreground. **Needs a
      device and a server-side handout trigger.**
- [ ] Task 291: load test at expected device count; record the numbers in §9

**Phase 2 — cleanup**
- [ ] Task 292: retire `/api/contacts/version` once no client calls it. **No client does
      as of task 288**; it waits for a release to have shipped, so installed PWAs on an older
      bundle keep working.
- [x] Task 293: instrument the unchanged/changed ratio
- [ ] Task 296: review those numbers after the first event

**Opened during implementation**
- [x] Task 294: tell a user when their downloaded map area no longer matches the event. The
      `race_area` version has no store to refresh — the area is fetched on demand when a bulk
      tile download starts — so what a change in it means is a user-visible fact rather than a
      refresh, and belonged in its own task.
- [x] Task 295: bound the version cache. Its doc comment justified never evicting by a key
      space of "role combinations", which stopped being true when tasks 269/283 keyed caches by
      patrol and by **user** — a slow leak on the endpoint every device calls on every
      foreground.

No feature flag. The mechanism is a strict improvement over "fetch once on
mount", and the lever that matters — the interval — is served, so load can be
shed without a release.

## 11. Open Questions

**None open.** The questions this PRD was drafted with are answered below, and the
one genuine unknown left — which events fire on return to a home-screen PWA on iOS —
is a Phase 1 device check rather than a decision to be argued.

### Decided

- **Interval 60 s, debounce 5 s, both served.** Set by the strictest dataset: scans
  should surface inside a minute. Contacts, portraits and profile only need "within a
  few minutes" and get better than that for free, because one multiplexed check
  covers them all at the same cost. The debounce compounds with the 5 s version-cache
  TTL (§8), so the worst case for a badly-timed foreground is ~10 s — acceptable
  precisely because a user who suspects the app is behind has a refresh control and
  does not have to wait for a tick.
- **A manual refresh control ships with the mechanism.** It is what makes the timing
  question low-stakes: the interval decides how fresh the app is when nobody is
  asking, and the button decides how fresh it is when somebody is. It bypasses the
  debounce, not the overlap guard, and it acknowledges the tap even when nothing
  changed (§7).
- **`visibilitychange` is sufficient, and this is now measured** (task 280, iOS 18.7 / Safari
  26.6.1, installed home-screen PWA, 2026-09-16). It fires with `visibilityState === 'visible'`
  on lock/unlock return, on app-switcher return, and on a bfcache restore — so the loop hears
  every resume path that matters, and no listener needed adding. `pageshow` and `focus` arrive
  alongside or before it and add no coverage; `focus` fired *twice* on one unlock, so adding
  either would mean two or three checks per resume instead of one. `freeze`/`resume` never fired
  on Safari at all. A device killed while locked returns as a cold start, where the mount-time
  check covers it. The original concern was real and the answer turned out to be "already
  correct" — which is only knowable by measuring, and the probe at `/genoptag` is kept so it
  stays knowable.
- **One key per dataset, not a single app-wide version.** A single version is one
  comparison, but it refetches everything when anything changes — including the
  contacts manifest, the largest payload on the device. Per-dataset keys, as §6
  requires. A dataset that later needs its own cadence can also be given its own
  interval on the same shape.
- **No separate photo key; portrait versions stay inside the contacts manifest.**
  With ~100–150 entries and photos arriving heavily in the first hour, the contacts
  version will churn early in the race — but the manifest is metadata only
  (`id`, `name`, `population`, `phone`, `crewFunction`, `stillInRace`,
  `portraitVersion`, groups), so a refetch is tens of kilobytes before compression
  and, critically, **refetches no images**: portraits still arrive per row, lazily
  (PRD 009 §7). Splitting the key would save a small payload at the cost of a second
  version to keep honest. Revisit only if the unchanged ratio outside the first hour
  disappoints — the expected early-race dip is written into §9 so it is not
  misdiagnosed as a wrongly-unstable version.
- **The response carries versions and `interval_seconds`, nothing else.** An "event
  state" payload — race started/ended, active SOS, a new update post — is attractive
  and is exactly how a small endpoint becomes a large one. Excluded; a future PRD can
  argue for it on its own terms.
- **A new scan arriving while the drawer is open just grows the list.** No notice,
  no toast. The row appearing is the confirmation.
- **A write updates its own store directly and lets the loop confirm.** A profile
  confirmation or photo upload already knows what it changed; forcing an immediate
  check would race the write against its own refetch for no user-visible gain.
- **`/api/config` stays separate.** Its contract is that everything it carries is
  public by definition, while `/api/sync` is per-caller and authenticated; folding
  one into the other would blur that. `interval_seconds` appearing in both is
  accepted duplication, and the sync response is the one that wins, because the
  02:00 lever must take effect on the next check without a config refetch.
