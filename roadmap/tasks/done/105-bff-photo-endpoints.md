# 105 — BFF: `PUT /api/me/photo` and `GET /api/me/photo`

**Status:** done
**Priority:** medium
**Created:** 2026-08-28
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-28
**Completed:** 2026-08-28

## Description

Depends on task 103. (Task 102's consent blocker was cleared 2026-08-28 — note that
it relaxes **none** of the safeguards below: consent is what justifies holding the
photo, not a reason to hold it carelessly.)

Upload and retrieval of the caller's own portrait, both behind `requireAuth`.
PRD 003 §6 Non-Functional is the spec for the upload: validate content type
**and magic bytes**, enforce a hard size limit, **re-encode** rather than trust
the uploaded bytes, and strip EXIF (notably GPS).

`GET` serves only the authenticated owner's portrait, at a display-appropriate
size, and must not be publicly enumerable.

Repo rule: **OpenAPI annotations are mandatory** on both.

## Acceptance Criteria

- [x] `PUT /api/me/photo` — multipart (**and** a raw body), size limit, decode-based
      validation, re-encode, EXIF stripped, writes via the task 103 command.
- [x] `GET /api/me/photo` — 200 for the owner, 401 unauthenticated, 404 with no
      portrait.
- [x] No endpoint accepts a user id from the client.
- [x] Full OpenAPI annotations on both.
- [x] Tests: oversized upload rejected, non-image with an image content-type
      rejected, metadata absent from the stored bytes, non-hash ref refused.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
- 2026-08-28 — Unblocked by task 102 and picked up.
- 2026-08-28 — **Validation is the decode, not the header.** The declared
  `Content-Type` is not consulted at all: `image.Decode` sniffs the actual bytes, so
  "magic bytes" is satisfied structurally rather than by a byte-prefix table someone
  has to maintain. Every upload is re-encoded from pixels to JPEG, which does three
  jobs at once — EXIF (incl. GPS) cannot survive, a polyglot file cannot survive, and
  the stored format is known rather than "whatever arrived". Worth noting the shape of
  this: **stripping EXIF is a consequence of re-encoding, not a separate step**, so it
  cannot be forgotten in a later refactor.
- 2026-08-28 — Size limit enforced on the **reader** (`http.MaxBytesReader`), not on
  `Content-Length`: a header is a claim, not a fact. 4 MiB, which is ~8× what the
  client's downscaled upload needs but leaves room for an un-updated client or a raw
  camera file from the `<input capture>` fallback.
- 2026-08-28 — Accepts a raw body as well as multipart. Not scope creep: the fallback
  path and every non-browser client send the file as the body, and it removes a class
  of "works in the app, not from the shell".
- 2026-08-28 — Server-side downscale to a 1024px longest edge, aspect preserved, never
  upscaled. Redundant with the client on the happy path and deliberately so — the
  server cannot assume the client did it, and PRD 007 sizes its offline cache against
  what is *stored*. Nearest-neighbour by hand rather than adding a resize dependency,
  because this is the exceptional path; task 104 needs real resampling and can bring
  the library with a visual check.
- 2026-08-28 — Noted a real limitation in the code rather than papering over it:
  re-encoding drops the EXIF **orientation** tag, so applying rotation is the client's
  job (it has the tag and the canvas). The `<input capture>` fallback is the case to
  watch on real devices — flagged to task 108.
- 2026-08-28 — GET reads the ref from the **caller's own projection row**; there is no
  user id anywhere in either route. Cross-person viewing is PRD 007's, with an access
  matrix and an audit log, and the handler doc says explicitly that it must not arrive
  here as a parameter. `Cache-Control: private` — this is a photograph of a minor and
  must not sit in a shared cache.
- 2026-08-28 — Three states kept distinct because conflating them lies to the user:
  **404** no portrait on file (incl. bytes gone missing — PRD 008 §8's "degrade to no
  photo, never fail"), **503** no database so portraits are *unavailable* rather than
  absent, **503** on a failed publish because the photo was not saved and the request
  is retryable.
- 2026-08-28 — Wiring: `data.Models` gained `People person.Queries` (nil-able, via a
  `peopleOrNil` adapter — same nil-interface trap as `raceAreasOrNil`). Deliberately
  **not** added to `users.User`: that struct is handed to the login chooser, which
  shows one holder of a shared number something about the others, so every field on it
  must be safe to show a stranger.
- 2026-08-28 — **Verified end to end against the live dev stack, not just in tests:**
  * logged in as a real projected member (PIN via the dev SMS log sender);
  * `PUT` with a **PNG** → stored as `512x512 JPEG`, `GET` served it back as
    `image/jpeg` (`file` confirms the codec, so the re-encode demonstrably happened);
  * non-image bytes sent as `Content-Type: image/jpeg` → **400**;
  * blob on disk is `-rw-------` in a `drwx------` directory (task 062's fix holding);
  * the projection row carries the ref and `portraitCapturedAt`.
- 2026-08-28 — **The check worth the most:** cleared `portraitRef` in the database and
  restarted the API. The replay restored the ref **and the original
  `portraitCapturedAt` to the second** — so the event really is on `NATHEJK`, the
  projection converges from the log alone, and the timestamp is replay-stable (which
  is exactly what task 109's purge depends on). Healthcheck after: 21 subjects,
  2 projections, **0 dead letters**.
- 2026-08-28 — ✅ All criteria met. `gofmt -l`, `go test ./...`, `go vet ./...`,
  `staticcheck ./...` green. Moving to done.
