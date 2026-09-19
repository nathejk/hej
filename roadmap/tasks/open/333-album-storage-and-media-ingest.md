# 333 — Album storage and media ingest, with a deliberate photo coordinate

**Status:** open
**Priority:** high
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] Album and album-item tables created, owned by their projection, following the `checkgroup` /
      `scan` table conventions (including the commentary style — these files explain *why*).
- [ ] Ingest reuses `internal/blob` and `internal/imaging`; no second content-addressing or
      thumbnailing implementation.
- [ ] EXIF is still stripped by re-encoding. A test asserts stored bytes carry no GPS.
- [ ] The coordinate is read from the original bytes before re-encoding and stored in its own nullable
      column, editable and deletable independently of the image.
- [ ] Coordinates are bounds-checked against the race area; the verdict is stored, and an
      out-of-bounds item is retrievable as such rather than silently unplotted.
- [ ] Album blobs are **excluded from the PRD 019 glimt purge** — asserted by a test, not by reading
      the purge code and concluding it is fine.
- [ ] Ordering within an album is explicit (curator-set), not incidental.
- [ ] Unpublished albums are invisible to every public read.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §8 / §10 (Phase 1).
