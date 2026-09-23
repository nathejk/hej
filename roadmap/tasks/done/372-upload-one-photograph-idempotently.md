# 372 — Upload one photograph, idempotently, without resurrecting a deletion

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent session (Zed)
**Started:** 2026-09-23
**Completed:** 2026-09-23

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

## What landed

`POST /api/admin/photos` in `adminupload.go`, wrapping `storeAlbumImage` rather than reimplementing it — that
pipeline reads GPS from the original bytes *before* `imaging.Prepare` re-encodes them, and a second copy of
that sequence would be a second place for somebody to tidy the read out of existence.

**The response names which of three things happened**, and that is the part worth arguing for. A re-upload
reported as plain success would tell a photographer their 300 files landed when 280 were already there —
indistinguishable from the batch having worked, so a duplicated card could never be diagnosed. And a re-upload
of a *deleted* photograph does nothing by design; reported as success that is a lie, reported as an error it
suggests something needs fixing. It needed a third word: `stored`, `already`, `deleted`.

**For a previously deleted photograph, nothing is published at all.** The fold already declines to clear
`deleted` (task 363), so republishing would technically be harmless — but relying on that would make the
safety of a takedown depend on a detail of a fold two packages away. Refusing to publish makes it depend on
nothing. The break-test showed the layering: with the explicit check disabled, resurrection was *still*
prevented twice over, and what the test caught was the wrong message.

The response deliberately carries **no thumbnail ref**. The id is unavoidably a content ref — it is the
photograph's name and the uploader supplied the bytes — but a second capability has no reason to leave the
server, since the browser addresses a thumbnail through the id. Asserted.

## A test-writing trap worth recording

`decodeUpload` originally decoded whatever came back. A 503 decodes "successfully" into a zero-valued struct,
so the first run reported six confusing downstream failures whose single cause was that `newTestApp` has no
publisher. It now fails loudly on any non-200 before decoding. Anybody adding tests to this surface will hit
the same thing.

## Verified live, through Traefik, with a real 1.6 MB JPEG

- first upload → `stored`, and `photoId` **equals** `blobRef` in the database, confirming the derived id;
- same file again → `already`, one row, one event;
- row marked deleted as a curator would, then re-uploaded → `deleted`, the row **stayed deleted**, and the
  message said so in Danish;
- a text file → 400; no credential → 401;
- the admin page's counts moved to "1 billeder / 1 uden album", so upload → event → projection → page is
  closed end to end;
- all three outcomes appeared in the log with the photo id and the client IP.

## Acceptance Criteria

- [x] One request, one file, one `photo.uploaded` event, with the id equal to the content ref of the stored
      full rendition — confirmed in the live database, not just asserted
- [x] Uploading identical bytes twice yields one library row and a response that says "already uploaded"
- [x] Re-uploading a deleted photograph does **not** undelete it, and the response says so — tested, verified
      by breaking it, and confirmed live
- [x] A `.mov`, a Canon raw, a `Thumbs.db` and a text file each get `400` with a plain Danish reason; an
      oversize file gets `413`. A zero-byte file (a card pulled mid-write) gets its own reason rather than a
      confusing "not an image"
- [x] A GPS fix is read before `Prepare`, bounds-checked, and returned with its verdict; the stored renditions
      contain no EXIF — asserted on the bytes read back out of the store
- [x] A broken stream yields `503`, and nothing claims to have saved
- [x] The handler name ends in `Handler`, the route path is a plain string literal, and it carries full
      OpenAPI annotations including `@Failure 401`
- [x] The ceiling clears a real camera JPEG — asserted against a 25 MB reference, and against glimt's 12 MB
      being lower
