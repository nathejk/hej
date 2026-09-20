# 333 — Album storage and media ingest, with a deliberate photo coordinate

**Status:** done
**Priority:** high
**Created:** 2026-09-19
**Picked up by:** agent session (Zed)
**Started:** 2026-09-19
**Completed:** 2026-09-19

## Description

PRD 011 §6 (section 1), §8. The data layer behind the curated albums: 3–5 albums of organizer-chosen
photographs, some of which carry a location that gets plotted on the maps (task 342).

**Two small tables**, each owned by its projection (PRD 008 §8):

- album — slug, title, description, sort order, published
- album item — album, ordinal, blob refs, caption, optional lat/lng, bounds verdict

Media goes through the existing `internal/blob` and `internal/imaging` path, which content-addresses
and makes a 320px thumbnail. Reuse it; do not add a second ingest path.

**The GPS question, which is the whole reason this task is not routine.** The existing pipeline
(`go/cmd/api/glimtmedia.go`) deliberately destroys EXIF — including GPS — by re-encoding to JPEG, and
**that must keep happening**. A curated photo's coordinate is therefore read from the original bytes
*before* re-encoding and written to a column.

The distinction matters and is worth stating plainly: **a coordinate in a column is a decision
somebody made and can review, correct or delete; a coordinate hidden in a file is a leak waiting to
happen.** Never let EXIF survive into stored bytes as a shortcut to getting the location.

**Bounds-check against the race area.** A phone with no fix, or a coordinate off in the North Sea,
produces a pin in the wrong place on a public page — worse than no pin. Store what was read, record
the verdict, and plot only what falls inside. A curator must be able to see that an item was rejected
rather than wonder why it is not on the map.

**Album media is organizer-owned and must not be subject to glimt retention.** This is a real trap:
PRD 019's purge job walks the blob store. Check `glimtpurge.go` before wiring ingest, and make sure
album blobs cannot be collected by it.

Curation tooling is PRD 011 §11 Q5 and not yet decided — so this task delivers storage and ingest with
a seam a tool can sit on later, not a UI.

## Acceptance Criteria

- [x] Album and album-item tables created, owned by their projection, following the `checkgroup` /
      `scan` table conventions (including the commentary style — these files explain *why*).
- [x] Ingest reuses `internal/blob` and `internal/imaging`; no second content-addressing or
      thumbnailing implementation.
- [x] EXIF is still stripped by re-encoding. A test asserts stored bytes carry no GPS.
- [x] The coordinate is read from the original bytes before re-encoding and stored in its own nullable
      column, editable and deletable independently of the image.
- [x] Coordinates are bounds-checked against the race area; the verdict is stored, and an
      out-of-bounds item is retrievable as such rather than silently unplotted.
- [x] Album blobs are **excluded from the PRD 019 glimt purge** — asserted by a test, not by reading
      the purge code and concluding it is fine.
- [x] Ordering within an album is explicit (curator-set), not incidental.
- [x] Unpublished albums are invisible to every public read.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §8 / §10 (Phase 1).
- 2026-09-19 — Picked up. Plan: `ReadGPS` in `internal/imaging`, an `album` projection alongside
  `glimt`, ingest in `cmd/api` reusing the glimt media path, and the purge fix.
- 2026-09-19 — **The purge trap is sharper than the PRD stated, and it is a real latent bug.** PRD 011
  §8 warned that "the purge job walks the blob store". It does not — it iterates *glimt rows*. The
  actual hazard is `RefsUsedElsewhere`, the shared-reference check in `glimtdelete.go`, which asks only
  the **glimt** table. Blobs are content-addressed, so identical bytes are one object, and an organizer
  curating an album from a photograph a participant also posted publicly is the *expected* workflow —
  not an edge case. Retention would have expired the glimt and deleted bytes a public album page still
  showed, blanking it months later with nothing connecting the two events.
- 2026-09-19 — Fixed by extracting `refsUsedElsewhere`, which asks every owner and unions the answers,
  with a comment stating the rule for whoever adds the third owner: **anything that stores a blob must
  be added here**, because the function cannot discover an owner on its own. It fails as a whole — if
  any source cannot be asked, nothing is deleted. Leaking disk is recoverable; this is not.
- 2026-09-19 — ✅ **Verified the regression test by breaking the fix.** Disabled the album branch and
  confirmed `TestGlimtDeleteKeepsBytesAnAlbumStillUses` and `TestAlbumRefCheckCoversThumbnails` both
  fail ("the shared object was deleted: an album page that used it is now blank"), then restored it and
  confirmed they pass. A regression test nobody has seen fail is a test that might assert nothing.
- 2026-09-19 — `imaging.ReadGPS` added next to `ReadOrientation`, which meant factoring out the shared
  TIFF-header parse. Worth it beyond tidiness: two independent endianness parsers is exactly how you
  get "GPS works on Android files and not iPhone ones". `TestOrientationAndGPSDoNotInterfere` guards the
  seam, and the refactor is covered by the existing orientation tests.
- 2026-09-19 — The GPS parser refuses more than it accepts, on purpose: **a half-parsed coordinate is a
  pin in the wrong place on a public page, which is worse than no pin.** Refused: unknown hemisphere
  (would silently flip a southern coordinate north), zero denominators (real cameras emit them),
  out-of-range degrees, and **exactly 0,0** — a legal spot in the Atlantic and also what a camera with
  no fix writes, the same call the checkpoint projection makes for an unset position.
- 2026-09-19 — `TestPrepareStillStripsGPS` is the test that matters most in the imaging package: it
  asserts the display image, the thumbnail **and the kept original** all come out with no GPS. Reading
  the coordinate must never turn into keeping it, and that is the property a future "simplification"
  would break.
- 2026-09-19 — Four bounds verdicts rather than three. `none`/`inside`/`outside` are obvious; the fourth,
  **`unknown`**, is the one worth having: there was a coordinate but no race area to judge it against,
  because no checkpoint has a position yet. `outside` is a statement about the photograph, `unknown` is
  a statement about us — folding them together would permanently condemn every photograph uploaded
  before the course was sited, with no way to tell those from genuinely stray coordinates. `unknown` is
  not plottable, asserted.
- 2026-09-19 — The verdict is decided at **ingest** and carried on the event rather than recomputed at
  read time, because the race area moves as organizers site checkpoints — a verdict recomputed next week
  would be a different verdict, silently, for a photograph nobody touched.
- 2026-09-19 — The fold downgrades an unrecognised verdict to `unknown`, never `inside`: the map read
  filters on that column, so a typo must make a photograph unplottable rather than plot one whose
  coordinate was never checked. Also drops a half-coordinate, and records `none` when a verdict arrives
  without a coordinate to be about.
- 2026-09-19 — The projection deliberately records **no curator, uploader or person at all**. Every read
  is served to an unauthenticated page, so a `curatorPersonId` column would be a personal identifier on
  the one surface that must name no person (task 337) — and since photo permission is settled upstream
  (§0b.2), there is no audit question it would answer. Written into the package doc so it reads as a
  decision rather than an omission.
- 2026-09-19 — Removal folds (`itemremoved`, `deleted`) are implemented and subscribed now even though
  task 335 adds the handlers. A projection that ignores an event it was not yet taught about would
  silently keep showing a photograph somebody took down, and a consumer's subject list is exactly the
  thing nobody remembers to extend. Album deletion marks the *items* too, because the map read looks at
  items across albums and has no reason to join the parent.
- 2026-09-19 — Added a dev fixture (`POST /api/dev/album-fixture`), following `devglimt.go`'s precedent.
  The argument is stronger here: nothing else in the app ever creates an album, so without it the
  covers, the grid, the unpublished state and the bounds verdicts cannot be looked at at all before the
  curation tool exists (§11 Q5). The fixture includes one **unpublished** album and one **out-of-bounds**
  coordinate, because those two states are otherwise impossible to see.
- 2026-09-19 — The fixture sets coordinates as *data*, not as EXIF, and the log should be honest about
  what that means: it does **not** exercise the EXIF read. Writing one would mean shipping an EXIF
  *writer* in production code — in a binary whose job is to remove metadata — to serve a development
  convenience. The parser is tested against constructed files in `internal/imaging` instead. Said in the
  file's header so nobody mistakes a green fixture for a tested parser.
- 2026-09-19 — Glimt's rate limits and storage ceilings are deliberately **not** applied to album
  ingest. They bound what an unaccountable participant on a phone can push into the store; an album
  ingest is an organizer action on a known set, and a limit that stopped a curator halfway through would
  cost more than it protects. The size cap stays.
- 2026-09-19 — `gofmt`, `go vet ./...` and `go test ./...` all clean. Moving to done.
- 2026-09-19 — **Note for task 334/342:** `Queries.Plottable` filters `boundsVerdict = 'inside'` **in
  SQL**, and joins the parent so an unpublished album's photographs cannot reach the map. Keep the
  filter there rather than in a handler — the point is that a handler cannot plot an unchecked
  coordinate by forgetting a condition.
