# 304 — POST /api/glimt, GET /api/glimt/feed, GET /api/glimt/version

**Status:** open
**Priority:** high
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §8. Create a glimt and read the feed.

**Create.** Caption (≤ 280 chars, optional), audience (`group` | `nathejk` | `public`,
defaulting to `group`), and 1–10 ordered media refs from task 303. Publishes
`NATHEJK.<year>.glimt.<glimtId>.created` through `internal/commands` — which returns
`ErrNoPublisher` → **503** when the stream is down, never a fake success. Audience is
immutable after creation; there is no update endpoint.

**Feed.** Chronological, newest first, paginated, filtered through the task 299 predicate for
the caller. Includes the caller's own glimt always.

**Version.** A cheap freshness probe for the client's `refreshIfStale()`. `version` goes in
the **response body**, not an ETag — `fetchWrapper` does not expose response headers.

## Acceptance Criteria

- [ ] `POST /api/glimt` validates caption length, media count 1–10, and audience value
- [ ] Audience defaults to `group` when absent; an unknown value is a 400
- [ ] Publishes the created event; 503 when no publisher
- [ ] `GET /api/glimt/feed` paginated, visibility-filtered, newest first
- [ ] `GET /api/glimt/version` returns a version in the body that changes when a glimt is created or hidden
- [ ] Test: a spejder's feed excludes a crew `group` glimt
- [ ] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
