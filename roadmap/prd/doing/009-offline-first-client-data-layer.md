# PRD 009 — Offline-first client data layer (shared budget, readiness and freshness)

**Status:** doing
**Author:** agent session (Zed)
**Created:** 2026-08-25
**Last updated:** 2026-09-01
**Approved:** 2026-09-01
**Shipped:**
**Target users:** all app users, indirectly — this is the mechanism behind every offline-capable feature

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.

Rescoped 2026-09-01 after a review against the shipped code. The original draft
argued for one generic sync engine, written *before* its consumers. That window
closed: PRD 002 shipped a Workbox tile cache, PRD 007 shipped a localStorage
directory cache, and PRD 002's track shipped an IndexedDB store — each
reasonably, each different. What is still genuinely shared, and still missing, is
narrower: one budget, one readiness surface, one freshness contract. The title
changed to say so. See §11.2 for the record of what was cut and why.

Approved 2026-09-01, once the two outstanding inputs were confirmed by the maintainer:
the priority order (§6) and the race area (derived from checkpoints, fixed during an
event). Tasks 183–195.
-->

---

## 1. Summary

Three things that only work if they are decided once, for all cached data on the
device: a **global storage budget with a priority order** (including which data is
*unrecoverable* if evicted), a **single readiness surface** telling the user what they
actually have, and a **freshness contract** so a change on the server reaches a device
during the event. Individual features keep owning their own storage — they already do —
but they stop each answering "how much may I keep, what do I sacrifice, and am I stale?"
on their own.

## 2. Problem & Motivation

- **What problem does this solve?** The app is used in a forest, at night, on a phone
  that has been running for hours. Connectivity is the exception, not the rule. Several
  features now cache data to survive that, and each does it in isolation:
  - **map tiles** — Cache API, via a Workbox runtime route in `vue/vite.config.ts`
    (`CacheFirst`, own `cacheName`, `maxEntries`, `maxAgeSeconds`);
  - **the contacts directory** — `localStorage`, via `vue/src/stores/contacts.store.ts`
    (a versioned payload with `schema`, `version`, `syncedAt`);
  - **the unshipped location track** — IndexedDB, via `vue/src/helpers/trackDb.ts`,
    which also calls `navigator.storage.persist()`;
  - **the app shell** — Workbox precache.

  Each is sound on its own terms. What none of them can do alone is the three things
  below.
- **Why a shared layer rather than per-feature caching?** Not to unify the storage code —
  that ship has sailed, and the variety is mostly justified. Three problems are
  irreducibly global:
  1. **The storage budget is global.** Tiles (~324 MB) and portraits compete for one
     origin quota, and the OS evicts per-origin, not per-feature. Nobody can budget in
     isolation. This is not hypothetical: `vue/vite.config.ts` explicitly defers the
     tiles-versus-unrecoverable-data question to this PRD's §11.1, in a comment, in
     shipped code. Task 161 is blocked on the same decision.
  2. **Eviction is not uniformly recoverable.** For tiles, the rulebook and the
     directory, eviction is a performance problem — re-fetch and move on. For the
     unshipped track and for anything not yet uploaded, it is **data loss**. Nothing
     currently expresses that difference, so an eviction policy cannot honour it.
  3. **The user needs one answer.** "Am I ready to go offline?" is a single question,
     and it cannot be answered by four independent progress indicators.
     `TrackStatusView.vue` already reports `navigator.storage.estimate()` for one
     dataset; there is no surface that reports all of them.
- **And one problem that is shared but not about storage at all:** *freshness*. PRD 007
  needs a directory change to reach a device during the event — immediately on
  foreground, within ~60 s while open — and "tell me if this dataset changed" is not
  portrait-specific. Task 171 raised this against the original draft, which framed sync
  purely as pre-event readiness. It has since been **built** for one dataset (tasks 155 and
  162, both done), which resolves the *whether* and leaves the *generalising*: either this
  PRD writes the shipped pattern down as the convention, or the second consumer invents its
  own — which is the duplication this PRD exists to prevent.
- **Why now?** Because two shipped features already reference decisions this document has
  not made (`vite.config.ts` → §11.1; tasks 161, 171 and 173 → this PRD), and a third
  (PRD 002's task 087, the 324 MB race-area prefetch) cannot be sized until the budget
  exists. The cost of it staying in `draft/` is no longer theoretical.

## 3. Goals

- One global storage budget, deliberately allocated in a stated priority order, that
  degrades gracefully instead of letting the OS choose what to lose.
- Data that **cannot be re-fetched** is protected by construction, not by each feature
  remembering to.
- The user can see, in one place, what is cached, when it synced, how much space it uses,
  and what is missing — and can trigger or clear a sync.
- A change on the server reaches a device that is online, during the event, by a
  mechanism each feature does not reinvent.
- Sensitive cached data expires on a **server-issued** deadline and can be purged.
- The app is honest: it never presents stale data as live, and never presents missing data
  as empty.

## 4. Non-Goals

- **A dataset registry, a declaration API, or a generic sync engine.** Cut 2026-09-01;
  see §11.2. Features keep their own fetch-and-store code. This PRD sets the policy they
  observe, not the mechanism they call.
- **Migrating the existing caches onto a common storage primitive.** Tiles belong in the
  Cache API because a service worker serves them; the track belongs in IndexedDB. Moving
  them would be churn with no user-visible benefit. One exception is flagged in §11.10.
- **Offline writes / full bidirectional sync.** Replaying user mutations (conflict
  resolution, ordering, idempotency) is out of scope. Note the location track already
  queues local writes for upload (tasks 082/083) — this PRD does not take that over, it
  only guarantees the queue's storage is not evicted from under it (§6, unrecoverable).
- **Making everything offline.** Some things are inherently live: an SOS, and PRD 007's
  patrol lookup, which is deliberately `no-store` and must stay that way (tasks 157, 170).
  A generic "make it available offline" treatment applied to it would silently undo the
  central privacy property of that feature.
- **Replacing Workbox** or hand-rolling a service worker. Extend what `vite-plugin-pwa`
  gives us.
- **Deciding each dataset's access rules.** PRD 007 owns who may see which portraits;
  this PRD transports whatever the server authorised.
- **Encrypting data at rest on the device.** See §8 — it offers little real protection in
  a browser and would imply a security property we cannot honour.

## 5. User Stories & Scenarios

- As a **samarit**, I want to know before I walk into the woods that I have everything I
  need on my phone, so that I am not surprised at 03:00.
- As a **spejder** with no signal, I want the rulebook, the map and the contact list to
  just work.
- As a **participant on a metered connection**, I want to be told the size before the app
  downloads hundreds of megabytes, and to be the one who decides.
- As a **user whose cache was evicted**, I want to be told that, rather than shown an app
  that looks empty.
- As a **crew member**, I want a member who withdrew ten minutes ago to show as withdrawn,
  not as callable.
- As a **participant whose phone died before an upload**, I want the track it recorded to
  still be there — never sacrificed to make room for map tiles.

### Primary path

1. At onboarding (PRD 005) or check-in, the app offers a **prepare-for-offline** action
   with a total size estimate, and requests storage persistence.
2. Each feature fetches and stores its own data; progress and completion are reported to a
   shared store.
3. A readiness view reports: what is cached, when, how large, and what is missing.
4. During the race, features read from their cache. Where data may be old, the UI says so
   with a timestamp.
5. While online, cheap version checks pull changes; the expensive tier is never fetched
   unprompted.

### Edge cases

- **Quota exceeded.** Evict by declared priority, never arbitrarily, and never
  unrecoverable data. Do not empty a whole cache to make room — the shipped tile config
  gets this right by refusing Workbox's `purgeOnQuotaError`, and that reasoning is the
  general rule, not a tile quirk. The user must be told what was dropped.
- **OS eviction.** Detect on launch, report honestly, re-sync opportunistically. Safari's
  seven-day inactivity eviction does not apply to installed PWAs (§8), which mitigates but
  does not remove this: a user who installs three weeks out and never reopens can still
  arrive empty, and PRD 005 pushes people to install *early*.
- **Partial sync** interrupted by a dead connection or a backgrounded app. Must resume,
  not restart, and never present a half-synced dataset as complete.
- **Metered connection.** The platform cannot tell us the connection type (§8), so nothing
  large may be fetched unprompted, and the size must be shown.
- **Server data changed** while offline or in the background: a version check on
  foreground and reconnect applies the delta (§6, freshness).
- **A field is removed, not changed** — a withdrawn member's phone number must *disappear*
  from a cached row, not merely stop being sent. A delta that only carries present fields
  cannot express this. Raised by task 171.
- **Permission narrowed** while offline. The device holds data it should not. It must be
  purged on the next sync, and the server must decide — the client cannot enforce a rule it
  did not evaluate.
- **Clock skew.** TTLs must not be defeated by a wrongly-set device clock; expiry is
  server-issued, never client-computed.
- **The dormant device.** A phone that never reopens the app keeps its cached data until the
  OS evicts it, and a service worker that never runs cannot purge anything. A baked-in
  server-issued expiry is the only lever we hold. Shared with task 173.
- **Cache schema change across an app upgrade.** For re-fetchable data, discard and
  re-fetch — `contacts.store.ts` already does this on a `schema` mismatch, and that is the
  convention. For unrecoverable data, discarding is data loss, so it must be migrated or
  kept readable. This distinction falls out of the unrecoverable flag below.
- **Storage unavailable** (private mode, exotic browser). Degrade to live-only rather than
  failing. Note `localStorage` throws on *access*, not use, in some Safari privacy modes.

## 6. Requirements

### Functional

- [ ] A **stated priority order** covering every cached dataset, with a global ceiling and
      per-dataset allocations, observed by the features that cache. **Decided 2026-09-01:**

      | rank | dataset | size | evictable |
      |---|---|---|---|
      | 1 | unshipped local writes (location track) | ~195 KB | **never** — unrecoverable |
      | 2 | app shell | ~1 MB | only by the OS |
      | 3 | directory index (names, groups, status, own phone) | < 1 MB | yes, last |
      | 4 | portrait thumbnails | ~1 MB | yes |
      | 5 | map tiles, **by descending zoom** (z16 first) | ~324 MB | yes, first |

      Ceiling: plan the whole origin inside **~1 GB** (iOS 16.4 baseline, §8), budgeting
      ~500 MB for tiles to leave headroom for a larger year. Ranks 1–4 together are under
      ~4 MB, so the order is not a scarcity contest between them — it exists to make one
      trade explicit: **tiles are what gets sacrificed**, highest zoom first, because they
      are the only dataset large enough to matter and z16 is 60% of them. The
      counter-argument is recorded in §11.1 and PRD 002 §11.2 and is real: evicting a tile
      is irreversible in the field. It loses to rank 1 anyway, because a discarded tile
      costs a participant a map they can survive without, while a discarded track is a
      recording that never existed.
- [ ] **`unrecoverable` is a declared property** of a dataset. Unrecoverable data sits at
      the top of the priority order regardless of size, is never evicted to make room for
      recoverable data, and is never discarded on a schema change. Today that is the
      unshipped location track; the property exists so the next such dataset is handled by
      construction. (Was §11.9; promoted, and already cited by shipped code.)
- [ ] **Quota handling** on both the Cache API and IndexedDB: `QuotaExceededError` is
      expected, not exceptional. A write that fails leaves everything already held intact
      and working. Never purge a whole cache to recover.
- [ ] **`navigator.storage.persist()` is requested** once, at install/onboarding, and the
      result is surfaced in the readiness view. (`trackDb.ts` already calls it for its own
      store; the request is per-origin, so it belongs to the app, not to one dataset.)
- [ ] **Storage introspection** via `navigator.storage.estimate()`, aggregated across
      datasets rather than reported per feature.
- [ ] A **readiness surface** showing, per dataset: last sync, item count, size, staleness,
      and a manual sync/clear control — plus the total and the persistence state.
- [ ] A **global "running on cached data" indicator** in the app shell, so no feature
      invents its own.
- [ ] **Tiered fetching.** Each dataset is either *cheap* (fetched opportunistically,
      unconditionally, as a side effect of being displayed) or *expensive* (never fetched
      without an explicit user action carrying a size estimate). Tiles split by zoom:
      z12–14 is ~56 MB (cheap-ish, opportunistic today), z15–16 is ~268 MB (expensive).
- [ ] **The tier governs transfers, not reads, and the two sync classes are separate.**
      *Bulk* transfers (tile sets, portrait images) are pre-race and user-initiated;
      *metadata deltas* — a version check and a few kilobytes of changed rows — are
      explicitly **permitted during the race on mobile data**, because a directory that
      cannot update is a safety problem and the bytes are negligible. A single "no syncing
      during the race" rule would forbid the thing PRD 007 §6 requires, so it is stated as
      two classes rather than one switch. Note also that "wifi-only" is not implementable
      on iOS (§8) — the bulk restriction is *user-initiated with a size estimate*, which is
      the closest honest equivalent. (Task 171.)
- [ ] A **freshness contract** for datasets read during the event — **generalising the
      pattern PRD 007 already shipped**, rather than designing a new one. It exists and
      works: `GET /api/contacts/version` returns a monotonic version for the caller's
      permitted set (task 155), `vue/src/composables/useContactsFreshness.ts` polls it on
      foreground, on a ~60 s interval while visible, and on `online` (task 162), and only a
      changed version triggers a manifest delta. This PRD's job is to state that shape as
      the convention for the next dataset and to fix the parts that must not be re-decided:
      a version endpoint separate from the manifest, cheap enough for a few hundred devices
      to poll continuously, answered from a projection read; the three trigger points; and
      metadata propagating ahead of images. Push is **not** usable for invalidation — iOS
      requires every web push to show a notification, so it would buzz phones over a
      corrected phone number. (Task 171.)
- [ ] **The poll interval is served, not built in** — `contacts_poll_seconds` on
      `/api/config` today. This generalises as a per-dataset runtime value, because the
      reason for the lever is load: during-race polling shares the BFF with PRD 002's
      position reporting, and the interval must be widenable *during* an event. Keep the
      shipped semantics: **0 disables the interval but not the foreground and reconnect
      checks**, so "reduce load" can never silently become "stop updating".
- [ ] **Deltas must be able to express removal**, field-level as well as row-level, so a
      cached value can be unset rather than merely not resent. (Task 171.)
- [ ] **Metadata/index kept separate from binaries**, so lists and search survive the
      eviction of large assets — names must work when portraits are gone.
- [ ] **Server-driven scope**: the server returns only what the caller may hold; the client
      never filters a broader payload.
- [ ] **Server-issued expiry and purge**, including a hard post-event deadline for
      sensitive datasets. Verification is task 173.
- [ ] **The tile budget is fixed for the duration of an event.** The race area is derived
      from this year's checkpoints — convex hull plus a 3 km buffer, served by
      `GET /api/race-area` (task 088) — and **does not change once the event is running**
      (maintainer, 2026-09-01). So the readiness view has an exact denominator (5,291 tiles /
      324 MB for 2026) and "is the map ready?" has a true yes-or-no answer rather than a
      moving one. Two consequences worth stating, since they are what the fixed area buys:
      the budget is computed once at event start, not re-derived; and no eviction policy
      needs a special case for a growing dataset. **Pre-event is different** — the checkpoint
      set is still being edited, so the client must re-check the area before the event and
      re-download only the tiles it does not already hold.

### Non-Functional

- **Honest UI**: stale is labelled stale, missing is labelled missing, and "synced" means
  synced.
- **Frugal**: no large transfer without consent, no *bulk* sync during the race by default
  (metadata deltas excepted, see above), minimal battery cost.
- **Privacy**: sensitive datasets carry short server-issued expiries and are purgeable; this
  layer must not make it easy to accumulate personal data on devices indefinitely. It must
  also not make it easy to accidentally cache something that was deliberately left live
  (§4).
- **Baseline** iOS/iPadOS Safari 16.4+ / Chrome 111+ per `.rules`.
- **Testable offline**: verifiable with the radio off, in CI where possible and on real
  devices where not.

## 7. UX / UI Notes

- **One readiness view**, in the **profile page** (PRD 003, as a section alongside "På denne
  enhed"): a list of datasets with size, last-synced time and state, a prominent "Forbered
  til offline" action, and total storage used. `views/TrackStatusView.vue` already does this
  for one dataset and is the pattern to generalise — and to link to, rather than duplicate.
- **A single global offline indicator** in the app shell when the app is running on cached
  data.
- **Per-feature staleness** is shown inline and quietly (a timestamp, not a warning), except
  where acting on stale data is risky — a phone number for a member who may have withdrawn
  is the live example.
- **First sync is hosted by onboarding** (PRD 005 step 6) — a natural moment, usually on
  wifi, when the user expects setup steps. It is **skippable**, consistent with PRD 005's
  rule that only login is mandatory, which makes §11.6 (re-surfacing a skipped sync) more
  important than it looks.
- shadcn-vue primitives (`Card`, `Progress`, `Badge`, `Button`, `Alert`) and Lucide icons
  (`WifiOff`, `RefreshCw`, `HardDrive`, `Check`) per `.rules`. `progress`, `alert` and
  `card` are generated in `vue/src/components/ui/`; **`badge` is not yet** — generate it
  rather than hand-rolling one. Page and section headings use `font-nathejk`; body text and
  controls stay on the system stack.

## 8. Technical Considerations

- **Frontend (Vue 3 / TS):**
  - `stores/offline.store.ts` — the one new piece: aggregate sync state, progress, storage
    estimate, persistence state and per-dataset status. Features report into it; the
    readiness view and the shell indicator read from it. It holds no data of its own.
  - `config/offline.ts` — the declaration: which datasets exist, their budgets, the priority
    order (array order *is* the order) and the `unrecoverable` flag. It sits in `config/`
    rather than `helpers/offline/` for the reason `config/cache.ts` gives: that file is
    imported by `vite.config.ts` at build time and must stay free of browser-only imports, and
    the tile budget is already split across the two (entry cap there, bytes here). Splitting
    them across halves of the tree would guarantee they drift. *(Decided in task 183.)*
  - `helpers/offline/` — the logic: eviction, the storage-kind adapters, quota handling. Kept
    free of feature specifics. Deliberately small; if it grows a query language or a plugin
    system, something has gone wrong (§4).
  - **Three storage kinds exist, and that is accepted:** the **Cache API** (via Workbox
    routes) for binaries served by the service worker — tiles, portraits; **IndexedDB** for
    structured data that is large or unrecoverable — the track; **`localStorage`** for small
    structured payloads read on a cold, offline route — the contacts directory, which is
    under ~1 MB for the largest role (task 161). The `localStorage` case is blessed with a
    caveat, see §11.10. Keeping indexes out of the binary caches is what makes "names survive
    image eviction" fall out naturally.
  - Workbox gets **a route and expiry policy per binary dataset**, not one undifferentiated
    runtime cache — a shared cache cannot be purged or evicted per dataset. Tiles already
    follow this (`TILE_CACHE_NAME`, and a `cacheKeyWillBeUsed` normalisation). Two hard-won
    details from that work generalise: cache only `status: 200`, because opaque responses are
    padded heavily for quota accounting and will silently blow a budget; and remember Workbox's
    `generateSW` mode *stringifies* config functions into `sw.js`, so any identifier from module
    scope becomes an undefined free variable at runtime.
  - `helpers/pwa.ts` already owns registration; this layer plugs into it rather than
    registering a second worker.
- **BFF (Go):** no new architecture. Two conventions:
  - a **version-check endpoint per cacheable dataset**, cheap enough to poll continuously.
    PRD 007's shipped `GET /api/contacts/version` is the reference implementation and its
    manifest establishes the shape: `version` **in the JSON body** rather than
    `If-None-Match`, deliberately, because `fetchWrapper` does not expose response headers.
    Adopt both as the convention; ETags remain for the browser's own conditional requests.
  - the **poll interval is served from `/api/config`** (`contacts_poll_seconds`, from
    `CONTACTS_POLL_SECONDS`, default 60), and remembered client-side across an offline start
    like the other runtime values. A value the client cannot be told is not a lever.
  - **server-issued expiry** on any sensitive payload, so a wrong device clock cannot defeat a
    purge. `go/cmd/api/portraitpurge.go` is the existing server-side half.
  - Every endpoint carries its own OpenAPI annotations per `.rules`. Datasets are authorised
    server-side per request.
- **Data / storage:** client-side only. No server schema change.
- **Platform limits** (established 2026-08-26, from WebKit's storage policy and Chrome's
  documented quota):

  | platform | origin quota |
  |---|---|
  | iOS/iPadOS **17+**, home-screen web app | **60% of total disk** — explicitly the same quota as in the browser |
  | Android Chrome | **60% of total disk** |
  | iOS **16.4–16.7** | **~1 GB**, raised in 200 MB prompts (pre-Safari-17 policy) |

  The stated baseline is iOS 16.4+, so the binding constraint is **~1 GB**. Plan the whole
  budget inside that and modern devices become a non-issue rather than a separate case.
  Everything is best-effort and evictable by default; WebKit grants `persist()` "based on
  heuristics like whether the website is opened as a Home Screen Web App", which is exactly
  this app's install-first onboarding, so the request should succeed. Safari's seven-day
  inactivity eviction does not apply to installed PWAs — another thing install-first buys.
- **The platform cannot tell us the connection type.** `navigator.connection` is unavailable
  in Safari, and `navigator.onLine` only says online or offline. "Sync only on WiFi" is not
  implementable on iOS. This is why §6 requires tiers and a size estimate instead: the engine
  cannot make the decision, so the user must.
- **Interaction with PRD 008** (done): worth noting the symmetry it establishes. The server
  keeps local projections of an event log so it can serve when the broker is down; the client
  keeps local replicas of server data so it can work when the network is down. Same reasoning,
  two tiers.
- **Dependencies & risks:**
  - **Risk: this arrives after its consumers — realised.** Tiles, the directory and the track
    all shipped their own storage. The response was to shrink this PRD to what is still shared
    rather than to retrofit them (§11.2). The residual risk is a fourth consumer shipping
    before the priority order exists, which is an argument for approving a small version of
    this soon rather than a complete one later.
  - **Risk: iOS eviction defeats the whole idea.** Mitigation is `persist()`, install-first,
    detection and re-sync — not prevention. The readiness view exists precisely because we
    cannot guarantee the cache is there.
  - **Risk: over-engineering.** A registry and declaration API for consumers that already
    exist would be pure ceremony; that is why they were cut. Hold the line at policy plus one
    store.
  - **Risk: encryption theatre.** Encrypting cached portraits at rest would need a key on the
    same device, so it deters nobody with devtools while implying a protection we cannot
    deliver. Deliberately excluded; PRD 007's honesty about exfiltration is the right posture.
  - ~~**Risk: budget starvation between datasets**~~ **Closed 2026-09-01** — the order is
    decided (§6, §11.1). It was never going to be a scarcity contest in practice: everything
    except tiles fits in ~4 MB.
  - **Risk: the priority order is written down and then not observed.** With no registry to
    enforce it, the order is a document, and three features cache independently. The mitigation
    is the quota-exhaustion test in §9 plus the reconciliation task in §10 — not good
    intentions. This is the honest cost of cutting the registry (§11.2).

## 9. Success Metrics

Measurable from the device or by inspection; this PRD introduces no telemetry endpoint, so
nothing here depends on one.

- Every cached dataset verifiably works with the radio off on real iOS and Android devices
  (the offline test protocol, §10).
- `navigator.storage.persist()` returns true on installed iOS and Android devices.
- No unrecoverable data is ever evicted: demonstrated by a quota-exhaustion test, not by
  field reports.
- Total storage stays within the stated ceiling with the race-area tile set present
  (~324 MB) plus every other dataset.
- The readiness view always explains the state — in particular, an evicted cache produces an
  explanation, never an empty-looking page. Verified by test, by clearing storage.
- A directory change made on the server is visible on a foregrounded device within ~60 s.
- No feature ships a second version-check or budget mechanism after this lands.

*Deliberately dropped: the original "≥ 90% of devices report a complete first sync". Nothing
reports that, and adding a readiness-telemetry endpoint to measure it would be a feature of
its own. If we want it, it needs its own decision — see §11.11.*

## 10. Rollout / Task Breakdown

Policy first, because two open tasks were blocked on it and it costs nothing to build; then the
surfaces; then the freshness gap; then the consumers.

Tasks created in `roadmap/tasks/open/` on approval (2026-09-01):

- [ ] **183** — encode the priority order + ceiling as data, with the `unrecoverable` property.
      *Unblocks task 161 and the decision deferred in `vite.config.ts`.*
- [ ] **184** — `offline.store`: aggregate status, storage estimate, persistence state
- [ ] **185** — request `navigator.storage.persist()` at onboarding, surface the result
- [ ] **186** — budget + priority-ordered eviction, incl. `QuotaExceededError` on Cache API and
      IndexedDB
- [ ] **187** — readiness view in the profile page (generalising `TrackStatusView`, linking to
      it rather than duplicating it)
- [ ] **188** — global offline / cached-data indicator in the app shell
- [ ] **189** — generate the shadcn-vue `badge` primitive
- [ ] **190** — document the version-check + poll convention, generalising tasks 155/162
- [ ] **191** — delta shape that can express field-level removal *(closes task 171)*
- [ ] **192** — reconcile the existing caches with the agreed budget — tiles, directory, track,
      shell. *No rewrites; declared sizes, ranks and flags. This is the task that decides
      whether this PRD took effect or merely produced a document.*
- [ ] **193** — server-issued expiry + post-event purge on the device *(with task 173)*
- [ ] **194** — prepare-for-offline step in onboarding, filling PRD 005's reserved slot
- [ ] **195** — offline test protocol: radio off on real devices, plus quota exhaustion

Already shipped, and **not** tasks here: the Workbox tile route and opportunistic tile caching
(task 087's cheap half), the IndexedDB track store, the `localStorage` directory cache, the
shell precache, and the freshness loop for one dataset (tasks 155, 162).

Existing open tasks that this PRD's approval unblocks or closes: **161** (declare the
directory's datasets — explicitly blocked on 009), **171** (freshness contract), **173**
(post-event purge verification), **087** (race-area tile prefetch, needed the budget — now
decided).

**Nothing in this PRD is waiting on an input.** The two that were outstanding — the priority
order and the race area — were settled on 2026-09-01 (§11.1), and the PRD was approved the same
day.

## 11. Open Questions

1. **What is the global ceiling, and what is the priority order?** **Answered 2026-09-01**
   (maintainer: "priority confirmed"). The order and the ceiling now live in §6 as a
   requirement; the measurements that produced them are kept here.

   Ceiling: **~1 GB** (iOS 16.4 baseline, §8), with **~500 MB planned for tiles, ~360 MB
   expected**. Order: unrecoverable local writes → app shell → directory index → portraits →
   tiles by descending zoom. **Tiles are what gets sacrificed**, z16 first.

   **The race area is settled too** (maintainer, 2026-09-01): it is *deduced from the
   checkpoints* — already built and served as `GET /api/race-area`, convex hull plus a 3 km
   buffer (task 088) — and **does not change during an event**. So this PRD needs no bounds
   handed to it, and the tile budget is a fixed, knowable number for the whole event rather
   than a moving target. That is now a §6 requirement, and it is what makes "is the map
   ready?" answerable exactly. It also retires the last open input this PRD had.

   The measurements behind the numbers:

   - **Ceiling:** plan inside **~1 GB** (iOS 16.4 baseline, §8).
   - **Tiles** — measured, not estimated (256 px tiles, central Zealand, 2026-08-26):

     | zoom | tile edge | DTK25 (PNG) | aerial (JPEG) |
     |---|---|---|---|
     | 12 | 5.53 km | 140 kB | 14 kB |
     | 14 | 1.38 km | 99 kB | 11 kB |
     | 16 | 0.35 km | 32 kB | 9 kB |

     ~0.45 MB/km² topo z12–16 plus ~0.11 MB/km² aerial. The scope decided in PRD 002 §11.2 is
     the whole race area — the convex hull of this year's checkpoints plus a 3 km buffer,
     **476 km² / 358 MB** as published for 2026 (428 km² geometric / 324 MB, plus an outward
     ~1.1 km disclosure grid that stops the buffer being inverted to recover checkpoint
     positions). The area is roughly the same size every year (maintainer), so **plan for
     ~500 MB, expect ~360 MB**:

     | race area | z12–16 | z12–15 |
     |---|---|---|
     | 300 km² | 232 MB | 93 MB |
     | **428 km² (2026)** | **324 MB** | **129 MB** |
     | 600 km² | 446 MB | 175 MB |
     | 700 km² | 517 MB | 202 MB |

     z16 alone is ~60% of the total and is **not** a candidate for trimming: DTK25 is a 508 DPI
     1:25.000 product with a native 1.25 m/px resolution, which is z16 (1.34 m/px), so z15
     halves the source map's resolution. z17 is 2× oversampled, carries no new information, and
     would cost an extra ~267 MB for a 20×20 km area — which is also why its tiles *shrink*. The
     aerial layer is different: 12.5 cm native imagery genuinely holds detail deeper. (It was
     also being fetched as PNG at ~15× the JPEG cost; fixed in `fix(040)`.)
   - **Directory + portraits:** under **~1 MB** for the largest role — ~151 banditter ≈ 0.7 MB,
     ~99 gøglere ≈ 0.4 MB, ~20 crew ≈ 0.1 MB at `thumb256` ≈ 4.5 KB (tasks 078, 104, 161).
   - **Location track:** ~**195 KB** per person for a 12-hour race at 30 s sampling — and
     **unrecoverable**.

   The budget's shape: **tiles are ~99% of it**, everything else is rounding. The one real
   trade inside the order — **may tiles be evicted to protect unrecoverable data?** — is
   answered **yes**. The counter-argument recorded in PRD 002 §11.2 and task 087 is real and
   was weighed: **evicting a tile is irreversible in the field**, since a tile discarded in a
   dead spot cannot be re-fetched. It loses anyway, because a lost tile costs a participant a
   map they can manage without, while a lost track is a recording that never existed.

2. **Should this PRD own a generic sync engine and dataset registry?** **Answered
   2026-09-01: no — cut.** The original draft's premise was that generalising *before* the
   consumers shipped would save a rewrite. Reviewing against the code, three consumers had
   already shipped incompatible-by-design storage: Workbox/Cache API for tiles,
   `localStorage` for the directory, IndexedDB for the track. A registry over three existing,
   working, differently-shaped implementations is ceremony, and §8 already named
   over-engineering as the risk. Kept: the budget, the readiness surface, the freshness
   contract — the parts that are still shared and still missing. Task 177 (thinning
   `contacts.store`) landed on the same reasoning from the other side, and
   `contacts.store.ts` says so in a header comment.

3. **Where does the readiness view live?** **Answered:** the profile page (PRD 003). Listed
   only because 003 is `done`, so this arrives as an addition to a shipped page.

4. **Is the member directory cached in full, or only the user's own team?** **Answered by PRD
   007 (approved 2026-08-31), which supersedes the question as originally posed.** What is
   cached is the crew/bandit/gøgler directory, scoped server-side by race role, holding name,
   population, groups, the person's **own** phone, crew function, in-race status and a
   portrait hash. **Spejdere are not cached and not browsable**; the patrol lookup is live and
   stores nothing. Postal addresses are not cached. **Guardian phone numbers are not a
   trade-off to weigh here at all** — per `.rules` they never enter the PWA except a user
   confirming their own, the BFF projects `phoneParent` out of these responses, and no dataset
   registered under this PRD may carry one. The original draft's framing of this as a
   disclosure judgement was wrong and has been removed.

5. **How long should sensitive caches live after the event, and can a purge be made reliable
   when the service worker may never run again?** Still open, and narrower than it was: only
   adults' records are cached, and no spejder data is on any device. The dormant device remains
   unsolved in principle — a baked-in server-issued expiry is the only lever we hold. Shared
   with PRD 007 §11 and task 173.

6. **How prominently should a skipped first sync be re-surfaced?** The step is skippable (PRD
   005 allows only login to be mandatory), so a user can reach the race with nothing cached —
   and skipping is a *reasonable* choice for someone on cellular facing a 324 MB estimate.
   Since blocking would contradict 005, the lever is nagging: a persistent readiness banner, a
   reminder at check-in, or the push in §11.7.

7. **Do we need a "prepare for offline" push at event start** — a notification a few hours
   before, prompting a final sync while the user still has coverage? This is the natural vehicle
   for the expensive tier: it catches people at home, on WiFi, before it matters.

8. **Which datasets are genuinely live-first?** Settled for two: **PRD 007's patrol lookup**
   (live, `no-store`, by decision) and **SOS**. Still open for **event updates** — an update
   that only arrives online is a defensible design, but an update nobody can read at 03:00 may
   not be.

9. **Should "unrecoverable" be a declared property?** **Answered: yes — promoted into §6.** The
   unshipped location track (tasks 082/083) exists only on the device until a batch is accepted;
   unlike tiles, the directory or the rulebook it cannot be re-fetched. It is small (~195 KB), so
   protecting it costs nothing, and it belongs at the top of the priority order regardless of
   size. `vite.config.ts` already cites this section to justify not purging the tile cache on a
   quota error.

10. **Should the contacts directory move from `localStorage` to IndexedDB?** New, 2026-09-01.
    `localStorage` is synchronous, string-only, ~5 MB per origin, and throws on *access* in some
    Safari privacy modes. At under ~1 MB the directory fits comfortably and the store handles the
    failure modes carefully, so there is no reason to move it today. The question is whether the
    5 MB ceiling is close enough to a growing crew directory to be worth pre-empting — and
    whether `localStorage` should be a blessed storage kind for future datasets or an accepted
    exception for this one. Recommendation: accepted exception, with a size assertion in the
    directory's tests.

11. **Do we want readiness telemetry?** New, 2026-09-01. The original "≥ 90% of devices report a
    complete first sync" metric had no mechanism behind it. Knowing, before the race, how many
    devices are actually prepared has obvious operational value — but it means a BFF endpoint
    receiving per-device cache state, which is a feature with its own privacy question, not a
    line in a metrics list. Left out of this PRD deliberately; raise it separately if the
    organisers want it.
