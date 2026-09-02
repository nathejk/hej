# PRD 002 — Event Map (own position, Danish topo + aerial layers, patrol scan history)

**Status:** done
**Author:** agent session (Zed / Claude Opus 5)
**Created:** 2026-08-24
**Last updated:** 2026-09-02
**Approved:** 2026-08-24
**Shipped:** 2026-09-02
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
- **Building offline tile caching.** Tiles are cached under **PRD 009's shared budget and
  priority order** rather than to an ad-hoc policy invented here.
  *(Revised 2026-08-25 — previously this read "the service worker must not attempt
  to precache them", which contradicted PRD 009. Revised again 2026-09-01: PRD 009 was
  rescoped and no longer offers a generic layer to register with, and task 087 has since
  shipped the tile `runtimeCaching` route in `vue/vite.config.ts`. So the Workbox config for
  tiles does live in this repo, under this PRD's task — what remains 009's is the budget,
  the priority order and the readiness surface. See §11.2.)*
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

*Ticked 2026-09-02 against the shipped code, not against task status — see the note at the end of
this section for the one place those two disagree.*

- [x] `Kort` occupies the full viewport minus the bottom nav: no page header, no
      padding, no scroll container. Map controls float over the map and respect
      `env(safe-area-inset-*)`.
      *`fullBleed` route meta in `router/index.ts` + `App.vue`; controls offset by `var(--sat)`.*
- [x] Three mutually exclusive base layers, selectable from an on-map control.
      **All three are Dataforsyningen WMS services, verified against live
      GetCapabilities + GetMap on 2026-08-24:**

      | Label | Service | WMS layer |
      |---|---|---|
      | `Topografisk 1:25.000` | `https://api.dataforsyningen.dk/dtk_25_DAF` | `dtk25` |
      | `Topografisk 1:50.000` | `https://api.dataforsyningen.dk/dtk_50_DAF` | `dtk_50` |
      | `Luftfoto` | `https://api.dataforsyningen.dk/orto_foraar_DAF` | `orto_foraar` |

      Layer names are **not** symmetric between services — `DTK50` returns a
      `ServiceException`, only `dtk_50` works — so do not infer names by analogy.
      *`config/map.ts`, `LayerSwitcher.vue`.*
- [x] The selected layer persists across navigation within the session and
      across reloads (`localStorage`, key namespaced `hej.map.*`).
      *`BASE_LAYER_STORAGE_KEY`, read and written in `MapsView.vue`.*
- [x] **Opening view:** centred on the user's own position when available,
      otherwise framed on **Sjælland**. The event area is deliberately *not* used
      as a default — it is not fully known to participants and the map must not
      reveal it.
      *`FALLBACK_BOUNDS` / `LOCATE_ZOOM` in `EventMap.vue`.*
- [x] **Failed tiles are retried** with exponential backoff and jitter (Leaflet has
      no built-in retry; one failed request otherwise leaves a permanently grey
      tile). The failure notice appears only after retries are exhausted, and clears
      itself when tiles load again.
      *`tileerror` handler in `EventMap.vue`, `&_retry=n` with jittered backoff.*
- [x] Own position is shown as a distinct marker plus an accuracy circle, updated
      continuously while the page is visible and permission is granted.
- [x] A locate/recentre button recentres and re-enables follow mode; manual
      panning disables follow mode.
      *`LocateButton.vue` + `following` state.*
- [x] A location **repair** affordance is available on the map when permission is
      denied or unavailable, using the shared `PermissionPrompt` (PRD 005 owns that
      component's API; this is a consumer). The initial permission *request* is
      onboarding's, not the map's.
- [x] Patrol registrations are fetched from the BFF and consist of two kinds:
      **checkpoint scan** and **bandit catch**, each with a kind, a label
      (post/bandit name), a timestamp and optional coordinates.
- [x] Registrations are rendered as map markers, visually distinguishable by kind
      (Lucide icons per repo convention), with a popup showing label + timestamp.
      *`scanIcon()` in `EventMap.vue`.*
- [x] The same registrations are available as a chronological list (newest first)
      in an overlay/bottom sheet, without leaving the map page.
      *`ScanList.vue`.*
- [x] Selecting a list entry pans/zooms the map to that registration and opens
      its popup.
      *`select` event → `focusScan(id)`.*
- [x] The registrations affordance is hidden when the signed-in user has no
      resolvable patrol.
      *The BFF returns an empty list for personnel roles rather than a 404.*
- [x] Registrations are re-fetched when the page is opened and on manual pull /
      refresh action; no aggressive polling.
- [x] All map UI copy is Danish, consistent with the rest of the app.

#### Position track (added 2026-08-26, §11.1)

- [x] While location permission is granted, the app records the user's position into
      **local persistent storage** as it is collected, independently of connectivity.
      *Task 082.*
- [x] The recorded track survives a page reload, the app being backgrounded, and the app
      being killed — an unshipped track is the one thing here that cannot be recovered
      from the server, so it must not live only in memory.
      *Task 082, measured on an iPhone across three cold starts.*
- [x] Every **2 minutes**, and **only when the track has new points**, the pending points
      are uploaded in one batch.
      *Task 083. Verified: the interval fires unprompted, and a flush with nothing new sends
      no request at all.*
- [x] Upload failure is not data loss: points stay pending and are retried on the next
      interval. A member who is offline for hours ships the backlog when they reconnect.
      *Task 083. Verified offline, and a 1,200-point backlog shipped as 500+500+200.*
- [x] Points already accepted by the server are not uploaded again, and a retried batch
      does not duplicate points — each point is identified by (person, timestamp) so
      duplicates are detectable rather than merely unlikely.
      *Task 083. Verified on the stream: 1,202 points published, 1,202 distinct timestamps.
      Duplicates are removed **at the reader** — see 083's log for why that is the only
      possible place.*
- [x] The BFF accepts a batch from the signed-in user, resolves the person from the
      session (never from the request body), and **publishes** it to the telemetry
      stream. It writes no SQL.
      *Task 084. The body cannot even name a person — unknown fields are a 400.*
- [x] Subjects are addressable **per person**, so a retention or erasure policy can be
      applied to one individual later.
      *Tasks 081/084. Verified: purging one person's subject left another's messages.*
- [ ] A team can see its **own** whole track after the race, and no other team's.
      **Not built, and deliberately left unticked.** Task 086 sits in `done/` — the board has no
      other folder for a task that is off it — but it was **closed without being implemented**
      (2026-08-28, maintainer's request) and superseded by **PRD 011, post-race experience**. Its
      analysis went there rather than being lost. So this requirement has moved PRDs; it has not
      been met, and ticking it because a task file says `done` is exactly the mistake this pass was
      run to catch.
- [x] The location consent copy states that the track is recorded and sent to the
      organizers (see Non-Functional → Privacy).
      *Task 085: `WelcomeStepLocation.vue` and the `/privatliv` page.*

### Non-Functional

*Checked against the source 2026-09-02, same pass as §6.*

- **Performance:** first meaningful map paint under 2s on a 4G connection; the
  map library must be lazily loaded so it does not weigh down the app shell
  bundle. The tile cache's *policy* — budget, priority, eviction — is PRD 009's (§11.2);
  its Workbox route is task 087's and lives here.
  *Lazy loading done: `defineAsyncComponent(() => import('@/components/map/EventMap.vue'))`, and
  `EventMap` is its own 153 kB chunk in the build output rather than part of the shell. The 2s
  paint figure has **not** been measured on 4G — it needs the device pass, and the tile service is
  the variable, not our bundle.*
- **Battery:** use a single `watchPosition` subscription owned by the store,
  suspended when the document is hidden.
  *Structurally done, and now **measured** (task 082, 2026-09-02): 2h 08m foregrounded on an iPhone with
  the map open cost **10 percentage points — ~4.7 pp/hour**, which projects to **~56 pp over a 12-hour
  race**. So a phone that starts full finishes around 44%; a phone that starts at 60% does not finish.
  That is a briefing-and-power-bank problem rather than a sampling-interval problem, and the figure
  measures the app as a participant uses it (screen, map, watch, recorder, uploader together) rather than
  the recorder alone. It is optimistic in one way that matters here: the measurement had WiFi and two bars
  throughout, whereas a phone hunting for signal in a forest — with the uploader retrying every two
  minutes — costs more.*
- **Accessibility:** controls are ≥44px touch targets with `aria-label`s; the
  registrations list is a real list navigable by screen reader; the map itself is
  acknowledged as not fully accessible, so the list is the accessible equivalent.
  *`aria-label`s present on the layer switcher, locate button and scan list; the shadcn-vue button
  sizes were bumped to ≥44px repo-wide for this reason (see `ui/button/index.ts`). Not verified
  with an actual screen reader — worth adding to the device pass rather than claiming.*
- **Night use:** avoid pure-white control chrome; markers must be legible on both
  topo and aerial backgrounds.
  *Marker icons are kind-coloured and were chosen against both backgrounds (task 045). A judgement
  best confirmed outdoors, in the dark, which is where it matters.*
- **Privacy:** ~~the user's position is not transmitted by the features this PRD
  ships~~ **— superseded 2026-08-26 (§11.1).** The position *is* now transmitted: recorded
  locally and uploaded every 2 minutes when it has changed, then retained indefinitely on
  a telemetry stream. That is a material change in what the app does with a minor's
  location, and three things follow from it rather than being optional:

  - **The consent copy must say so.** *Done 2026-08-26 (task 085):* the pre-prompt now says
    the route is recorded and sent to the organizers, and links to a fuller `/privatliv`
    page covering location, portrait, profile data and retention. Backed by an educational
    talk in the start area — reinforcement rather than the consent mechanism, since the
    permission is granted during onboarding, before anyone gets there. Note the talk reaches
    participants, not parents: for a minor's location track kept without an end date,
    `/privatliv` is the only account a parent will see. **Wording still needs maintainer
    review.**
  - **Per-person subjects**, so that erasure is expressible later (§11.1).
  - **The track is only shown to its own team**, never across teams — the same race
    dynamic that keeps portraits from crossing populations in PRD 007.
- **Security:** the Dataforsyningen API token must not be committed to the
  frontend source (see Technical Considerations).
- **Data economy:** aerial tiles are heavy; do not preload layers the user has
  not selected. The aerial layer is requested as JPEG rather than PNG for this reason —
  measured at ~9–14 kB/tile against ~137–154 kB as PNG.
- **Battery vs. fidelity:** the track's sampling interval is a battery decision as much
  as a data one, over a 12-hour night in which the phone is also a torch and a comms
  device. Must be measured on a real device, not assumed (§11.1).

## 7. UX / UI Notes

**Layout.** The app shell (`vue/src/App.vue`) currently renders a top bar
(app name + "Log ud") above a scrolling `<main>`. For this page the shell must
support a *full-bleed* route: the top bar is hidden and the map is flush to the
top safe-area inset, with only the bottom nav below it. Sign-out remains
reachable from every other page — PRD 003 puts it in a user menu in the top bar's
trailing corner, which is simply absent on this route.

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
  - `POST /api/track` — accepts one batch of position points from the signed-in user and
    publishes it to the telemetry stream. The person is resolved from the **session**,
    never from the body, so a member cannot report a track as somebody else. `202` on
    accept, `401` unauthenticated, `413`/`400` on an oversized or malformed batch. Rate
    limited: a 2-minute cadence needs nothing like the login limiter's headroom, and an
    unbounded ingest endpoint is the one place a client bug becomes a broker problem.
  - `GET /api/team/track` — the signed-in user's **own team's** track, for the post-race
    view. `200` with the team's points, `401` unauthenticated, `200` empty when the user
    has no team (consistent with `GET /api/patrol/scans`).
- **Data / storage:** no *domain* persistence owned by this feature. **Position track
  (decided §11.1, 2026-08-26):** the BFF publishes batches to a telemetry stream that is a
  sibling of `NATHEJK` — not a direct write, and deliberately not a projection into SQL.
  The post-race team view reads the stream back by subject filter on demand, because one
  team-race is ~8,600 points and projecting millions of points into MariaDB to serve a
  view used once per team after the race is the more expensive option. Subjects are keyed
  per person so erasure stays expressible.

  The telemetry stream itself must exist on the broker before anything can publish to it.
  `hej` does not create streams — nothing in the repo calls the stream library's `Create`,
  and the `NATHEJK` stream is owned by the `nathejk` repo, which owns the broker. So the
  new stream is a **cross-repo prerequisite**, not something this PRD can land alone. Note
  also that the library's `Create(name)` accepts no retention options, so the age cap
  §11.1 leaves open for later is today an operator action (`nats stream edit`) rather than
  a code change — worth knowing before calling it "cheap to change".

  Registration
  shape (BFF response, snake_case per existing handlers):
  `{ id, kind: "checkpoint" | "bandit", label, lat, lng, scanned_at }` with
  `lat`/`lng` nullable. Client-side, only the selected base layer is persisted
  (`localStorage`); the position track needs IndexedDB rather than `localStorage`, which
  is synchronous, string-only and ~5 MB.
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
  - **No longer blocked by a draft.** PRD 008 shipped, the tile cache route shipped with task
    087, and PRD 009 was approved 2026-09-01 with the tile budget decided (~500 MB planned,
    tiles last in the eviction order). The map, layers and scan history were unblocked
    throughout and ship first — do not hold them.
  - Tile fetching policy stays here (do not preload unselected layers), as does the Workbox
    route; the **storage budget and cross-dataset priority order are PRD 009's**.
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

**Also landed — ticked 2026-08-26 after checking the board rather than the list.**
These were written as "open" but had in fact shipped as tasks 040, 041, 042, 045, 046,
047 and 049; the same staleness PRD 008's list had:

- [x] Task: Add `leaflet` (+ `@types/leaflet`) via the `ui` container and create
      a lazily loaded `EventMap.vue` with a configurable base layer set in
      `src/config/map.ts`. *(task 040)*
- [x] Task: Implement the three base layers (DTK 1:25.000 `dtk25`, DTK 1:50.000
      `dtk_50`, Luftfoto `orto_foraar`) incl. attribution, max zoom, and the token
      from `GET /api/config`. *(task 040; aerial format corrected to JPEG 2026-08-26)*
- [x] Task: Build `LayerSwitcher.vue` (touch-friendly, persists the choice in
      `localStorage`). *(task 041)*
- [x] Task: Render own position marker + accuracy circle and `LocateButton.vue`
      (follow / recentre / denied states). Keep only the **denied/repair**
      affordance here — the first location *request* belongs to onboarding
      (PRD 005), not to the map. *(task 042)*
- [x] Task: Scan markers with kind-specific Lucide icons and popups. *(task 045)*
- [x] Task: Build `ScanList.vue` bottom sheet (chronological list, empty state,
      row → pan/zoom + open popup). *(task 046)*
- [x] Task: Degradation + polish — tile-failure notice, position timeout message,
      night-legible control styling, ≥44px targets, `aria-label`s. *(tasks 047, 049)*

**Open:**

- [x] Task: Register map tiles as a PRD 009 dataset + supply tile-set sizing
      numbers to its storage budget (replaces the former "exclude tiles from
      caching" task). *Sizing measured 2026-08-26 against the live service and recorded in
      PRD 009 §11.1; the caching work itself is now task 087.*

**Position track — created 2026-08-26 from §11.1 (tasks 081-086):**

- [x] 081 — Declare the telemetry stream on the broker (cross-repo prerequisite)
- [~] 082 — Client-side track recording into IndexedDB, survives reload/kill. *Built and verified
      on a device across three cold starts; **still in `doing/`** for one criterion: battery cost
      over a representative period. The last attempt was ~8 minutes with the screen on, which
      measures nothing, and iOS has no Battery Status API so the figure has to be read out of
      Settings → Battery by hand after a couple of hours with the phone in a pocket.*
- [x] 083 — Batched upload every 2 minutes when changed, with offline backlog
- [x] 084 — BFF `POST /api/track` publishing to the telemetry stream
- [x] 085 — Location consent copy updated to cover recording and upload (+ /privatliv page)
- [—] 086 — Post-race team track view. **Closed 2026-08-28 without being implemented**, superseded
      by PRD 011. In `done/` only because the board has no folder for "off the board".

**Tile caching — created 2026-08-26 from §11.2:**

- [x] 088 — Derive the race area from checkpoints and serve it to the client
- [~] 087 — Cache map tiles for the race area (z12–16, ~358 MB as published). *Both halves now
      shipped (2026-09-02): tiles are cached as the map is browsed, and the whole race area can be
      downloaded from the readiness view — user-initiated with the size shown, resumable, cancellable,
      tiered so an interruption leaves the orientation view complete. **Unverified against the live
      service on a device**, which is the one thing that would confirm the downloaded URLs are the ones
      the map asks for; first item for the device pass.*

**Legend:** `[x]` done · `[~]` partly done, see the note · `[—]` closed unbuilt, moved elsewhere.

### Measured 2026-09-02 — what "coverage while the app is open" costs in practice

First device measurement of the position track (task 082, iPad 6th gen, iPadOS 17.7.10, installed): over a
10m 44s period the recorder captured **12 of an expected 22 points — 55% coverage**, with 3 gaps totalling
7m 22s. Those gaps match the 3 backgroundings (6m 14s) almost to the second, and iOS killed the app zero
times.

So the loss is entirely the documented platform limit — a web app does not run while backgrounded — and not
sampling failure or throttling. §11.1's framing was right, and now has a number: **three glances away from
the app in eleven minutes cost 45% of the wall clock.**

Two consequences worth carrying into PRD 011, which is what shows a team its own route:

- a track is a **dotted record of where the app was open**, not a route. Drawing it as a continuous line
  will overstate it, and the gaps are the interesting part rather than noise to smooth over.
- accuracy on that device was **35–40 m** (Wi-Fi only, no GPS), which is fine for "which end of the forest"
  and not fine for "which path did they take".

### Finding added 2026-09-02 — asking for location too early can wedge an install

From the iPad runs (tasks 197–200). A device arrived with **Location Services off for the whole device**.
The app asked for location anyway. Afterwards, that installed instance could **never ask again**: no
dialog, no error, nothing — not even after Location Services was switched back on. A **fresh install
worked immediately**, with no code change in between.

The best explanation is per-origin permission state inside the installed app, which enabling the
device-wide setting does not reset. What matters for this PRD is that the sequence is ordinary and will
happen at an event: onboarding asks for location early (PRD 005 step 5), and any participant whose device
has location switched off is a candidate for it.

Three things follow, and only the first is built:

1. **The failure now says what it is** and offers the two safe recovery steps (tasks 197, 200). Before
   this, it was silence.
2. **The destructive fix — reinstalling — is documented for organisers, not participants**, because it
   clears an unshipped position track. See `roadmap/offline-test-protocol.md`.
3. **Whether to ask at all when the platform is not ready** is unresolved. There may be no way to detect
   "Location Services is off device-wide" before asking — the Permissions API does not report it, and
   WebKit answers `prompt` for a granted permission anyway (see `location.store`'s `GRANT_KEY` note). If
   there is no reliable pre-check, the honest mitigation is the one now in place: ask, and explain the
   failure well.

### Open question added 2026-09-02 (from the iPad device run)

**Should the position track accept a coarse fix?** Task 198 made the *map* fall back to a low-accuracy
position when high accuracy cannot be satisfied — which on a Wi-Fi-only iPad is always, since there is no
GPS receiver in it. `track.store` was deliberately left asking for high accuracy only, because the two
surfaces are not equivalent:

- the map draws an **accuracy circle**, so a ±500 m fix is visibly approximate and honest;
- a recorded track is drawn as a **line**, which implies a precision a Wi-Fi fix does not have — and that
  line is what a team is shown after the race (PRD 011).

So a coarse track is either better than no track, or a misleading artefact. It needs a decision rather
than a default. Note the sampling reasoning in §11.1 assumed GPS: 30 s puts points 33–50 m apart against
a 10–30 m GPS error, and neither number survives a ±500 m source.

### Closed 2026-09-02

Every task derived from this PRD is in `roadmap/tasks/done/`: the map, the three base layers, the layer
switcher, own position and the locate button, scan markers and the registrations list, degradation and
polish (tasks 040–047), the telemetry stream and the position track (081–085), the race area and its tile
cache (088, 087), and the recorder's own measurement (082).

**What this PRD delivered, measured rather than asserted.** Six device runs over two days on an iPhone and
an iPad produced numbers for every claim that could be tested:

| | |
|---|---|
| position accuracy | **3.9 / 10.5 / 20 m** (iPhone, GPS) · **35–40 m** (Wi-Fi-only iPad) |
| coverage while the app is open | good; **2% of a 22-hour day**, because a web app does not run backgrounded |
| iOS kills | **8 of 28** backgroundings, recorder resumed correctly every time |
| battery | **~4.7 pp/hour** foregrounded → **~56 pp over a 12-hour race** |
| tile cache after ordinary browsing | 252 tiles / 26.5 MB, unprompted |
| storage quota | 19 GB (iPad) · 41 GB (iPhone), `persisted()` true on both |

**Two things did not ship, and both are recorded rather than quietly dropped:**

1. **The post-race team track left this PRD.** Task 086 was closed unbuilt and superseded by **PRD 011**,
   which is why one §6 criterion stays unticked. The measurements above are its inheritance, and the
   important one is uncomfortable: a track is a **dotted record of where the app was open**, at 10–35 m
   accuracy, with gaps wherever the phone was in a pocket. Drawn as a continuous line it will claim more
   than it knows.
2. **The bulk tile download has never met the live tile service on a phone.** Its correctness rests on
   generating URLs byte-identical to the map's, which is argued in code and unobserved in the field — and
   it fails invisibly if wrong. First item in `roadmap/offline-test-protocol.md`, along with the awkward
   offline scenarios (OS-cleared cache, full origin) that no unit test reaches.

**What the device runs cost, and returned.** They found four real bugs that no amount of local testing
would have: a silent location failure (197), a coarse-fallback that could not reach the device it was
written for (199), 48 pointless GPS attempts per quarter-hour while backgrounded (202), and a download
button that flickered and said nothing (203). Three of the four were failures of *honesty* rather than of
logic — the app doing something reasonable and being unable to say so — which is worth remembering when
the next feature is specified.

## 11. Open Questions

### Reopened 2026-08-25 by architecture decisions taken in PRDs 008/009

Two rules were established after this PRD was approved, and both land directly on
it. Neither invalidates the agreement — the map, layers and scan history are
unaffected — but the **position-reporting** and **tile-caching** designs need
revisiting here before they are built.

1. ~~**Where does the position track go?**~~ *Answered 2026-08-26: **a sibling
   telemetry stream, fed by the BFF from batched client uploads.*** The full design:

   **Client records locally, first.** The PWA keeps the track in local storage as it is
   collected, so a member walking through a dead spot is still recorded. Connectivity
   affects when the track *ships*, never whether it exists.

   **Upload every 2 minutes, but only if the track changed.** A fixed interval keeps the
   pattern predictable and cheap; the change check stops a stationary phone — a member
   asleep at a checkpoint, or one whose GPS has not moved — from paying for uploads that
   carry nothing. Batching is also what makes the envelope overhead affordable (see the
   sizing below).

   **The BFF publishes to a telemetry stream that is a sibling of `NATHEJK`.** Not the
   domain stream, and the measurements make the reason concrete rather than aesthetic.
   One 12-hour race, all 827 live participants, batched every 2 minutes:

   | sampling | points/person | KB/person | messages/event | MB/event | vs. `NATHEJK` |
   |---|---|---|---|---|---|
   | 5 s | 8,640 | 729 | 297,720 | 589 | 33× |
   | 10 s | 4,320 | 409 | 297,720 | 330 | 18× |
   | **30 s — chosen** | **1,440** | **195** | **297,720** | **157** | **9×** |
   | 60 s | 720 | 141 | 297,720 | 114 | 6× |

   The entire `NATHEJK` stream today is **18 MiB / 29,102 messages** — the whole domain
   history of the event. A single race's telemetry is 6–33× that in bytes and ~10× in
   message count. Projections replay `NATHEJK` **from sequence zero on every boot**, so
   putting coordinates there would make every future restart drag a few hundred megabytes
   of telemetry past every projector, forever, to rebuild read models that do not want it.
   That is the argument: not that coordinates are not events (though they are not), but
   that mixing them destroys the domain stream's replay economics.

   Keeping them separate also gives retention for free — the two can be aged out
   independently — and the no-direct-writes rule survives intact, because the BFF still
   only publishes.

   **Retention: indefinite for now**, deliberately, to allow analysis and validation of
   the concept. Two notes on making that reversible rather than permanent:

   - JetStream's defaults already *are* indefinite (`Limits` retention, unlimited age and
     bytes), so this costs no configuration — but it also means nobody had to choose it,
     which is worth stating out loud given what is being retained.
   - **Subjects must be addressable per person**, so that a later retention or erasure
     policy is expressible at all. `nats stream purge --subject` can then remove one
     person's track. A design that keyed only by team, or lumped everyone into one
     subject, would make per-person deletion impossible without rewriting the stream.

   This is a location track of identifiable minors kept without an end date. Nothing here
   needs to delete anything today; the requirement is that deleting stays *cheap*.

   **Post-race the team sees its own whole track.** A member-facing read, so it belongs
   to this PRD. One team of six at 30 s sampling is ~8,600 points across ~2,160
   messages — fine to read back on demand from the stream by subject filter, and much
   cheaper than projecting millions of points into MariaDB to serve a view used once,
   after the race, by each team. **This is a deliberate departure from "reads come from
   projections"** and should be recognised as one rather than copied casually: it is
   justified by the read being bulk, cold, and non-critical, none of which is true of the
   directory or the scan history.

   **Sampling interval: 30 s** *(decided 2026-08-26 — "team are walking, we do not need
   sub-30 s resolution")*. That lands at **1,440 points per person** for a 12-hour race,
   **~195 KB per person**, and **~157 MB per event** — 9× the whole `NATHEJK` stream rather
   than 18–33×.

   The decision is better founded than a cost compromise, which is worth recording because
   it means finer sampling should not be revisited casually. At walking pace, 30 s puts
   consecutive points **33 m apart at 4 km/h and 50 m at 6 km/h**. Typical GPS error for a
   phone under forest canopy at night is **10–30 m**. So below roughly 30 s the *spacing
   between* points becomes smaller than the error *on* each point: the extra samples
   describe GPS noise, not movement. 5 s sampling would cost 3.75× the bytes to record a
   6 m stride that the receiver cannot resolve.

   It also relaxes the battery problem: 30 s is coarse enough that continuous
   high-accuracy `watchPosition` is not obviously the right acquisition mode, which the
   next paragraph makes more important than it first appears.

   **Platform limitation — a web app cannot record while backgrounded.** This is the real
   constraint on the feature, and the sampling decision does not address it.

   Geolocation is only available to a document, and a backgrounded document does not run:
   on iOS, JavaScript in a backgrounded web app is suspended. There is no background
   geolocation for web apps on any platform. Two escape routes that look plausible and are
   not:

   - **Screen Wake Lock** keeps the screen from dimming *while the document is visible*.
     Per the spec, "previously acquired locks are automatically released when document
     becomes inactive" — it buys no background execution, only a lit screen. And a lit
     screen for 12 hours is both a battery problem and a **light-discipline** problem in a
     night race.
   - **Periodic Background Sync** (Chrome-only regardless) can wake a service worker, but
     service workers have no access to the Geolocation API. Dead end even on Android.

   So as currently designed the track records **only while the app is open and visible**.
   For a night hike with the phone in a pocket that is a track of fragments, not a route.
   `MapsView.vue` makes it stricter still: it calls `location.stopWatch()` on
   `document.hidden` and on unmount, so today recording would also stop simply by
   navigating away from the map.

   **Accepted 2026-08-26 (maintainer): "fine with fragmented trace, just track as much as
   possible."** So this is a known limitation to work within, not a blocker, and the goal is
   coverage rather than continuity. Consequences:

   1. **The recorder must not live in the map view.** It belongs at app level — running
      while signed in and permission is granted — so leaving `/maps` does not stop it. This
      is the single change that most increases coverage, since "as much as possible" means
      recording whenever the app is foregrounded at all, not only on the map page. Drawing
      the live marker and recording the track are different concerns that happen to share a
      data source (task 082).
   2. **"The team's entire track" cannot be promised** as literally entire. What is
      deliverable is "everywhere you were while the app was open". Task 086 should say so in
      the UI rather than presenting gaps as if the member stood still.
   3. **Concept validation still works** — the pipeline, the batching, the retention and
      the post-race view can all be proven on fragmentary data.
   4. If a genuinely continuous track ever becomes a requirement, it needs something other
      than a PWA for the recording half.

   Task 082 should still measure the real behaviour on a device, not to decide whether to
   build it, but to know what coverage to expect and to catch anything worse than predicted.
2. **Do map tiles move onto the shared offline layer?** Tiles are the **largest** cached
   dataset by far, so leaving them outside the global budget largely defeats it — portraits
   (PRD 007) and tiles otherwise compete for the same per-origin quota with no agreed
   priority, and the OS decides what to evict.

   **Answered, in two halves.** *Storage policy:* yes — tiles are declared under PRD 009's
   budget and priority order (009 §11.1). *Mechanism:* no — PRD 009 was rescoped on
   2026-09-01 and cut its generic sync engine and dataset registry, partly because this
   cache had already shipped on Workbox and worked. So the tile cache stays where it is and
   observes 009's policy; there is nothing to migrate onto.

   **Scope decided 2026-08-26, then superseded the same day by measuring the actual race
   area.** The decision history is kept because the reasoning still applies if the area
   grows: first a 10 km radius of the current location, then 8 km with eviction, with a
   corridor ruled out because the race area is known but a given team's route is not.

   **The race area, derived from this year's checkpoints (2026-08-26).** 12 checkpoints
   exist on the stream for 2026; 9 carry coordinates. `NATHEJK.<year>.checkpoint.<id>.updated`
   carries `position: {latitude, longitude}` — so checkpoint definitions *do* have
   coordinates, which also part-answers the "scan data source" question below.

   | | |
   |---|---|
   | checkpoint bounding box | 55.6516–55.8525 N, 12.0578–12.3018 E (North Zealand) |
   | checkpoint extent | **22.4 km N–S × 15.3 km E–W** |
   | convex hull of the 9 | 220 km², 60 km perimeter |
   | hull + 3 km buffer | 428 km² (the geometric area) |
   | **as published to the client** | **476 km²** — see the disclosure grid below |

   **The published polygon is snapped outward to a ~1.1 km grid, which costs 11%.** Not
   tidiness — a fix for a real leak found while building task 088. A buffered hull is
   *invertible*: every vertex sits on a circle of radius `BufferKm` around a vertex of the
   original hull, and those vertices **are** checkpoint positions. Since the buffer distance
   has to be published too (the client must not add its own), an unsnapped polygon lets
   anyone offset inward and recover the outermost posts exactly. The first implementation did
   exactly that, emitting input latitudes at full float precision, because sampling a circle
   from angle 0 puts a vertex at its centre's own latitude.

   Snapping to a grid coarser than that inversion's precision destroys it, and the trade is
   explicit: **+48 km² and +34 MB of tiles** to turn "there is a post at this spot" back into
   "the event is in this region" — which the client must know anyway, since it caches tiles
   for it. Snapping is always outward, so coverage is never lost.

   **Tile costs measured *in the race area*, not extrapolated.** This mattered: North
   Zealand's cartography is denser than the rural Zealand sample used earlier, and topo
   tiles are up to **43% larger** there (z15: 59.5 kB vs 41.7 kB; z16: 41.0 kB vs 31.9 kB).

   | region | km² | tiles | topo | + aerial | total |
   |---|---|---|---|---|---|
   | 8 km radius (the superseded decision) | 201 | 2,587 | 131 MB | 29 MB | 160 MB |
   | race area, geometric | 428 | 5,291 | 264 MB | 60 MB | 324 MB |
   | **race area as published, z12–16** | **476** | **5,858** | **292 MB** | **66 MB** | **358 MB** |
   | race area as published, z12–15 | 476 | 1,590 | 122 MB | 21 MB | 143 MB |

   **Recommendation: cache the whole race area and drop the radius.** It is roughly twice
   the bytes of an 8 km radius and better on every other axis — complete coverage instead of
   a moving window, **no eviction logic at all**, deterministic contents, and all of it
   fetchable before the start while the participant still has coverage. A follow-me radius
   can only ever be filled where the network already works, which is precisely where the
   cache is not needed. 324 MB also sits comfortably inside the ~1 GB iOS 16 floor alongside
   portraits, the directory and the app shell.

   Note z16 alone is 195 MB of that 324 MB — 60%. If budget pressure appears, capping the
   topo at z15 (the app's own `LOCATE_ZOOM`) brings the whole area to 129 MB. And the aerial
   layer is 60 MB that nobody navigates by at night, so capping *it* at z14 while keeping
   topo at z16 gives ~270 MB. Both are levers to pull later rather than now.

   **Three caveats that shape the implementation:**

   1. **3 of 12 checkpoints have no coordinates** (Post 3A, Post 3B, "Til Gøgl"). *Accepted
      as normal, 2026-08-26 (maintainer): the derivation is an indication of the area, not a
      survey.* The 3 km buffer absorbs it — checkpoints in a night hike sit within one
      bounded area, so a hull around 9 of 12 plus 3 km is very likely to contain the other
      three. Worth counting and logging so a *systematic* collapse in coordinate coverage is
      visible, but not worth chasing individual gaps.
   2. **The checkpoint set is still changing** — the last `checkgroups.sorted` event is 16
      days old — and the area moves from year to year. So the area must be **derived from
      live data at sync time, not hardcoded**. What *is* stable is its size (below), so the
      storage budget can be fixed permanently even though the polygon cannot.
   3. Which means the client has to *learn* the area, so `hej` needs checkpoint positions:
      a small projection plus a way to serve the area to the client. **Built 2026-08-26 as
      task 088**: a `checkpoint` projection, `checkpoint.ComputeRaceArea`, and an
      authenticated `GET /api/race-area`. Deliberately *not* added to `GET /api/config`,
      whose own contract states everything it carries is public by definition — which the
      race area is not.

   **The area is roughly the same size every year** (maintainer), which makes the budget a
   one-time decision rather than an annual re-derivation. Planning envelope, using race-area
   tile sizes:

   | race area | z12–16 | z12–15 |
   |---|---|---|
   | 300 km² | 232 MB | 93 MB |
   | **428 km² (2026, measured)** | **324 MB** | **129 MB** |
   | 600 km² | 446 MB | 175 MB |
   | 700 km² | 517 MB | 202 MB |

   So **plan for ~450 MB and expect ~325 MB** at z12–16, which fits inside the ~1 GB iOS 16
   floor with room for portraits, the directory and the app shell.

   **How it is downloaded matters as much as how big it is** *(maintainer, 2026-08-26: "we
   should be aware if we start downloading several 100 MB on a mobile connection — at least
   we should cache if relevant map is being browsed")*. 325 MB pulled silently over rural
   mobile data is a real cost to a participant, possibly against a data cap, on a battery
   that has to last the night.

   **The platform will not help us decide.** The Network Information API
   (`navigator.connection`) is **not available in Safari**, so on iOS — the primary platform
   — the app cannot tell WiFi from cellular. "Download only on WiFi" is not implementable.
   `navigator.onLine` distinguishes only online from offline. So the decision has to be the
   user's, made with the size in front of them.

   Three tiers, in increasing cost:

   | tier | tiles | size | when |
   |---|---|---|---|
   | **cache while browsing** | — | **free** | always on |
   | z12–14 (orientation) | 404 | 56 MB | candidate for automatic |
   | z15 (+detail) | 1,026 | +73 MB | explicit opt-in |
   | z16 (native scale) | 3,861 | +195 MB | explicit opt-in |

   1. **Cache tiles as they are browsed.** Every tile fetched to draw the map is stored.
      This costs *nothing extra* — the bytes are already being downloaded — and it means a
      participant who looks at the map around the assembly area arrives with that area
      cached without being asked. This is the "at least" and should be unconditional.
   2. **z12–14 is 56 MB** and buys whole-area orientation at 5.4 m/px. Small enough to
      consider downloading without ceremony, though still worth announcing.
   3. **z15–z16 is the expensive 268 MB** and should be an explicit, user-initiated
      download with the size stated and progress shown — ideally prompted before the event
      while the participant is at home on WiFi (PRD 009 §11.7 already proposes a "prepare
      for offline" push a few hours before the start; this is exactly what it is for).

   **Correction to earlier advice in this section:** capping the topo at z15 was described as
   a cheap release valve. It is not cheap. DTK25 is a 508 DPI 1:25.000 product, so its native
   resolution is **1.25 m/px** — which is **z16** (1.34 m/px). z15 is half the linear
   resolution of the source map, a real loss of detail rather than a free saving, and z17 is
   2× oversampled, which is why its tiles shrink and carry no new information. **z16 is the
   correct ceiling**; dropping to z15 is a fidelity decision, not a housekeeping one.

   Rule of thumb for other shapes, using race-area tile sizes: **0.73 MB/km²** at z12–16,
   **0.28 MB/km²** at z12–15 (both layers, aerial as JPEG).

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
