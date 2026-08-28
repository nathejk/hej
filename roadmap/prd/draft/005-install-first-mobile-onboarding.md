# PRD 005 — Install-first mobile onboarding (install, confirm, permissions)

**Status:** draft
**Author:** agent session (Zed)
**Created:** 2026-08-25
**Last updated:** 2026-08-25
**Approved:**
**Shipped:**
**Target users:** participant (spejder, bandit), postmandskab, guide, samarit — i.e. every in-event app user

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 1. Summary

Make installation the entry point of Hej Nathejk. On phones and tablets the app
opens an **install wall** instead of the login screen: the user must add the app
to their home screen and reopen it from there before they can sign in. Once
running installed, a short **onboarding flow** takes them through login, a
one-time **confirmation of their profile and contact number**, a **self-portrait**
for night-time identification, then location, then push notifications. That mobile
flow is the whole point of this
PRD. Desktop is deliberately minimal: a single unauthenticated landing page that
explains the app is for your phone and points you at it. What desktop should
*eventually* offer is not decided and is not decided here.

## 2. Problem & Motivation

- **What problem does this solve?** The app's core in-event features only work
  when it is installed. Web Push on iOS/iPadOS is *only* available to
  home-screen web apps (16.4+), so a participant who signs in from Safari can
  never receive event updates, and a browser tab is also far more likely to be
  closed, lose its service worker, or get evicted mid-event. Today nothing stops
  that: a user can sign in in a tab, decline nothing, and silently end up in a
  configuration where updates and position reporting don't work. Support then
  cannot distinguish "not installed" from "broken".
- Permissions today are also asked in the wrong place. `PermissionPrompt.vue`
  exists, but each view decides on its own when to prompt, so whether a user
  ever grants location or notifications is incidental. During an event we need
  both settled *before* the user needs them, not the first time they open the
  map at 02:00 in a forest with no signal.
- **Why now?** PRD 002 (map, position reporting) and push (task set behind
  `notifications.store`) are landing in `doing/`. Both are worthless without an
  installed app and granted permissions, so the gate must exist before they
  ship. Desktop visitors also currently hit a login screen for an app that is
  not meant to be used on desktop.
- **Evidence.** Codebase: `vue/vite.config.ts` already ships a `standalone`
  manifest and `public/push-sw.js`; `notifications.store.ts` `available` getter
  degrades silently; `router/index.ts` sends every unauthenticated visitor
  straight to `/login` regardless of device or display mode. Platform: iOS Web
  Push requires the web app be added to the Home Screen (Safari 16.4 release
  notes) — the constraint is external and non-negotiable.

## 3. Goals

- Every mobile/tablet user who is signed in is running the app installed, with a
  live service worker.
- A user's registered contact number is deliberately acknowledged as reachable
  before they start using the app.
- **Check-in at the start is measurably faster**, because members arrive at the
  counter with their own data already verified instead of having it read back to
  them one field at a time.
- Location and push permission decisions happen once, up front, in a
  predictable, explained order — never as an unexplained native dialog.
- A desktop visitor is not shown a login form for an app they cannot use — they
  get a short, honest explanation and a way to reach the app on their phone.
  Nothing more is promised.
- The gate is legible: a user who is blocked always sees *why* and *what to do
  next*, with platform-correct instructions.
- The gate is safe: no user can be permanently locked out by a false negative in
  device or install detection.

## 4. Non-Goals

- Native app store distribution (iOS/Android wrappers). Out of scope.
- Push **delivery** / fan-out (`/api/push/send`, message authoring). Already a
  separate later PRD; this PRD only settles subscription at onboarding time.
- Reworking the login mechanism itself (phone + SMS PIN stays as is, see
  `session.store.requestPin`/`verify`).
- **Any real desktop experience.** Desktop scope is undecided; this PRD ships
  only a static landing page so desktop visitors are not dumped at a login
  screen. No read-only content browsing, no desktop login, no organizer console.
  Whatever desktop becomes is a separate PRD, and the landing page is built so it
  can be replaced wholesale without touching the mobile flow.
- Building offline content sync. This PRD **hosts** PRD 009's first-sync step as
  an optional onboarding step (step 6); the mechanism, budget and readiness UI are
  009's. *(Revised 2026-08-25 — previously this non-goal excluded offline sync
  outright, contradicting 009.)*
- A full profile / settings surface for revisiting permissions — that belongs to
  PRD 003 (profile page), which this PRD depends on for the "I declined, let me
  fix it later" path.

## 5. User Stories & Scenarios

- As a **participant on Android/Chrome**, I want the app to offer to install
  itself in one tap so that I get the real app without hunting through menus.
- As a **participant on iPhone**, I want clear instructions to add the app to my
  home screen so that I can receive event notifications at all.
- As a **participant logging in for the first time**, I want to see the details
  Nathejk holds for me and confirm my parent's number is reachable, so that an
  adult can actually be reached if I get hurt or need to go home early.
- As an **organizer on the check-in counter**, I want to see at a glance that a
  member already verified their own data, so that I can wave them through instead
  of reading fields back to them.
- As an **organizer**, I want stale guardian numbers surfaced before the event
  rather than discovered at 02:00 when I need to arrange a pickup.
- As a **participant**, I want to take a photo of myself during setup so that the
  people I meet on the route at night can tell who they are talking to.
- As a **samarit or guide at 03:00**, I want to see the face of the member I am
  looking for, so that I can identify them in the dark without asking a tired
  teenager to spell their name.
- As a **participant** who just installed, I want to be told why the app wants
  my location and notifications before it asks, so that I say yes deliberately.
- As a **parent or curious visitor on a laptop**, I want to understand what this
  site is and that the app lives on my phone, so that I am not stuck at a login
  form.

### Happy path — Android / Chrome phone

**Install wall (W1–W4):**

W1. User opens `https://hej.nathejk.dk` in Chrome.
W2. Device is mobile, display-mode is `browser` → router sends them to `/install`
    (the install wall). No login form is reachable.
W3. The wall explains the app in one screen and shows **"Installér app"**, wired
    to the captured `beforeinstallprompt` event.
W4. User accepts; Chrome installs. The wall switches to "Åbn appen" / "Du kan nu
    lukke denne fane" state.

**Onboarding (steps 1–6):** the user opens the app from the home screen;
`display-mode: standalone` passes the install gate and the router lands on
`/welcome`.

1. **Login**: phone number → SMS PIN → `session.verify`.
2. **Profile confirmation** (first login only, **spejder only**): the registered
   details are shown, with the **parent/guardian emergency contact number** masked
   to its last two digits (`11 22 33 **`). They type the two missing digits and
   tick *"Dette nummer kan kontaktes i løbet af Nathejk"*. Skipped for users who
   have already started the event (§11) — note that skipping **this** step does not
   skip step 3.
3. **Portrait**: explanation, then the camera. The photo is for identifying people
   during the race — much of which happens at night, when faces are hard to see.
   Captured via `getUserMedia` using PRD 003's capture component, confirmed by the
   user before upload.
4. **Location**: explanation screen, then the native prompt via `location.store`.
   Granted or denied, the flow continues.
5. **Notifications**: explanation screen, then `notifications.store.enable()`
   (permission + push subscription + POST to BFF).
6. **Offline preparation**: PRD 009's first sync — portraits, map tiles, the
   rulebook and the rest — with one combined progress view and a size estimate.
   Skippable, and best done here because the user is usually on wifi with the app
   open. *(Added 2026-08-25: PRD 009 placed a first-sync step inside this flow while
   this PRD's sequence did not contain one.)*

A **vehicle step** also belongs in this flow, for bandit/gøgler/crew only — owned by
PRD 010, which specifies it. It is not numbered here because its position depends on
PRD 010's approval; when it lands it sits after the portrait and before the
permissions, since it is another "about you" question rather than a device prompt.

Onboarding then marks itself complete and redirects to `/maps`.

### Happy path — iPhone / Safari

Identical except **W3–W4**: iOS fires no `beforeinstallprompt`, so the wall shows
illustrated **Share → Tilføj til hjemmeskærm** instructions instead of a button,
and there is no install-accepted event to switch state on. Everything from opening
the app on the home screen onwards is the same.

### Desktop

1. User opens the site in a desktop browser.
2. Device is not mobile → they land on a static landing page: brand header, one
   short paragraph saying Hej Nathejk is an in-event app for your phone, and a
   way to get there (see Open Questions).
3. No login form, no bottom nav, no map, no app content.

### Edge cases

- **Already installed, opened in a browser tab anyway.** We cannot reliably know
  the app is installed from a tab (`getInstalledRelatedApps` is Chromium-only,
  and only for related *native* apps). So the wall always shows, but always
  offers "Jeg har allerede installeret appen — åbn den" plus the manual
  fallback below.
- **Detection false negative** (desktop touchscreen classified as tablet,
  standalone not detected, PWA-hostile browser, in-app webview like the Facebook
  browser where installation is impossible). The wall must carry a
  low-prominence **"Fortsæt i browseren"** escape hatch that sets a persisted
  override and lets the user proceed to normal login. Non-installed users are a
  degraded experience, not a forbidden one — being unable to help someone at
  02:00 is worse than an unsubscribed push. This is **separate from** the
  "desktop version" link below; see §11.
- **User already started the event.** Profile confirmation is skipped — starting
  implies the data was already verified. **The portrait step still runs**, and so do
  the location and notification steps: those are per-device facts and say nothing about
  the profile, and a photo is missing whether or not the guardian number was confirmed
  (clarified 2026-08-25). A member who started the event without a portrait is exactly
  the person personnel will fail to identify at 03:00, so skipping the nudge for them
  would remove it from the cohort that needs it most.
- **The user is not a spejder.** Only spejder have a guardian number on file
  (confirmed by the shared-go survey — `PhoneParent` exists on `spejder` and on no
  other population). Bandits, gøgler and crew therefore **skip the guardian
  confirmation entirely**; they may still be shown their own details to check. The
  step must not render an empty guardian field as though data were missing.
- **The member does not know the guardian number.** Expected and not rare: young
  scouts, a guardian who recently changed number, or two households with
  different numbers on file. This is **not** a failure state and must not be
  worded as one — offer "jeg kender ikke nummeret" alongside "nummeret er
  forkert", let them into the app, and flag the record either way. The signal
  "this member could not confirm the number" is valuable to organizers even when
  the number turns out to be correct.
- **Profile confirmation fails** — wrong digits, or the user says a detail is
  wrong. The step surfaces the out-of-band correction channel, **flags the record
  for follow-up**, and **lets them continue into the app**. Blocking a participant
  out of a safety app over a stale guardian number is the worse failure.
- **Repeat logins.** Confirmation is per-user server-side state, so it does not
  reappear after a reinstall, a new device, or clearing site data.
- **Session already valid but onboarding never completed** (e.g. user installed,
  logged in, then killed the app during the location step). Onboarding resumes at
  the first unsettled step.
- **Camera denied or unavailable.** The portrait step degrades: offer the
  `<input type="file" accept="image/*" capture="user">` fallback, and if that also
  fails, continue without a photo. Never block.
- **Permission already `denied` at OS level.** Skip the native call, show a
  short "you can enable this later in settings" note, continue. Do not block.
- **User declines both permissions.** They reach the app. The map shows the
  "location off" state (PRD 002) and the profile page (PRD 003) offers to
  re-enable.
- **iOS standalone + no push support** (iOS < 16.4). Outside the browser
  baseline in `.rules`; the notification step reports `unavailable` and is
  skipped rather than shown as a failure.
- **Logout.** Returns to onboarding step 1 (login), not to the install wall —
  the app is still installed. Profile confirmation does not run again.

## 6. Requirements

### Functional

- [ ] Classify the visiting device as `mobile` (phone/tablet) or `desktop`, using
      coarse-pointer + touch capability and, where available,
      `navigator.userAgentData.mobile`; never viewport width alone.
- [ ] Detect installed/standalone via `matchMedia('(display-mode: standalone)')`
      (plus `minimal-ui`/`fullscreen`) OR the iOS-only `navigator.standalone`.
- [ ] Mobile + not standalone → all app routes redirect to `/install`.
- [ ] `/install` shows a one-tap install button when `beforeinstallprompt` was
      captured, and platform-specific manual instructions otherwise (iOS Safari,
      Android non-Chrome, in-app webview).
- [ ] `/install` includes a persisted **"Fortsæt i browseren"** override that
      unblocks login for that browser.
- [ ] `/install` includes a small, low-prominence link to the **desktop version**
      (`/desktop`) for visitors who do not want to install. Distinct from the
      override above: it leads out of the app flow rather than into it (§11).
- [ ] Desktop → `/desktop`, a static unauthenticated landing page. Desktop never
      reaches `/login`, `/install`, or `/welcome`, and no role may log in there
      (§11). The page renders no participant-facing app content, so it carries no
      data-exposure decision.
- [ ] Mobile + standalone → `/welcome`, a linear onboarding flow with steps
      `login` → `confirm profile` → `portrait` → `location` → `notifications`,
      resumable, each permission step preceded by an in-app explanation
      (reuse/extend `PermissionPrompt.vue`) before any native dialog.
- [ ] The **profile confirmation** step shows the user's registered details and
      the **parent/guardian emergency contact number** masked to its last two
      digits (`11 22 33 **`), requires those two digits to be typed, and requires
      a checkbox *"Dette nummer kan kontaktes i løbet af Nathejk"*. Both are
      needed to advance.
- [ ] The step explains **why** the number matters — emergencies *and* arranging
      pickup if the member resigns mid-event.
- [ ] Profile confirmation applies to **spejder only** — they are the only
      population with a guardian number on file. Other roles skip it.
- [ ] Profile confirmation is **skipped** when the user has already started the
      event, or has confirmed previously. Skipping it must **not** skip the portrait
      step — the two are independent (§11).
- [ ] Confirmation state is **server-side, per user** — not `localStorage` — so it
      survives reinstalls, new devices and cleared site data.
- [ ] The step offers non-punitive "nummeret er forkert" **and** "jeg kender ikke
      nummeret" paths that surface the correction channel and still let the user
      into the app.
- [ ] Records that were not confirmed are **flagged for organizer follow-up**,
      distinguishing "reported wrong" from "could not confirm".
- [ ] The **portrait** step captures a self-portrait with the front camera,
      explains that it is used to identify people during the race (largely at
      night), lets the user retake before uploading, and is **skippable** — with
      the profile page (PRD 003) as the place to add one later.
- [ ] The portrait step runs for **every** user who has none, including those whose
      profile confirmation was skipped because they had already started the event.
      Verification status and portrait status are unrelated facts.
- [ ] A user with no portrait is **nudged again after onboarding**, not asked once
      and forgotten. Onboarding is a single moment and the step is skippable, so a
      one-shot prompt means the members most likely to decline are exactly the ones
      who stay unidentifiable. The nudge must be dismissible per session and must
      stop permanently once a portrait exists — a prompt that cannot be silenced
      trains people to ignore it.
- [ ] Portrait capture reuses PRD 003's capture component and upload endpoint
      rather than implementing a second one.
- [ ] Details are **read-only** at confirmation time; in-app editing is out of
      scope here (§12).
- [ ] Onboarding never hard-blocks on a permission decline or a failed profile
      confirmation; only **login** is mandatory.
- [ ] Per-device state (permissions) persists in `localStorage`; per-user state
      (profile confirmation) comes from the BFF. Returning users go straight to
      `/maps`.
- [ ] Once onboarding is complete the app behaves exactly as today.
- [ ] A dev/QA override (query param or `localStorage` flag, gated to non-prod)
      bypasses the install and device gates.

### Non-Functional

- **Reach.** Baseline stays iOS/iPadOS Safari 16.4+ / Chrome 111+ per `.rules`.
- **No lockout.** Every gate has a user-reachable escape hatch. This is a safety
  app.
- **Privacy.** The location explanation must state plainly what is shared, with
  whom, and for how long, before the native prompt. Do not request geolocation
  or notification permission before an explanation screen is shown.
- **Language.** All copy in Danish, matching the existing views.
- **Accessibility.** Install instructions must be readable as text, not only as
  an image; the flow must be operable without gestures.
- **Performance.** Device/install classification is synchronous and
  dependency-free so the gate does not add a redirect flash on cold start.
- **Offline.** `/install` and `/welcome` are part of the precached shell.

## 7. UX / UI Notes

New routes:

| Route | Name | Meta | Purpose |
|---|---|---|---|
| `/install` | `install` | `public` | Mobile install wall |
| `/welcome` | `welcome` | `public` | Post-install onboarding (owns login) |
| `/desktop` | `desktop` | `public`, `desktop` | Desktop landing page (placeholder scope) |

**`/login` is removed as a standalone route.** The credential step lives *inside*
`/welcome`: `LoginView.vue` is refactored into `WelcomeStepLogin.vue` so the SMS-PIN
logic is not duplicated. Two shipped call sites must move with it (they currently
hard-code `{ name: 'login' }`):

- `router/index.ts` — the guard's unauthenticated fallback becomes `{ name: 'welcome' }`
  (or `{ name: 'install' }` when the install gate has not passed).
- `App.vue` — `signOut()` redirects to `{ name: 'welcome' }`, landing on step 1.
  There is **one** sign-out action with **one** destination; PRD 003 moves the
  control into a top-bar user menu (`UserMenu.vue`) but does not change where it
  goes.

*(Made explicit 2026-08-25: this previously said `/login` was "retained … inside
`/welcome`", which left it ambiguous whether the route still existed and what those
two call sites should do. PRD 004's route inventory should drop `/login` too.)*

New/changed components (all shadcn-vue where a primitive exists — `Card`,
`Button`, `Progress` or a stepper built from `Separator`, `Alert` for the denied
states; hand-rolled only for the platform install illustration):

- `views/InstallView.vue` — install wall.
- `views/WelcomeView.vue` — onboarding shell; renders the current step and
  progress.
- `views/DesktopView.vue` — static landing page. Intentionally thin and
  self-contained: it shares no components with the mobile app so replacing it
  later cannot regress the app.
- `components/onboarding/InstallInstructions.vue` — platform-specific steps.
- `components/onboarding/WelcomeStepLogin.vue`, `…StepConfirmProfile.vue`,
  `…StepPortrait.vue`, `…StepLocation.vue`, `…StepNotifications.vue`,
  `…StepOfflineSync.vue` (step 6, driven by PRD 009's `offline.store`).
- `…StepPortrait.vue` wraps PRD 003's capture component; it must not fork it. The
  face guide, retake affordance and explicit confirm-before-upload are requirements
  **on that component**, specified in PRD 003 §7 — people will not accept a photo
  they did not get to approve.
- The confirm-profile step uses shadcn-vue `Card`, `Input` (a 2-digit numeric
  input with `inputmode="numeric"`), `Checkbox` and `Label`; the masked number is
  rendered as text, never as an editable field.
- `components/PermissionPrompt.vue` — **this PRD owns the component's API**, adding
  a `variant` prop so it serves both the existing compact card and a full-screen
  onboarding explanation. PRD 002 (map repair affordance) and PRD 003 (status rows)
  are consumers of that API, not co-owners. Today the props are
  `{ title, message, cta, icon? }` with `accept`/`dismiss` emits.
- Primitives needed that are **not yet generated** in `vue/src/components/ui/`:
  `progress`, `alert`, `checkbox`. Generate them per PRD 004's on-demand rule.

Shell behaviour (`App.vue`): the top bar **and** `BottomNav` are hidden on
`/install`, `/welcome` and `/desktop`. `showShell` becomes
`isAuthenticated && onboardingComplete && !route.meta.public` — stated as a full
expression because today it is `isAuthenticated && route.name !== 'login'`, and the
`login` term disappears with the route. Note `fullBleed` is **not** used on the
onboarding routes: it only suppresses the header *inside* `showShell`, so it would
be a no-op there.

All page-level headlines use `font-nathejk` per `.rules`; icons are Lucide
(`Download`, `Share`, `PlusSquare`, `MapPin`, `Bell`, `Monitor`).

## 8. Technical Considerations

- **Frontend (Vue 3 / TS):**
  - New `helpers/platform.ts` — `isMobileDevice()`, `isStandalone()`,
    `installPlatform()` returning `'chromium' | 'ios-safari' | 'other' |
    'webview'`. Pure functions, unit-testable with an injectable
    `navigator`/`matchMedia`.
  - New `stores/install.store.ts` — captures `beforeinstallprompt` (registered in
    `main.ts` before mount so the event is not missed), exposes
    `canPrompt`/`promptInstall()`, `installed` (from the `appinstalled` event),
    and the `continueInBrowser` override persisted in `localStorage`.
  - New `stores/onboarding.store.ts` — step machine + per-device completion flag
    in `localStorage`; derives step state from `session.store`,
    `location.store.permission` and `notifications.store.permission` so it is
    self-healing rather than a blindly persisted cursor.
  - `router/index.ts` — extend the global guard: device/install/onboarding gates
    run **before** the existing auth check; add `desktop?: boolean` to
    `RouteMeta`. Guard order: dev override → device class → standalone →
    onboarding complete → auth → roles. The device gate is session-independent by
    design (§11), so it can short-circuit before `session.ensureReady()` and
    spare desktop visitors a pointless `/api/me` round-trip.
  - `helpers/pwa.ts` — unchanged; `UpdatePrompt` must not overlay the wall.
- **BFF (Go):** this PRD **does** require BFF work, for the profile-confirmation
  step only — the install and device gates remain pure client state, and the BFF
  continues to authorize every endpoint independently (the install gate is UX,
  never security).
  - Per `go-bff-layout`, extend `go/cmd/api/profile.go` (introduced by PRD 003)
    rather than adding a parallel handler file. Note the write path needs the
    persistence from **PRD 008** — `commands.Commands` is an empty struct today.
  - Confirmation state is **derived, not just stored**: the response must report
    `confirmation_required` computed from "has verified" **OR** "has started the
    event" (`types.MemberStatusRacing` onwards). The client must not reimplement that
    rule. The stored field is `verified_at` — one name across PRDs 003, 005 and 006.
  - **The verification flag is published as a domain event** (decided 2026-08-25;
    see PRD 008 §8 — nothing writes directly to the database). `hej` is therefore a
    publisher, and `hq` learns about verification by consuming the same log. This is
    what makes the check-in goal reachable without one service calling another's
    API, which is forbidden.
  - **Cross-repo.** Verification is a fact about a *member*, and members are not
    owned by this repo. `types.MemberStatus` and the member read model live in
    `github.com/nathejk/shared-go` (`types/member.go`,
    `tables/spejder/`); the lifecycle projection and the organizer-facing views
    live in `hq`. `hej`'s Go tree does not import shared-go at all today — its
    `internal/users.Directory` returns `{ID, Role, PatrolID, PatrolName}` from a
    mock. So recording verification means:
    1. shared-go: add the field to the member type + querier, and a
       `member.verified` style message.
    2. `hej`: depend on shared-go, publish the event on confirm, and project
       `verified_at` onto **PRD 006's `person` read model in `hej`** — not onto a
       shared-go table. The shared-go member type gains the field only when PRD 006's
       projection is lifted there. *(Clarified 2026-08-25: this and PRD 006 named two
       different homes for the same column, which is a data-migration risk.)*
    3. `hq`: surface it on the check-in view — without which the check-in speed
       goal is not realised, since the counter cannot see the flag.
    Remember the two-repo release loop from `go-bff-layout`: shared-go must be
    committed, pushed and version-bumped before a `GOWORK=off` build sees it.
  - **Directory dependency.** Everything the confirmation step reads comes from
    the member directory, which is **mocked today**. PRD 006 replaces it. The
    shared-go survey (2026-08-25) also established two facts that constrain this
    PRD directly:
    - `PhoneParent` exists **only** on `spejder` — hence the spejder-only rule in
      §6.
    - **No photo storage exists anywhere** in shared-go, `hq` or `tilmelding`, and
      **no per-person verification flag exists**. Both are entirely new write-side
      work, not fields to be read.
  - See §11 for **why this is a member field rather than a new `MemberStatus`**.
  - "Has started the event" needs a source. This is the one unknown on the Go
    side — it likely already exists as event/patrol state (PRD 002 touches the
    directory for patrol identity); coordinate rather than adding a second
    notion of "started".
- **API endpoints (OpenAPI annotations mandatory, per `.rules` and the style in
  `go/cmd/api/auth.go`):**
  - `GET /api/me/profile` — **owned by PRD 003**; this PRD adds
    `confirmation_required` and `verified_at`, plus the contact number in masked
    form. `200` / `401`.
  - `POST /api/me/profile/confirm` — body carries the two digits and the
    acknowledgement flag. `204` / `400` (wrong digits) / `401` / `409` (already
    confirmed). Wrong digits must be rate-limited like the PIN endpoint.
  - `POST /api/me/profile/report-incorrect` — flags the record for organizer
    follow-up, with a reason distinguishing "wrong" from "unknown to me". `204` /
    `401`. **Required**, not optional: a guardian number nobody can confirm is an
    operational problem that must reach a human before the event, not just a dead
    end in the UI.
  - `PUT /api/me/photo` — **owned by PRD 003**; consumed unchanged here.
  - **The full contact number must never be sent to the client for this step.**
    Send it masked; verify the digits server-side. Otherwise the check is
    theatre — the answer would be in the network response.
- **Data / storage:**
  - Client: `localStorage` keys only, namespaced `hej.install.*` /
    `hej.onboarding.*`. Per-user confirmation is **not** stored client-side.
  - Server: a `verified_at` timestamp on the member, plus the acknowledged
    contact number so a later number change can invalidate the verification.
    Modelled as its own field, **not** as a `MemberStatus` value (§11), and
    materialised as a **projection of the verification event** rather than a direct
    write (PRD 008).
- **Dependencies & risks:**
  - ~~**Risk: portrait consent is unresolved and blocking.**~~ **Resolved 2026-08-28
    (task 102, recorded in PRD 003 §6):** the parental consent is already held from
    sign-up, and the portrait is an in-race safety feature, purged after the event.
    The onboarding portrait step therefore needs **no consent gate of its own** — it
    should explain the purpose (identification during the race, largely at night)
    and take the photo.
  - **Risk: portraits are useless without a viewing surface** — now specified as
    PRD 007. Capturing before that ships means asking every member for a photo
    that nothing consumes, so sequence accordingly.
  - **Risk: night-time identification implies offline availability.** Personnel
    looking up a face at 03:00 in a forest may have no signal. PRD 007 treats this
    as its central requirement; it also constrains how portraits are sized here,
    since the thumbnail served for identification should be generated at upload.
  - **Risk: the check-in benefit depends on `hq`, not on this repo.** The app can
    collect verifications perfectly and check-in gets no faster until the counter
    view shows the flag and the staff procedure changes to trust it. Treat the
    `hq` change and the procedural change as part of the rollout, not as
    follow-up.
  - **Risk: verification is only as useful as it is trusted.** If staff keep
    reading fields back anyway, the step costs members time and saves nobody any.
    Worth agreeing the counter procedure with the organizers before building.
  - **Risk: this PRD now depends on PRD 003.** The confirmation step needs
    `GET /api/me/profile` and the extended user directory. PRD 003 is still in
    `draft/` and unapproved, so either it lands first or the confirmation step
    ships in a second phase behind the runtime flag. The install + permission
    flow must not be held hostage to it.
  - **Risk: "has started the event" may not exist** as a queryable fact yet. If
    not, the skip rule cannot be implemented as specified and every returning
    user gets confirmed once instead — acceptable, but confirm before building.
  - No new runtime dependencies. If the desktop page ends up wanting a QR code,
    generate it as a static build-time asset rather than adding a library.
  - **Risk: detection is heuristic.** iPadOS reports itself as macOS Safari;
    desktops with touchscreens exist; `display-mode` can be unreliable in
    embedded webviews. Mitigated by the escape hatch, not by better sniffing.
  - **Risk: `beforeinstallprompt` timing.** It fires once, early; it must be
    captured before the Vue app mounts or the one-tap install silently degrades
    to manual instructions.
  - **Risk: gating login behind install increases drop-off** at the worst
    possible moment (a participant needing help). The escape hatch is the
    mitigation and must not be hidden so well that support cannot talk a user
    through it.
  - **Interaction with PRD 002/003:** the map's location handling and the
    profile page's permission controls must consume the same stores, not
    re-implement permission logic.

## 9. Success Metrics

- ≥ 95% of authenticated sessions during an event originate from a standalone
  display mode (i.e. the wall works and the escape hatch is rare).
- ≥ 85% of onboarded users have an active push subscription registered with the
  BFF.
- ≥ 80% of onboarded users have granted location.
- ≥ 90% of first-time users complete the guardian-number confirmation without
  using the "wrong"/"unknown" paths.
- **Median check-in time per member falls measurably** versus the previous event,
  and verified members are visibly faster through the counter than unverified
  ones. Needs a baseline measured *before* rollout, or the claim is unfalsifiable.
- ≥ 70% of members verify before arriving at check-in.
- Every unconfirmed guardian number is surfaced to organizers before event start,
  with zero cases of a pickup or emergency call failing on a number the app had
  already flagged.
- < 2% of authenticated sessions use the "Fortsæt i browseren" override.
- No support reports of users unable to reach the app on a supported device.

## 10. Rollout / Task Breakdown

Sequencing: platform detection and stores first (they are what everything else
consumes), then the wall, then onboarding, then the router gate wiring. The
desktop landing page is the smallest and least certain piece — build it last, and
keep it a stub rather than delaying the mobile flow on an undecided scope. Ship
behind a runtime flag in `config/runtime.ts` so the gate can be disabled without a
rollback if it misfires during an event.

Proposed tasks for `roadmap/tasks/open/`:

- [ ] Task: platform detection helper (`helpers/platform.ts`) + unit tests
- [ ] Task: install store — capture `beforeinstallprompt`, `promptInstall`, browser override
- [ ] Task: onboarding store — resumable step machine derived from permission state
- [ ] Task: shared-go — `verified_at` + acknowledged number on the member type & querier
- [ ] Task: shared-go — `member.verified` event message; bump version in `hej` and `hq`
- [ ] Task: BFF — publish the verification event via `commands.Commands` (PRD 008)
- [ ] Task: BFF — project `verified_at` + `confirmation_required` onto the member read model
- [ ] Task: BFF — `POST /api/me/profile/confirm` with server-side digit check and rate limiting
- [ ] Task: WelcomeStepPortrait — camera capture, retake, skip, reuse PRD 003 upload
- [ ] Task: portrait nudge — re-prompt after onboarding while no portrait exists
      (dismissible per session, silenced permanently once one is uploaded)
- [ ] Task: hq — surface verification on the check-in view. **Tracked on `hq`'s
      board, not this one**; listed here as a rollout dependency, since the check-in
      goal is not realised without it.
- [ ] Task: measure baseline check-in time before rollout
- [ ] Task: WelcomeStepConfirmProfile — masked number, 2-digit input, acknowledgement checkbox
- [ ] Task: "nummeret er forkert" / "jeg kender ikke nummeret" paths + follow-up flag
- [ ] Task: InstallView — one-tap install (Chromium) with platform instructions fallback
- [ ] Task: InstallView — "fortsæt til desktop-version" link + browser override
- [ ] Task: InstallInstructions component — iOS Safari / Android / webview variants
- [ ] Task: refactor LoginView into an onboarding login step
- [ ] Task: WelcomeView shell + location and notification explanation steps
- [ ] Task: WelcomeStepOfflineSync — host PRD 009's first sync as step 6 (skippable)
- [ ] Task: remove the `/login` route; repoint the router guard fallback and
      `App.vue`'s `signOut()` at `welcome`
- [ ] Task: generate the `progress`, `alert` and `checkbox` shadcn-vue primitives
- [ ] Task: PermissionPrompt full-screen variant
- [ ] Task: DesktopView — static landing page for non-mobile visitors
- [ ] Task: router guard — device / standalone / onboarding gates + `desktop` route meta
- [ ] Task: App.vue shell — hide chrome on install/welcome/desktop routes
- [ ] Task: runtime flag + dev/QA gate bypass
- [ ] Task: manual test matrix — iOS Safari, Android Chrome, Android Firefox, desktop, in-app webview

## 11. Decisions

Answered questions are recorded here rather than deleted, so the reasoning
survives.

- **2026-08-25 — Verification is published as a domain event.** Not written to a
  database by the app, per the architecture rule that nothing writes directly to SQL
  and that services may not call each other's APIs. This is also what makes the
  check-in goal achievable: `hq` sees verification by consuming the same log, with
  no coupling between the two services. See PRD 008 §8.
- **2026-08-25 — The portrait is an onboarding step, and its purpose is
  night-time identification.** Much of the race runs at night, when faces are hard
  to see, so personnel need to know who they are talking to. This settles PRD 003's
  open "photo purpose" question — the photo is **operational, not decorative** —
  and that has two consequences neither PRD had accounted for:
  1. **Personnel must be able to view other members' portraits.** PRD 003
     explicitly non-goals that ("Viewing other people's profiles or photos … a
     separate feature with its own consent and access story"). Capturing portraits
     without that viewing surface delivers none of the stated value. **That
     surface is now PRD 007** (offline portrait identification), which also settles
     that spejdere and banditter must not see each other.
  2. **Consent and retention become blocking, not follow-up.** ~~These are
     photographs of identifiable minors…~~ **Answered 2026-08-28 (task 102):**
     consent is already held from sign-up, the basis is safety/identification, and
     portraits are purged after the event. The step needs no in-app consent gate.
     The consent text question collapses with it — what remains is *explaining the
     purpose*, not obtaining permission. Whether participants may see each other's
     portraits is still PRD 007's matrix.
  The step is **skippable** — a participant who declines still gets into the app,
  with the profile page as the place to add one later.
- **2026-08-25 — Faster check-in is an explicit goal of this step.** The value of
  pre-arrival verification is not only data quality: it is that a member who has
  verified can be waved through the counter instead of having every field read
  back to them. This means the flag must be **visible to check-in staff in `hq`**,
  or the goal is not realised — see §8, and note that this puts work outside this
  repo.
- **2026-08-25 — Verification is a member *field*, not a new `MemberStatus`.**
  Recorded as `verified_at` (plus the acknowledged number), orthogonal to the
  lifecycle. `MemberStatusVerified = "verified"` was proposed and rejected for
  three reasons, in increasing order of severity:
  1. **The type forbids it.** `types.MemberStatus` documents itself as answering
     one question — "what is true of this member right now?" — with *exclusive*
     states, and says outright: "Anything that is several facts at once … belongs
     in its own field, not here." A member is `seated` **and** verified; those are
     two facts.
  2. **It would break seat accounting.** `MemberStatusSeated` is documented as
     the paid-seat count: "the count of seated members is what the team has
     actually paid for." If verifying moves a member `seated → verified`, every
     member who verifies silently drops out of that count — a billing-adjacent
     regression, against the model PRD 001 established.
  3. **A status is overwritten, and this fact must survive.** Statuses advance:
     the moment check-in flips the member to `racing`, a `verified` status is
     gone. That destroys exactly the measurement that justifies the feature
     ("how many members verified before arriving?"), makes it impossible to
     invalidate verification when a number changes, and leaves no audit of who
     acknowledged what. A timestamp survives every later transition.
  The pull towards a status is real and worth naming: `hq` already renders status
  columns and filters, so a new status would surface at the counter "for free".
  That convenience is not worth the three costs above — and the cost of a status
  is not free either: `Valid()`, `allMemberStatuses` ("anything added to
  types.MemberStatus must be added here too"), the legacy-value mapping, the
  `spejderstatus` projection, `GetByStatuses`, the SOS timeline and the persisted
  strings ("changing one is a data migration, not a rename") all move. Adding a
  field to the member read model and one column to the check-in view is smaller.
- **2026-08-25 — One guardian contact per member.** Members currently have a
  single contact on file, so the step confirms exactly one number and needs no
  primary/secondary selection or multi-number sequence. Worth noting this is a
  property of today's data, not a designed constraint: if a second contact is
  ever added, this step needs revisiting — and "which number did the member
  actually acknowledge?" becomes ambiguous unless the acknowledged number is
  recorded (§8 already provides for storing it).
- **2026-08-25 — The number confirmed is the parent/guardian emergency contact.**
  Not the member's own number: they already proved control of that by receiving
  the SMS PIN. Its purpose is operational as well as emergency — Nathejk must be
  able to reach an adult if something happens, *and* if the member resigns
  mid-event and a pickup has to be arranged. The step's copy should reflect both
  reasons, since "in case of emergency" alone understates how routinely the number
  gets used and invites a shrug.
- **2026-08-25 — Profile confirmation belongs here, not in PRD 003.** PRD 003
  owns the profile *page* and its read model (`GET /api/me/profile`); this PRD
  owns the first-run *gate*, because the step machine, ordering and completion
  state already live here. Building a second blocking flow inside a passive page
  was the alternative and was rejected.
- **2026-08-25 — The number is masked as `11 22 33 **`.** The user supplies the
  two digits that are *not* on screen, so this is a genuine recall check rather
  than a copying exercise: a user who cannot complete it has discovered that the
  number on file is not one they know. Paired with an explicit acknowledgement
  checkbox (*"Dette nummer kan kontaktes i løbet af Nathejk"*), the step captures
  both attention and consent. The digits must be verified **server-side** and the
  full number never sent to the client, or the answer is in the payload.
- **2026-08-25 — Confirmation is skipped for users who have started the event.**
  Starting implies the data was already verified. Permissions are still requested,
  since they are per-device facts and say nothing about the profile.
- **2026-08-25 — Skipping confirmation does NOT skip the portrait.** The two are
  independent facts, and conflating them would have removed the photo nudge from the
  cohort that needs it most: a member who has already started the event and has no
  portrait is precisely the person personnel will fail to identify in the dark.
  Verification says something about the *guardian number*; it says nothing about
  whether there is a face on file.

  It follows that the nudge cannot be a one-shot onboarding step either. The step is
  skippable by design (only login is mandatory), so asking once means the members most
  likely to decline are the ones who stay unidentifiable all event. A user with no
  portrait is therefore re-prompted after onboarding — dismissible per session, and
  silenced permanently the moment a portrait exists, because a prompt that cannot be
  quieted is one people learn to ignore.
- **2026-08-25 — Confirmation state is per-user and server-side.** Per-device
  `localStorage` would re-prompt a participant after a reinstall or a new phone,
  potentially mid-event. This is why the PRD acquired BFF scope (§8).
- **2026-08-25 — The install wall links to the desktop version.** A small,
  low-prominence "fortsæt til desktop-version" link for people who do not want to
  install. Note this is **not** the lockout escape hatch: it leads to the
  `/desktop` stub, so it cannot rescue a user whose device wrongly fails the
  install check. Both affordances are therefore kept, and they must be worded
  distinctly. If `/desktop` ever becomes a real content surface, revisit whether
  they should merge.
- **2026-08-25 — No desktop login, for any role.** Organizers on a laptop were
  considered and rejected for this PRD. The device gate is therefore a plain
  redirect, not role-aware: it runs before authentication and needs no knowledge
  of the session, which keeps the guard order in `router/index.ts` simple. A
  consequence worth naming: because the gate precedes auth, there is no
  role-based bypass to fall back on if an organizer does need laptop access
  mid-event — the dev/QA override (§6) would be the only route in, and it is not
  intended for that. If organizer desktop access is ever wanted it is a new PRD
  that revisits this gate, not a flag.

## 12. Open Questions

1. **What is desktop actually for?** Undecided, and out of scope here — but it
   needs an owner before anyone invests in `/desktop`. Candidates: a read-only
   public content surface (rulebook/FAQ/schedule), or permanently just a
   signpost. An organizer console is off the table (see §11). Until that is
   answered the page stays a stub. Note that any content surface reopens a
   data-exposure question: `contacts` holds named organizers' phone numbers and
   `updates` may carry operational information, so "just show the public pages"
   is not automatically safe.
2. **Is tablet really the same as phone here?** iPadOS supports installation and
   Web Push 16.4+, so functionally yes — but iPadOS is hard to distinguish from
   desktop Safari. Are we comfortable that some iPads will be classified as
   desktop and see the landing page instead of the app? This matters more now
   that desktop is a dead end rather than a usable surface.
3. **Portrait consent & retention for minors** — **ANSWERED 2026-08-28 (task 102):**
   consent is held from sign-up (captured by the guardian, outside the app), the
   basis is safety/identification during the race, and portraits are purged after
   the event. No longer blocking the portrait step. The one part still open is
   whether participants may see each other's portraits — PRD 007's matrix.
4. **Who owns the portrait *viewing* surface?** Answered: **PRD 007**. Its access
   matrix defines who may see a given member's face, which is what the consent
   text in question 3 has to state.
5. **Are the permissions truly optional?** This PRD assumes login is mandatory,
   and that the portrait and both permissions are skippable. If push is considered
   mandatory for participants during an event, the flow needs a blocking variant
   and a different escape story.
6. **What is the correction channel** when a number is wrong — a phone number, an
   email, the patrol leader, or purely the in-app flag
   (`POST /api/me/profile/report-incorrect`)? PRD 003 has the same open question;
   answering it there answers it here.
7. **Where does verification live — field or status?** Answered in §11 (field).
   Listed here only because it was proposed as
   `MemberStatusVerified = "verified"` and reversing later is a data migration.
8. **Does "has started the event" exist** as a queryable fact? Partly answered:
   `types.MemberStatusRacing` means "signed in at the start and on the trail", so
   the skip rule can read the member's status — which arrives with PRD 006.
9. **Who else should see the verification flag** besides check-in — patrol
   leaders chasing their own members, an organizer dashboard counting unverified
   members before event start?
10. **Will editing open up later?** Noted as likely for a few fields. If so, a
    number change should probably invalidate the verification and re-trigger this
    step — worth designing the storage for now (§8) even if editing ships later.
11. **Do we want server-side install/permission metrics** (§9), or is client-side
    sufficient? The §9 metrics are not measurable without one.
12. **Does the escape hatch need rate-limiting or an expiry** (e.g. re-ask after
    7 days) so it does not become the default path?
13. **What does the desktop page link to** — a QR code, a plain URL, or nothing at
    all? An "SMS me the link" affordance would reuse the existing SMS
    infrastructure but adds an endpoint, so it is not assumed.
