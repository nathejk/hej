# PRD 009 — Offline-first client data layer (local replicas of everything cacheable)

**Status:** draft
**Author:** agent session (Zed)
**Created:** 2026-08-25
**Last updated:** 2026-08-25
**Approved:**
**Shipped:**
**Target users:** all app users, indirectly — this is the mechanism behind every offline-capable feature

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 1. Summary

One mechanism for keeping local copies of server data on the device, so that
**anything that can work offline does**: portraits, map tiles, the member
directory, the rulebook, contacts, schedule, updates. Each dataset declares how it
syncs, how big it is and how long it lives; a single sync engine, cache policy and
storage budget serve all of them, with one honest place in the UI telling the user
what they have.

## 2. Problem & Motivation

- **What problem does this solve?** The app is used in a forest, at night, on a
  phone that has been running for hours. Connectivity is the exception, not the
  rule. Yet each feature has so far been designed with its own idea of offline:
  PRD 007 specifies a bespoke portrait sync engine, PRD 002 caches map tiles, the
  rulebook and contacts are precached incidentally by Workbox, and the member
  directory is not cached at all. Left alone this produces four sync
  implementations, four cache-eviction stories, four progress indicators, and no
  single answer to "what will work when I have no signal?"
- **Why a shared layer rather than per-feature caching?** Three things only work
  when solved once:
  1. **The storage budget is global.** Portraits and map tiles compete for the same
     quota, and the OS evicts per-origin, not per-feature. Nobody can budget
     sensibly in isolation — PRD 007 already had to flag "agree a split with PRD
     002" as a task, which is a symptom of a missing layer.
  2. **Eviction and staleness are the same problem everywhere.** Every dataset
     needs "when did I last sync, is it stale, re-fetch when a network appears".
     Written four times, it will behave four ways.
  3. **The user needs one answer.** "Am I ready to go offline?" is a single
     question. It cannot be answered by four independent progress bars.
- **Why now?** PRD 007 is about to encode a portrait-specific sync engine, and PRD
  002 is already in `doing/` with map tiles. Generalising after both ship means
  rewriting them; generalising now means 007 becomes a consumer instead of an
  implementation.
- **Evidence.** `vite.config.ts` already configures `VitePWA` with Workbox,
  `registerType: 'prompt'`, a custom `push-sw.js` and `includeAssets` — so a
  service worker and a precache exist, but only for the app shell. Nothing in
  `vue/src/stores/` caches server data: `scans.store`, `location.store` and
  `session.store` are all live-fetch. The gap is data, not the shell.

## 3. Goals

- Every dataset that *can* be available offline *is*, without each feature
  inventing a mechanism.
- One global storage budget, deliberately allocated, that degrades gracefully
  instead of letting the OS choose what to lose.
- The user can see what is cached, when it synced, and how much space it uses —
  and can trigger or clear a sync.
- A feature author adds a cached dataset by declaring it, not by writing sync code.
- Sensitive cached data expires and can be purged.
- The app is honest: it never presents stale data as live, and never presents
  missing data as empty.

## 4. Non-Goals

- **Offline writes / full bidirectional sync.** Queuing user mutations for later
  replay is a much larger problem (conflict resolution, ordering, idempotency) and
  is out of scope. The one existing candidate — position reporting during the race
  — is PRD 002's, and audit events (PRD 007) are fire-and-forget batches, not sync.
- **Making everything offline.** Some things are inherently live (a fresh event
  update, an SOS). This PRD makes caching *possible and uniform*, not *universal*.
- **Replacing Workbox** or hand-rolling a service worker. Extend what
  `vite-plugin-pwa` gives us.
- **A client-side database abstraction / ORM.** Storage primitives, not a query
  engine.
- **Deciding each dataset's access rules.** PRD 007 owns who may see which
  portraits; this PRD transports whatever the server authorised.
- **Encrypting data at rest on the device.** See §8 — it offers little real
  protection in a browser and would imply a security property we cannot honour.

## 5. User Stories & Scenarios

- As a **samarit**, I want to know before I walk into the woods that I have
  everything I need on my phone, so that I am not surprised at 03:00.
- As a **spejder** with no signal, I want the rulebook, the map, my patrol's faces
  and the contact list to just work.
- As a **participant on a metered connection**, I want to be warned before the app
  downloads tens of megabytes.
- As a **user whose cache was evicted**, I want to be told that, rather than shown
  an app that looks empty.
- As a **feature author**, I want to declare "this dataset is cacheable, this is
  its budget and TTL" and get sync, progress and eviction handling for free.

### Primary path

1. At onboarding (PRD 005) or check-in, on wifi, the app performs a **first full
   sync** across every registered dataset, showing one combined progress view and a
   total size estimate.
2. Each dataset is fetched incrementally and stored under its own cache with its own
   budget.
3. A readiness view reports: what is cached, when, how large, and what is missing.
4. During the race, features read from cache first. Where a dataset is live-first,
   cache is the fallback and the UI says so.
5. When a network reappears, stale datasets re-sync in the background, cheapest
   first.

### Edge cases

- **Quota exceeded.** Evict by declared priority, never arbitrarily. The user must
  be told what was dropped. Portraits and map tiles losing a coin toss is not
  acceptable behaviour.
- **OS eviction** (iOS clears service-worker caches for unused web apps within
  days). Detect on launch, report honestly, re-sync opportunistically. This is the
  single most likely real-world failure, and it interacts with PRD 005 pushing
  users to install *early*: install three weeks out, never reopen, arrive empty.
- **Partial sync** interrupted by a dead connection or a backgrounded app. Must
  resume, not restart, and never present a half-synced dataset as complete.
- **Metered connection.** Never large-sync unprompted; use `navigator.connection`
  where available, and prompt with a size estimate where it is not.
- **Server data changed** while offline. Version/etag per dataset; the delta applies
  on reconnect.
- **Permission narrowed** while offline (a crew member reassigned, so they may no
  longer see certain portraits). The device holds data it should not. It must be
  purged on the next sync, and the server must be the one that decides — the client
  cannot be trusted to enforce a rule it did not evaluate.
- **Clock skew.** TTLs must not be defeated by a device clock set wrongly; prefer
  server-issued expiry over client-computed.
- **Storage unavailable** (private mode, exotic browser). Degrade to live-only
  rather than failing.

## 6. Requirements

### Functional

- [ ] A **dataset registry**: each cacheable dataset declares a name, fetch/delta
      strategy, storage kind (structured vs binary), size estimate, priority, TTL
      and whether it is cache-first or live-first.
- [ ] A **sync engine** that is incremental, resumable, cancellable, and reports
      progress per dataset and in aggregate.
- [ ] **Budget management**: a global ceiling, per-dataset allocations, and
      priority-ordered eviction when the ceiling is hit.
- [ ] **Storage introspection** via `navigator.storage.estimate()`, surfaced to the
      user.
- [ ] A **readiness surface** showing per dataset: last sync, item count, size,
      staleness, and a manual sync/clear control.
- [ ] **Cache-first reads** for registered datasets, with a documented staleness
      indicator where data may be old.
- [ ] **Metadata/index separate from binaries**, so that search and lists survive
      the eviction of large assets (PRD 007 needs exactly this: names must work when
      images are gone).
- [ ] **TTL and purge**, including a hard post-event purge for sensitive datasets.
- [ ] **Server-driven scope**: the server returns only what the caller may hold; the
      client never filters a broader payload.
- [ ] Initial dataset registrations: **portraits** (007), **map tiles** (002),
      **member directory / own team** (006), **rulebook**, **contacts**,
      **schedule**, **patrol scan history** (002 — cache-first with a visible
      staleness timestamp, since it is read at checkpoints with poor coverage).
      Updates and SOS stay live-first (§11.8).
- [ ] A documented way to add a dataset, so the next feature does not hand-roll one.

### Non-Functional

- **Offline-first is the default posture** for anything registered.
- **Honest UI**: stale is labelled stale, missing is labelled missing, and "synced"
  means synced.
- **Frugal**: no large transfer on metered connections, no sync during the race by
  default, minimal battery cost.
- **Privacy**: sensitive datasets carry short TTLs and are purgeable; the layer must
  not make it easy to accumulate personal data on devices indefinitely.
- **Baseline** iOS/iPadOS Safari 16.4+ / Chrome 111+ per `.rules`.
- **Testable offline**: features must be verifiable with the radio off, in CI where
  possible and on real devices where not.

## 7. UX / UI Notes

- **One readiness view**, in the **profile page** (PRD 003 §7, as a section
  alongside "På denne enhed"): a list of datasets with size, last-synced time and
  state, a prominent "Forbered til offline" action, and total storage used. *(Placement
  decided 2026-08-25 rather than left open in two documents.)*
- **A single global offline indicator** in the app shell when the app is running on
  cached data, so no feature has to invent its own.
- **Per-feature staleness** is shown inline and quietly (a timestamp, not a
  warning), except where acting on stale data is risky.
- **First sync is hosted by onboarding** (PRD 005 step 6) — a natural moment,
  usually on wifi, when the user has the app open and expects setup steps. It is
  **skippable**, consistent with PRD 005's rule that only login is mandatory.
- shadcn-vue primitives (`Card`, `Progress`, `Badge`, `Button`, `Alert`) and Lucide
  icons (`WifiOff`, `RefreshCw`, `HardDrive`, `Check`) per `.rules`. Note `progress`,
  `badge` and `alert` are **not yet generated** in `vue/src/components/ui/`. Page and
  section headings use `font-nathejk`; body text and controls stay on the system
  stack.

## 8. Technical Considerations

- **Frontend (Vue 3 / TS):**
  - `helpers/offline/` — the registry, the sync engine, and the budget/eviction
    policy, kept free of feature specifics.
  - `stores/offline.store.ts` — sync state, progress, storage estimate, per-dataset
    status; the single source for the readiness view and the shell indicator.
  - **Two storage kinds, deliberately:** the **Cache API** (via Workbox routes) for
    binaries — portraits, map tiles — because that is what a service worker serves
    from; **IndexedDB** for structured data and indexes. Keeping them apart is what
    makes "names survive image eviction" fall out naturally.
  - Extend the existing `VitePWA`/Workbox configuration with a route and expiry
    policy per binary dataset, rather than one undifferentiated runtime cache — a
    shared cache cannot be evicted or purged per dataset.
  - `helpers/pwa.ts` already owns registration; this layer plugs into that rather
    than registering a second worker.
- **BFF (Go):** no new architecture, but a **convention** every cacheable endpoint
  follows: a manifest/delta shape with `If-None-Match`/version support, so the sync
  engine is generic. Each endpoint still carries its own OpenAPI annotations per
  `.rules`. Datasets are authorised server-side per request (PRD 006 supplies the
  role).
- **Data / storage:** client-side only. No server schema change.
- **Interaction with PRD 008:** the *mechanism* is independent, but the dataset
  registrations are not: portraits and the member directory cannot be populated
  until PRD 008 (persistence, blob store) and PRD 006 (directory) land, and the
  manifest/delta convention below needs endpoints that do not exist before them. The
  cheap datasets (rulebook, contacts, schedule) are already servable today, which is
  why they come first in §10. Worth noting the symmetry: the server keeps local
  projections of an event log so it can serve when the broker is down; the client
  keeps local replicas of server data so it can work when the network is down. Same
  reasoning, two tiers.
- **Dependencies & risks:**
  - **Risk: this layer arrives after its consumers.** PRD 002 is in `doing/` with
    map tiles, and PRD 007 specifies its own sync engine. If this is not agreed
    first, it becomes a refactor of two shipped features instead of a foundation.
    That is the main reason to decide it now.
  - **Risk: iOS eviction defeats the whole idea.** Mitigation is detection and
    re-sync, not prevention; the readiness view exists precisely because we cannot
    guarantee the cache is there.
  - **Risk: over-engineering.** A registry and eviction policy for two datasets
    would be silly; for six it is not. The line to hold is that this stays a thin
    mechanism, not a framework — if it grows a query language, something has gone
    wrong.
  - **Risk: encryption theatre.** Encrypting cached portraits at rest would need a
    key on the same device, so it deters nobody with devtools while implying a
    protection we cannot deliver. Deliberately excluded; PRD 007's honesty about
    exfiltration is the right posture.
  - **Risk: budget starvation between datasets** if priorities are set casually.
    They need a deliberate order, decided with the features' owners.

## 9. Success Metrics

- Every registered dataset verifiably works with the radio off on real iOS and
  Android devices.
- ≥ 90% of devices report a complete first sync before the race starts.
- Zero incidents of the app appearing empty due to silent eviction (i.e. the
  readiness view always explains the state).
- No feature ships its own sync engine after this lands.
- Total storage stays within the agreed ceiling, with no OS-initiated eviction of a
  high-priority dataset during an event.

## 10. Rollout / Task Breakdown

Sequence the mechanism, then migrate consumers one at a time, cheapest first
(rulebook/contacts), then the directory, then portraits, then map tiles — so the
riskiest, largest datasets move onto a mechanism that is already proven.

Proposed tasks for `roadmap/tasks/open/`:

- [ ] Task: dataset registry + declaration API
- [ ] Task: sync engine — incremental, resumable, progress reporting
- [ ] Task: budget + priority-ordered eviction policy
- [ ] Task: storage estimate + `offline.store`
- [ ] Task: Workbox route/expiry policy per binary dataset
- [ ] Task: IndexedDB metadata/index store separate from binary caches
- [ ] Task: readiness view (dataset list, sizes, last sync, manual sync/clear)
- [ ] Task: global offline indicator in the app shell
- [ ] Task: BFF manifest/delta convention + etag support (documented for reuse)
- [ ] Task: register rulebook + contacts + schedule
- [ ] Task: register member directory / own team
- [ ] Task: register portraits (replaces PRD 007's bespoke engine)
- [ ] Task: register map tiles (coordinate with PRD 002)
- [ ] Task: TTL + post-event purge for sensitive datasets
- [ ] Task: first-sync step in onboarding (coordinate with PRD 005)
- [ ] Task: offline test protocol, radio off, real devices

## 11. Open Questions

1. **What is the global storage ceiling**, and what are the per-dataset priorities?
   Needs the participant counts and tile-set size. **Not blocked on PRD 007 or 002
   shipping** — only on their sizing *answers* (007 §11.4/§11.5, and 002's tile-set
   estimate), which both have tasks to supply.

   **Platform ceilings established 2026-08-26**, from WebKit's storage policy and Chrome's
   documented quota, so the budget can be planned against real limits:

   | platform | origin quota |
   |---|---|
   | iOS/iPadOS **17+**, home-screen web app | **60% of total disk** — explicitly the same quota as in the browser |
   | Android Chrome | **60% of total disk** |
   | iOS **16.4–16.7** | **~1 GB**, raised in 200 MB prompts (pre-Safari-17 policy) |

   The repo's stated baseline is **iOS 16.4+**, so the binding constraint is the **~1 GB**
   figure, not 60% of disk. Recommendation: plan the whole budget — tiles, portraits,
   directory, app shell, unshipped position track — to fit comfortably inside ~1 GB, which
   then makes modern devices a non-issue rather than a separate case.

   Two policy details that matter more than the numbers:

   - Everything is **best-effort and evictable** by default. `navigator.storage.persist()`
     is required, and WebKit grants it "based on heuristics like whether the website is
     opened as a Home Screen Web App" — which is exactly this app's install-first
     onboarding, so the request should actually succeed.
   - Safari's seven-day inactivity eviction **does not apply to installed PWAs**. Another
     thing the install-first decision buys.
   - Quota is an upper bound with no guarantee, so `QuotaExceededError` must be handled on
     both Cache API and IndexedDB rather than treated as unreachable.

   **Map tile sizing, measured against the live service** (256 px tiles, central Zealand,
   2026-08-26) rather than estimated:

   | zoom | tile edge | DTK25 (PNG) | aerial (JPEG) |
   |---|---|---|---|
   | 12 | 5.53 km | 140 kB | 14 kB |
   | 14 | 1.38 km | 99 kB | 11 kB |
   | 16 | 0.35 km | 32 kB | 9 kB |

   Cumulative for a square race area, topo z12–16: **~0.46 MB/km²**, plus ~0.11 MB/km² for
   the aerial layer. So 10×10 km ≈ **51 MB**, 20×20 km ≈ **184 MB**, 30×30 km ≈ **400 MB**
   (topo only).

   Two findings that change the shape of the budget:

   - **Stopping the topo at z16 is nearly free in information terms.** DTK25 is a 1:25.000
     map, and its tiles *shrink* with zoom (140 kB at z12 → 20 kB at z17) because they are
     the same cartography upsampled. Caching z17 for a 20×20 km area costs an extra
     **267 MB and contains no additional map information**. The aerial layer is different —
     12.5 cm native imagery genuinely holds detail deeper.
   - The aerial layer was being fetched as **PNG**, costing ~15× JPEG. Fixed in the map
     code (`fix(040)`); the JPEG figures above are what the budget should assume.

   Outstanding input: the **race area bounds**, which the maintainer will specify.
2. **Does PRD 002's map-tile caching move onto this layer**, given it is already in
   `doing/`? 002 §11.2 now flags this and its non-goal has been revised to point
   here. Tiles are probably the largest dataset, so leaving them out defeats the
   shared budget — but retrofitting is real work.

   **Partially answered 2026-08-26:** tiles *are* to be cached, scoped to a specified race
   area and possibly a restricted zoom range (maintainer). The presumption is now this
   shared layer rather than something map-local, but the mechanism remains this PRD's call.
3. **Where does the readiness view live** — *decided*: the profile page (PRD 003
   §7). Listed only because 003 must actually carry it.
4. **Is the member directory cached in full, or only the user's own team?** Full
   makes offline search work everywhere, but puts a copy of every participant's
   personal details on every device — which is a much bigger disclosure than
   portraits and probably unacceptable for minors' addresses and guardian numbers.
   Strong recommendation: **only what the user may see, and only the fields the UI
   needs**.
5. **How long should sensitive caches live** after the event, and can a purge be
   made reliable when the service worker may never run again? (Shared with PRD 007
   §11.)
6. **How prominently should a skipped first sync be re-surfaced?** The step is
   skippable (PRD 005 allows only login to be mandatory), so a user can reach the
   race with nothing cached. A mandatory sync would contradict 005's rule, so the
   lever is nagging rather than blocking: a persistent readiness banner, a reminder
   at check-in, or the push in §11.7 below.
7. **Do we need a "prepare for offline" prompt at event start** — a push
   notification a few hours before, prompting a final sync while the user still has
   coverage?
8. **Which datasets are genuinely live-first?** Proposed: updates and SOS. Confirm
   — an event update that only arrives online is a reasonable design, but an update
   nobody can read at 03:00 may not be.
9. **The unshipped position track is a dataset with a property nothing else here has.**
   Added 2026-08-26 from PRD 002 §11.1: the app now records a location track locally and
   uploads it every 2 minutes when changed (tasks 082/083). Until a batch is accepted by
   the server, those points exist **only on the device**.

   That puts it in the same category as portrait bytes (PRD 007): unlike tiles, the
   directory or the rulebook, it **cannot be re-fetched** if evicted — it is simply gone.
   Every other dataset here treats eviction as a performance problem; for these two it is
   data loss.

   It is small — ~195 KB per person for a 12-hour race at the chosen 30 s sampling — so it
   costs
   almost nothing to protect. The question for this PRD is whether "unrecoverable" should
   be a **declared property** of a registered dataset, so the sync engine and the eviction
   priority order treat it and portraits differently by construction rather than by each
   feature remembering to. Recommendation: yes, and it should sit at the top of the
   priority order regardless of size.
