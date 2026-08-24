# PRD 002 — Event Map (own position, Danish topo + aerial layers, patrol scan history)

**Status:** doing
**Author:** agent session (Zed / Claude Opus 5)
**Created:** 2026-08-24
**Last updated:** 2026-08-24
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
  tracking). Out of scope; see PRD 003 for the location-sharing *preference*
  surface and Open Questions here for the eventual tracking PRD.
- **Offline map tiles / pre-caching of map areas.** Tiles come from the network;
  the service worker must not attempt to precache them.
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
4. If permission is not yet granted, the existing soft `PermissionPrompt`
   overlays the map ("Vis din placering"); accepting triggers the native prompt
   via `location.store.request()`.
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
- [ ] The existing soft location pre-prompt behaviour from PRD 001 is preserved
      (shown only when permission could still be gained and not dismissed).
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
  bundle. Tile requests must not be precached by the service worker.
- **Battery:** use a single `watchPosition` subscription owned by the store,
  suspended when the document is hidden.
- **Accessibility:** controls are ≥44px touch targets with `aria-label`s; the
  registrations list is a real list navigable by screen reader; the map itself is
  acknowledged as not fully accessible, so the list is the accessible equivalent.
- **Night use:** avoid pure-white control chrome; markers must be legible on both
  topo and aerial backgrounds.
- **Privacy:** the user's position is never sent to the BFF by this feature.
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
  - Base-layer definitions, lifted from the sibling repo and to be
    parameterised (see token handling below):

    ```ts
    'Topografisk 1:25.000': L.tileLayer.wms('https://api.dataforsyningen.dk/dtk_25_DAF', {
      layers: 'DTK25',
      format: 'image/png',
      transparent: true,
      attribution: '&copy; Styrelsen for Dataforsyning og Effektivisering',
      token: '<token>',
      maxZoom: 19,
    } as L.WMSOptions)

    Luftfoto: L.tileLayer(
      'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',
      { attribution: '&copy; Esri — Esri, DigitalGlobe, Earthstar Geographics', maxZoom: 19 },
    )
    ```

    The 1:50.000 layer follows the same WMS pattern against the corresponding
    Dataforsyningen service; the exact service path and layer name must be
    verified before implementation (Open Questions).
- **Dataforsyningen token:** the sibling repo hard-codes the token in frontend
  source. That is a leak we should not repeat verbatim. Options, in order of
  preference:
  1. BFF tile proxy — `GET /api/map/tiles/{layer}` forwards to Dataforsyningen
     with a server-held token (best privacy/secret hygiene, adds latency and
     load on the BFF).
  2. BFF hands the token to authenticated clients at runtime
     (`GET /api/map/config`), keeping it out of the git tree but not out of the
     browser.
  3. Build-time injection via a Vite `define`/env var (no secret in git, still
     public in the bundle).
  Since the token is a public-ish quota key for an open public service, option 2
  or 3 is likely proportionate; a decision is needed (Open Questions).
- **BFF (Go):** per `go-bff-layout` — a new handler file `go/cmd/api/scans.go`
  behind `app.requireAuth`, reading the patrol identity from the session, and a
  `Scans` facade on `internal/data` with an interface + **mock** implementation
  seeded with plausible checkpoint/bandit registrations, mirroring how
  `internal/users` was introduced in PRD 001. The real source (Nathejk records /
  jetstream projection of scan events) replaces the mock without touching the
  handler.
  - Note: `session.Session` currently carries `UserID` + `Role` only. Resolving
    a patrol needs either an extension of the user directory (`users.User` gains
    a patrol id) or a lookup by user id. Prefer extending the directory, since
    the same information is needed by PRD 003.
- **API endpoints (OpenAPI annotations mandatory, matching the style in
  `go/cmd/api/auth.go`):**
  - `GET /api/patrol/scans` — the signed-in user's patrol registrations
    (checkpoint scans + bandit catches), newest first. `200` with a list,
    `401` unauthenticated, `404`/empty list when the user has no patrol
    (decide which — see Open Questions).
  - `GET /api/map/config` — *only if* token option 2 is chosen: base-layer
    endpoints/token + default centre/zoom.
- **Data / storage:** no new persistence owned by this feature. Registration
  shape (BFF response, snake_case per existing handlers):
  `{ id, kind: "checkpoint" | "bandit", label, lat, lng, scanned_at }` with
  `lat`/`lng` nullable. Client-side, only the selected base layer is persisted
  (`localStorage`).
- **Dependencies & risks:**
  - New frontend dependency: `leaflet` (+ types, + CSS import). Bundle size
    mitigated by lazy loading.
  - External services: `api.dataforsyningen.dk` (WMS, token, quota, uptime) and
    the aerial tile provider. Both are third parties with terms of use and
    attribution requirements; the aerial source (ArcGIS World Imagery) should be
    confirmed as acceptable for this use, otherwise use the Danish orthophoto
    service from Dataforsyningen.
  - WMS at night in rural areas with weak coverage: tiles may fail; the UI must
    degrade rather than break.
  - Service-worker/Workbox config must exclude tile hosts from precaching and
    from any runtime cache that could blow the storage quota.
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

- [ ] Task: Support `fullBleed` route meta in `App.vue` (hide top bar, no scroll
      wrapper) and apply it to `/maps`.
- [ ] Task: Add `leaflet` (+ `@types/leaflet`) via the `ui` container and create
      a lazily loaded `EventMap.vue` with a configurable base layer set in
      `src/config/map.ts`.
- [ ] Task: Implement the three base layers (DTK 1:25.000, DTK 1:50.000,
      Luftfoto) incl. attribution, max zoom and token handling per the decided
      option.
- [ ] Task: Build `LayerSwitcher.vue` (touch-friendly, persists the choice in
      `localStorage`).
- [ ] Task: Extend `location.store.ts` with `watchPosition`, `following` state
      and visibility-based suspend/resume.
- [ ] Task: Render own position marker + accuracy circle and `LocateButton.vue`
      (follow / recentre / denied states); keep the soft permission pre-prompt as
      an overlay.
- [ ] Task: BFF — `internal/data` `Scans` facade + interface + seeded mock
      (checkpoint scans and bandit catches).
- [ ] Task: BFF — `GET /api/patrol/scans` handler behind `requireAuth`, resolving
      the patrol from the session. OpenAPI annotations.
- [ ] Task: Extend the user directory/session to carry a patrol identity (shared
      with PRD 003).
- [ ] Task: Frontend `scans.store.ts` + scan markers with kind-specific Lucide
      icons and popups.
- [ ] Task: Build `ScanList.vue` bottom sheet (chronological list, empty state,
      row → pan/zoom + open popup).
- [ ] Task: Degradation + polish — tile-failure notice, position timeout message,
      night-legible control styling, ≥44px targets, `aria-label`s.
- [ ] Task: Ensure the service worker/Workbox config excludes map tile hosts from
      caching.

## 11. Open Questions

**Resolved before approval (2026-08-24), by querying the live services:**

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
- **Token handling** — **build-time env var** (`VITE_DATAFORSYNINGEN_TOKEN`), no
  BFF proxy and no `/api/map/config`. Rationale: the token is a public quota key
  for an open public service, not a credential; a proxy would add latency and put
  tile traffic through the BFF for no security gain; and the runtime-config option
  buys nothing over build-time since the value ends up in the browser either way.
  What matters is that it is **not committed** — it goes in the (gitignored)
  `docker-compose.override.yml` for dev and the deploy environment for prod. The
  map degrades with a clear notice when the token is missing.

**Still open:**

- **Default view** — *resolved 2026-08-24*: centre on the user's own position, and
  when unavailable frame **Sjælland**. The event area is deliberately not used as a
  default, because it is not fully known to participants. See task 049.
- **Overview-zoom rendering** — at national zoom the DTK25 raster looks speckled (a
  508 DPI 1:25.000 map downsampled far past its design scale).
  `topo_skaermkort_DAF` (layer `topo_skaermkort`, verified working with the same
  token) is Dataforsyningen's on-screen map and renders correctly there. Add it as a
  fourth layer, auto-swap to it below ~zoom 11, or accept the speckle?
- **Patrol identity** — how is a signed-in user's patrol resolved once the real
  Nathejk directory lands? `internal/users` is mocked and now carries a seeded
  patrol id.
- **Scan data source** — is there an existing event stream/projection of checkpoint
  scans and bandit catches, and does it carry coordinates, or only post ids we must
  join to post positions? Mocked for now.
- **Bandit catches for the bandit role** — should a signed-in `bandit` see their
  *own* catches (the patrols they caught), or is the registrations view
  spejder-only?
- **Personnel view** — do `postmandskab` / `guide` / `samarit` need anything
  plotted (e.g. their own post), or just the base map?
- **Own position marker semantics** — heading/compass rotation, or is a plain dot
  enough for a first version?
- **Empty-patrol response** — `200` with an empty list or `404`? (Implementation
  chose `200` + empty list; confirm.)
