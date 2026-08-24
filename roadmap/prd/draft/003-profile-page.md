# PRD 003 — Profile Page (own details, self-portrait, device permission status)

**Status:** draft
**Author:** agent session (Zed / Claude Opus 5)
**Created:** 2026-08-24
**Last updated:** 2026-08-24
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
  `permission` / `available`. `app.store.ts` handles PWA install. A profile page
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

- **Editing personal details in the app.** Name, address and phone numbers are
  sourced from Nathejk records and are read-only here. Corrections go through the
  existing (out-of-app) channel.
- **Viewing other people's profiles or photos.** Own profile only. Organizer/
  personnel-facing photo browsing is a separate feature with its own consent and
  access story.
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
     action when available (reusing `app.store.ts`).
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

- [ ] A `Min profil` page reachable from the navigation for every signed-in role.
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
      - Installed as app — from `app.store`.
- [ ] Permission states are re-synced whenever the page becomes visible, so a
      change made in browser settings is reflected on return.
- [ ] Rows whose permission is blocked show platform-appropriate guidance instead
      of a non-functional button.
- [ ] Sign-out is available from this page (it is the conventional place for it,
      and PRD 002 removes the top bar on the map page).
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

**Placement.** Two options:

1. A destination in `vue/src/config/navigation.ts` (Lucide `User` /
   `CircleUser`), which for most roles pushes the list past 5 entries and into
   the existing `MoreMenu` overflow sheet — acceptable, since profile is not a
   frequent-use page.
2. An avatar button in the shell's top bar (replacing/next to "Log ud"), which is
   the more conventional pattern but conflicts with PRD 002 hiding the top bar on
   the map page.

Proposal: **option 1** (a nav destination, landing in the overflow sheet), with
sign-out moved onto the profile page. It keeps the shell simple and works with
the full-bleed map.

**Page structure** (scrolling, standard shell with top bar):

1. **Header block** — portrait (circular, ~96–128px) centred or leading, with the
   name beside/below it and the role as a subtle badge.
2. **Mine oplysninger** — a read-only definition list: `Navn`, `Adresse`,
   `Telefon`, `Forælders telefon`. A footnote: how to get details corrected.
3. **På denne enhed** — preference/status rows: icon, label, status text, and a
   trailing action or guidance. Uses the same visual language as
   `PermissionPrompt.vue` but in a compact row form.
4. **Log ud** — a low-emphasis destructive action at the bottom.

**Capture flow.** Tapping the portrait opens a full-screen capture surface: live
camera preview, a shutter button, a camera-flip control if more than one camera
exists, and cancel. After the shot: preview with "Brug billede" / "Tag igen".
Upload shows inline progress on the portrait, not a modal spinner.

**New / changed frontend files (in `vue/`):**

- `src/views/ProfileView.vue` — the page.
- `src/components/profile/ProfilePhoto.vue` — portrait + capture entry point +
  upload state.
- `src/components/profile/PhotoCapture.vue` — camera preview, shutter, preview/
  confirm, client-side downscale.
- `src/components/profile/PreferenceRow.vue` — icon + label + status + action row.
- `src/stores/profile.store.ts` — fetch details, upload/replace photo.
- `src/config/navigation.ts` — new `profile` destination.
- `src/router/index.ts` — `/profile` route (auth-guarded, all roles).
- `src/App.vue` — sign-out relocated (or duplicated) if we go with option 1.

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
    be extended (interface + mock, same pattern as PRD 001) to carry name,
    address, own phone and parent phone — or a second lookup added. PRD 002 also
    needs the directory extended (patrol identity), so coordinate the two.
  - The photo is *app-owned* data, not directory data: it is written by the app
    and therefore needs real persistence (MariaDB row + blob on disk/object
    store), unlike the mocked read-only details.
  - Serve the photo through an authenticated handler rather than a static public
    path, so the URL is not a bearer-less capability.
- **API endpoints (OpenAPI annotations mandatory, matching the style in
  `go/cmd/api/auth.go` and `push.go`):**
  - `GET /api/me/profile` — name, address, phone, parent phone, role, photo
    presence/URL. `200` / `401`.
  - `PUT /api/me/photo` — upload/replace the portrait (multipart). `200` / `400`
    (bad type/too large) / `401` / `413`.
  - `GET /api/me/photo` — return the caller's portrait (or `404`). Authenticated.
  - `DELETE /api/me/photo` — only if removal is in scope (Open Questions).
- **Data / storage:**
  - New table, e.g. `user_photo` — `user_id` (PK), `content_type`, `bytes` or a
    storage key, `width`, `height`, `created_at`, `updated_at`.
  - Blob location: DB blob (simplest, given the small user count and short
    retention) vs filesystem volume vs object storage — decision needed. In the
    docker dev stack a filesystem path needs a volume; a DB blob avoids that.
  - Read-only details are **not** persisted by us; they are read from the
    directory (mock for now).
  - Retention/purge job for photos after the event, per the GDPR decision.
- **Dependencies & risks:**
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

- [ ] Task: Extend the user directory interface + mock with name, address, own
      phone and parent phone (coordinate with PRD 002's patrol identity).
- [ ] Task: BFF — `GET /api/me/profile` handler behind `requireAuth`. OpenAPI
      annotations.
- [ ] Task: Frontend — `/profile` route, `profile` nav destination (Lucide),
      `ProfileView.vue` skeleton + `profile.store.ts`.
- [ ] Task: Frontend — read-only details block (Danish labels, `tel:` links,
      "Ikke registreret" and hidden-row rules).
- [ ] Task: Frontend — `PreferenceRow.vue` + "På denne enhed" section wired to
      `notifications.store`, `location.store` and `app.store`, incl. re-sync on
      `visibilitychange`.
- [ ] Task: Frontend — platform-aware "blocked permission" guidance copy in one
      place.
- [ ] Task: Move/duplicate sign-out onto the profile page.
- [ ] Task: Decide + document photo consent, retention and access policy (GDPR
      blocker for the photo tasks).
- [ ] Task: BFF — `user_photo` storage (table + blob strategy) via an
      `internal/data` facade.
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
  long are photos kept, and who besides the user may see them? This must be
  answered before the photo track is implemented.
- **Photo purpose** — the request says "cover photo … to add to your profile".
  Is it purely decorative/for the user, or is it intended for personnel to
  identify people? The answer changes both the consent story and who can read it.
- **Camera roll** — allow choosing an existing picture, or camera capture only
  (to guarantee it is a current self-portrait)?
- **Cropping** — is a fixed square/circle crop with a "fit the frame" guide
  enough, or do we need a pinch-to-zoom crop step?
- **Removal** — may a user delete their photo (`DELETE /api/me/photo`), or only
  replace it?
- **Blob storage** — DB blob, mounted volume, or object storage? (Affects
  `docker-compose.yml` and backup.)
- **Details source** — which Nathejk record fields map to `Adresse` and
  `Forælders telefon`, and what happens when a participant has two guardians
  registered?
- **Editability** — confirmed non-editable for now; is there an intended
  correction channel we should link to (a phone number, an email, the leader)?
- **Nav placement** — profile as a nav destination (overflow sheet) vs an avatar
  in the top bar? (Interacts with PRD 002's full-bleed map.)
- **Other preferences** — the request anticipates more preferences once the app is
  fully scoped. Candidates to confirm: notification categories (event updates vs
  urgent only), quiet hours, language, text size, "share my position with my
  patrol/leader" (needs its own PRD), and PWA install state. Which, if any,
  belong in v1?
- **Location sharing wording** — "location sharing" currently means only "the map
  may read my position locally"; nothing is shared with the server. Should the
  row say so explicitly, to avoid implying server-side tracking?
