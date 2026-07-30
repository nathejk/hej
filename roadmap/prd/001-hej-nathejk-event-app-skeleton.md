# PRD 001 — "Hej Nathejk" Event App Skeleton (PWA shell + phone login)

**Status:** in-review
**Author:** agent session
**Created:** 2026-07-30
**Last updated:** 2026-07-30
**Target users:** every event user must sign in. App roles: **spejder** and **bandit** (the two main roles), plus **postmandskab**, **guide**, and **samarit**. Each role sees its own navigation set.

---

## 1. Summary

Build the foundational full-stack skeleton for **"Hej Nathejk"**, the companion
app used *during* the Nathejk event: a Vue 3 (TS) mobile-first, installable web
app ("add to home screen") backed by a dedicated Go **BFF**. It behaves like a
native app and acts as the container for all in-event content — maps, contacts,
the rulebook, and live event updates.

**The app cannot be used until the user signs in.** Login is passwordless and
phone-based: the username is the user's **phone number**. When the BFF
recognizes the number it knows the user's **type/role** (and therefore which
pages to show) and sends a **PIN code by SMS**; the user enters that PIN as their
one-time password to establish a session. The signed-in role then drives the
app-style bottom navigation bar (up to 5 icons, with a burger/overflow menu when
there are more than 5 pages). The skeleton also ships with a "new version
available" reload mechanism and the plumbing to request and use the browser
**Geolocation** and **Notification/Push** APIs.

## 2. Problem & Motivation

- **What problem does this solve?** During the event, users need one place on
  their phone for maps, contacts, the rulebook, and live updates — but only the
  right people should get in, and each role should see the right sections. The
  repo has no frontend shell or backend yet; both the app frame and a login gate
  need to exist before any feature can be built. Phone + SMS-PIN login is chosen
  because every participant already has a phone number on file and carries the
  phone at the event, so it's the lowest-friction, credential-free way to
  authenticate on site.
- **Why now?** This shell + login gate is the prerequisite for all in-event
  features. It defines the app's frame, the authentication model, routing,
  install/update behavior, and how we ask for location and notification
  permissions.
- **Evidence.** The app is explicitly intended for on-site mobile use, added to
  the home screen so it launches like a regular app; users must sign in with
  their phone number and an SMS PIN; recognizing the number tells us the user
  type and which pages to show.

## 3. Goals

- Establish a Vue 3 (TS) SPA skeleton following repo conventions (`vue/`,
  Vite, Tailwind, PrimeVue Lara preset, Pinia).
- Establish a Go **BFF** skeleton following repo conventions (`go/cmd/api`, `app`
  transport helpers, `internal/data` + `commands` facades) that serves the SPA
  in production and exposes the JSON API the app needs.
- **Gate the whole app behind login.** Nothing beyond the login screen is
  reachable without an authenticated session.
- **Phone-number + SMS-PIN authentication:** username is the phone number; a
  recognized number triggers an SMS PIN that is entered as a one-time password to
  create a session.
- **Role from recognition:** recognizing the phone number determines the user's
  type/role, which in turn determines which pages appear in navigation.
- **Role-aware navigation:** the set (and type) of pages shown in the bottom nav
  is derived from the signed-in user's role, computed from a single declarative
  config filtered against the session.
- Brand it as **"Hej Nathejk"** (app name, manifest, home-screen label, install
  title).
- Deliver an app-like mobile UX: full-height layout, safe-area aware, and a
  persistent bottom navigation bar.
- Bottom nav supports up to 5 primary destinations; when there are more than 5
  pages, the 5th slot becomes a "More" (burger) entry that opens the remaining
  destinations.
- Make the app installable to the home screen (PWA manifest + icons + standalone
  display) so it launches chrome-less like a native app.
- Detect when a new version of the app has been released and let the user
  reload into it without manually clearing anything.
- Request and manage **Geolocation** permission, and expose current location to
  the app (for map/location features built later).
- Request and manage **Notification** permission (and register for **Push**), so
  the app can deliver event updates.

## 4. Non-Goals

- No full implementations of the content pages (maps, contacts, rulebook,
  updates) — this PRD delivers the shell plus placeholder/stub views. The real
  destinations are wired into the nav so the frame is correct.
- No map rendering, contact data, rulebook content, or update feed content
  (each becomes its own follow-up PRD).
- **No user/account creation or self-signup, and no real user directory yet.**
  For this skeleton, recognized users + their role are provided by a **mock**
  (a hardcoded/in-memory list of phone number → role) behind a small interface,
  so the real lookup against Nathejk records can be dropped in later without
  touching handlers. Populating/administering the real directory is out of
  scope; people who sign up live elsewhere (tilmelding).
- No password storage. Authentication is a one-time SMS PIN only.
- No role/permission *administration* UI. Roles are read to drive navigation;
  managing them is out of scope.
- No domain aggregates, JetStream consumers, or event sourcing beyond the
  minimum storage needed for auth (PIN/session) and push subscriptions. The BFF
  skeleton is the transport/wiring frame plus the login flow, not the domain.
- No offline data caching beyond what's needed to make the app installable and
  to serve the shell.
- No backend push-delivery infrastructure beyond the minimal subscription
  endpoint needed to prove the notification plumbing — actual message fan-out is
  a later PRD.

## 5. User Stories & Scenarios

- As an event user, I want to sign in with my phone number and an SMS PIN so
  that I can use the app without remembering a password.
- As a recognized user, I want the app to already know my role so that I only
  see the sections relevant to me.
- As a `spejder`, I want a bottom navigation bar so that I can jump between
  maps, contacts, rulebook, and updates with one thumb tap.
- As a `postmandskab`/`guide`/`samarit`, when I'm signed in I want to see the
  sections relevant to my role (which may be more, fewer, or different than a
  `spejder`'s or `bandit`'s).
- As a `spejder`, when there are more sections than fit in the bar, I want a
  "More" menu so that I can still reach every section.
- As a signed-in user, I want the app to ask for my location and permission to
  send notifications so that map and live-update features work.
- As a signed-in user, when a new version is released, I want to be told and be
  able to reload into it so I'm never stuck on a stale build during the event.

**Primary happy path (login)**

1. User opens "Hej Nathejk" (installed to home screen, standalone). Not signed
   in → the login screen is shown; nothing else is reachable.
2. User enters their **phone number** and submits.
3. BFF looks the number up. If recognized, it generates a PIN, sends it by
   **SMS**, and responds that a PIN was sent (without revealing role yet).
4. User receives the SMS and enters the **PIN**.
5. BFF verifies the PIN (correct, not expired, within attempt limit),
   establishes a **session**, and returns the user's identity + role.
6. App loads the shell, builds the bottom nav from the role's allowed sections,
   and (in context) requests notification + location permission.
7. On a new release, the app surfaces a non-blocking "A new version is available
   — Reload" prompt.

**Edge cases**

- **Phone not recognized:** the flow does not grant access, and the UI is
  **identical to the recognized case** (anti-enumeration). After submitting the
  phone number the user always lands on the PIN-entry screen showing reassurance
  copy along the lines of: *"If we know you, we have sent you an SMS. If you
  don't receive an SMS and you feel we should know you, please reach out."* No
  PIN was actually sent, so verification simply never succeeds for an unknown
  number. "Reach out" is a tap-to-call link to the **nødtelefon**.
- **Wrong PIN / expired PIN / too many attempts:** clear error; allow resend
  after a cooldown; lock out after N attempts.
- **Resend PIN:** user can request a new PIN after a short cooldown; a new PIN
  invalidates the previous one.
- **Session expiry mid-use:** the app returns to the login screen; after
  re-auth, the nav re-derives from the (possibly changed) role.
- **A recognized number maps to multiple people/roles:** needs a rule (see Open
  Questions).
- **≤ allowed sections fit:** all shown in the bar; no burger. More than fit:
  4 primary + a 5th "More" opening the overflow list.
- **Permission denied / dismissed:** app degrades gracefully; denied
  notifications/location must not break the app.
- **iOS quirks:** Web Push only works when installed to the home screen; SMS
  autofill of the PIN benefits from the `one-time-code` autocomplete hint.
- Small / notched devices: nav respects safe-area insets.

## 6. Requirements

### Functional

**Authentication (gate)**

- [ ] The app is unusable until authenticated: a login screen is the only thing
      an unauthenticated user can reach; all other routes redirect to it.
- [ ] **Phone entry step:** user enters a phone number; the app calls the BFF to
      request a PIN.
- [ ] BFF **recognizes** the phone number against a **mock user directory**
      (hardcoded/in-memory phone → role, behind an interface so it can be swapped
      for real Nathejk lookup later) and, if known, generates a short-lived PIN
      and sends it via SMS. The response and resulting UI are **identical whether
      or not the number was recognized** (anti-enumeration) — the app always
      advances to the PIN-entry screen.
- [ ] The PIN-entry screen shows reassurance copy for the case where no SMS
      arrives: *"If we know you, we have sent you an SMS. If you don't receive an
      SMS and you feel we should know you, please reach out."* "Reach out" is a
      tap-to-call link to the **nødtelefon** (emergency phone).
- [ ] **PIN entry step:** user enters the PIN; the app calls the BFF to verify.
- [ ] BFF verifies PIN correctness, expiry, and attempt count; on success it
      establishes a session and returns identity + role.
- [ ] PIN is single-use and time-limited (**6 digits, 10-minute TTL**); resend
      is allowed after a **60s cooldown** and invalidates prior PINs; failures
      are rate-limited and locked out after **5 attempts**.
- [ ] A successful login yields a session lasting **at least 7 days** (survives
      app restarts / re-launches from the home screen).
- [ ] `GET /api/me` returns the current session identity + role, or 401 when not
      signed in; the app uses it on load to decide login vs shell.
- [ ] Sign-out clears the session and returns to the login screen.

**Shell & navigation**

- [ ] Vue 3 SPA scaffold under `vue/` per `vue3-spa-layout` (Vite, Tailwind,
      PrimeVue unstyled + Lara preset, Pinia, `@/*` alias, container-based dev).
- [ ] App branded **"Hej Nathejk"** throughout (title, manifest, install prompt,
      home-screen label).
- [ ] Root `App.vue` renders the app shell (scrollable content + fixed bottom
      nav) for authenticated users; the login screen is shown otherwise.
- [ ] A `BottomNav` component driven by a declarative destination list (icon +
      label + route + **required role(s)/visibility rule**).
- [ ] Navigation is **filtered by the signed-in user's role**; the ≤5 / burger
      rule is applied to that filtered set.
- [ ] **Route guards** enforce auth (redirect to login when unauthenticated) and
      role (redirect to an allowed page when the role can't access a route).
- [ ] Bottom nav renders at most 5 slots; if allowed destinations exceed 5,
      render 4 + a "More" burger opening an overflow sheet of the rest.
- [ ] Active-route highlighting in the nav (and overflow list).
- [ ] Placeholder views — **Maps, Contacts, Rulebook, Event Updates** — plus
      enough additional role-gated stubs to exceed 5 for some role and
      demonstrate the burger, wired into `router/index.ts`.

**BFF**

- [ ] **Go BFF scaffold** under `go/` per `go-bff-layout`: `cmd/api`
      (`main.go`, `routes.go`, resource files), `app` transport helpers,
      `internal/data` + `commands` stubs, env-based config, SPA-fallback file
      server for production.
- [ ] Auth endpoints (below) with SMS delivery via `internal/sms`.
- [ ] Phone recognition + role lookup via a **mock directory** behind an
      interface (a handful of seeded phone → role entries for dev/testing).

**PWA / permissions / updates**

- [ ] PWA install support: manifest (name "Hej Nathejk", short_name, maskable
      icons, `display: standalone`, theme/background, start_url, scope) linked
      from `index.html`, plus iOS `apple-touch-icon` + `apple-mobile-web-app-*`.
- [ ] Service worker registered for installability, shell serving, and push.
- [ ] **Geolocation:** permission-aware location service/store (request, read,
      handle granted/denied/unavailable).
- [ ] **Notifications/Push:** request `Notification` permission, subscribe via
      the SW (`PushManager.subscribe` with VAPID public key), POST subscription
      to the BFF for storage.
- [ ] Version/update detection with a user-visible "reload to update" affordance
      that activates the new version and reloads.

**State**

- [ ] Pinia stores: `session.store` (identity/role, drives nav + guards),
      `app.store` (update flag, nav overflow), and permission/location state
      (`permissions.store` / `location.store`).

### Non-Functional

- **Security:** PINs stored hashed with short expiry; strict rate limiting and
  lockout on request + verify to resist brute force and SMS-bombing; session via
  secure, HttpOnly, SameSite cookie; anti-enumeration responses; client-side role
  gating is UX only — the BFF authorizes every protected endpoint.
- **Mobile-first:** phone viewport first; one-handed use; tap targets ≥ 44px.
- **Standalone feel:** safe-area handling; no horizontal scroll.
- **Secure context:** HTTPS required (Geolocation, Notification, Push, service
  workers, secure cookies). Dev domain `*.local.nathejk.dk` provides TLS.
- **Login UX:** numeric PIN input with `inputmode="numeric"` +
  `autocomplete="one-time-code"` for OS autofill; clear resend/cooldown state.
- **Permission UX:** never request location/notifications on first paint; request
  in context after login with a short rationale.
- **Privacy:** location not persisted server-side by this skeleton; push
  subscriptions stored only to enable delivery; phone numbers handled per GDPR
  (event may involve minors) — document what's collected and why.
- **Accessibility:** login and nav are keyboard/screen-reader reachable; active
  nav state not by color alone.
- **Performance:** views lazy-loaded; shell loads fast on poor on-site networks.
- **i18n-ready:** labels, prompts, and login copy come from a labels layer.

## 7. UX / UI Notes

- **Login screen:** two steps — (1) phone-number entry, (2) PIN entry — with a
  branded "Hej Nathejk" header. Show sent/resend/cooldown state and clear errors.
  PIN field uses one-time-code autofill. No password field ever. The phone step
  **always** advances to the PIN step regardless of recognition; the PIN screen
  carries reassurance copy: *"If we know you, we have sent you an SMS. If you
  don't receive an SMS and you feel we should know you, please reach out."* where
  "reach out" is a `tel:` link to the **nødtelefon**.
- **Branding:** "Hej Nathejk" as app name/title/manifest; theme + background
  colors defined for the manifest and login screen.
- **App shell:** `App.vue` = content + fixed bottom nav; content scrolls; nav
  respects safe-area insets. Only rendered when authenticated.
- **Bottom nav:** horizontally distributed icons + small labels (PrimeIcons or
  `components/icons/` SVGs). The five app roles are **spejder** and **bandit**
  (main) plus **postmandskab**, **guide**, and **samarit**; each has its own
  destination set. Suggested `spejder`/`bandit` set: Maps, Contacts, Rulebook,
  Updates. Service roles (postmandskab / guide / samarit) may see more/different
  entries — enough to exercise the "More" overflow. Active item emphasized.
- **Overflow ("More"):** 5th slot is a burger opening a bottom sheet of the
  remaining allowed destinations.
- **Permission prompts:** soft in-app pre-prompt (why we want location /
  notifications) before the native prompt, shown contextually after login.
- **Update prompt:** lightweight non-blocking notice (Toast or banner above the
  nav): "A new version is available" + "Reload". Style via the Lara preset.
- **New routes/views/components (in `vue/`):**
  - `src/App.vue` — shell layout (auth-gated).
  - `src/views/LoginView.vue` — phone + PIN flow.
  - `src/components/BottomNav.vue`, `MoreMenu.vue`, `UpdatePrompt.vue`,
    `PermissionPrompt.vue`.
  - `src/config/navigation.ts` — destinations + role/visibility rules.
  - `src/views/MapsView.vue`, `ContactsView.vue`, `RulebookView.vue`,
    `UpdatesView.vue`, plus role-gated stubs.
  - `src/stores/session.store.ts`, `app.store.ts`, `permissions.store.ts`
    (or `location.store.ts` / `notifications.store.ts`).
  - `src/helpers/` — `fetchWrapper` (attaches session, handles 401 → login).
  - `src/router/index.ts` — routes + auth & role guards.

## 8. Technical Considerations

- **Frontend (Vue 3 / TS):** Scaffold per `vue3-spa-layout`. On load, call
  `GET /api/me` via `@/helpers` `fetchWrapper`; 401 → show `LoginView`, otherwise
  populate `session.store` and render the shell. A global router guard enforces
  auth and role. `fetchWrapper` treats any 401 as "session lost" → route to
  login. Navigation is data-driven and role-filtered; the ≤5/burger rule runs
  over the filtered set. Use `vite-plugin-pwa` for manifest + SW + the
  `registerSW`/`needRefresh` update hook; the SW handles `push` and
  `notificationclick`. Add deps via the `ui` container.
- **BFF (Go):** Scaffold the single `cmd/api` binary per `go-bff-layout`
  (`main.go` env config + wiring, `routes.go` httprouter + SPA fallback,
  per-resource handlers via `app.WriteJSON`/`ReadJSON`/error helpers,
  `internal/data` + `commands` facades). Module path `nathejk.dk`; dependencies
  flow inward only. SMS is sent through the existing `internal/sms` abstraction
  (cpsms today). Recognition + role lookup goes through a small **user-directory
  interface** whose only implementation for now is a **mock** (seeded phone →
  role map for the five roles — spejder, bandit, postmandskab, guide, samarit);
  a real implementation reading Nathejk records replaces it later without
  changing handlers. In production the binary serves API + built Vue bundle from
  one origin; in dev Vite proxies `/api`.
- **Auth flow / storage:**
  - `POST /api/auth/request-pin` — body `{ phone }`. Normalize the number, look
    it up **in the mock directory**; if recognized, generate a random **6-digit**
    PIN, store it **hashed** with a **10-minute** expiry + attempt counter keyed
    by phone, and send via SMS. Always respond the same way (anti-enumeration).
    Rate-limit per phone + per IP; enforce a **60s resend cooldown**.
  - `POST /api/auth/verify` — body `{ phone, pin }`. Verify hash/expiry/attempts
    (lock out after **5** failed attempts); on success create a session (secure
    HttpOnly SameSite cookie, **≥ 7-day** lifetime) carrying the user id + role,
    delete the PIN, and return identity + role.
  - `POST /api/auth/logout` — clear the session.
  - `GET /api/me` — current identity + role, or 401.
  - PIN records are ephemeral (10-min TTL); store in MariaDB or an
    in-memory/keyed store with TTL. Sessions are a signed/opaque cookie with a
    ≥ 7-day lifetime.
- **Push/Notifications:** Web Push (VAPID). Frontend subscribes and POSTs the
  subscription to the BFF; iOS requires home-screen install. VAPID key pair
  provisioned (public key to client, private key held by BFF as a secret).
- **Version/update mechanism (decided: service-worker driven):**
  `vite-plugin-pwa` precaches the shell and emits a `needRefresh` event when a
  new SW is waiting; the app shows the update prompt and, on confirm, calls
  `skipWaiting` + reloads. No `/api/version` endpoint or polling is needed.
- **API endpoints (all require OpenAPI annotations):**
  - `POST /api/auth/request-pin` — send SMS PIN for a (recognized) phone.
  - `POST /api/auth/verify` — verify PIN, create session, return identity+role.
  - `POST /api/auth/logout` — end session.
  - `GET /api/me` — current identity + role (or 401).
  - `POST /api/push/subscription` — register a push subscription.
  - `GET /api/push/public-key` — VAPID public key (optional; may bake into build).
- **Data / storage:** transient PIN records (phone, PIN hash, expiry, attempts);
  sessions; push subscriptions (endpoint, p256dh, auth, created_at, user id).
  The user directory + role mapping is a **mock** for now (seeded in code); the
  real source is a later dependency. Location is not persisted.
- **Build/versioning:** embed a build id (Vite `define` from git SHA /
  `package.json`); Go binary uses `internal/vcs`.
- **Dependencies & risks:**
  - SMS delivery goes through `internal/sms`. **Assumption:** a working SMS
    sender exists and is available to the BFF; the skeleton codes against the
    `internal/sms` interface and does not build a new provider. (No local mock
    exists yet — see Open Questions for dev testing.)
  - The real user directory is deferred (mocked here), so the recognition
    interface must be clean enough to swap without touching handlers.
  - SMS cost / abuse: rate limiting and lockout are mandatory to prevent
    SMS-bombing and enumeration.
  - Phone-number normalization (country code, formatting) must be consistent
    between the stored directory and user input.
  - `vite-plugin-pwa` (+ Workbox); Web Push library on the Go side later.
  - Stale SW caching; iOS Web Push limits; GDPR/minors for phone + location +
    push consent.
  - Icons/manifest assets need producing (multiple sizes + maskable).

## 9. Success Metrics

- An unauthenticated user sees only the login screen; every other route
  redirects to it.
- A recognized phone number receives an SMS PIN and, on entering it, is signed
  in with the correct role; an unrecognized number cannot gain access and the
  response does not reveal recognition.
- Wrong/expired PINs and excess attempts are handled with clear errors and
  rate-limited/locked out.
- Signing in as different roles produces the correct navigation set; deep-linking
  to a disallowed route redirects to an allowed page.
- Bottom nav shows all allowed destinations when ≤5 and collapses to 4 + burger
  when >5.
- The app installs and launches standalone as "Hej Nathejk" (Lighthouse PWA
  installable passes).
- Location + notification permissions can be requested; on grant, a push
  subscription is stored by the BFF; denials degrade gracefully.
- After a new deploy, an open session detects the new version and one tap on
  "Reload" lands on the new build.

## 10. Rollout / Task Breakdown

Single foundational phase; no feature flag needed. Proposed tasks to create in
`roadmap/tasks/open/`:

- [ ] Task: Scaffold Vue 3 + Vite + Tailwind + PrimeVue (Lara) + Pinia frontend
      under `vue/` per `vue3-spa-layout`.
- [ ] Task: Scaffold the Go BFF (`go/cmd/api` + `app` helpers + `internal/data`
      & `commands` stubs + SPA-fallback file server) per `go-bff-layout`.
- [ ] Task: BFF phone recognition + role lookup via a **mock user directory**
      behind an interface (seed a few phone → role entries; normalize phone
      numbers). Real Nathejk lookup is a later task.
- [ ] Task: BFF `POST /api/auth/request-pin` — generate + hash + store PIN, send
      via `internal/sms`, anti-enumeration response, rate limiting. OpenAPI.
- [ ] Task: BFF `POST /api/auth/verify` — verify PIN, create session, return
      identity+role; attempt limiting/lockout. OpenAPI.
- [ ] Task: BFF `GET /api/me` + `POST /api/auth/logout` (session cookie). OpenAPI.
- [ ] Task: Frontend `LoginView.vue` — phone step + PIN step, resend/cooldown,
      one-time-code autofill, error states.
- [ ] Task: `session.store` + `fetchWrapper` (attach session, 401 → login) +
      global auth/role router guards.
- [ ] Task: Brand the app as "Hej Nathejk" (titles, manifest name, meta).
- [ ] Task: Implement `App.vue` mobile app shell (full-height, safe-area,
      content + fixed bottom nav), rendered only when authenticated.
- [ ] Task: Build data-driven, role-filtered `BottomNav.vue` with the ≤5 /
      >5-burger rule and active-route highlighting.
- [ ] Task: Build `MoreMenu` overflow sheet for destinations beyond the 4th.
- [ ] Task: Add placeholder views — Maps, Contacts, Rulebook, Updates (+ extra
      role-gated stubs) — and register them in the router.
- [ ] Task: Add PWA support via `vite-plugin-pwa` — manifest, icons, service
      worker, standalone display, iOS meta tags.
- [ ] Task: Implement geolocation permission service/store.
- [ ] Task: Implement notification permission + Web Push subscription flow
      (frontend subscribe + POST to BFF).
- [ ] Task: BFF `POST /api/push/subscription` handler + storage. OpenAPI.
- [ ] Task: Provision VAPID key pair; expose public key (bake in or
      `GET /api/push/public-key`). OpenAPI if endpoint added.
- [ ] Task: Add soft `PermissionPrompt.vue` pre-prompts with rationale.
- [ ] Task: Implement version/update detection + `UpdatePrompt.vue` reload flow
      (service-worker `needRefresh`).
- [ ] Task: Embed build/version id at build time (Vite `define`).

## 11. Decisions & Open Questions

**Decided (locked for this skeleton):**

- **Roles:** `spejder` and `bandit` (main) plus `postmandskab`, `guide`,
  `samarit`. The mock directory seeds phone → role entries for these.
- **"Reach out":** a tap-to-call (`tel:`) link to the **nødtelefon** on the
  no-SMS reassurance copy.
- **PIN policy:** 6 digits, 10-minute TTL, 5 verify attempts before lockout,
  60-second resend cooldown, per-phone + per-IP request rate limiting.
- **Session:** ≥ 7-day lifetime via a secure HttpOnly SameSite cookie; survives
  home-screen relaunches.
- **SMS:** assumed available via `internal/sms`; no new provider built.
- **Update mechanism:** service-worker `needRefresh` flow (no `/api/version`
  endpoint needed).
- **Permission timing:** contextual, after login (location on Maps, notifications
  on Updates), each behind a soft pre-prompt.
- **Push scope:** capture + store the subscription only; delivery/fan-out is a
  later PRD.
- **Push identity:** subscriptions are tied to the logged-in user (available via
  session).
- **Header:** the shell has a minimal top app bar that (at least) hosts sign-out.

**Open:**

- **Real source of recognized users + role mapping** — *deferred*. A later task
  wires the real Nathejk records (participants, personnel, klan/patrulje leaders
  …) behind the same recognition interface.
- **A phone number matching more than one person/role** — resolution rule
  (highest-privilege role, ask the user, or deny)?
- **Nødtelefon number** — the actual phone number for the `tel:` link.
- **Per-role destination sets** — the concrete pages, order, and icons for each
  of the five roles (determines exactly when the burger overflow appears).
- **Phone normalization** — default country (assume Danish `+45`?) and accepted
  input formats for matching against the directory.
- **Icon assets & brand colors** — who provides "Hej Nathejk" icons (incl.
  maskable) and theme/background colors for the manifest and login screen.
