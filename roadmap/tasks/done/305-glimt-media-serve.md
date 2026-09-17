# 305 — GET media by ordinal — visibility-checked, immutable caching

**Status:** done
**Priority:** high
**Created:** 2026-09-17
**Picked up by:** agent session (Zed)
**Started:** 2026-09-17
**Completed:** 2026-09-17

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

- [x] `GET /api/glimt/items/:glimtId/media/:ordinal` serves the feed variant; `?variant=thumb` the thumbnail
- [x] Visibility enforced through the task 299 predicate
- [x] **Test: a `group`-scoped ref returns 403 for a member of another group**
- [x] Test: the same ref returns 200 for a member of the author's group
- [x] Test: a moderator gets 200 for any scope
- [x] `immutable` cache headers set; 304 on a conditional request
- [x] 404 for an unknown glimt or ordinal, without leaking which
- [x] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-17 17:45 — Picked up. Modelled on `streamPortrait`.
- 2026-09-17 17:55 — **⚠️ Route path changed from what PRD 019 §8 specifies.** httprouter *panics at
  construction* if a wildcard segment sits beside static siblings, and `/api/glimt/` already has
  `feed` and `media`. Confirmed by the panic, not by reading docs. So the path is
  `/api/glimt/items/:glimtId/media/:ordinal`. The contacts pane hit the same wall and answered it
  the same way (`/api/contacts/people/:personId/photo`) — there is a comment in `routes.go` about
  it that I had read earlier in the day and still walked into. PRD §8's table updated for **every**
  per-glimt path, since delete, report and hide/unhide (tasks 306–308) would each hit the same
  panic.
- 2026-09-17 18:05 — ✅ The critical test passes: a bandit asking for a spejder `group`-scoped item
  gets **403**, with a well-formed URL and the object present. Verified it actually *runs* rather
  than skipping — it asserts the fixture at `+4530000002` really is a bandit and skips loudly if
  the mock directory ever changes, so it cannot silently degrade into a same-group test that
  passes for the wrong reason.
- 2026-09-17 18:10 — Chose **403, not 404**, for a refusal. The glimt id came from somewhere — a
  forwarded link, a screenshot — so pretending it does not exist buys nothing, while an honest
  refusal is what tells a member their photo was not shared with this person. The id is not the
  secret; the bytes are. Also asserted no JPEG magic bytes appear in a 403 body.
- 2026-09-17 18:15 — Cache headers: `private, max-age=31536000, immutable` plus the content hash as
  the ETag. `immutable` is what actually removes requests at the finish line (PRD 019 §0a.3);
  `private` is non-negotiable even for public-scope media on this route, because a shared cache
  keyed on URL alone would serve one member's group-scoped photo to the next caller of the same
  URL. The public page's own unauthenticated route (task 323) is where a shared cache belongs, and
  that is a large part of why it exists separately.
- 2026-09-17 18:20 — The 304 is answered **before** opening the object, so a conditional request
  costs no filesystem read. Browsers send it on every navigation back into a grid they already
  hold, which during the post-race browse is most requests.
- 2026-09-17 18:25 — `?variant=thumb` falls back to the full item when no thumbnail exists rather
  than 404ing. Not laziness: task 303 deliberately lets a thumbnail fail without failing the
  upload, so thumbnail-less items really occur, and a 404 would break a tile whose photo is fine.
- 2026-09-17 18:30 — Also covered: hidden media refused to others but **not to its author**;
  public-scope media still 401 on this authenticated route; missing bytes degrading to 404 per
  PRD 008 §8; and a path-shaped ref that somehow reached a row never becoming a filesystem path.
- 2026-09-17 18:35 — ✅ All criteria met. `gofmt` clean, `go build ./...`, full `go test ./...`
  green. Moving to done.

### Note for tasks 306, 307, 308, 319 and 323

Use `/api/glimt/items/:glimtId/...` for anything addressing a single glimt, and `/api/glimt/hold/...`
for the collection index. Adding `/api/glimt/:glimtId/...` will **panic the router at startup**, not
fail a test — so it would take the whole service down rather than one endpoint.
