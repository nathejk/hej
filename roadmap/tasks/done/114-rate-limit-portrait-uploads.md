# 114 — Rate-limit portrait uploads: 10 per hour per member

**Status:** done
**Priority:** medium
**Created:** 2026-08-29
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-29
**Completed:** 2026-08-29

## Description

`/api/track` was rate-limited (task 084); `PUT /api/me/photo` was not. An authenticated
member could loop uploads, and unlike the track endpoint each one costs real work: a
decode of up to ~20 MP, several resamples, and two or three objects written into the one
directory that must be backed up (PRD 008 §8).

Ceiling set by the maintainer: **10 uploads per hour**.

## Acceptance Criteria

- [x] Ten uploads an hour per member; the eleventh is refused with **429**.
- [x] Keyed by **user**, not IP.
- [x] Checked **before** the body is read.
- [x] The refusal message is in Danish.
- [x] Tests for the ceiling and for per-user isolation; all gates green.

## Progress Log

- 2026-08-29 — Ten an hour is generous against the real use (take a photo, dislike it,
  retake it a few times) and far below what it takes to keep a CPU busy or fill a disk,
  so the ceiling costs no legitimate member anything.
- 2026-08-29 — Keyed by **user**, following `trackLimiter`'s reasoning rather than the
  auth endpoints': participants share networks — a patrol on one phone's hotspot, a whole
  klan behind one carrier NAT — so an IP key would throttle a group because one member is
  retrying. There is a test for exactly that, because it is the mistake that would look
  fine until the event.
- 2026-08-29 — Checked **before** the body is read. That is the point of having it here: a
  member at the ceiling should not first push 8 MiB up a mobile link and then be told no,
  and the server should not spend the decode.
- 2026-08-29 — Added `RateLimitMessageResponse` rather than reusing `RateLimitResponse`.
  The existing one says "rate limit exceeded; please try again later", which is fine for
  the auth endpoints whose text the UI never shows — but `ProfilePhoto.vue` puts the
  server's message **straight in front of the member**, and that member is often twelve
  and Danish. Now: *"Du kan uploade 10 billeder i timen. Prøv igen senere."* Mirrors
  `ServiceUnavailableResponse`, which already takes a message for the same reason.
- 2026-08-29 — Deliberately **no `Retry-After` header.** The limiter is a sliding window
  and exposes no "when does the oldest hit expire?", so the only honest value available is
  the whole hour — which would tell a client to wait up to an hour when the real wait may
  be a minute. A wrong number is worse than none; if an automated client ever needs it,
  the limiter should learn to compute it properly.
- 2026-08-29 — Worth recording two properties of `internal/ratelimit` that this relies on:
  a **denied** request is not recorded, so hammering the endpoint does not push the window
  out (the wait stays bounded by the ten uploads actually made); and keys are never
  pruned, so the map grows by one entry per member who ever uploads. At a few thousand
  members that is kilobytes, and an hour-long window holds entries 60× longer than the
  existing minute-long ones — still bounded by the roster, so no janitor was added.
- 2026-08-29 — Verified live against the dev stack: uploads 1–10 returned 200, and 11 and
  12 returned `429 {"error":"Du kan uploade 10 billeder i timen. Prøv igen senere."}`.
  Note all ten produced the *same* content hash, which is content addressing working —
  ten identical uploads cost one object, so this limit is about CPU and bandwidth rather
  than disk in the repeat case.
- 2026-08-29 — ✅ `gofmt`, `go test ./...`, `vet`, `staticcheck`, `npm run type-check`,
  `docker build --target prod` all green.

## Known interaction, worth watching on the device pass

An upload that fails validation still consumes one of the ten — including a **HEIC** file
from the iOS file picker, which is rejected as "not an image" (see task 111's follow-up).
A confused member could in principle spend their hour on ten failed attempts. Judged
acceptable rather than papered over: the realistic behaviour is two or three tries, and
the fix for HEIC is client-side conversion, not a more forgiving limiter. If the device
pass shows people hitting it, the cheap change is to stop counting attempts that never got
past the decode.
