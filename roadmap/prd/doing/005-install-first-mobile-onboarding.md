# PRD 005 — Install-first mobile onboarding (install, confirm, permissions)

**Status:** doing
**Author:** agent session (Zed)
**Created:** 2026-08-25
**Last updated:** 2026-08-30 (impl. — §10 carries the shipped/outstanding state)
**Approved:** 2026-08-30
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
flow is the whole point of this PRD.

Anyone in a **browser** — desktop or phone — gets a **normal website** rather than the app.
The website is anonymous: **there is no login outside the installed app** (§11, 2026-08-30). A
phone or tablet on it sees one call to action, "Installér som app". Building that website is
**not part of this PRD**; all this PRD ships is a placeholder page, so nobody is dumped at a
login form for an app they cannot use.

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
- **Why now?** PRD 002 (map, position reporting) is in `doing/` and push
  subscription has shipped (tasks 016/017/100). Both are worthless without an
  installed app and granted permissions, so the gate must exist before the event.
  Desktop visitors also currently hit a login screen for an app that is
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
  before they start using the app, and members arrive at check-in having already
  looked at their own data instead of having it read back to them field by field.
  *(Revised 2026-08-30: this was previously stated as a measurable check-in speed
  goal. Realising that requires the admin portal to surface the flag, which is out
  of scope — see §4. The verification is still recorded and published; who consumes
  it is a later decision.)*
- Location and push permission decisions happen once, up front, in a
  predictable, explained order — never as an unexplained native dialog.
- No browser visitor is shown a login form for an app they cannot use — they get a short,
  honest placeholder, and a phone gets an install call to action. Nothing more is promised.
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
- **Any real desktop website.** Desktop scope is undecided and belongs to its own
  PRD; this PRD ships only a placeholder page so desktop visitors are not dumped at
  a login screen. No read-only content browsing, no desktop login, no organizer
  console. The placeholder is built so it can be replaced wholesale without touching
  the mobile flow.
- **Any change to the admin portal (`hq`).** *(Added 2026-08-30.)* Verification is
  recorded here and published as a domain event; surfacing it at the check-in
  counter is a separate piece of work on `hq`'s own board, not a requirement of this
  PRD and not a condition for shipping it. Changes to `shared-go` **are** in scope,
  since the event and the member field have to be declared somewhere shared.
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
  form. *(Anything beyond that is a later desktop PRD.)*

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
   tick *"Dette nummer kan kontaktes i løbet af Nathejk"*. This is a **recognition
   check, not a security check** (§11). Skipped for users who
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
6. **Offline preparation** (slot, owned by PRD 009): its first sync — portraits, map
   tiles, the rulebook and the rest — with one combined progress view and a size
   estimate. Skippable, and best done here because the user is usually on wifi with
   the app open. Absent from the flow until 009 is approved, and **tracked on 009's
   tasks, not this PRD's** *(revised 2026-08-30)*.

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
2. Device is not mobile → the browser leaves the app for a **static placeholder page**
   (`/desktop.html`) that is not part of the SPA: brand headline and "more to come…". On a
   device that would qualify for the PWA it also carries a top **"Installér app"** banner back
   to the install instructions, so a misclassified tablet has a route in. The real desktop
   website is a separate PRD. *(Revised 2026-08-30 — originally a Vue route inside the app;
   see task 140.)*
3. No login form, no bottom nav, no map, no app content.

### Edge cases

- **Already installed, opened in a browser tab anyway.** We cannot reliably know
  the app is installed from a tab (`getInstalledRelatedApps` is Chromium-only,
  and only for related *native* apps). So the wall always shows, but always
  offers "Jeg har allerede installeret appen — åbn den" plus the manual
  fallback below.
- **Detection false negative** (desktop touchscreen classified as tablet,
  standalone not detected, PWA-hostile browser, in-app webview like the Facebook
  browser where installation is impossible). The wall links to the **anonymous
  website**, and nothing more: there is no route from a browser into the app
  *(revised 2026-08-30, task 143 — the earlier "Fortsæt i browseren" escape hatch let a tab
  into the login flow, and login is now PWA-only)*. For an in-app webview the instructions say
  to reopen the link in Safari or Chrome, where installing is possible; for a desktop wrongly
  classified as mobile the website is the correct destination anyway. What this gives up is
  named in §11 and in the Non-Functional list below.
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
- **The member does not know the guardian number, or it is wrong.** Expected and not rare: young
  scouts, a guardian who recently changed number, or two households with different numbers on
  file. This is **not** a failure state and must not be worded as one.

  **The member is offered the chance to fix it** *(added 2026-08-30, task 148)*: the masked number
  becomes an editable field, they type the full number, and they confirm **that** number as
  reachable. Better than a flag for the obvious reason — the person standing there is the one most
  likely to know the right number — and it turns the step from "verify our data" into "make sure we
  can reach an adult", which is what it was always for.

  If they cannot supply one either, "jeg kender ikke nummeret" still records the report, they still
  reach the app, and the record is still flagged. The signal "this member could not confirm the
  number" is valuable to organizers even when the number turns out to be correct.
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
- [ ] **Ambiguous devices are classified as `mobile`** (§11, decided 2026-08-30).
      Touch devices where a PWA makes sense — phones and tablets — are the target;
      desktop computers are not. When the signals disagree (iPadOS reporting itself as
      macOS Safari, a touchscreen laptop), the tie-break is `mobile`.
- [ ] Detect installed/standalone via `matchMedia('(display-mode: standalone)')` OR the
      iOS-only `navigator.standalone`, with an explicit `display-mode: browser` veto.
      *(Corrected 2026-08-30, task 142: this previously also accepted `minimal-ui` and
      `fullscreen`. The manifest only ever requests `standalone`, so those two cannot occur on
      an installed launch — only on an uninstalled one, where they made a browser tab read as
      installed and skip the wall. A fullscreen video did it; so did some Android browsers'
      chrome-less modes.)*
- [ ] Mobile + not standalone → all app routes redirect to `/install`.
- [ ] `/install` shows a one-tap install button when `beforeinstallprompt` was
      captured, and platform-specific manual instructions otherwise (iOS Safari,
      Android non-Chrome, in-app webview).
- [ ] `/install` links to the **anonymous website** and offers no way into the app. There is
      no persisted "continue in browser" override *(revised 2026-08-30, task 143: **login
      exists only in the installed app**, so a browser tab has nowhere to continue to)*.
- [ ] **The website is anonymous** — no login form, no participant data, no session — and on a
      device that qualifies for the PWA it shows a top call-to-action box, "Installér som app",
      linking to `/install`.
- [ ] Desktop → a **static placeholder page outside the SPA** (`/desktop.html`), reached by a
      full-page navigation rather than a route change. Desktop never reaches `/welcome` or
      `/install`, and no role may log in there (§11). The page renders no participant-facing
      app content, so it carries no data-exposure decision. It shows an **"Installér app"**
      banner — only on devices that qualify for the PWA — linking back to the install
      instructions, which is also the way back for a device the detection misread.
      *(Revised 2026-08-30, task 140: the placeholder must not be the app or part of it.)*
- [ ] Mobile + standalone → `/welcome`, a linear onboarding flow, resumable, each
      permission step preceded by an in-app explanation
      (extend `PermissionPrompt.vue`) before any native dialog. The canonical step
      order is:
      1. `login` (mandatory)
      2. `confirm profile` (spejder, first run — skippable by rule, see below)
      3. `portrait`
      4. *(slot)* `vehicle` — bandit/gøgler/crew only, specified by PRD 010; absent
         until 010 is approved
      5. `location`
      6. `notifications`
      7. `offline first sync` — specified by PRD 009; absent until 009 is approved
      Steps 4 and 7 are **flag-gated slots**: the step machine must treat the
      sequence as data, so an unapproved PRD cannot change this list
      *(clarified 2026-08-30 — §5 and §6 previously disagreed on the step count)*.
- [ ] The **profile confirmation** step shows the user's registered details and
      the **parent/guardian emergency contact number** masked to its last two
      digits (`11 22 33 **`), requires those two digits to be typed, and requires
      a checkbox *"Dette nummer kan kontaktes i løbet af Nathejk"*. Both are
      needed to advance.
- [ ] The masking is a **recognition device, not a confidentiality control**
      (§11, decided 2026-08-30). `GET /api/me/profile` continues to return
      `phone_parent` in full to its owner, as PRD 003 shipped it; the step is not
      required to be tamper-proof and must not be described as if it were.
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
- [ ] Either path **opens the number for editing**: the member types a full guardian number and
      confirms *that* as reachable *(added 2026-08-30, task 148)*. The field starts empty, not
      prefilled — prefilling invites editing one digit of a number they have just said they do not
      recognise, and makes "corrected" indistinguishable from "retyped".
- [ ] A member-supplied number is recorded as **what the member acknowledged**, not as an
      overwrite of the register: `phone_parent` stays the register's value, and the app also records
      which register value the acknowledgement was made against. That is what keeps "the
      acknowledgement is stale, ask again" distinct from "the register is wrong, fix it" — two
      states that look identical if compared with one field and call for opposite responses.
- [ ] A member who can supply no number at all is still never blocked.
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
      trains people to ignore it. It reads `profile.store.hasPhoto` (already shipped
      by PRD 003 via `has_photo`); this PRD owns the nudge surface itself, of which
      there must be exactly one.
- [ ] Portrait capture reuses PRD 003's `components/profile/PhotoCapture.vue` and
      `PUT /api/me/photo` rather than implementing a second one.
- [ ] Details are **read-only** at confirmation time; in-app editing is out of
      scope here (§12).
- [ ] Onboarding never hard-blocks on a permission decline or a failed profile
      confirmation; only **login** is mandatory.
- [ ] A step counts as settled only when the thing it exists to achieve is actually done. In
      particular the notifications step needs a **push subscription registered with the BFF**,
      not merely a granted permission — the two are independent, and a grant with no
      subscription delivers nothing *(added 2026-08-30, task 144)*.
- [ ] A step that is on screen is never replaced because state resolved underneath it. The step
      machine owns the order; the flow advances when the user finishes a step *(added
      2026-08-30, task 144: steps that sync their own state on mount were completing the flow
      before the user saw them)*.
- [ ] Per-device state (permissions) persists in `localStorage`; per-user state
      (profile confirmation) comes from the BFF. Returning users go straight to
      `/maps`.
- [ ] Once onboarding is complete the app behaves exactly as today.
- [ ] A dev/QA override (query param or `localStorage` flag, gated to non-prod)
      bypasses the install and device gates.

### Non-Functional

- **Reach.** Baseline stays iOS/iPadOS Safari 16.4+ / Chrome 111+ per `.rules`.
- **No route into the app outside the installed app** *(revised 2026-08-30, task 143;
  this requirement previously read "No lockout — every gate has a user-reachable escape
  hatch")*. Login is PWA-only. The trade is deliberate and its cost is real: a phone that
  genuinely cannot install a PWA now has no way to sign in at all. It is accepted because such
  a device cannot do Web Push, a reliable service worker or background sync either — so the
  app's core features were already unavailable to it (see the browser baseline in `.rules`) —
  and the one common case, an in-app webview, is recoverable by reopening the link in Safari or
  Chrome. Every gate still has a **user-reachable exit**: the website.
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
| `/desktop.html` | — | *(not a route)* | Static placeholder **outside the app** — a plain file, no Vue (task 140). The real desktop site is a separate PRD. |

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
- The desktop placeholder is **not a component and not a view** — it is
  `public/desktop.html`, a plain file outside the app (task 140). A route inside the SPA
  would still boot Pinia, the router, the guard and the service worker to render one
  sentence, and the real desktop site will not be a Vue view in this repo.
- `components/onboarding/InstallInstructions.vue` — platform-specific steps.
- `components/onboarding/WelcomeStepLogin.vue`, `…StepConfirmProfile.vue`,
  `…StepPortrait.vue`, `…StepLocation.vue`, `…StepNotifications.vue`. The
  offline-sync step is a slot filled by PRD 009 and specified there.
- `…StepPortrait.vue` wraps PRD 003's `components/profile/PhotoCapture.vue`; it must
  not fork it. The face guide, retake affordance and explicit confirm-before-upload
  are requirements **on that component**, specified in PRD 003 §7 — people will not
  accept a photo they did not get to approve.
- The confirm-profile step uses shadcn-vue `Card`, `Input` (a 2-digit numeric
  input with `inputmode="numeric"`), `Checkbox` and `Label`; the masked number is
  rendered as text, never as an editable field.
- `components/PermissionPrompt.vue` — **this PRD owns the component's API**, adding
  a `variant` prop so it serves both the existing compact card and a full-screen
  onboarding explanation. PRD 002 (map repair affordance) and PRD 003 (status rows)
  are consumers of that API, not co-owners. Today the props are
  `{ title, message, cta, icon?, moreTo?, moreLabel? }` with `accept`/`dismiss`
  emits — `moreTo`/`moreLabel` arrived with task 085's location copy, and task 101
  shipped the blocked-permission guidance, so the full-screen variant extends that
  work rather than replacing it *(corrected 2026-08-30)*.
- Primitives needed that are **not yet generated** in `vue/src/components/ui/`:
  `progress` and `checkbox` (`alert` now exists). Generate them per PRD 004's
  on-demand rule.

Shell behaviour (`App.vue`): the top bar **and** `BottomNav` are hidden on
`/install` and `/welcome` — the desktop placeholder needs no mention, being outside the app
entirely. `showShell` becomes
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
    and `installed` from the `appinstalled` event. *(No `continueInBrowser` override: removed
    2026-08-30, task 143.)*
  - New `stores/onboarding.store.ts` — step machine + per-device completion flag
    in `localStorage`; derives step state from `session.store`,
    `location.store.permission` and `notifications.store.permission` so it is
    self-healing rather than a blindly persisted cursor.
  - `router/index.ts` — extend the global guard: device/install/onboarding gates
    run **before** the existing auth check. Guard order: dev override → device class →
    standalone → onboarding complete → auth → roles. The device gate is session-independent by
    design (§11), so it can short-circuit before `session.ensureReady()` and
    spare desktop visitors a pointless `/api/me` round-trip — and for a desktop visitor it
    leaves the app entirely (a full-page navigation to the static placeholder), rather than
    routing within it.
  - `helpers/pwa.ts` — unchanged; `UpdatePrompt` must not overlay the wall.
- **BFF (Go):** this PRD **does** require BFF work, for the profile-confirmation
  step only — the install and device gates remain pure client state, and the BFF
  continues to authorize every endpoint independently (the install gate is UX,
  never security).
  - Per `go-bff-layout`, extend `go/cmd/api/profile.go` (shipped by PRD 003)
    rather than adding a parallel handler file. The write facade
    `commands.Commands` is real as of PRD 008 (task 056), so the publish path
    exists *(corrected 2026-08-30)*.
  - Confirmation state is **derived, not just stored**: the response must report
    `confirmation_required` computed from "has verified" **OR** "has started the
    event" (`types.MemberStatusRacing` onwards). The client must not reimplement that
    rule. The stored field is `verified_at` — one name across PRDs 003, 005 and 006.
  - **The verification flag is published as a domain event** (decided 2026-08-25;
    see PRD 008 §8 — nothing writes directly to the database). `hej` is therefore a
    publisher. Any other service that wants to know about verification learns it by
    consuming the same log; no service calls another's API. Building that consumer is
    not part of this PRD (§4).
  - **Cross-repo — revised 2026-08-30 during implementation (task 132).** Verification is a
    fact about a *member*, and members are not owned by this repo, so the plan was to declare
    a `member.verified` message in shared-go. It was **declared in `hej` instead**, following
    the precedent `portrait.go` set in PRD 003: events this service publishes are owned by the
    projection that consumes them, while shared-go carries the messages other services
    publish. With `hq` out of scope (§4) there is no second party, so a shared-go type would
    have been an unused export in a module three repos depend on, plus a version bump in each.
    The message lives in `go/nathejk/table/person/verified.go`, and that whole package is bound
    for shared-go anyway, so moving it later is mechanical.

    `hej` already depends on shared-go (`go/go.mod`, task 052) *(corrected 2026-08-30 — this
    previously said `hej` did not import it at all)*. Recording verification therefore means,
    in this repo only:
    1. Publish `person.MemberVerified` on `NATHEJK.<year>.member.<personId>.verified`.
    2. Project `verified_at` (plus the acknowledged number) onto **PRD 006's `person` read
       model in `hej`** — not onto a shared-go table. *(Clarified 2026-08-25: this and PRD 006
       named two different homes for the same column, which is a data-migration risk.)*
    No `hq` work is required to ship this PRD; surfacing the flag at the check-in counter is
    `hq`'s own board *(revised 2026-08-30)*. If shared-go does gain the message later, the
    two-repo release loop from `go-bff-layout` applies: commit, push and version-bump before a
    `GOWORK=off` build sees it.
  - **Directory dependency.** Everything the confirmation step reads comes from the
    member directory, which is **real** as of PRD 006 (task 077 swapped the mock for
    the `person` projection) *(corrected 2026-08-30)*. Two facts from that work
    constrain this PRD directly:
    - `PhoneParent` exists **only** on `spejder` — hence the spejder-only rule in
      §6. It is nullable on `person`, and null means "not applicable".
    - Portrait storage now exists (PRD 003, tasks 103–105), but **no per-person
      verification flag exists** — that remains new write-side work.
  - See §11 for **why this is a member field rather than a new `MemberStatus`**.
  - "Has started the event" is available: PRD 006's `person` projection carries
    member status (task 080), and `types.MemberStatusRacing` onwards means started.
    Read that rather than introducing a second notion of "started".
- **API endpoints (OpenAPI annotations mandatory, per `.rules` and the style in
  `go/cmd/api/auth.go`):**
  - `GET /api/me/profile` — **owned by PRD 003 and already shipped**; this PRD adds
    `confirmation_required` and `verified_at`. It keeps returning `phone_parent` in
    full to its owner — masking happens in the UI (§11). `200` / `401` / `404`.
  - `POST /api/me/profile/confirm` — body carries the two digits and the
    acknowledgement flag. `204` / `400` (wrong digits) / `401` / `409` (already
    confirmed, already started, or no guardian number on file) / `429` / `503` (broker
    down — a confirmation the log never saw must not answer `204`). Wrong digits are
    rate-limited like the PIN endpoint — not as a secrecy measure, just so the endpoint
    cannot be hammered.
  - `POST /api/me/profile/guardian` — a member-supplied replacement number: body carries the
    full number and the acknowledgement. `204` / `400` (unparseable, or acknowledgement missing) /
    `401` / `429` / `503`. Normalized server-side with `internal/phone` before publishing, since
    every comparison downstream — and the login lookup — depends on normalized numbers. A separate
    endpoint from `confirm` deliberately: agreeing with what we hold and replacing it are different
    acts, with different validation and different meaning in the log, and one body carrying two
    mutually exclusive fields is the shape that produces "which did the client mean?" bugs
    *(added 2026-08-30, task 148)*.
  - `POST /api/me/profile/report-incorrect` — flags the record for organizer
    follow-up, with a reason distinguishing "wrong" from "unknown to me". `204` /
    `401`. **Required**, not optional: a guardian number nobody can confirm is an
    operational problem that must reach a human before the event, not just a dead
    end in the UI. The flag is stored in this repo; how organizers read it is a
    follow-up, not a blocker.
  - `PUT /api/me/photo` — **owned by PRD 003**; consumed unchanged here.
  - The digits are verified **server-side** so the acknowledgement is recorded
    against a real answer, but the number is deliberately **not** kept from the
    client (§11). This is a sanity check that the member looked at the number and
    recognised it — not an authentication factor.
- **Data / storage:**
  - Client: `localStorage` keys only, namespaced `hej.install.*` /
    `hej.onboarding.*`. Per-user confirmation is **not** stored client-side.
  - Server: a `verified_at` timestamp on the member, the acknowledged contact number, **and the
    register's value at the moment of acknowledgement** *(added 2026-08-30, task 148)*. Three
    fields rather than two because a member may acknowledge a number that is *not* the one on file:
    comparing the register against the value it had at acknowledgement answers "has it changed
    since?" (stale → ask again), while comparing the acknowledged number against it answers "did
    the member correct us?" (→ fix the register). With only two fields those two states are
    indistinguishable, and a member who corrected us would be re-asked forever while the register
    stayed wrong.
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
  - **Risk: the verification is not consumed by anyone yet.** The app can collect
    verifications perfectly and nothing downstream changes until some organizer
    surface reads the published event. That is accepted: the data-quality signal and
    the member having looked at their own record stand on their own, and the
    consumer is a later, separate piece of work (§4).
  - **Risk: PRD 003's endpoints are the foundation here** — `GET /api/me/profile`,
    `PUT /api/me/photo` and `PhotoCapture.vue`. PRD 003 is `done`, so this is no
    longer a sequencing risk *(corrected 2026-08-30)*.
  - **Risk: "has started the event"** — resolved: member status is projected onto
    `person` (task 080) *(corrected 2026-08-30)*.
  - No new runtime dependencies. The desktop placeholder needs none.
  - **Risk: detection is heuristic.** iPadOS reports itself as macOS Safari;
    desktops with touchscreens exist; `display-mode` can be unreliable in
    embedded webviews. Mitigated by erring towards mobile and by the website being a
    reachable exit — not by better sniffing.
  - **Risk: `beforeinstallprompt` timing.** It fires once, early; it must be
    captured before the Vue app mounts or the one-tap install silently degrades
    to manual instructions.
  - **Risk: gating login behind install increases drop-off** at the worst possible moment (a
    participant needing help). With the escape hatch removed (task 143) there is no mitigation
    beyond the install instructions themselves, so their clarity *is* the mitigation — and the
    webview variant, which is the likeliest way to arrive unable to install, must keep telling
    the user exactly how to get out of it.
  - **Interaction with PRD 002/003:** the map's location handling and the
    profile page's permission controls must consume the same stores, not
    re-implement permission logic.

## 9. Success Metrics

All session-level figures below need a reporting path that does not exist yet (§12
Q11). Until one is agreed they are **stated intentions, not measurements** — either
add a metrics task or accept that this section cannot be evaluated.

- ≥ 95% of authenticated sessions during an event originate from a standalone
  display mode. Since login is PWA-only (§11), a counter-example is a bug rather than a
  user choice.
- ≥ 85% of onboarded users have an active push subscription registered with the
  BFF.
- ≥ 80% of onboarded users have granted location.
- ≥ 90% of first-time users complete the guardian-number confirmation without
  using the "wrong"/"unknown" paths.
- ≥ 70% of members verify before arriving at check-in. Countable in this repo from
  `verified_at` versus member status, with no `hq` involvement.
- Every unconfirmed guardian number is surfaced to organizers before event start,
  with zero cases of a pickup or emergency call failing on a number the app had
  already flagged.
- No support reports of users unable to reach the app on a supported device.

## 10. Rollout / Task Breakdown

Sequencing: platform detection and stores first (they are what everything else
consumes), then the wall, then onboarding, then the router gate wiring. The
desktop placeholder is the smallest piece — build it as a stub and do not delay the
mobile flow on it. Ship behind a runtime flag in `config/runtime.ts` so the gate can
be disabled without a rollback if it misfires during an event.

Tasks, with their board ids and state as of 2026-08-30. Two are outstanding; everything else
shipped. Later fixes against this PRD: **141** (gate redirect loop), **142** (browser tab read as
installed), **143** (website anonymous / login PWA-only), **144** (flow ended after the portrait),
**145** (top bar behind the status bar).

- [x] Task: platform detection helper (`helpers/platform.ts`) + unit tests — **116**
- [x] Task: install store — capture `beforeinstallprompt`, `promptInstall` — **117**
- [x] Task: onboarding store — resumable step machine derived from permission state — **118**
- [x] Task: shared-go — `member.verified` event message; bump version in `hej` — **132**
      *(landed as a `hej`-local message instead — see §8 and task 132)*
- [x] Task: BFF — publish the verification event via `commands.Commands` — **133**
- [x] Task: BFF — project `verified_at` + derive `confirmation_required` onto PRD
      006's `person` read model
- [x] Task: BFF — `POST /api/me/profile/confirm` with server-side digit check and rate limiting — **135**
- [x] Task: WelcomeStepPortrait — wrap `PhotoCapture.vue`, retake, skip, reuse `PUT /api/me/photo` — **129**
- [ ] Task: portrait nudge — re-prompt after onboarding while `hasPhoto` is false — **146, NOT DONE**
      (dismissible per session, silenced permanently once one is uploaded)
- [x] Task: WelcomeStepConfirmProfile — masked number, 2-digit input, acknowledgement checkbox — **127**
- [x] Task: "nummeret er forkert" / "jeg kender ikke nummeret" paths + follow-up flag — **128, 136**
- [x] Task: InstallView — one-tap install (Chromium) with platform instructions fallback — **119**
- [x] Task: InstallView — link to the anonymous website — **121, 143** *(shipped first as a "fortsæt i
      browseren" override, removed by task 143)*
- [x] Task: InstallInstructions component — iOS Safari / Android / webview variants — **120**
- [x] Task: refactor LoginView into an onboarding login step — **125**
- [x] Task: WelcomeView shell + location and notification explanation steps — **124, 131**
- [x] Task: remove the `/login` route; repoint the router guard fallback and
      `App.vue`'s `signOut()` at `welcome`
- [x] Task: generate the `progress` and `checkbox` shadcn-vue primitives — **122**
- [x] Task: PermissionPrompt full-screen variant — **130**
- [x] Task: DesktopView — static placeholder page for non-mobile visitors — **123**, replaced by **140**
      *(shipped, then replaced by task 140: a plain page outside the app)*
- [x] Task: router guard — device / standalone / onboarding gates — **137**, fixed by **141**
- [x] Task: App.vue shell — hide chrome on install/welcome routes — **138**
- [x] Task: runtime flag + dev/QA gate bypass — **139**
- [ ] Task: manual test matrix — iOS Safari, Android Chrome, Android Firefox, desktop, in-app webview — **139, NOT RUN**

## 11. Decisions

Answered questions are recorded here rather than deleted, so the reasoning
survives.

- **2026-08-30 — All five steps stay in the flow, and a step whose question is already answered
  is skipped.** Confirmed by the maintainer after seeing the flow on a device. The canonical
  sequence remains login → confirm profile → portrait → location → notifications; the portrait step
  is **not** dropped even though PRD 007 (which displays portraits) has not shipped.

  "Skipped when already settled" is the derived machine's behaviour and is deliberate: a member who
  has already granted location has nothing to decide, and an explanation screen before a dialog that
  will never appear is noise. Note what *settled* has to mean for this to be safe — the thing the
  step exists to achieve, not a proxy for it. Task 144 is the cautionary case: the notifications
  step counted a granted permission as settled, when what it exists to create is a **push
  subscription**, and the two are independent.
- **2026-08-30 — A member who cannot confirm the number can replace it.** The masked field opens
  up, they type the full number, and they confirm that one instead (task 148). Recorded here because
  it changes what the step *is*: not "verify our data" but "make sure we can reach an adult", which
  is what it was always for — and the person standing there is the one most likely to know the right
  number. It also shrinks PRD 005 §12's open correction-channel question to the residual case of a
  member who can supply nothing at all.
- **2026-08-30 — The website is anonymous, and login exists only in the installed app.**
  Settled by the maintainer after seeing the flow on a device: from the install wall, the
  "Fortsæt i browseren" escape hatch led into `/welcome` and the login flow — which made the
  browser a second, degraded way to *be a user*. It is not. The browser experience is a public
  website: no login, no session, no participant data. A phone or tablet visiting it gets a
  call-to-action box at the top, "Installér som app", linking to the install instructions.

  Three consequences, and the third is a genuine cost:

  1. **The persisted "continue in browser" override is deleted**, not merely relabelled. It had
     nothing left to unblock, and a persisted flag that silently changes the gate's behaviour on
     any device that ever tapped it — with nothing in the UI to reveal it — is a footgun. (It
     was also a live suspect while diagnosing task 142, precisely because it is invisible.)
  2. **The profile page's install row goes with it** (task 121 added it as the way back from the
     override). The page is only reachable inside the installed app, so the row could only ever
     have said "installed".
  3. **The no-lockout guarantee in §6 is given up.** A phone that genuinely cannot install a PWA
     now has no way to sign in. Accepted: such a device has no Web Push, no reliable service
     worker and no background sync either, so the app's core features were already beyond it,
     and the common case — arriving in a Facebook in-app browser — is recoverable by reopening
     the link in Safari or Chrome, which the webview instructions already say. What every gate
     still has is a reachable *exit*: the website.

  The dev/QA bypass (§6, task 139) is unaffected and remains non-prod only. It is not a
  substitute for the removed hatch, and PRD 005 §11's note about organizer laptop access still
  stands: that would be a new PRD, not a flag.
- **2026-08-30 — Ambiguous devices are classified as mobile.** The target is touch
  devices where a PWA makes sense — phones and tablets; desktop computers are not.
  Since iPadOS reports itself as macOS Safari and touchscreen laptops exist, detection
  cannot be exact, so the tie-break has to be chosen deliberately: **ambiguous →
  mobile**. A false positive costs a desktop user one click on the "Fortsæt i
  browseren" escape hatch (now: one tap on the website link); a false negative leaves an iPad
  user at a placeholder page
  with no route into the app at all. The asymmetry decides it. This also means the
  exit from the wall is load-bearing rather than a nicety — it is what makes the aggressive
  tie-break safe, and it must stay discoverable enough for support to talk someone
  through it over the phone.
- **2026-08-30 — The masked number is a sanity check, not a security check.**
  `GET /api/me/profile` already returns `phone_parent` in full to its owner (PRD 003,
  shipped), so a determined user can read the two hidden digits out of the network
  response. That is accepted. The purpose of the step is to make the member *look at
  the number and recognise it*; nobody is being authenticated by it, and the number is
  the user's own guardian's, not a secret being protected from them. The PRD
  previously required the full number never to reach the client and called the
  alternative "theatre" — that framing is withdrawn, and PRD 003's endpoint is
  unchanged.
- **2026-08-30 — No `hq` work is in scope, and faster check-in is not a goal of
  this PRD.** Verification is recorded here and published as a domain event; any
  organizer-facing consumer is separate work on `hq`'s own board. `shared-go` changes
  are in scope, since the message has to be declared somewhere shared. Consequence
  worth naming: nothing downstream reacts to a verification on the day this ships —
  the value until then is the data-quality signal and the member having checked their
  own record.
- **2026-08-30 — The desktop side of this PRD is a placeholder only, and it is not part of
  the app.** The scope of this PRD is: on a phone or tablet, prompt for installation and
  deliver the app experience only when installed; on anything else, show an ordinary website.
  Building that website is a separate PRD. **Clarified by the maintainer later the same day
  (task 140): the placeholder is a plain static page, not a view in the SPA** — for now
  "Hej Nathejk — more to come…", with an "Installér app" banner shown only on devices that
  qualify for the PWA, linking back to the install instructions. A route inside the app would
  boot the whole application to render one sentence, sit inside the service worker's scope, and
  be thrown away rather than replaced when the real site arrives.

  The install wall therefore carries **one** non-install affordance. It was the "Fortsæt i
  browseren" escape hatch; **as of the decision below it is a link to the website instead.** Note the placeholder's own banner is the mirror image and is
  *not* redundant with it: it exists so a device wrongly classified as desktop still has a
  route to the wall.

- **2026-08-25 — Verification is published as a domain event.** Not written to a
  database by the app, per the architecture rule that nothing writes directly to SQL
  and that services may not call each other's APIs. It also means any future
  consumer — a check-in view, a pre-event report — can be added without coupling to
  this service. See PRD 008 §8.
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
- **2026-08-25 — Faster check-in was an explicit goal of this step.**
  **Superseded 2026-08-30** (see above): the counter-side change lives in `hq` and is
  out of scope, so check-in speed is a possible downstream benefit rather than a goal
  or a metric here. The reasoning is kept because it is why verification is recorded
  as a durable, published fact rather than a client-side flag.
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
  both attention and consent. The digits are checked server-side so the
  acknowledgement is recorded against a real answer — but see the 2026-08-30
  decision above: this is not a confidentiality control.
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
- **2026-08-25 — The install wall linked to the desktop version.** **Superseded
  2026-08-30:** the link is dropped. It could never rescue a user whose device
  wrongly failed the install check (it leads to the `/desktop` placeholder), and two
  similar-looking low-prominence links invite exactly the mix-up support cannot
  untangle over the phone. If `/desktop` ever becomes a real website, revisit whether
  the wall should point at it.
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

Answered items are moved to §11 rather than kept here, so this list is only what
still gates the work.

1. **Are the permissions truly optional?** This PRD assumes login is mandatory,
   and that the portrait and both permissions are skippable. If push is considered
   mandatory for participants during an event, the flow needs a blocking variant
   and a different escape story.
2. **What is the correction channel** when a number is wrong — a phone number, an
   email, the patrol leader, or purely the in-app flag
   (`POST /api/me/profile/report-incorrect`)? PRD 003 has the same open question;
   answering it there answers it here.
3. **Who eventually consumes the verification event** — a check-in view, patrol
   leaders chasing their own members, a pre-event report counting unverified members?
   Out of scope here (§4), but it decides what the event payload has to carry, so it
   is worth a rough answer before the message is declared in shared-go: a message is
   cheap to add fields to and expensive to reshape.
4. **Will editing open up later?** Noted as likely for a few fields. If so, a
   number change should probably invalidate the verification and re-trigger this
   step — worth designing the storage for now (§8) even if editing ships later.
5. **Do we want server-side install/permission metrics** (§9), or is client-side
   sufficient? Most of §9 is not measurable without one.
6. ~~**Does the escape hatch need rate-limiting or an expiry**~~ — **moot as of 2026-08-30
   (§11):** there is no escape hatch. A browser tab cannot reach the app at all, so there is no
   default path for it to become.
