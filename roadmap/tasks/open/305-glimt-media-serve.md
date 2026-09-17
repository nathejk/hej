# 305 — GET media by ordinal — visibility-checked, immutable caching

**Status:** open
**Priority:** high
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §8. Serves a stored variant: `GET /api/glimt/:glimtId/media/:ordinal`, with
`?variant=thumb`.

**A blob URL is a capability.** An unauthenticated `GET` on a `sha256` path would make every
"group only" glimt readable by anyone holding the hash, so this handler must apply **the same
task 299 predicate as the feed** — not a looser check, not no check. This is the single most
important test in the Glimt backend.

Media is content-addressed and therefore immutable: serve it with a long
`Cache-Control: public, max-age=…, immutable` and support conditional requests. That is most
of the answer to the post-race load spike (PRD 019 §0a.3) and it costs one header.

Public-scope media is served from a **separate** path (`/api/public/...`, task 323) that
ignores the session cookie. This handler is the authenticated one.

## Acceptance Criteria

- [ ] `GET /api/glimt/:glimtId/media/:ordinal` serves the feed variant; `?variant=thumb` the thumbnail
- [ ] Visibility enforced through the task 299 predicate
- [ ] **Test: a `group`-scoped ref returns 403 for a member of another group**
- [ ] Test: the same ref returns 200 for a member of the author's group
- [ ] Test: a moderator gets 200 for any scope
- [ ] `immutable` cache headers set; 304 on a conditional request
- [ ] 404 for an unknown glimt or ordinal, without leaking which
- [ ] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
