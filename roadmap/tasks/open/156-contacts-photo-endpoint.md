# 156 — `GET /api/contacts/{personId}/photo?size=thumb`

**Status:** open
**Priority:** medium
**Created:** 2026-08-31

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

## Acceptance Criteria

- [ ] Endpoint behind `app.requireAuth`, authorized via task 151's function.
- [ ] `?size=thumb` serves `thumb256`; unrecognised or absent size falls back the same
      way `/me/photo` does.
- [ ] `403` and `404` are byte-identical responses.
- [ ] Reuses `portrait.go` / `photo.go` helpers; no duplicated decode/normalize logic.
- [ ] `ETag` / `304` supported so the sync engine can skip unchanged images.
- [ ] Tests: permitted fetch succeeds; non-permitted fetch is indistinguishable from a
      missing person; a spejder subject is never fetchable through this route.
- [ ] OpenAPI annotations present.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §8.
