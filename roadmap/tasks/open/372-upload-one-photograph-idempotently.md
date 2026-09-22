# 372 — Upload one photograph, idempotently, without resurrecting a deletion

**Status:** open
**Priority:** high
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

## Description

`POST /api/admin/photos` takes **one** file and publishes a `photo.uploaded` event. A new handler in
`go/cmd/api/` that **wraps** `storeAlbumImage` (`go/cmd/api/albummedia.go`) rather than replacing it:
PRD 022 §8.4 keeps that pipeline exactly as it is — `imaging.ReadGPS` on the original bytes, then
`imaging.Prepare` (which re-encodes and therefore strips all EXIF including GPS), then `blobs.Put`,
with a failed thumbnail logged rather than fatal.

The photo id is **derived from the content hash** — `blob.ComputeRef` of the stored full rendition,
which is a deterministic function of the uploaded bytes because `Prepare` is deterministic (PRD 022
§8.5). Two properties follow, and they are in tension:

- A re-upload must **not** create a second row. Re-dragging a folder is routine, not an error, and a
  photographer who does it should not end up with the card twice.
- A re-upload must **not** resurrect a row a curator deleted, because that delete may have honoured
  somebody's objection. This is the rule `album`'s `created` fold already follows: the upsert **does
  not touch `deleted`** (see task 363).

The handler therefore tells the browser *which* case it was, so the UI can say "allerede uploadet"
rather than claiming a fresh success — and for a previously deleted photograph, say so plainly rather
than silently doing nothing, which would look like a bug to the one person who could explain it.

Limits per PRD 022 §8.9: **32 MB** per file via `http.MaxBytesReader` (glimt's 12 MB is too low —
camera JPEGs from a full-frame body routinely exceed it, and a photographer hitting `413` on a normal
file would reasonably conclude the tool is broken), a generous read deadline for a large file on a
hotel connection, `413` over the limit and `400` if it does not decode. **The decode is the
validation**; the declared content type is not trusted, as everywhere else in this service. If the
event stream is unavailable the response is `503` and says so — the projection is downstream of the
log, so a silent failure would look like success until a reload.

## Acceptance Criteria

- [ ] One request, one file, one `photo.uploaded` event, with the id equal to the content ref of the
      stored full rendition
- [ ] Uploading identical bytes twice yields one library row and a response that says "already
      uploaded"
- [ ] Re-uploading a deleted photograph does **not** undelete it, and the response says so — tested
- [ ] A `.mov`, a raw file and a `Thumbs.db` each get `400` with a plain Danish reason; a 40 MB file
      gets `413`
- [ ] A GPS fix is read before `Prepare`, bounds-checked, and stored with its verdict; the stored
      renditions contain no EXIF — asserted on the bytes
- [ ] A broken stream yields `503`, and nothing claims to have saved
- [ ] The handler name ends in `Handler`, the route path is a plain string literal, and it carries full
      OpenAPI annotations including `@Failure 401`
