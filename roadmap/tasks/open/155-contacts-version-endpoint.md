# 155 — `GET /api/contacts/version` (freshness poll)

**Status:** open
**Priority:** high
**Created:** 2026-08-31

## Description

Backs the during-event freshness requirement (PRD 007 §6, §8 "Keeping the directory
fresh"): directory changes must be visible immediately on foreground and within ~60 s
while the app is open.

Returns a **monotonic version for the caller's permitted set and nothing else**. The
client polls this and only fetches the manifest delta when the version differs.

Why a separate endpoint rather than polling the manifest: this is called by a few
hundred devices every 60 s while the app is open, and it is the app's first
**continuous** during-race traffic — landing on the same BFF as PRD 002's position
reporting. It must be trivially cheap: a small integer or hash, `ETag`-able, answered
from a projection read rather than any recomputation.

Push is **not** an option for invalidation: iOS 16.4+ web push requires every push to
produce a user-visible notification (see `vue/public/push-sw.js`, which always calls
`showNotification`), so it would either buzz phones over a corrected phone number or
risk the permission. Documented in PRD 007 §8.

## Acceptance Criteria

- [ ] `GET /api/contacts/version` behind `app.requireAuth`, returning a version scoped
      to the caller's permitted set.
- [ ] Version changes when anything in that set changes — including a withdrawal, a
      number purge, or a new portrait.
- [ ] Version does **not** change when something outside the caller's set changes.
- [ ] `ETag` / `304` supported.
- [ ] Answered from a projection read; no scan of the whole person table per request.
- [ ] Spejder gets `403`; unauthenticated `401`.
- [ ] OpenAPI annotations present.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §6 / §8.
