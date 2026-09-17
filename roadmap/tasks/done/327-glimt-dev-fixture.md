# 327 — Glimt dev fixture: something to actually look at

**Status:** done
**Priority:** high
**Created:** 2026-09-18
**Picked up by:** agent session (Zed)
**Started:** 2026-09-18
**Completed:** 2026-09-18

## Description

**There is currently no way to see Glimt working.** The projection is empty on every dev database
because nothing has ever posted, so the feed, the card, the carousel and the composer can be built and
tested but not *reviewed* — as the maintainer put it: "no images so difficult to have an opinion on
carousel".

The map does not have this problem: `cmd/simscan` and `internal/mapfixture` exist precisely so PRD 016's
paths could be exercised on a device against real entities. Glimt has no equivalent, and needs one.

**A dev-only endpoint, not a separate binary.** `cmd/seed` publishes member events and refuses real
event years; a glimt needs *media in the blob store*, which the seeder cannot write. Going through
`POST /api/dev/...` means the fixture travels the real path — normalize, strip metadata, store
content-addressed, publish — which is what the architecture asks of seeded data (PRD 008 §8,
`cmd/seed`'s own header). Registered under `ENV=development` only, following `dev.go`'s rule: the check
is "is this route registered", not "is this caller allowed".

Images are generated in-process. No fixture assets in the repo: a photograph of a real person committed
as test data is exactly the thing this feature is careful about, and a generated pattern is enough to
judge a grid, a carousel and a thumbnail.

**Because attribution is frozen at creation (task 302), the fixture can invent holds** without needing
real person rows — only `authorPersonId` (ownership) and `authorGroup` (visibility) have to be real for
the caller to see the result.

The set should cover what the views actually have to render:

- several holds, so the hold index and grid have something to browse
- all three audiences, so the chips and the public page differ
- one and many media items, so the single-item path and the carousel both appear
- portrait, landscape and square, so the aspect-ratio handling is visible
- a long caption and an empty one
- one owned by the caller (so `Slet` and "Dit hold" appear) and the rest not (so `Anmeld` appears)
- one hidden, so the moderation view and the "Skjult" badge have a subject

## Acceptance Criteria

- [x] `POST /api/dev/glimt-fixture` exists **only** when `ENV=development`
- [x] Generates images in-process; no binary fixtures committed
- [x] Goes through the real upload + publish path, not direct SQL or direct blob writes
- [x] Covers: multiple holds, all three audiences, 1..n media, mixed orientations, own vs others
- [x] Idempotent enough to run twice without producing a mess — see the note: it is **additive**, and says so
- [x] Documented in the task log with the exact command to run it
- [x] `go test ./...` passes

## How to run it

With the dev stack up and a session in the browser (log in first, then from the same browser):

```sh
curl -X POST https://hej.local.nathejk.dk/api/dev/glimt-fixture \
  -H "Cookie: hej_session=<paste from devtools>"
```

Or simply, from the browser console while signed in:

```js
await fetch('/api/dev/glimt-fixture', { method: 'POST' }).then((r) => r.json())
```

Then open **Glimt** in the bottom nav. Six glimt appear across five holds.

## Progress Log

- 2026-09-18 00:00 — Task created. Raised by the maintainer while reviewing task 316: the carousel
  cannot be assessed with an empty feed.
- 2026-09-18 — **A dev-only endpoint, not a `cmd/seed` case.** `seed` publishes member events and
  cannot write blobs, and a glimt is a row that *references* media. A handler takes the real path —
  decode-validate, EXIF strip, re-encode, content-address, publish — which is exactly what `seed`'s own
  header argues for. Registered under `ENV=development` following `dev.go`'s rule: the check is "is
  this route registered", not "is this caller allowed", so there is no handler left behind for a
  misconfiguration to reach.
- 2026-09-18 — **The trick that makes this cheap: attribution is frozen at creation (task 302), so the
  fixture can invent holds** — "TEST Ørnene", patrulje 42–46 — without needing real person rows. Only
  `authorPersonId` (ownership) and `authorGroup` (visibility) have to be real, and both come from the
  caller. That is why this is one file rather than a member seeder plus a glimt seeder.
- 2026-09-18 — Authored **as the caller**, so the group-scoped glimt are visible to whoever ran it. A
  fixture nobody can see would be worse than none — which is what a synthetic-author-only version
  would have produced, since a `group` glimt from an invented person in an invented group is invisible
  to everybody.
- 2026-09-18 — The set is chosen to cover **what the views must render**, not to look like a lot of
  data: one single-item glimt (the non-carousel path), one with four mixed-orientation items (the
  carousel and its dots), a long caption and an empty one, one owned by the caller (so `Slet` and "Dit
  hold" appear) and the rest not (so `Anmeld` appears), two public ones for the public page, and one
  **reported** so the "Skjult" badge and the moderation queue have a subject. The test asserts each of
  those, because a fixture that quietly narrowed to one landscape image with a short caption would
  still have looked like it worked.
- 2026-09-18 — Timestamps are spread backwards over four hours. Without that every card reads "Lige
  nu" and the relative-time formatting — one of the things a fixture exists to expose — goes unseen.
- 2026-09-18 — Images are generated with `image/draw` and no font dependency. Three properties earn
  their code: a **distinct colour per glimt** so a feed card and a grid tile can be matched by eye;
  **corner markers**, which are the first thing a bad `object-fit` eats, so a wrong crop is obvious;
  and **countable blocks** for the item ordinal, legible at thumbnail size where a numeral would not
  be. No photographs are committed — a picture of a real person as test data is precisely what this
  feature is careful about.
- 2026-09-18 — The report is published as a **real report event** rather than by setting `hiddenAt`,
  so the fixture exercises the fold that hides it (task 301) instead of faking its result.
- 2026-09-18 — ✅ `gofmt` clean, `go build ./...`, full `go test ./...` green. 6 new tests.

### ✅ Verified end to end (2026-09-18)

Run against the live dev stack. Logged in as a real 2026 spejder — the phone number was kept in a shell
variable and never printed, since it belongs to a participant who is very likely a minor.

What the run confirmed:

| | |
|---|---|
| fixture | `HTTP 201`, six glimt created |
| projection | folded correctly: 5 holds, 3 audiences, media counts 4/1/2/1/2/1 |
| media | 11 items — 6 landscape, 3 portrait, 2 square, **all 11 with thumbnails** |
| report | `hiddenAt` set and `reportCount=1` **by the fold**, not by the fixture |
| feed | 5 of 6 visible — the reported one is hidden **even from its reporter**, which is the predicate working |
| payload | no `author`, no `person_id`, no `phone`, **no blob ref** — task 302's rule holds end to end |
| media bytes | `200 image/jpeg`, `private, max-age=31536000, immutable`, content-hash ETag |
| conditional | `304` on `If-None-Match` |
| unauthenticated | `401` |
| thumbnail economics | 2,961 bytes vs 28,288 full — **~10× smaller**, which is the post-race browse's whole margin |

### 🐞 A bug the tests missed and looking at the output caught

The corner markers and blocks rendered **almost black instead of white**, on a dark purple background —
so the very thing they exist for (making a wrong `object-fit` crop obvious) was invisible.

Cause: `color.RGBA` in Go is **alpha-premultiplied**, so `{255, 255, 255, 220}` is out of gamut (R > A)
and renders dark. Every assertion in the test file passed happily — the image decoded, was the right
size, and differed per glimt. None of them looked at a pixel.

Fixed to opaque white, and the markers and blocks were enlarged after seeing them at 320px: the first
version's blocks were a few pixels across at the size they are actually read.

Added `TestDevFixtureImage_MarkersAreActuallyLight`, which samples a corner pixel against the
background across all six palette entries. It would have failed at luma 0.

**This is the whole argument for the task.** A fixture that exists so a feature can be looked at only
works if somebody looks — and the first thing looking found was a bug in the fixture itself.

### Note on the current dev database

The fixture was applied **twice** (once before the colour fix, once after), so the dev feed currently
holds **10 visible glimt**: six with dark markers and six with white ones. That is the additive
behaviour documented below, and side by side it happens to show the fix. A stream purge clears both
batches.

### What this fixture is not for

It answers **geometry**, not aesthetics. The generated patterns are built so a wrong crop, a broken
aspect ratio or a mis-sized thumbnail is *unmissable* — that is what the corner markers and countable
blocks are, and they caught two real bugs within an hour (the invisible markers here, and the
top-aligned carousel slides in task 316).

They are useless for judging whether a crop **feels** right, which is a question about a subject in a
photograph: where the faces are, whether the horizon survives, whether 15% off the top matters.
Maintainer, 2026-09-18: "difficult to assess zoom with these similar looking stock photos."

For that, **post real photographs through the composer** — it works, and it is the same upload path.
One landscape, one portrait, and one glimt mixing the two will show more about the crop rules than any
synthetic set can. Do not be tempted to commit photographs as fixtures instead: a picture of a real
person as test data is exactly what this feature is careful about.

### Note

**It is additive, not idempotent.** Running it twice creates twelve glimt, not six, because each gets a
fresh uuid — and the stream is append-only, so they cannot be un-published, only purged with the event
store. The response body says so. That is the same trade `cmd/simscan` makes and is fine for a dev
broker; it is worth knowing before running it ten times while iterating on CSS.
