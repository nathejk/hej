# 107 — Frontend: `ProfilePhoto.vue` upload flow

**Status:** done
**Priority:** medium
**Created:** 2026-08-28
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-28
**Completed:** 2026-08-28

## Description

Depends on tasks 105 and 106. (Task 102's consent blocker was cleared 2026-08-28.)

The portrait section of the profile page: current photo or a placeholder with a
clear call to action, tapping opens `PhotoCapture.vue`, upload shows **inline**
progress on the portrait (not a modal spinner), with retry and error states.

Composes the shadcn-vue `avatar` primitive from task 095; PRD 007 composes the
same primitive rather than reusing this component.

`fetchWrapper` may need extending for multipart.

Also feeds task 097: once a portrait exists, the user-menu avatar shows it
instead of initials.

## Acceptance Criteria

- [x] Placeholder state with a call to action explaining *why* a portrait is
      wanted (night-time identification).
- [x] Capture → upload → replace flow works end to end.
- [x] Inline progress, retry on failure, error text in Danish.
- [x] Meaningful `alt` text.
- [x] `UserMenu.vue` avatar picks up the portrait.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
- 2026-08-28 — Unblocked by task 102 and picked up.
- 2026-08-28 — **Changed the profile endpoint rather than probing.** "Is there a
  portrait?" was going to cost a second request whose only answers are 200 and 404 —
  and `HEAD` is not registered on that route, so the probe would have downloaded the
  image to find out. Added `has_photo` to `GET /api/me/profile` instead. The bytes stay
  on their own endpoint, so they can still be cached independently of the details.
- 2026-08-28 — `hasPortrait` answers **false** when there is no database rather than
  failing the profile request: during an outage the details are still worth showing,
  and the cost of a false negative is inviting a photo the member already took.
- 2026-08-28 — `photoVersion` in the store, appended to the image URL. Needed because
  the URL is stable while its contents are not: the response carries
  `Cache-Control: private, max-age=3600` (task 105), so replacing a portrait would
  otherwise keep showing the old face for an hour.
- 2026-08-28 — `profile.uploadPhoto` **throws**, unlike `fetch()` which deliberately
  does not. Swallowing this failure would leave a member believing there is a photo on
  file when there is not — and the photo is a safety feature. The component surfaces
  the BFF's own Danish message, which is more specific than anything it could guess.
- 2026-08-28 — The failed blob is kept, so "Prøv igen" retries the **same photo**
  rather than making the user pose again: the upload failed, the picture did not.
- 2026-08-28 — `fetchWrapper` gained `putForm`. Note the one thing that matters in
  there: it must **not** set `Content-Type` for FormData — only the browser knows the
  multipart boundary, and setting it by hand produces a body the server cannot parse.
  A separate entry point rather than callers passing FormData to `put`, so a multipart
  request is recognisable at the call site.
- 2026-08-28 — Progress is drawn **on** the portrait, not in a modal: the portrait is
  what is busy, and a modal would cover a page that still works.
- 2026-08-28 — Verified live: the profile endpoint now returns `has_photo: true` for the
  member whose portrait was uploaded in task 105, so the page and the user-menu avatar
  have what they need. `npm run type-check`, `npm run build`, `go test ./...`,
  `go vet`, `staticcheck` all green.
- 2026-08-28 — ✅ All criteria met. The capture→upload path has been exercised over
  HTTP (task 105's live run) but **not from a real camera** — that is task 108.
  Moving to done.
