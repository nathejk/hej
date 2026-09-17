# 304 — POST /api/glimt, GET /api/glimt/feed, GET /api/glimt/version

**Status:** done
**Priority:** high
**Created:** 2026-09-17
**Picked up by:** agent session (Zed)
**Started:** 2026-09-17
**Completed:** 2026-09-17

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

- [x] `POST /api/glimt` validates caption length, media count 1–10, and audience value
- [x] Audience defaults to `group` when absent; an unknown value is a 400
- [x] Publishes the created event; 503 when no publisher
- [x] `GET /api/glimt/feed` paginated, visibility-filtered, newest first
- [x] ~~`GET /api/glimt/version`~~ → **a `glimt` key on `/api/sync`**, which changes when a glimt is created *or hidden*. See the deviation note below.
- [x] Test: a spejder's feed excludes a crew `group` glimt
- [x] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-17 16:10 — Picked up.
- 2026-09-17 16:15 — **Deviation from PRD 019 §8, deliberate.** `routes.go` carries an explicit
  instruction above `/api/sync`: the multiplexed freshness check "replaced a per-dataset version
  endpoint (`/api/contacts/version`, retired in task 292) — do not add another one; add a key
  here." The PRD's endpoint table predates that convention being visible to its author. Added a
  **`glimt` key to `app.syncDatasets()`** backed by the `Version()` querier from task 301, and
  corrected the PRD's §8 table rather than leaving it describing something that does not exist.
  Seven endpoints would be seven requests per foreground from a few hundred devices on a mobile
  link, where round-trip count dominates payload size.
- 2026-09-17 16:25 — `glimtVersionFor` is the **only derivation in `syncversion.go` that is not
  cached**, and the comment says why: the others cache by permitted set because their answer is
  shared (every device holds the same race area), whereas a Glimt feed is per-caller by
  construction — it contains the caller's own group *and* their own hidden posts — so a cache
  would be one entry per member, growing with the event and sharing nothing. Affordable only
  because the derivation is a single indexed aggregate; if that changes it needs a cache first.
- 2026-09-17 16:35 — Caption limit is counted in **runes, not bytes**. The captions are Danish, so
  æ/ø/å cost two bytes each in UTF-8 and a byte limit would silently give a member writing "på vej
  gennem skoven" a shorter caption than one writing in ASCII. A test posts 280 æ (560 bytes) and
  expects acceptance.
- 2026-09-17 16:40 — Audience handling is a deliberate **asymmetry**: absent → the narrowest
  (`group`), unknown → 400. The safe value is what you get for free, and a client sending
  "everyone" by mistake must not be quietly read as having said `public`. Both directions tested.
- 2026-09-17 16:45 — Ordinals are assigned from the **request array order**, not taken from the
  client. A client-supplied ordinal would be a second source of truth for the same fact, free to
  disagree with the array it arrived in — and the disagreement would surface as a silently
  reordered glimt.
- 2026-09-17 16:50 — Duplicate refs in one glimt are rejected. The composer de-duplicates, so it is
  a client bug; allowing it would show the same photo twice with no way for the member to tell
  which copy to remove.
- 2026-09-17 17:00 — Added `visibleGlimt`, which re-checks **every row against `MaySeeGlimt`
  before serialising**. Belt and braces: the SQL already narrowed the query, so this should never
  drop anything — but "should never" is doing a lot of work in a feature whose failure mode is a
  group-scoped photograph of a child shown to a stranger. PRD 019 §8 says the predicate is the
  authority and the WHERE clause is an optimisation; this is where that ordering becomes true
  rather than merely stated. A test forces the stub to ignore the filter and asserts the handler
  still drops the row (and logs it).
- 2026-09-17 17:10 — `data.WithGlimt` option rather than a sixth positional parameter to
  `NewModels`, following the note there: seventeen call sites would each read a little worse while
  telling the reader nothing. `Models.Glimt` may be nil, and handlers must report that as
  **unavailable, not empty** — an empty feed is a legitimate state the client caches, so serving
  one because the database is down would leave a device showing "nothing was shared" all event.
- 2026-09-17 17:20 — `stubGlimt` applies the **real predicate** (converting back to
  `users.GlimtFeedFilter` and calling `Matches`) rather than returning everything. A stub that
  ignored the filter would make every visibility test in the package vacuous — the lesson
  `stubPeople.ListByAppRoles` already records for the contacts manifest.
- 2026-09-17 17:25 — Two existing sync tests failed once the `glimt` key existed, correctly: their
  apps have no projection, so the key was reported unavailable. Fixed by wiring `stubGlimt` into
  them, and added a test asserting a **missing projection is unavailable rather than absent** —
  absence means "you may not hold this", which a client cannot recover from.
- 2026-09-17 17:30 — Test-helper collisions with `postJSON` (auth_test.go) and `getWithCookies`
  (scans_test.go). Added `postGlimtJSON`/`readBody` rather than widening the existing helpers,
  which have a dozen callers between them.
- 2026-09-17 17:35 — ✅ All criteria met. `gofmt` clean, `go build ./...`, full `go test ./...`
  green. PRD 019 §8 updated. Moving to done.

### Note for task 313 (the client store)

The client must read freshness from **`/api/sync`'s `glimt` key**, not from a Glimt-specific version
endpoint — there isn't one. `useFreshnessLoop` already drives that check for every other dataset, so
the store hooks into the existing loop rather than adding a poller.
