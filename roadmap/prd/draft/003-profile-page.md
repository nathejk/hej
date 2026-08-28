# PRD 003 — Profile Page (own details, self-portrait, device permission status)

**Status:** draft
**Author:** agent session (Zed / Claude Opus 5)
**Created:** 2026-08-24
**Last updated:** 2026-08-28
**Approved:**
**Shipped:**
**Target users:** all signed-in roles (spejder, bandit, postmandskab, guide, samarit)

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See the `prd` skill for the lifecycle.
-->

---

## 1. Summary

A `Min profil` page where a signed-in user can see the details Nathejk holds
about them (name, address, own phone, parent phone — all read-only), take a
portrait photo of themselves with the device camera to attach to their profile,
and see at a glance whether push notifications and location sharing are enabled
for the web app on this device — with a one-tap way to fix it when they are not.

## 2. Problem & Motivation

- **What problem does this solve?**
  1. **Identity confidence.** After PIN login the user has no way to confirm
     *who* the app thinks they are, or that Nathejk holds the right contact
     details for them and their parents. Today the only identity surface is the
     role-filtered nav.
  2. **Recognisability.** During the event, personnel and organizers need to
     match a person to a record; participants have no photo on file. A
     self-taken portrait is the cheapest way to get one, captured by the person
     themselves rather than transcribed from paperwork.
  3. **Silent permission failures.** The app depends on two device permissions —
     push notifications (event updates) and location (the map, PRD 002). Both are
     requested contextually behind soft pre-prompts, and both fail *silently* if
     denied or if the app is not installed to the home screen (iOS Web Push).
     There is nowhere to see or repair that state, so a user can miss every event
     update without knowing.
- **Why now?** The permission plumbing already exists and is unused as a status
  surface: `vue/src/stores/notifications.store.ts` tracks
  `permission` / `subscribed` / `available`, and `location.store.ts` tracks
  `permission` / `available`. PWA install state lands in `install.store` (PRD 005;
  `app.store.ts` holds only the update flag). A profile page
  is the natural home for all of it, and it becomes the place where every future
  user-level preference lands as the app is scoped out.
- **Evidence.** Product request from the organizers; PRD 001 explicitly left
  "notification permission repair" and any user-details surface out of the
  skeleton.

## 3. Goals

- A user can verify, in one screen, that Nathejk has the right details for them
  and their parents — and knows who to contact if not.
- A user can attach a recognisable photo of themselves to their profile in under
  30 seconds, from the phone they already have in hand.
- A user can tell, without guessing, whether this device will receive event push
  notifications and whether the map can show their position — and can act on it
  when it will not.
- The page is a stable, extensible home for user-level preferences added later.

## 4. Non-Goals

- **First-login profile confirmation** — the one-time, blocking "confirm your
  details and contact number" step is owned by **PRD 005**, which has the
  onboarding step machine. This page is the passive, re-entrant view of the same
  data; PRD 005 consumes `GET /api/me/profile` and extends it with
  `confirmation_required` / `verified_at`. Coordinate the two, but do not build
  a second gate here.
- **Editing personal details in the app.** Name, address and phone numbers are
  sourced from Nathejk records and are read-only here. Corrections go through the
  existing (out-of-app) channel.
- **Viewing other people's profiles or photos.** Own profile only. **Note:** PRD
  005 established (2026-08-25) that the portrait's purpose is identifying members
  during the race, largely at night — which means a personnel-facing viewing
  surface is no longer optional to the photo's value, it is the point of it. That
  surface is specified in **PRD 007** (offline portrait identification), including
  its access control and the rule that spejdere and banditter cannot see each
  other. Still out of scope *here*, but now a known dependency rather than a
  hypothetical future feature.
- **Avatar editing** beyond what is needed for a usable portrait (see Open
  Questions on cropping) — no filters, stickers, or drawing.
- **Uploading an existing photo from the camera roll** unless the platform gives
  us that for free (decision needed; see Open Questions).
- **Face recognition / automatic matching.**
- **Account deletion or data-export self-service** (GDPR requests stay in the
  existing process for now).
- **Changing the phone number used to log in.**
- **Push message delivery/fan-out** — still a later PRD (PRD 001 captured
  subscriptions only). This page reports and repairs *subscription state*, it
  does not send anything.

## 5. User Stories & Scenarios

- As a **participant**, I want to see the name, address and phone numbers Nathejk
  has for me so that I can check they are correct before the event.
- As a **participant**, I want to see my parent's phone number as registered so
  that I know who will be called if something happens.
- As a **participant**, I want to take a photo of myself and attach it to my
  profile so that personnel can recognise me and so my profile feels like mine.
- As a **participant**, I want to see whether notifications and location are
  switched on for this app so that I do not miss event updates or lose the map.
- As a **participant** who previously denied notifications, I want to be told how
  to turn them back on so that I can fix it myself.
- As **personnel** (guide/samarit/postmandskab), I want the same page so that my
  own registered details and permissions are verifiable.

**Primary happy path**

1. User opens `Min profil` (see UX notes for where it lives in the nav).
2. The page shows a portrait placeholder, their name, and read-only rows for
   address, own phone and parent phone.
3. User taps the portrait placeholder → "Tag et billede".
4. The camera opens (front camera preferred); the user takes a shot, sees a
   preview, and confirms or retakes.
5. On confirm the image is uploaded to the BFF; the portrait updates in place and
   persists across reloads and devices.
6. Below, a "På denne enhed" section lists:
   - **Notifikationer** — status (til / fra / blokeret / ikke understøttet) with a
     "Slå til" action when actionable.
   - **Placering** — status with a "Slå til" action when actionable.
   - **Installeret som app** — whether the PWA is installed, with the install
     action when available (reusing `install.store` from PRD 005).
7. Tapping "Slå til" runs the existing store flow
   (`notifications.enable()` / `location.request()`); the row updates immediately.

**Edge cases and errors**

- **Permission already denied at OS/browser level:** the native prompt will not
  reappear. The row shows "Blokeret" plus platform-specific guidance
  ("Slå til i indstillinger for din browser"), not a dead button.
- **iOS not installed to home screen:** Web Push is unavailable; the notification
  row explains that installing the app is a prerequisite and links to the install
  guidance.
- **Camera unavailable or permission denied:** show the reason and keep the rest
  of the page usable. No blocking dialog.
- **Upload fails / offline:** keep the local preview, show a retry action, do not
  silently drop the photo.
- **Large image:** downscale client-side before upload to stay within a size
  limit.
- **Details missing in Nathejk records** (e.g. no parent phone): render the row
  with a clear "Ikke registreret" rather than an empty gap.
- **Personnel with no parent phone** (adults): hide the parent row entirely
  rather than showing "Ikke registreret".
- **Replacing an existing photo:** allowed; the previous one is superseded.
- **Session lost (401):** `fetchWrapper` already routes to login.

## 6. Requirements

### Functional

- [ ] A `Min profil` page reachable for every signed-in role from a **user menu in
      the top-right of the app bar** (an avatar button showing the portrait, or
      initials while none exists).
- [ ] Read-only details, fetched from the BFF: **name**, **address**, **own phone
      number**, **parent phone number**. Rendered as labelled rows, clearly
      non-editable, with a short line on how to get a correction made.
- [ ] Phone numbers are tappable (`tel:`) and formatted for Danish numbers.
- [ ] A portrait section showing the current profile photo, or a placeholder with
      a clear call to action when none exists.
- [ ] Capture a photo using the device camera, defaulting to the front/selfie
      camera, with a preview and retake/confirm step before upload.
- [ ] The photo is uploaded to the BFF, associated with the signed-in user, and
      is shown on the profile on subsequent loads and on other devices.
- [ ] The photo can be replaced. (Whether it can be *removed* — see Open
      Questions.)
- [ ] Images are downscaled and re-encoded client-side before upload (target:
      longest edge ~1024px, JPEG/WebP, well under the server limit).
- [ ] A "På denne enhed" section with a row per capability, each showing an
      explicit state and, where possible, an action:
      - Push notifications — from `notifications.store` (`available`,
        `permission`, `subscribed`).
      - Location sharing — from `location.store` (`available`, `permission`).
      - Installed as app — from `install.store` (PRD 005).
      - Kamera — whether the portrait camera is usable on this device, with
        platform-aware guidance when blocked. Camera is the third permission the
        app needs (PRD 005 requests it during onboarding) and had no status row
        until this was added on 2026-08-25.
- [ ] Permission states are re-synced whenever the page becomes visible, so a
      change made in browser settings is reflected on return.
- [ ] Rows whose permission is blocked show platform-appropriate guidance instead
      of a non-functional button.
- [ ] **Sign-out lives in the user menu**, not on the profile page and not as a
      standalone top-bar button. There is exactly one sign-out action in the app.
- [ ] The section is structured so additional preferences can be appended without
      a redesign (a preferences list, not a bespoke layout).
- [ ] All copy in Danish.

### Non-Functional

- **Privacy:** a portrait of (often) a minor is sensitive personal data. Capture
  must be explicitly initiated by the user, the purpose must be stated on the
  page, and the image must only ever be served to the owning user (and to
  whichever organizer surface is later authorised — out of scope here).
- **GDPR:** define retention (see Open Questions — proposal: photos are deleted
  after the event). Document the legal basis and the parental-consent position
  before this ships.
- **Security:** upload endpoint requires an authenticated session, validates
  content type and magic bytes, enforces a hard size limit, re-encodes rather
  than trusting the uploaded bytes, and strips EXIF (notably GPS) metadata.
  Stored images must not be publicly enumerable.
- **Performance:** the profile fetch is a single request; the photo is served at
  a display-appropriate size, not full resolution.
- **Accessibility:** each permission row states its status in text (not colour
  alone); the capture control is a real button with an `aria-label`; the photo
  has meaningful `alt` text.
- **Offline:** the page renders cached details where possible and clearly marks
  actions that need connectivity.

## 7. UX / UI Notes

**Placement — decided 2026-08-28: a user menu in the top-right of the app bar.**

The shell's top bar carries an **avatar button** at its trailing edge, replacing
the current standalone "Log ud" button. Tapping it opens a small menu with:

1. A header row — name and role (non-interactive), so the menu also answers "who
   am I signed in as?" without navigating.
2. **Min profil** (Lucide `CircleUser`) → `/profile`.
3. A separator, then **Log ud** (Lucide `LogOut`), styled as the low-emphasis
   destructive item at the bottom.

The avatar shows the user's portrait once one exists, and their initials until
then — which doubles as a persistent, unobtrusive nudge to take one.

This replaces the earlier proposal (a nav destination in the `MoreMenu` overflow
sheet, with sign-out moved onto the profile page). Reasons for the change: it is
the conventional pattern users already expect, it keeps profile and sign-out out
of the per-role destination list — which this page and PRD 007 were about to push
past the 5 visible nav slots — and it puts sign-out somewhere predictable instead
of at the bottom of a long scrolling page.

**Consequence for the full-bleed map (PRD 002).** `/maps` hides the top bar, so
the user menu is not present there. That is accepted rather than worked around:
the map's top-right corner is already taken by the layer switcher, and the bottom
nav is always available to leave the map. Neither profile nor sign-out is an
in-map action. The `profile` destination is therefore **not** added to
`navigation.ts`.

**Component.** Use the shadcn-vue **`dropdown-menu`** primitive (to be generated;
not in `components/ui/` yet) anchored to the avatar button, with the shadcn-vue
**`avatar`** primitive for the trigger. Do not hand-roll a popover. The trigger
must meet the ≥44px tap-target rule and carry an `aria-label` ("Din profil og
konto"); the menu must be keyboard- and screen-reader-navigable, which is the
main reason for using the primitive.

**Page structure** (scrolling, standard shell with top bar):

1. **Header block** — portrait (circular, ~96–128px) centred or leading, with the
   name beside/below it and the role as a subtle badge. The page `<h1>` and each
   section heading use **`font-nathejk`** per `.rules`; body text and controls stay
   on the system sans stack.
2. **Mine oplysninger** — a read-only definition list: `Navn`, `Adresse`,
   `Telefon`, `Forælders telefon`. A footnote: how to get details corrected.
3. **På denne enhed** — preference/status rows: icon, label, status text, and a
   trailing action or guidance. Uses the same visual language as
   `PermissionPrompt.vue` (whose API is owned by PRD 005) but in a compact row form.
4. **Offline-parathed** — PRD 009's readiness surface: which datasets are cached,
   when they last synced, total storage used, and a manual sync/clear control.
   Placement decided here 2026-08-25 so it is not left open in two documents.
5. **Mine køretøjer** — the caller's registered vehicles, with edit, remove and add.
   Owned by **PRD 010** (vehicle registration); shown only for the roles that may
   bring a vehicle (bandit, gøgler, crew), so it is absent for spejder.

There is deliberately **no sign-out button on the page** — it lives in the user
menu (see Placement above).

**Capture flow.** Tapping the portrait opens a full-screen capture surface: live
camera preview with a **face-guide overlay** (a circular framing guide — the same
guide PRD 005's onboarding portrait step relies on), a shutter button, a
camera-flip control if more than one camera exists, and cancel. After the shot:
preview with "Brug billede" / "Tag igen". Upload shows inline progress on the
portrait, not a modal spinner. **This component is owned here** and reused by PRD
005 — it must not be forked.

**New / changed frontend files (in `vue/`):**

- `src/views/ProfileView.vue` — the page.
- `src/components/profile/ProfilePhoto.vue` — portrait + capture entry point +
  upload state. Composes the shadcn-vue `avatar` primitive (which must be
  generated — it is not in `components/ui/` yet); PRD 007 composes the same
  primitive for its thumbnails rather than reusing this component.
- `src/components/profile/PhotoCapture.vue` — camera preview, shutter, preview/
  confirm, client-side downscale.
- `src/components/profile/PreferenceRow.vue` — icon + label + status + action row.
- `src/stores/profile.store.ts` — fetch details, upload/replace photo.
- `src/components/UserMenu.vue` — avatar trigger + dropdown menu (name/role
  header, `Min profil`, `Log ud`). Owns the `signOut()` call currently in
  `App.vue`.
- `src/router/index.ts` — `/profile` route (auth-guarded, all roles).
- `src/App.vue` — the top bar's trailing "Log ud" button is replaced by
  `<UserMenu />`; `signOut()` moves into that component. Note **PRD 005 lands the
  shell rewrite** (`showShell`, hidden chrome on onboarding routes); this change is
  a diff on top of that, and there is exactly one sign-out action with one
  destination, named in 005.
- `src/config/navigation.ts` — **unchanged**; profile is not a nav destination.

## 8. Technical Considerations

- **Frontend (Vue 3 / TS):**
  - `profile.store.ts` calls `GET /api/me/profile` through `@/helpers`
    `fetchWrapper` (401 handling already centralised). Keep `session.store` as
    the identity source of truth; the profile store holds the richer detail.
  - Capture uses `navigator.mediaDevices.getUserMedia({ video: { facingMode:
    'user' } })` into a `<video>`, then draws to a `<canvas>` for the still and
    downscale. Fall back to `<input type="file" accept="image/*" capture="user">`
    where `getUserMedia` is unavailable or blocked — this fallback is also the
    simplest possible v1 if we want to cut scope.
  - Streams must be stopped (`track.stop()`) on confirm/cancel/unmount, or the
    camera indicator stays on.
  - Upload as `multipart/form-data`, or as a base64/JSON body — note that
    `fetchWrapper` today is JSON-oriented, so it may need a multipart-aware
    method (prefer extending it over bypassing it, to keep 401 handling).
  - Permission rows read the existing stores; call `syncPermission()` on mount
    **and** on `visibilitychange`, since a user may leave to browser settings and
    return.
  - "Blocked" guidance must be platform-aware (iOS Safari vs Android Chrome
    wording differs); keep the strings in one place.
- **BFF (Go):** per `go-bff-layout`, a new `go/cmd/api/profile.go` with handlers
  behind `app.requireAuth`, plus an `internal/data` facade for profile details
  and photo storage.
  - The current directory (`internal/users`) returns only `{ ID, Role }`. It must
    be extended to carry name, address, own phone and parent phone — **this is
    now PRD 006's job** (member directory), which replaces the mock with a real
    person-grained projection. Do not extend the mock here; depend on PRD 006.
    Note its survey found `PhoneParent` exists **only** on `spejder`, so
    `Forælders telefon` must render as "not applicable" rather than blank for
    bandits, gøgler and crew.
  - **The photo does not get written to the database by this handler.** Per the
    architecture rule (PRD 008 §8: nothing writes directly to SQL), upload
    **publishes a portrait event** carrying a content hash plus metadata, and the
    bytes go to a **content-addressed blob store**. The SQL row is a *projection* of
    that event. *(Rewritten 2026-08-25 — this section previously described a direct
    MariaDB row + blob write, which predates the rule.)*
  - The blob store is the one thing **not rebuildable from the log**, so it is the
    entire backup scope (PRD 008 §8) and must stay clear of projection tables — a
    replay must never be able to truncate portraits.
  - The Go binary has no database or publisher today (dev provisions MariaDB but
    nothing uses it; `commands.Commands` is an empty struct), so both arrive with
    **PRD 008**.
    Generate an identification **thumbnail** at upload time too — PRD 007 syncs
    those to devices for offline use and needs one canonical small version. This PRD
    owns thumbnail generation, since it owns upload.
  - Serve the photo through an authenticated handler rather than a static public
    path, so the URL is not a bearer-less capability.
- **API endpoints (OpenAPI annotations mandatory, matching the style in
  `go/cmd/api/auth.go` and `push.go`):**
  - `GET /api/me/profile` — name, address, phone, parent phone, role, photo
    presence/URL. `200` / `401`. **PRD 005 extends this response** with
    `verified_at` and a `confirmation_required` flag plus a masked contact number
    for its first-login confirmation step — design the payload with that in mind.
    *(Field name aligned 2026-08-25: `verified_at` everywhere, not `confirmed_at`.)*
    This field is **not owned here**: it is projected from PRD 005's verification
    event onto PRD 006's person read model, and this endpoint only reads it.
  - `PUT /api/me/photo` — upload/replace the portrait (multipart). Publishes the
    portrait event and stores the bytes. `200` / `400` (bad type/too large) / `401`
    / `413`.
  - `GET /api/me/photo` — return the caller's **own** portrait at profile size (or
    `404`). Authenticated. Note the overlap with PRD 007's
    `GET /api/portraits/{personId}`: that endpoint serves *identification
    thumbnails of other people* under 007's access matrix, and own-portrait requests
    must **not** be routed through it — its `403`/`404` responses are deliberately
    indistinguishable, which is wrong for "my own photo".
  - `DELETE /api/me/photo` — only if removal is in scope (Open Questions). Must
    publish a portrait-removed event, not delete a row.
- **Data / storage:**
  - A `portrait` **projection** keyed by person id: `content_hash`,
    `content_type`, `width`, `height`, `updated_at`. **No `bytes` column** — bytes
    never live in a row a replay would truncate (PRD 008 §8).
  - Blob location: object store or mounted volume, content-addressed. **A DB blob
    is not an option** under PRD 008's backup model. The decision is owned by PRD
    008 §11 (infrastructure), not here.
  - Read-only details are **not** written by us; they arrive as a projection of
    the member stream owned by PRD 006 (mocked until then).
  - Retention/purge job for photos after the event, per the GDPR decision.
- **Dependencies & risks:**
  - **Blocked by PRD 006** (directory fields) for track (a), and by **PRD 006 +
    PRD 008** (persistence, publisher, blob store) for track (b). Neither
    dependency was listed here before 2026-08-25.
  - `getUserMedia` requires a secure context — satisfied by the Traefik dev setup
    and prod.
  - iOS Safari camera quirks (autoplay/`playsinline` requirements, permission
    prompts inside standalone PWAs) are the main implementation risk; test on a
    real device early.
  - EXIF orientation: photos may come in rotated; canvas re-encode must respect
    orientation or faces will be sideways.
  - Sensitive-data handling of minors' portraits is the main *product* risk —
    consent and retention must be settled before launch, not after.
  - Depends on the real Nathejk directory for the details to be truthful; against
    the mock, the page shows seeded data.

## 9. Success Metrics

- A signed-in user sees their own correct name, address, phone and parent phone
  (verified against the directory/mock seed).
- A user can take a portrait on iOS Safari (installed PWA) and Android Chrome and
  see it persist after a reload and on a second device.
- Uploaded images are stored re-encoded, EXIF-stripped, and under the size limit;
  oversized or non-image uploads are rejected with a clear error.
- Every permission row shows a state that matches the real device state,
  including after the user changes it in browser settings and returns.
- A user who previously denied notifications can, following only the on-page
  guidance, get the notification row to "til".
- No unauthenticated request can retrieve a portrait.

## 10. Rollout / Task Breakdown

Two loosely coupled tracks: (a) details + permission status, which is
low-risk and shippable on its own, and (b) photo capture + storage, which carries
the platform and privacy risk. Ship (a) first so the page exists, then (b).

The GDPR/consent decision is a **blocker for (b)**, not for (a).

Proposed tasks to create in `roadmap/tasks/open/`:

- [ ] Task: Add name, address, own phone and parent phone to the
      `users.Directory` **interface** (mock updated as a test double only) — PRD 006
      supplies the real implementation. Do not grow the mock into a data source.
- [ ] Task: BFF — `GET /api/me/profile` handler behind `requireAuth`. OpenAPI
      annotations.
- [ ] Task: Frontend — `/profile` route, `ProfileView.vue` skeleton +
      `profile.store.ts`.
- [ ] Task: Frontend — read-only details block (Danish labels, `tel:` links,
      "Ikke registreret" and hidden-row rules).
- [ ] Task: Frontend — `PreferenceRow.vue` + "På denne enhed" section wired to
      `notifications.store`, `location.store`, `install.store` (PRD 005) and the
      camera state, incl. re-sync on `visibilitychange`.
- [ ] Task: Frontend — `notifications.store.syncSubscription()` reading the live
      `PushSubscription`, so the push row is accurate after a reload (today
      `subscribed` is only set inside `enable()`).
- [ ] Task: Generate the shadcn-vue primitives this page needs (`avatar`,
      `dropdown-menu`, and `badge` if used) in `vue/src/components/ui/`.
- [ ] Task: Frontend — `UserMenu.vue` in the top-right of the app bar (avatar
      trigger with portrait/initials, name+role header, `Min profil`, `Log ud`),
      replacing the standalone "Log ud" button in `App.vue`.
- [ ] Task: Frontend — platform-aware "blocked permission" guidance copy in one
      place.

- [ ] Task: Decide + document photo consent, retention and access policy (GDPR
      blocker for the photo tasks).
- [ ] Task: BFF — portrait event + `portrait` projection + content-addressed blob
      store write path via `commands.Commands` (read side exposed through
      `data.Models`). No direct SQL write.
- [ ] Task: BFF — server-side thumbnail generation at upload (EXIF-correct, fixed
      size) for PRD 007's offline sync.
- [ ] Task: BFF — `PUT /api/me/photo` (validate type/size, re-encode, strip EXIF)
      and `GET /api/me/photo` (authenticated). OpenAPI annotations.
- [ ] Task: Frontend — `PhotoCapture.vue` (getUserMedia preview, shutter,
      retake/confirm, downscale + orientation fix, stream teardown) with a
      `<input capture>` fallback.
- [ ] Task: Frontend — `ProfilePhoto.vue` upload flow with progress, retry and
      error states; extend `fetchWrapper` for multipart if needed.
- [ ] Task: Device testing pass — iOS Safari standalone PWA + Android Chrome
      (camera, permissions, orientation).
- [ ] Task: Photo retention/purge job per the agreed policy.

## 11. Open Questions

- **Consent & retention for portraits** — participants are frequently minors.
  What is the legal basis, is parental consent needed (and captured where?), how
  long are photos kept, and who besides the user may see them? **Escalated
  2026-08-25:** PRD 005 makes the portrait operational (night-time identification)
  and captures it during onboarding, and PRD 007 defines the audience — so this is
  now blocking for three PRDs. "Who may see them" is answered by PRD 007's access
  matrix; what is missing is the legal basis for that audience.
- **Photo purpose** — **answered 2026-08-25:** identification of members during
  the race, much of which happens at night when faces are hard to see. Not
  decorative. This is why the consent question above is blocking, and why capture
  moved into onboarding (PRD 005).
- **Camera roll** — allow choosing an existing picture, or camera capture only?
  Now that the purpose is identification (see above), a current camera capture is
  clearly preferable to an arbitrary library photo; the `<input capture="user">`
  fallback should stay a fallback.
- **Cropping / framing** — *decided 2026-08-25*: a fixed circular crop with a
  **face guide** overlay (the guide PRD 005's onboarding step assumes). Pinch-to-zoom
  cropping stays out of scope unless testing shows the guide is not enough.
- **Removal** — may a user delete their photo (`DELETE /api/me/photo`), or only
  replace it?
- **Blob storage** — object store vs mounted volume, content-addressed. *Owned by
  PRD 008 §11 Q4*, tracked here only because it affects this page's upload path.
- **Details source** — which Nathejk record fields map to `Adresse` and
  `Forælders telefon`? Members currently have **one** contact on file (confirmed
  2026-08-25, see PRD 005 §11), so the two-guardian case is not a concern today —
  but the field naming should not assume it stays that way.
- **Editability** — confirmed non-editable for now, though a few fields are
  likely to open up later; is there an intended correction channel we should link
  to (a phone number, an email, the leader)? **PRD 005 needs this answer too** —
  its confirmation step must tell a user what to do when a number is wrong. If
  editing does open up, a number change should invalidate PRD 005's confirmation.
- **Nav placement** — *decided 2026-08-28*: **a user menu in the top-right of the
  app bar** (avatar → `Min profil` + `Log ud`), superseding the earlier decision to
  use an overflow-sheet nav destination with sign-out on the page. Profile is
  therefore not a nav destination at all. Still open, and unowned by any single
  PRD: the **per-role destination order** for the bottom nav — PRD 007's
  identification view can still push service roles past the 5 visible slots. PRD
  001 §11 lists this as open; taking profile out of the list relieves the pressure
  but does not settle it.
- **Other preferences** — the request anticipates more preferences once the app is
  fully scoped. Candidates to confirm: notification categories (event updates vs
  urgent only), quiet hours, language, text size, "share my position with my
  patrol/leader" (needs its own PRD), and PWA install state. Which, if any,
  belong in v1?
- **Location sharing wording** — "location sharing" currently means only "the map
  may read my position locally"; nothing is shared with the server. Should the
  row say so explicitly, to avoid implying server-side tracking?
