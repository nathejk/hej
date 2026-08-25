# PRD 002 — Event Map (own position, Danish topo + aerial layers, patrol scan history)

**Status:** doing
**Author:** agent session (Zed / Claude Opus 5)
**Created:** 2026-08-24
**Last updated:** 2026-08-25
**Approved:** 2026-08-24
**Shipped:**
**Target users:** spejder (patrol members) primarily; bandit, postmandskab, guide, samarit secondarily

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See the `prd` skill for the lifecycle.
-->

---

## 1. Summary

Turn the placeholder `Kort` page (`vue/src/views/MapsView.vue`) into a real,
full-bleed map that fills everything above the bottom navigation. It shows the
user's live position, offers three switchable base layers (Danish topographic
1:25.000 and 1:50.000, plus aerial imagery), and plots where the user's patrol
has been registered during the event — checkpoint scans and bandit catches —
available both as map markers and as a chronological list.

## 2. Problem & Motivation

- **What problem does this solve?** During Nathejk the patrol navigates at night
  across a large rural area using paper maps and guesswork about where they are
  and how far they have come. The app already asks for location permission
  (task 015 / PRD 001) but has nowhere to show it. Patrols and their leaders
  also have no way to see their own progress: which posts they have reached and
  when/where they were caught by bandits.
- **Why now?** PRD 001 deliberately deferred map rendering ("Map rendering that
  consumes `position` lands in a later feature PRD" — `location.store.ts`). The
  shell, auth, roles and geolocation plumbing are shipped (tasks 001–023 done),
  so the map is the first real feature page and the highest-value one: `Kort` is
  the first destination in `vue/src/config/navigation.ts`.
- **Evidence.** Explicit product request from the event organizers; an existing
  Nathejk repo already renders these exact layers with Leaflet, so the approach
  is proven in-platform (the base-layer definitions in this PRD are lifted from
  that repo).

## 3. Goals

- A patrol member can open `Kort` and, within a few seconds, see where they are
  on an official Danish topographic map.
- The user can switch between the map renderings they are used to on paper
  (DTK 1:25.000, DTK 1:50.000) and aerial imagery.
- A patrol can see its own event history — checkpoint scans and bandit catches —
  in both a spatial (map) and a temporal (list) view.
- The map is usable one-handed, at night, on a mid-range phone with poor mobile
  coverage.
- Screen real estate is maximised: nothing but the bottom nav competes with the
  map.

## 4. Non-Goals

- **Live tracking of other patrols/personnel** on the map (no fan-out of other
  users' positions). Own position only.
- **Continuous position reporting to the BFF** (breadcrumb trail / server-side
  tracking). Out of scope **for this PRD's shipping scope**; see PRD 003 for the
  location-sharing *preference* surface. **Note (2026-08-25):** §11.1 makes this PRD
  the owner of the *design decision* for how a position track would be transported
  (telemetry stream vs direct write) even though building it stays out of scope.
  Deciding is in scope; implementing is not.
- **Building offline tile caching.** Tiles are registered as a **PRD 009 dataset**
  (cache-first, budgeted, priority-ordered) rather than cached ad hoc here; this PRD
  must not add its own precache, runtime cache or storage policy for tile hosts.
  *(Revised 2026-08-25 — previously this read "the service worker must not attempt
  to precache them", which contradicted PRD 009. See §11.2.)*
- **Routing, navigation instructions, distance-to-next-post calculations.**
- **Drawing the course, post locations we have not yet visited, or zone
  boundaries.** Only *our own* registrations are plotted.
- **Editing or correcting a scan.** The map is read-only.
- **Scanning itself** (QR/NFC at a checkpoint) — that is a separate feature owned
  by post personnel tooling, not this page.

## 5. User Stories & Scenarios

- As a **spejder**, I want to see my own position on a 1:25.000 topographic map
  so that I can work out where we are without waiting for a landmark.
- As a **spejder**, I want to switch to aerial imagery so that I can recognise
  terrain features (forest edges, buildings) that the topo map abstracts away.
- As a **spejder**, I want to see the posts we have already been scanned at so
  that I know our progress and can tell a leader where we came from.
- As a **spejder**, I want to see where and when a bandit caught us so that we
  can account for our route.
- As a **postmandskab / guide / samarit**, I want the same map so that I can
  orient myself in the terrain around my post.

**Primary happy path**

1. User taps `Kort` in the bottom nav.
2. The map fills the screen above the nav, centred on the event area, DTK
   1:25.000 selected by default.
3. If location permission is already granted, a position marker with an accuracy
   circle appears and the map recentres on it; the map then follows the user
   until they pan manually.
4. If permission was **denied** earlier (or the user skipped it during
   onboarding), a repair affordance overlays the map ("Vis din placering");
   accepting triggers `location.store.request()`. The *first* request happens in
   onboarding (PRD 005), not here — revised 2026-08-25, since PRD 005 owns
   settling permissions up front rather than at 02:00 in a forest.
5. The user taps the layer control and switches to `Luftfoto`; the position
   marker, scan markers and zoom level are preserved.
6. Scan markers (checkpoint = one icon, bandit catch = another) are plotted;
   tapping one opens a popup with the post/bandit name and a timestamp.
7. The user taps a "Registreringer" handle to raise a bottom sheet listing the
   same registrations newest-first; tapping a row closes the sheet, pans to that
   marker and opens its popup.

**Edge cases and errors**

- **Permission denied / unavailable:** map still works fully; a locate button
  shows a disabled/denied state with a short hint on how to re-enable it in
  browser settings. No blocking dialog.
- **Position not yet fixed / times out:** locate button shows a pending state;
  after the store's 10s timeout, a non-blocking message ("Kunne ikke finde din
  placering").
- **Outside the event area:** we do not clamp the position; the user can pan
  back via the locate button.
- **Tile server unreachable or token rejected:** the map renders grey tiles; show
  a dismissible notice that map images could not be loaded. Position and markers
  must still render.
- **No registrations yet:** the list shows an empty state ("Ingen registreringer
  endnu") and no scan markers.
- **User has no patrol** (personnel roles, or a spejder whose patrol cannot be
  resolved): the registrations affordance is hidden entirely; the map works.
- **Scan without coordinates** (e.g. a manual registration): it appears in the
  list, marked as having no position, and is not plotted.
- **Backgrounded app:** watching position stops when the page is hidden and
  resumes on focus, to save battery.

## 6. Requirements

### Functional

- [ ] `Kort` occupies the full viewport minus the bottom nav: no page header, no
      padding, no scroll container. Map controls float over the map and respect
      `env(safe-area-inset-*)`.
- [ ] Three mutually exclusive base layers, selectable from an on-map control.
      **All three are Dataforsyningen WMS services, verified against live
      GetCapabilities + GetMap on 2026-08-24:**

      | Label | Service | WMS layer |
      |---|---|---|
      | `Topografisk 1:25.000` | `https://api.dataforsyningen.dk/dtk_25_DAF` | `dtk25` |
      | `Topografisk 1:50.000` | `https://api.dataforsyningen.dk/dtk_50_DAF` | `dtk_50` |
      | `Luftfoto` | `https://api.dataforsyningen.dk/orto_foraar_DAF` | `orto_foraar` |

      Layer names are **not** symmetric between services — `DTK50` returns a
      `ServiceException`, only `dtk_50` works — so do not infer names by analogy.
- [ ] The selected layer persists across navigation within the session and
      across reloads (`localStorage`, key namespaced `hej.map.*`).
- [ ] **Opening view:** centred on the user's own position when available,
      otherwise framed on **Sjælland**. The event area is deliberately *not* used
      as a default — it is not fully known to participants and the map must not
      reveal it.
- [ ] **Failed tiles are retried** with exponential backoff and jitter (Leaflet has
      no built-in retry; one failed request otherwise leaves a permanently grey
      tile). The failure notice appears only after retries are exhausted, and clears
      itself when tiles load again.
- [ ] Own position is shown as a distinct marker plus an accuracy circle, updated
      continuously while the page is visible and permission is granted.
- [ ] A locate/recentre button recentres and re-enables follow mode; manual
      panning disables follow mode.
- [ ] A location **repair** affordance is available on the map when permission is
      denied or unavailable, using the shared `PermissionPrompt` (PRD 005 owns that
      component's API; this is a consumer). The initial permission *request* is
      onboarding's, not the map's.
- [ ] Patrol registrations are fetched from the BFF and consist of two kinds:
      **checkpoint scan** and **bandit catch**, each with a kind, a label
      (post/bandit name), a timestamp and optional coordinates.
- [ ] Registrations are rendered as map markers, visually distinguishable by kind
      (Lucide icons per repo convention), with a popup showing label + timestamp.
- [ ] The same registrations are available as a chronological list (newest first)
      in an overlay/bottom sheet, without leaving the map page.
- [ ] Selecting a list entry pans/zooms the map to that registration and opens
      its popup.
- [ ] The registrations affordance is hidden when the signed-in user has no
      resolvable patrol.
- [ ] Registrations are re-fetched when the page is opened and on manual pull /
      refresh action; no aggressive polling.
- [ ] All map UI copy is Danish, consistent with the rest of the app.

### Non-Functional

- **Performance:** first meaningful map paint under 2s on a 4G connection; the
  map library must be lazily loaded so it does not weigh down the app shell
  bundle. This PRD adds no tile caching of its own; caching is PRD 009's (§11.2).
- **Battery:** use a single `watchPosition` subscription owned by the store,
  suspended when the document is hidden.
- **Accessibility:** controls are ≥44px touch targets with `aria-label`s; the
  registrations list is a real list navigable by screen reader; the map itself is
  acknowledged as not fully accessible, so the list is the accessible equivalent.
- **Night use:** avoid pure-white control chrome; markers must be legible on both
  topo and aerial backgrounds.
- **Privacy:** the user's position is not transmitted by the features this PRD
  ships — it is read locally to draw the marker. If §11.1 settles on a telemetry
  stream, transmission arrives with that later work, not here.
- **Security:** the Dataforsyningen API token must not be committed to the
  frontend source (see Technical Considerations).
- **Data economy:** aerial tiles are heavy; do not preload layers the user has
  not selected.

## 7. UX / UI Notes

**Layout.** The app shell (`vue/src/App.vue`) currently renders a top bar
(app name + "Log ud") above a scrolling `<main>`. For this page the shell must
support a *full-bleed* route: the top bar is hidden and the map is flush to the
top safe-area inset, with only the bottom nav below it. Sign-out remains
reachable from other pages (and, when PRD 003 lands, from the profile page).

Proposal: add an optional `meta: { fullBleed: true }` on the route and have
`App.vue` skip the header and the `overflow-y-auto` wrapper for such routes.

**Overlays on the map (floating, safe-area aware):**

- Top-right: layer switcher — a compact button (Lucide `Layers`) opening a small
  panel with the three layer names as radio options. Avoid Leaflet's default
  desktop-oriented layers control if it does not meet the touch-target rule.
- Bottom-right, above the nav: locate button (Lucide `LocateFixed` /
  `Locate` / disabled state).
- Bottom, above the nav: a compact "Registreringer (n)" handle that raises a
  bottom sheet (same interaction language as the existing `MoreMenu.vue`
  overflow sheet). The sheet is dismissible and covers at most ~60% of the
  height so context is retained.
- The existing `PermissionPrompt.vue` renders as an overlay card near the top of
  the map rather than pushing the map down.

**Markers.**

- Own position: filled dot with a heading-agnostic ring plus a translucent
  accuracy circle.
- Checkpoint scan: Lucide `Flag` (or `MapPin`) marker in the brand accent.
- Bandit catch: Lucide `Skull` (or `ShieldAlert`) marker in a warning colour.
- Popup: label (post/bandit name), kind, and a Danish-formatted timestamp.

**New / changed frontend files (in `vue/`):**

- `src/views/MapsView.vue` — replaced: hosts the map, overlays and sheet.
- `src/components/map/EventMap.vue` — the map instance, layers, markers
  (lazily imported).
- `src/components/map/LayerSwitcher.vue` — touch-friendly base-layer control.
- `src/components/map/LocateButton.vue` — recentre / follow toggle.
- `src/components/map/ScanList.vue` — bottom-sheet list of registrations.
- `src/config/map.ts` — base-layer definitions, default centre/zoom, min/max zoom.
- `src/stores/scans.store.ts` — fetch + cache the patrol's registrations.
- `src/stores/location.store.ts` — extended with `watch()` / `stopWatch()` and a
  `following` flag.
- `src/router/index.ts` — `fullBleed` route meta for `/maps`.
- `src/App.vue` — honour `fullBleed`.

## 8. Technical Considerations

- **Frontend (Vue 3 / TS):**
  - **Map library:** Leaflet, matching the other Nathejk repo (`leaflet` +
    `@types/leaflet`). It supports WMS out of the box, which the Danish topo
    services require. Install via the `ui` container per the docker dev stack.
  - The map is created in `onMounted` on a plain `div` ref and destroyed in
    `onBeforeUnmount`; Leaflet's DOM is deliberately kept outside Vue's
    reactivity. Marker layers are diffed from store state via watchers.
  - `EventMap.vue` is loaded with a dynamic `import()` so Leaflet (and its CSS)
    stays out of the initial shell chunk.
  - `location.store.ts` gains a `watchPosition`-based subscription (single
    subscription, ref-counted or page-owned) plus `visibilitychange` handling.
    The existing `request()` / `syncPermission()` / permission states are reused
    unchanged.
  - Base-layer definitions. **Use the layer names from the §6 table and the
    Danish orthophoto** — the snippet below was corrected on 2026-08-25; it
    previously showed `layers: 'DTK25'` and an Esri World Imagery aerial layer,
    both superseded by §11's resolutions. Do not infer layer names by analogy:
    `dtk_50_DAF` rejects `DTK50`.

    ```ts
    'Topografisk 1:25.000': L.tileLayer.wms('https://api.dataforsyningen.dk/dtk_25_DAF', {
      layers: 'dtk25',
      format: 'image/png',
      transparent: true,
      attribution: '&copy; Styrelsen for Dataforsyning og Effektivisering',
      token,            // from GET /api/config, see below
      maxZoom: 19,
    } as L.WMSOptions)

    Luftfoto: L.tileLayer.wms('https://api.dataforsyningen.dk/orto_foraar_DAF', {
      layers: 'orto_foraar',
      format: 'image/png',
      attribution: '&copy; Styrelsen for Dataforsyning og Effektivisering',
      token,
      maxZoom: 19,
    } as L.WMSOptions)
    ```

    The 1:50.000 layer follows the same WMS pattern against `dtk_50_DAF`, layer
    `dtk_50` (resolved in §11).
- **Dataforsyningen token — decided and shipped.** The token is delivered **at
  runtime** by the BFF: env var `DATAFORSYNINGEN_TOKEN` on the `api` service,
  served to the SPA through the **existing `GET /api/config`** endpoint. No BFF tile
  proxy, no new `/api/map/config`, no build-time `VITE_` var.

  *(Corrected 2026-08-25. §11 previously recorded a build-time env var
  (`VITE_DATAFORSYNINGEN_TOKEN`) as the decision, and §8 listed three open options.
  Both were contradicted by the code that actually shipped — see `go/cmd/api/env.go`,
  `config.go`, `routes.go` and the comments in both compose files. The rationale for
  what shipped: one published image runs in any environment, which a build-time var
  cannot do.)* The token is a public quota key, not a credential, but it stays out
  of git: `docker-compose.override.yml` in dev, deploy environment in prod. The map
  degrades with a clear notice when it is missing.
- **BFF (Go):** per `go-bff-layout` — a new handler file `go/cmd/api/scans.go`
  behind `app.requireAuth`, reading the patrol identity from the session, and a
  `Scans` facade on `internal/data` with an interface + **mock** implementation
  seeded with plausible checkpoint/bandit registrations, mirroring how
  `internal/users` was introduced in PRD 001. The real source (Nathejk records /
  jetstream projection of scan events) replaces the mock without touching the
  handler.
  - Note: `session.Session` carries `UserID` + `Role`. **Landed 2026-08-25:**
    `users.User` now carries `PatrolID` + `PatrolName`, so patrol resolution goes
    through the directory as intended. PRD 006 replaces the seeded values with a
    real projection.
- **API endpoints (OpenAPI annotations mandatory, matching the style in
  `go/cmd/api/auth.go`):**
  - `GET /api/patrol/scans` — the signed-in user's patrol registrations
    (checkpoint scans + bandit catches), newest first. `200` with a list,
    `401` unauthenticated, and `200` with an empty list when the user has no
    patrol (**decided by implementation, 2026-08-25** — not `404`; personnel with
    no patrol are legitimate users, per the `users.User` contract).
  - `GET /api/config` — **already shipped**, and the delivery mechanism for the
    Dataforsyningen token plus any other runtime frontend config. This PRD documents
    it rather than introducing it; no `/api/map/config` is added.
- **Data / storage:** no *domain* persistence owned by this feature. If §11.1
  settles on a telemetry stream, the position track lands later as a projection of
  that stream (PRD 008 §8), not as a direct write. Registration
  shape (BFF response, snake_case per existing handlers):
  `{ id, kind: "checkpoint" | "bandit", label, lat, lng, scanned_at }` with
  `lat`/`lng` nullable. Client-side, only the selected base layer is persisted
  (`localStorage`).
- **Dependencies & risks:**
  - New frontend dependency: `leaflet` (+ types, + CSS import). Bundle size
    mitigated by lazy loading.
  - External services: `api.dataforsyningen.dk` — WMS, token, quota, uptime. A
    third party with terms of use and attribution requirements. **Resolved:** the
    aerial layer is the Danish orthophoto (`orto_foraar_DAF`) from the same
    provider, not Esri World Imagery, so there is one host, one token and one
    failure mode (§11).
  - WMS at night in rural areas with weak coverage: tiles may fail; the UI must
    degrade rather than break.
  - **Blocked by drafts for two sub-scopes only:** PRD 008 for the position
    telemetry mechanism (§11.1) and PRD 009 for tile caching (§11.2). The map,
    layers and scan history are unblocked and ship first — do not hold them.
  - Tile fetching policy stays here (do not preload unselected layers); the
    **storage budget and cache policy are PRD 009's**, so this PRD adds no Workbox
    configuration of its own.
  - The scan data source is the real dependency risk: until the Nathejk scan
    projection is available, this ships against a mock and its usefulness during
    a real event is unproven.
  - Privacy/GDPR: participants are often minors; showing own position only, and
    not transmitting it, keeps this feature low-risk. Any future tracking needs
    its own consent story.

## 9. Success Metrics

- Opening `Kort` with permission granted shows the user's position on the
  1:25.000 topo map within 5 seconds on 4G.
- All three base layers render tiles and can be switched without losing position,
  markers or zoom; the choice survives a reload.
- Denying or having no location permission leaves a fully usable map (verified
  manually on iOS Safari and Android Chrome, installed as PWA).
- A patrol with seeded registrations sees the same set in the list and on the
  map; tapping a list row pans to the matching marker.
- The map occupies the full area above the bottom nav on iOS (notch) and Android,
  with no clipped controls.
- Lighthouse: no regression in the app-shell bundle size beyond the lazily loaded
  map chunk.

## 10. Rollout / Task Breakdown

Single phase, frontend-heavy. The BFF scan endpoint can ship against a mock in
parallel with the map work; the map should be built so it renders with zero
registrations. No feature flag needed — the page already exists as a
placeholder, so shipping simply replaces it.

Suggested sequencing: shell/full-bleed + map + layers → position → scans
endpoint (mock) → markers + list → polish/degradation states.

Proposed tasks to create in `roadmap/tasks/open/`:

**Already landed (ticked 2026-08-25 during a consistency pass — the code shipped
ahead of the PRD's task list):**

- [x] Task: Support `fullBleed` route meta in `App.vue` (hide top bar, no scroll
      wrapper) and apply it to `/maps`.
- [x] Task: Extend `location.store.ts` with `watchPosition`, `following` state
      and visibility-based suspend/resume.
- [x] Task: BFF — `internal/data` `Scans` facade + interface + seeded mock
      (checkpoint scans and bandit catches).
- [x] Task: BFF — `GET /api/patrol/scans` handler behind `requireAuth`, resolving
      the patrol from the session. OpenAPI annotations.
- [x] Task: Extend the user directory/session to carry a patrol identity
      (`users.User.PatrolID`/`PatrolName`; PRD 006 replaces the seed).
- [x] Task: Frontend `scans.store.ts` (markers/UI still open below).

**Open:**

- [ ] Task: Add `leaflet` (+ `@types/leaflet`) via the `ui` container and create
      a lazily loaded `EventMap.vue` with a configurable base layer set in
      `src/config/map.ts`.
- [ ] Task: Implement the three base layers (DTK 1:25.000 `dtk25`, DTK 1:50.000
      `dtk_50`, Luftfoto `orto_foraar`) incl. attribution, max zoom, and the token
      from `GET /api/config`.
- [ ] Task: Build `LayerSwitcher.vue` (touch-friendly, persists the choice in
      `localStorage`).
- [ ] Task: Render own position marker + accuracy circle and `LocateButton.vue`
      (follow / recentre / denied states). Keep only the **denied/repair**
      affordance here — the first location *request* belongs to onboarding
      (PRD 005), not to the map.
- [ ] Task: Scan markers with kind-specific Lucide icons and popups.
- [ ] Task: Build `ScanList.vue` bottom sheet (chronological list, empty state,
      row → pan/zoom + open popup).
- [ ] Task: Degradation + polish — tile-failure notice, position timeout message,
      night-legible control styling, ≥44px targets, `aria-label`s.
- [ ] Task: Register map tiles as a PRD 009 dataset + supply tile-set sizing
      numbers to its storage budget (replaces the former "exclude tiles from
      caching" task).

## 11. Open Questions

### Reopened 2026-08-25 by architecture decisions taken in PRDs 008/009

Two rules were established after this PRD was approved, and both land directly on
it. Neither invalidates the agreement — the map, layers and scan history are
unaffected — but the **position-reporting** and **tile-caching** designs need
revisiting here before they are built.

1. **Where does the position track go?** Architecture rule: *nothing writes
   directly to the database; every state change is published as an event.* But, as
   observed, **a new coordinate does not represent an event** — it is telemetry,
   meaningful only in aggregate. The two statements collide, and this PRD owns the
   resolution. PRD 008 §8 sets out the options and recommends a **separate
   telemetry stream** with short, age-capped retention and its own subjects: the
   no-direct-writes rule survives, domain replay stays clean, and retention comes
   for free — which matters for a position track of identifiable minors. The
   alternatives are a documented direct-write carve-out, or coordinates as domain
   events (not recommended).
2. **Do map tiles move onto the shared offline layer?** PRD 009 introduces one
   sync engine, one storage budget and one readiness surface for everything
   cacheable. Tiles are probably the **largest** cached dataset, so leaving them
   outside that budget largely defeats it — portraits (PRD 007) and tiles otherwise
   compete for the same per-origin quota with no agreed priority, and the OS decides
   what to evict. Retrofitting is real work; doing it later is more.

Both are flagged rather than resolved, because they are this PRD's calls to make.

### Resolved before approval (2026-08-24), by querying the live services:

- **DTK 1:50.000 service** — `https://api.dataforsyningen.dk/dtk_50_DAF`, layer
  **`dtk_50`**, same token as DTK25. Confirmed via GetCapabilities + a GetMap that
  returned a PNG. **Caveat to flag to the organizers:** the service states it is
  *"opdateres ikke efter år 2017"* — the 1:50.000 raster is frozen at 2017 (newest
  dated layer `dtk_50_2017`), so it will not show recent forest/road changes.
  DTK25 is current and remains the default.
- **Layer naming** — do not guess by analogy. `dtk_25_DAF` happily answers to
  `DTK25`, `dtk25` and `dtk_25`, but `dtk_50_DAF` **rejects** `DTK50` with a
  `ServiceException`. Use the names in the table in §6.
- **Aerial source** — use the **Danish orthophoto** (`orto_foraar_DAF`, layer
  `orto_foraar`) rather than the sibling repo's Esri World Imagery. Verified
  working with the same token. It is 12.5 cm Danish imagery vs Esri's mixed global
  basemap, it comes from the same provider under terms we already accept, and it
  keeps every layer on one host (one token, one attribution, one failure mode).
- **Token handling** — ~~build-time env var (`VITE_DATAFORSYNINGEN_TOKEN`), no BFF
  proxy and no `/api/map/config`~~. **Superseded 2026-08-25:** this recorded
  decision was contradicted by the code that shipped. The token is delivered **at
  runtime** via the existing `GET /api/config`, from `DATAFORSYNINGEN_TOKEN` on the
  `api` service (see §8). The reasoning that changed the answer: a runtime value
  lets one published image run in any environment, which a build-time var cannot.
  Unchanged: no BFF tile proxy, no `/api/map/config`, and the token is never
  committed — `docker-compose.override.yml` in dev, deploy environment in prod. The
  map degrades with a clear notice when it is missing.

**Still open:**

- **Default view** — *resolved 2026-08-24*: centre on the user's own position, and
  when unavailable frame **Sjælland**. The event area is deliberately not used as a
  default, because it is not fully known to participants. See task 049.
- **Overview-zoom rendering** — at national zoom the DTK25 raster looks speckled (a
  508 DPI 1:25.000 map downsampled far past its design scale).
  `topo_skaermkort_DAF` (layer `topo_skaermkort`, verified working with the same
  token) is Dataforsyningen's on-screen map and renders correctly there. Add it as a
  fourth layer, auto-swap to it below ~zoom 11, or accept the speckle?
- **Patrol identity** — *resolved by PRD 006*: `users.User` carries `PatrolID` /
  `PatrolName`, seeded today, replaced by the real person projection there.
- **Scan data source** — is there an existing event stream/projection of checkpoint
  scans and bandit catches, and does it carry coordinates, or only post ids we must
  join to post positions? Mocked for now. Note PRD 008: the real source arrives as a
  projection, not a direct read of another service.
- **Bandit catches for the bandit role** — should a signed-in `bandit` see their
  *own* catches (the patrols they caught), or is the registrations view
  spejder-only?
- **Personnel view** — do `postmandskab` / `guide` / `samarit` need anything
  plotted (e.g. their own post), or just the base map?
- **Own position marker semantics** — heading/compass rotation, or is a plain dot
  enough for a first version?
- **Empty-patrol response** — *resolved*: `200` with an empty list (not `404`).
  Personnel legitimately have no patrol, per the `users.User` contract.
