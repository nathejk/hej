# 156 — `GET /api/contacts/{personId}/photo?size=thumb`

**Status:** done
**Priority:** medium
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

## Description

Serves one directory member's portrait thumbnail, authorized per request (PRD 007 §8).

Reuse the existing machinery rather than building a parallel path: thumbnails are
already generated and stored at upload (task 104, `thumb256` at ~4.5 KB; task 110 added
the multi-rendition `person.PortraitThumb` list), and `go/cmd/api/photo.go` already
serves `/me/photo?size=thumb`. **Follow that `?size=` convention** rather than inventing
a second way to ask for a rendition.

`403` and `404` must be **indistinguishable**, or the endpoint becomes an oracle telling
a bandit which ids are gøglere.

Never a static path — an authenticated handler, so the URL is not a bearer-less
capability (the rule PRD 003 established).

## Implementation

Handler in `go/cmd/api/contacts.go`, tests in `contactsphoto_test.go`, route in
`routes.go`. Extracted `portraitRefForSize` and `streamPortrait` from `photo.go` so both
handlers share them.

**Route is `/api/contacts/people/{personId}/photo`, not `/api/contacts/{personId}/photo`.**
Forced: httprouter refuses a wildcard segment alongside static siblings, and
`/api/contacts/manifest` and `/api/contacts/version` already occupy that position. It
would panic at startup. The extra segment also reads better beside the coming
`/api/contacts/patrols/...`, and task 167's profile route will sit at
`/api/contacts/people/{personId}`. PRD 007 §8's path has been updated to match.

**Defaults to `thumb`, unlike `/me/photo` which defaults to `full`.** Deliberate
divergence in the *default* while keeping the same `?size=` vocabulary: the directory only
ever needs a face at list or dialog size, and defaulting to full would mean a careless
client shipping ~800 KB portraits of colleagues to every device. Tested, because this is
the kind of default that gets "tidied up" for consistency later.

**One refusal for every reason.** No such person, not permitted, no portrait, and missing
bytes all return the same 404 with the same body — asserted by comparing status *and* body
between a refusal and a genuine absence. Extracting `streamPortrait` helped here: there is
one exit path, so it is hard to accidentally introduce a distinguishable one.

**ETag is the content hash**, checked before the blob read, so an unchanged portrait costs
no disk I/O. That is what lets the sync engine skip images it already holds without a size
or date heuristic.

## Acceptance Criteria

- [x] Endpoint behind `app.requireAuth`, authorized via task 151's function
      (`mayListSubject`, which returns true if any of the subject's populations is
      listable).
- [x] `?size=thumb` serves `thumb256`; unrecognised or absent size falls back the same
      way `/me/photo` does — shared code, so they cannot drift.
- [x] `403` and `404` are byte-identical responses.
- [x] Reuses `portrait.go` / `photo.go` helpers; no duplicated decode/normalize logic.
- [x] `ETag` / `304` supported so the sync engine can skip unchanged images.
- [x] Tests: permitted fetch succeeds; non-permitted fetch is indistinguishable from a
      missing person; a spejder subject is never fetchable through this route (asserted
      for all four viewer roles, including that the bytes never appear in the body).
- [x] OpenAPI annotations present.

## Progress Log

- 2026-08-31 17:30 — Picked up. Plan: extract the rendition/streaming logic from photo.go,
  then add the authorized handler.
- 2026-08-31 17:45 — Hit an httprouter constraint: a `:personId` wildcard cannot sit beside
  the static `manifest`/`version` routes. Moved to `/api/contacts/people/:personId/photo`
  and will use the same prefix for the profile route.
- 2026-08-31 17:55 — Chose `thumb` as this endpoint's default size, diverging from
  `/me/photo`'s `full`, with a test to stop it being "corrected" later.
- 2026-08-31 18:05 — ✅ All criteria met. Indistinguishability asserted on status and body,
  not just status. `go vet ./...` and `go test ./...` pass; `gofmt` clean.
