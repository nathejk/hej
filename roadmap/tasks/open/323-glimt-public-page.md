# 323 — /offentligt/glimt — the public page on hej.nathejk.dk

**Status:** open
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §0, §7, §8. The public feed is served **by this service**, on `hej.nathejk.dk` — not a
separate site and not an export into PRD 013.

A server-rendered page at `/offentligt/glimt`: a plain thumbnail grid with the hold attribution
(number, name, group) under each entry. No login, no bundle, no client-side state, working
without JavaScript. Plus:

- `GET /api/public/glimt` and `GET /api/public/glimt/:glimtId/media/:ordinal`
- `POST /api/public/glimt/:glimtId/report` — **unauthenticated**, rate-limited by IP. A parent
  who spots a problem is precisely the person we want to hear from, and they have no session.

Two same-origin traps, both of which must be handled rather than discovered:

1. **The service worker's navigation fallback would swallow the URL** and serve the app shell to
   an installed member following a public link. Needs the `navigateFallbackDenylist` entry
   (task 315).
2. **A logged-in member's browser will send `hej_session`** to these handlers. They must
   **ignore it entirely**. If they ever read it, the public page silently becomes a different
   page for members than for parents, and "is this public-safe?" becomes untestable.

Serves `audience = public AND hiddenAt IS NULL` and nothing else, regardless of who asks. No
personal name, no portrait, no arm number, no phone number — ever.

## Acceptance Criteria

- [ ] `/offentligt/glimt` server-rendered, works with JavaScript disabled
- [ ] Public API endpoints serve only public, non-hidden glimt
- [ ] **Test: an authenticated member's cookie changes nothing about the response**
- [ ] Test: a hidden glimt disappears from the public page
- [ ] Test: the payload contains no personal name, `personId` or phone number
- [ ] Unauthenticated report endpoint, IP rate-limited
- [ ] `navigateFallbackDenylist` entry present and verified in the built `sw.js`
- [ ] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
