# PRD 014 — Development device simulation

**Status:** doing
**Author:** agent session (Zed / Claude), with knj
**Created:** 2026-09-12
**Last updated:** 2026-09-12
**Approved:** 2026-09-12
**Shipped:**
**Target users:** developer (and, secondarily, anyone doing pre-event QA on a laptop)

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 1. Summary

A development-only layer that lets the whole app — install wall, onboarding, login,
map, contacts, profile, offline states — be exercised from a laptop browser by
**simulating a mobile device**, rather than by switching the gates that produce
those flows off. Plus a dev control panel for the state resets that testing those
flows repeatedly requires, and a small number of dev affordances (SMS PIN, fake
position) for the parts of a flow a laptop cannot supply on its own.

Compiled out of production builds, and refused by the BFF outside
`ENV=development`.

## 2. Problem & Motivation

**What problem does this solve?**

PRD 005 made the app installed-mobile-only, and correctly so: `router/gates.ts`
sends a desktop browser out of the SPA entirely (`LEAVE_APP` →
`/desktop.html`) and sends a mobile browser tab to the install wall. That is the
right product behaviour and this PRD does not touch it.

The cost lands on development. The dev environment runs on this laptop, and the
laptop is precisely the device the gates exclude. Today there is exactly one
escape hatch — `?nogate=1` in `config/gates.ts`, which sets
`hej.gates.bypass` and makes `gatesEnabled()` return `false` — and it is the
wrong shape for development work, for a specific reason:

> `gatesEnabled()` is consulted once, in `router/index.ts:133`, and it disables
> **step 2, 3 and 4 together** — device class, the install wall *and the
> onboarding redirect*. So the only way onto the laptop today is a way that also
> skips the onboarding flow, which is one of the flows most in need of testing.

It is also, by its own comment, deliberately not this: *"It is **not** the answer
to 'an organizer needs laptop access' — PRD 005 §11 says that would be a new
PRD, not a flag."* This is that new PRD, scoped to development rather than to
organizers.

Beyond the gate, four further things make laptop testing slower than it should be:

1. **Platform-specific branches are unreachable.** `installPlatform()` and
   `detectPlatform()` (`config/permissions.ts`) drive the iOS vs Chromium vs
   webview install instructions and the per-platform "permission blocked"
   guidance. On a Mac, the iOS copy — the copy most participants will read —
   cannot be seen at all.
2. **Re-testing a flow means hand-clearing storage.** Onboarding completion, the
   session, the gate override, the runtime-config mirror, the contacts cache and
   the track database live across localStorage, IndexedDB and the SW caches.
   Re-running onboarding currently means a DevTools archaeology session.
3. **Position.** `location.store` / `track.store` already narrow to a small slice
   of `navigator.geolocation`, but on a laptop there is no plausible position and
   no *movement*. Chrome's sensor override can pin one coordinate; it cannot walk
   a route, which is what the map, the position marker and the scan history
   actually respond to.
4. **Login needs a phone number.** The PIN is already reachable
   (`sms.LogSender`, `cmd/api/main.go:356`, so it is in `docker compose logs api`),
   but going to the terminal and back on every login — including every
   switch-profile and shared-number `choose` test — is friction paid dozens of
   times a day.

**Why now?** PRD 005 and 007 are in `doing/`, 010/011/013 are drafted, and each of
them adds flows that will be built and re-tested many times. The friction
compounds per feature; the fix is one-off.

**Evidence.**

- `config/gates.ts` lines 55–71 name the problem and explicitly decline to solve it.
- `router/gates.ts:59` — `if (!isMobileDevice()) return LEAVE_APP`.
- The existing `SHOW_LAYOUT_DEBUG` / `LayoutDebug.vue` pair is the same need
  answered once already, for layout only.
- Direct request from the developer, 2026-09-12: *"for speed and ease I need to be
  able to test all the various flows in development on this laptop."*

## 3. Goals

- Every route and every flow the app has is reachable on a laptop, **with the real
  gate chain running** — the app should believe it is on an installed phone, not
  believe there is no gate.
- Platform-conditional UI (iOS / Android / Chromium / webview) can be inspected on
  demand, without owning the device.
- Re-running a stateful flow (onboarding, login, first sync) is one click, not a
  DevTools ritual.
- The map and scan history can be exercised with a plausible, *moving* position.
- Zero production surface: no dev code in a production bundle, no dev endpoint on a
  production BFF, and a test that proves both.
- Nothing in the existing device-detection or gate logic changes behaviour for a
  real user. `platform.spec.ts`, `gates.spec.ts` and `rolegate.spec.ts` keep
  passing unmodified.

## 4. Non-Goals

- **Not desktop support.** The product remains installed-mobile-only (PRD 005), and
  `/desktop.html` (PRD 013) remains the browser experience. This adds no supported
  configuration.
- **Not organizer laptop access.** Still out of scope, still its own PRD if wanted.
  This layer is unavailable in production by construction, which is what keeps the
  two apart.
- **Not a replacement for device testing.** §7.4 defines the residual mobile-only
  checklist; this PRD explicitly does not claim to cover it.
- **Not a change to `install_gate`.** The runtime kill switch keeps its current
  meaning and its current operational purpose.
- **Not removing `?nogate=`.** It stays, unchanged, as the way to verify the kill
  switch itself.
- **Not seeding test data.** The dev dataset comes from the JetStream broker
  (PRD 008); this PRD consumes it and does not define it.
- **Not automated E2E.** A simulation layer makes E2E easier later, but no test
  runner is in scope.

## 5. User Stories & Scenarios

- As a **developer**, I want the app to treat my laptop as an installed phone, so
  that I can walk the real onboarding chain without deploying to a handset.
- As a **developer**, I want to flip "installed" off, so that I can see the install
  wall — and flip the simulated platform to iOS, so I can see the Add-to-Home-Screen
  copy an iPhone user actually reads.
- As a **developer**, I want to reset onboarding and my session in one click, so
  that testing step 3 of the flow does not cost more than writing it did.
- As a **developer**, I want a fake position that moves, so that the map's centring,
  accuracy and staleness states and the track log are all reachable.
- As a **developer**, I want the login PIN in the UI, so that logging in does not
  require a second terminal.

### Primary happy path

1. `docker compose up`; open `https://hej.local.nathejk.dk` on the laptop.
2. Without the dev layer, the gate fires and the browser leaves for
   `/desktop.html`. So the entry point must work **from that page**: the dev
   affordance is the URL form, `?dev=iphone`, which the developer can also reach by
   editing the address bar of the placeholder. It writes the profile and reloads
   into the app.
3. The app now reads as *mobile, standalone, iOS*. The real gate chain runs and
   lands on `/welcome` — genuine onboarding, not a bypass.
4. Log in: the PIN appears in the dev panel (or is autofilled), including the
   shared-number `choose` branch for a seeded multi-profile number.
5. Onboarding's location step: the fake provider answers, so the pre-prompt →
   granted → position path completes.
6. Onboarding's portrait step: the laptop webcam serves `getUserMedia` for real.
7. Onboarding's notification step: with `VAPID_*` set, desktop Chrome does real Web
   Push — permission, subscribe, `push-sw.js`, `notificationclick` deep link.
8. Land on `/maps`. Toggle "fake movement" and watch the marker, the tile cache and
   the track log respond.
9. Click **Reset onboarding** and repeat from step 3 in ~5 seconds.

### Edge cases and error scenarios

- **A stale profile left on.** A dev profile persists in localStorage, so the
  laptop could be left simulating a phone indefinitely. The dev panel must always
  be visible while a profile is active, and it must say what is being simulated —
  a silent simulation is a debugging trap, where the developer chases a bug that
  is their own override. `?dev=off` clears it, mirroring `?nogate=0`.
- **The profile is active on a real phone.** Possible: localStorage is per-origin
  and the dev origin is reachable from a phone on the LAN. Harmless (it can only
  claim mobile/standalone, which is true there) but the panel must still show it.
- **`?dev=` in production.** The parse is inside an `import.meta.env.PROD` guard,
  so the branch is compiled out — the parameter does nothing. Same technique as
  `initGateOverride`.
- **Fake position while a real one is available.** The fake provider wins when
  enabled, and the panel says so; otherwise a real geolocation on a laptop with
  Wi-Fi positioning would silently contradict the simulated route.
- **Firefox / Safari on the laptop.** No `beforeinstallprompt` and no
  `navigator.standalone`, so the simulated values are what the app sees. That is
  the point, and it means the layer works the same in every dev browser.
- **The dev PIN endpoint hit with no such phone.** 404 with no body detail; it must
  not become a phone-number oracle even in dev, because the dev database holds real
  member data replayed from the broker.

## 6. Requirements

### Functional

**Device simulation**

- [ ] A persisted dev device profile: `{ mobile: boolean, standalone: boolean,
      platform: 'ios' | 'android' | 'chromium' | 'webview' | 'other' }`, in
      `localStorage['hej.dev.device']`.
- [ ] `helpers/platform.ts`'s `defaultEnv()` yields to the profile, so
      `isMobileDevice()`, `isStandalone()` and `installPlatform()` return the
      simulated answers **without their signatures, logic or injectability
      changing**. The override lives in the environment, not in the predicates.
- [ ] `detectPlatform()` in `config/permissions.ts` honours the same profile, so
      `blockedGuidance()` renders the iOS/Android copy on demand.
- [ ] `?dev=iphone|ipad|android|chromium|tab|desktop|off` writes or clears the
      profile before the router's first navigation (i.e. from `main.ts`, alongside
      `initGateOverride()`), and works when typed onto `/desktop.html`.
- [ ] The gate chain is otherwise untouched: with `dev=iphone` the app runs
      steps 2–4 for real and onboarding actually gates.
- [ ] `dev=tab` produces mobile-but-not-standalone, i.e. the install wall.
- [ ] `dev=desktop` clears simulation, so the desktop hand-off can be tested too.

**Dev panel**

- [ ] `components/DevPanel.vue`, rendered only when `import.meta.env.DEV`,
      alongside `LayoutDebug` in `App.vue`. Collapsed to a small handle by default;
      its expanded state persists.
- [ ] Shows, always, the *effective* values: simulated vs real `mobile`,
      `standalone`, `platform`, plus `__BUILD_ID__`, gate state and
      `install_gate`.
- [ ] Device profile controls (the toggles above), applied without a reload where
      possible and with one where not.
- [ ] **Reset onboarding** — clears the onboarding store's persisted completion.
- [ ] **Log out / clear session** — drops the session cookie and session store.
- [ ] **Force offline** — drives `app.store.online` to `false` (the store already
      accepts a corrected value) so PRD 009's offline states can be produced
      without DevTools throttling, which also kills the dev server's HMR socket.
- [ ] **Clear caches** — SW caches, `trackDb`, contacts cache, runtime-config
      mirror; then unregister the service worker.
- [ ] **Fake safe-area insets** — sets `--sat/--sar/--sab/--sal` (`assets/main.css`)
      to iPhone-with-notch values, so the shell geometry a phone produces is
      inspectable. Uses the existing indirection rather than `env()` directly.
- [ ] **Fake position** — on/off, a pinned coordinate in the event area, and an
      optional slow track playback.

**Position simulation**

- [ ] A dev geolocation provider satisfying the slice `location.store` /
      `track.store` already depend on, injected at the same seam rather than by
      patching `navigator`.
- [ ] Answers `getCurrentPosition` and `watchPosition`, with configurable accuracy,
      so the map's accuracy-circle and staleness states are reachable.
- [ ] Optional playback over a small hard-coded polyline, at a walking pace, so
      movement-driven behaviour (recentring, track logging, tile prefetch) fires.
- [ ] Can simulate a **failure** (`PERMISSION_DENIED`, `POSITION_UNAVAILABLE`,
      timeout), since `track.store`'s failure-dedup path and the map's "location
      off" state are otherwise hard to reach deliberately.

**Login in dev**

- [ ] `GET /api/dev/pin?phone=` returns the currently issued PIN for a number.
      Registered **only** when the BFF's env is `development`; absent otherwise —
      not present-and-403, so a production binary has no such route at all.
- [ ] OpenAPI annotations, per repo rules, with the description stating plainly
      that the route does not exist outside development.
- [ ] The dev panel shows the PIN for the number currently being logged in, with a
      copy affordance. Autofill is acceptable but must not skip the input, since
      the input's own behaviour (paste, iOS autofill, validation) is under test.

**Safety**

- [ ] Every frontend branch keyed on `import.meta.env.PROD` / `DEV`, so Vite
      compiles it out — the pattern `gates.ts:23` already uses.
- [ ] A unit test asserting the profile is inert and unwritable when `PROD`.
- [ ] A Go test asserting `/api/dev/pin` is not routed outside `development`.
- [ ] A build-output check (script or CI step) asserting the production bundle
      contains neither `hej.dev.device` nor the dev panel.

### Non-Functional

- **Security / privacy.** The dev database holds real personal data about minors
  replayed from the broker (see the phpMyAdmin note in `docker-compose.yml`). The
  dev PIN route is therefore dev-only *by absence*, loopback-equivalent in reach
  (the BFF is not on Traefik in dev), and logs nothing new. **No guardian phone
  number may appear anywhere in this layer** — the panel shows no member fields at
  all beyond the number being logged in, which is the developer's own input. That
  is the `.rules` hard rule, and a debug panel is exactly the kind of surface it
  exists to pre-empt.
- **Performance.** Zero cost in production (compiled out). In dev, the profile is
  one synchronous localStorage read behind the same memoisation-free contract
  `platform.ts` already documents — no module-level caching, since the profile can
  change at runtime.
- **Honesty over convenience.** The layer must never make the app *look* like it
  works when it does not. Anything simulated is labelled as simulated in the panel;
  this is the difference between a testing tool and a source of false confidence.
- **Accessibility / i18n.** N/A for the panel itself — it is developer-facing and
  may be English and untranslated. The *flows it exposes* are of course still
  subject to the app's Danish copy requirements.

## 7. UX / UI Notes

### 7.1 The panel

Bottom-left handle (`LayoutDebug` occupies the opposite corner), deliberately ugly
and unmistakably not product UI: monospace, high-contrast, no brand font, no
shadcn-vue styling. It is not part of the app's design system and should never be
mistaken for it — which is also why it does **not** get a shadcn `Drawer`, despite
one existing in `components/ui/drawer/`. The repo rule prefers a catalogue
component where one fits; here fitting in is the anti-requirement.

When a profile is active, the handle shows a persistent marker (e.g. `SIM: iphone`)
even collapsed.

### 7.2 Routes and views

No new routes. No changes to `router/index.ts` beyond none-at-all — the gates are
consulted as they are; only what `platform.ts` reports changes. `App.vue` gains one
conditional render next to `LayoutDebug`.

### 7.3 What becomes testable on the laptop

| Flow | Today | With this PRD |
|---|---|---|
| Install wall, all four platform variants | phone only | `?dev=tab` + platform toggle |
| Onboarding, all steps | bypassed by `nogate` | real chain, resettable |
| Login, shared number, `choose`, switch profile | terminal round-trip per attempt | in-panel PIN |
| Map position, movement, accuracy, failure | pinned coordinate at best | fake provider + playback |
| Contacts pane, role gating, polling | reachable but hard to reset | reachable + resettable |
| Offline / cache states (PRD 009) | DevTools throttling (breaks HMR) | force-offline toggle |
| Portrait capture | works (webcam) | unchanged |
| Web Push, subscribe, notification click | works in Chrome with VAPID set | unchanged, documented |
| Safe-area / notch geometry | `LayoutDebug` reports, cannot reproduce | fake insets |

### 7.4 What remains mobile-only

To be recorded as a short pre-release smoke checklist rather than pretended away:

- iOS Add to Home Screen, and the resulting splash / status-bar chrome.
- iOS Web Push (16.4+, home-screen only) — the platform where push matters most.
- Android `beforeinstallprompt` accept path and the richer install dialog.
- Real GPS: drift, indoor loss, accuracy variance, backgrounding, and iOS killing a
  backgrounded standalone app (`location.store`'s comment at line 90).
- Real camera framing and portrait quality.
- Touch ergonomics, one-handed reach, sunlight legibility.

## 8. Technical Considerations

### Frontend (Vue 3 / TS)

**A dev-only module tree, `src/dev/`.** Revised during implementation (task 208) after the
first cut leaked into production — see the note at the end of this section.

- **New:** `src/dev/devDevice.ts` — the profile: read, write, `?dev=` parse, `PROD`
  inertness. Plus `devDevice.spec.ts`.
- **New:** `src/dev/platformSim.ts` — profile → `PlatformEnv`, including the fake
  user-agent table. Plus `devPlatform.spec.ts`.
- **New:** `src/dev/bootstrap.ts` — the single entry point; registers the provider.
- **New:** `src/dev/DevPanel.vue`.
- **New:** `src/dev/devGeolocation.ts` — the fake position provider + playback.
- **Changed:** `src/helpers/platform.ts` — `defaultEnv()` consults a **registered**
  provider (`setDevEnvProvider`), and exports `devNavigator()` for `permissions.ts`.
  The predicates and the `PlatformEnv` seam are untouched, which is what keeps
  `platform.spec.ts` valid as written.
- **Changed:** `src/config/permissions.ts` — `detectPlatform()` sniffs
  `devNavigator()` when present. Note this file currently claims to be "the ONLY
  user-agent sniff"; the comment needs updating to say where the override enters.
- **Changed:** `src/main.ts` — `await import('@/dev/bootstrap')` inside
  `if (import.meta.env.DEV)`, before the first navigation.
- **Changed:** `src/App.vue` — the panel as an async component, likewise DEV-gated.
- **Changed:** `src/stores/location.store.ts`, `src/stores/track.store.ts` — accept
  the dev provider at the existing injection point. No logic change.
- **Unchanged, deliberately:** `src/router/gates.ts`, `src/router/index.ts`,
  `src/config/gates.ts`. If this PRD ends up editing gate logic, the design has
  drifted.

**Nothing in `src/dev/` may be imported statically from product code**, and this is the
rule the whole safety argument rests on. It was learned the expensive way: the first
implementation guarded every *use* with `import.meta.env`, and the production bundle still
contained the fake user-agent strings and the panel's copy — measured in
`dist/assets/index-*.js`, not hypothesised. An `import.meta.env` guard hides code; it does not
remove code something still imports. So the layer is reached only through a dynamic import
inside a folded-away branch, which is why `platform.ts` takes a *registered* provider instead of
importing the simulation, and why `App.vue` uses `defineAsyncComponent`. Task 217's bundle scan
exists to keep this true.

### BFF (Go)

- **New:** `go/cmd/api/dev.go` — the PIN lookup, plus `dev_test.go` asserting the
  route's absence outside `development`.
- **Changed:** route registration (`routes.go`) — conditional on env; the env is
  already available (`cmd/api/env.go`).
- Requires reading a currently-issued PIN from `app.pins`. If the store has no
  read accessor, adding one is dev-facing surface on a security-relevant type, so
  it should be a clearly named, separately reviewed accessor rather than widening
  the interface.
- **No change** to `sms.LogSender` — the logged PIN stays as the fallback.

### API endpoints

| Method | Path | Notes |
|---|---|---|
| `GET` | `/api/dev/pin?phone=` | **Dev only.** Returns `{ "pin": "1234" }`, or 404 if none issued. OpenAPI-annotated per `.rules`, with the description stating it is absent outside `ENV=development`. |

No changes to `/api/config`: the dev layer is client-side and build-gated, so
serving it a flag would put dev switches on a public production endpoint — the
opposite of the safety property here. This is the one place where the existing
runtime-config pattern is deliberately *not* followed.

### Data / storage

No schema changes. New localStorage keys, namespaced `hej.dev.*` to keep them
distinguishable from product keys (`hej.gates.bypass`, `hej.install-gate`,
`hej.show-*`, `hej.dataforsyningen-token`) — and so "clear all dev overrides" is a
prefix scan.

### Dependencies & risks

- **No new npm or Go dependencies.**
- **Risk: false confidence.** A simulated iPhone is not an iPhone. Mitigated by
  §7.4 and by the panel labelling every simulated value.
- **Risk: the override leaks into a real device's session.** Mitigated by the
  always-visible marker and `?dev=off`.
- **Risk: dev-only code drifts and breaks the dev build.** Mitigated by the unit
  tests being ordinary tests, run in CI like the rest.
- **Risk: scope creep into a general "admin panel".** The non-goals are the guard;
  anything that would be useful in production does not belong here.
- **Docs:** `docker-compose.override.yml` should carry `VAPID_*` for the push path
  (§5 step 7), and `README.md` should gain a short "testing on the laptop" section
  pointing at `?dev=`.

## 9. Success Metrics

- **Coverage:** every route in `router/index.ts`, and every step of the onboarding
  chain, reachable on the laptop with the gates running. Target: 100% of routes,
  verified once by hand at completion and recorded in the closing task.
- **Reset cost:** onboarding re-runnable in under 10 seconds without DevTools
  (today: minutes, with per-key storage editing).
- **Platform copy:** all four `installPlatform()` branches and all three
  `detectPlatform()` guidance variants viewable on demand.
- **Production cleanliness:** the bundle check finds no dev strings; the Go test
  finds no dev route. Both in CI, so the metric holds rather than being measured
  once.
- **Qualitative:** the number of "deploy to phone to check one thing" cycles per
  feature drops noticeably. Worth an explicit note in the first feature built after
  this ships (likely PRD 007 or 010).

## 10. Rollout / Task Breakdown

No feature flag and no phased rollout: the whole PRD is dev-only, so it ships
inert. Sequencing is by value — steps 1–2 deliver most of the benefit and touch six
files, so they should land and be used before the rest is judged.

**Phase 1 — simulation (the core)**

- [ ] **Task 206:** add `config/devDevice.ts` — persisted dev device profile, `?dev=` parse, `PROD`-inert, with tests, wired into `main.ts` before the router's first navigation
- [ ] **Task 207:** have `platform.ts`'s `defaultEnv()` and `permissions.ts`'s `detectPlatform()` honour the dev profile

(The `main.ts` wiring was originally listed as a third task and was folded into 206
when the tasks were written: it is two lines, and separating it would leave 206
complete but inert.)

**Phase 2 — the panel**

- [ ] **Task 208:** add `DevPanel.vue` with effective-state readout and device-profile toggles
- [ ] **Task 209:** dev panel resets — onboarding, session, caches, service worker
- [ ] **Task 210:** dev panel force-offline toggle driving `app.store.online`
- [ ] **Task 211:** dev panel fake safe-area insets via the `--sat/--sar/--sab/--sal` seam

**Phase 3 — position**

- [ ] **Task 212:** add `helpers/devGeolocation.ts` — fake position provider with accuracy and failure modes
- [ ] **Task 213:** fake track playback over a hard-coded event-area polyline
- [ ] **Task 214:** inject the dev provider into `location.store` and `track.store` at the existing seam

**Phase 4 — login**

- [ ] **Task 215:** add dev-only `GET /api/dev/pin` with OpenAPI annotations and an absence test
- [ ] **Task 216:** surface the dev PIN in the dev panel

**Phase 5 — guardrails and docs**

- [ ] **Task 217:** CI check that the production bundle contains no dev-layer strings
- [ ] **Task 218:** document laptop testing in `README.md`, including the VAPID setup for push
- [ ] **Task 219:** write the mobile-only smoke checklist (§7.4) into `roadmap/`

## 11. Open Questions

1. **Should the panel be available in a staging/preview build, or `DEV` only?**
   `DEV` only is proposed, because it is the only line Vite draws for us that
   cannot be crossed at runtime. A staging exception would need its own mechanism
   and would weaken the safety story.
2. **Where does the fake polyline come from?** A hand-picked route near the 2026
   event area is proposed. Deriving it from a real seeded track would be more
   realistic but puts a participant's actual movements into committed source —
   which the `.rules` privacy posture argues against.

   **Answered 2026-09-12 (task 213): hand-picked, committed as source.** Real track
   data would be a minor's movement history in the repository.
3. **Does `app.pins` expose a read accessor today, or does one need adding?** If it
   needs adding, is a dev-only accessor on that type acceptable, or should the dev
   route re-derive the PIN some other way? (Needs a look at the `pins`
   implementation in `internal/`.)

   **Answered 2026-09-12 (task 215), and the answer was worse than the question
   assumed.** PINs are stored **bcrypt-hashed only**, so an issued PIN is genuinely
   unreadable — no accessor could expose it. What the endpoint needs is plaintext
   *retention*, which is a real concession rather than a getter. Implemented as a
   separate constructor, `pin.NewDevStoreWithPlaintextRecall()`, selected once at the
   single call site that knows `ENV`: `pin.NewStore()` is byte-for-byte unchanged in
   behaviour, so a production process never holds a plaintext PIN in memory at all.
   The accessor reports only the code — not attempts, expiry or send time — refuses
   expired records, and neither consumes the PIN nor counts an attempt.
4. **Should `?dev=` also be able to simulate a *role*** (spejder / bandit / lead),
   given `roleGate` and PRD 007's role-dependent contacts pane? Attractive, but it
   would mean simulating an identity the BFF does not agree with — the endpoints
   would still answer 403 — so it may create more confusion than it removes.
   Proposed: no, log in as a seeded member of the role instead.
5. **Should the dev panel be able to fire a test push to itself**, or is that
   better as a BFF dev endpoint? Deferred; not needed for phase 1.
6. **Does anything in `LayoutDebug.vue` become redundant** once the panel exists,
   and if so should they merge? Proposed: leave both, because `LayoutDebug` is
   driven by `SHOW_LAYOUT_DEBUG` and is usable *in production on a real phone*,
   which is precisely what this panel must never be.
