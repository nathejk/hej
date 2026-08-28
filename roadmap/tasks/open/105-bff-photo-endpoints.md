# 105 — BFF: `PUT /api/me/photo` and `GET /api/me/photo`

**Status:** open
**Priority:** medium
**Created:** 2026-08-28

## Description

Blocked by task 102, depends on task 103.

Upload and retrieval of the caller's own portrait, both behind `requireAuth`.
PRD 003 §6 Non-Functional is the spec for the upload: validate content type
**and magic bytes**, enforce a hard size limit, **re-encode** rather than trust
the uploaded bytes, and strip EXIF (notably GPS).

`GET` serves only the authenticated owner's portrait, at a display-appropriate
size, and must not be publicly enumerable.

Repo rule: **OpenAPI annotations are mandatory** on both.

## Acceptance Criteria

- [ ] `PUT /api/me/photo` — multipart, magic-byte + size validation, re-encode,
      EXIF stripped, writes via the task 103 command.
- [ ] `GET /api/me/photo` — 200 for the owner, 401 unauthenticated, 404 with no
      portrait.
- [ ] No endpoint accepts a user id from the client.
- [ ] Full OpenAPI annotations on both.
- [ ] Tests: oversized upload rejected, non-image with an image content-type
      rejected, GPS EXIF absent from the stored bytes, cross-user access refused.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
