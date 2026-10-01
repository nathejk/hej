# PRD 027 — Keep the photographer's original

**Status:** done
**Author:** agent session, with the maintainer
**Created:** 2026-10-01
**Last updated:** 2026-10-01
**Approved:** 2026-10-01
**Shipped:** 2026-10-01
**Target users:** organizer (the 2–3 photographers/curators who run the admin tool)

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 1. Summary

Stop treating a downscaled copy as the archive's master. Every photograph handed in to the library is currently
re-encoded to 1600px on its longest edge and the photographer's file is discarded; this PRD stores **the uploaded
file itself, byte for byte, metadata included** as a backed-up original, and demotes the 1600px copy to what it
actually is — a display rendition, offered as the scale set `XLarge (1600px)`.

An original is the original. Every *reader* — the viewer, the public album page, the PWA — continues to be served a
scaled, metadata-stripped rendition and is never handed the original.

## 2. Problem & Motivation

**What problem does this solve?**

`cmd/api/albummedia.go` calls `imaging.Prepare(raw, maxGlimtEdge, …, keepOriginal: false)`. So for every
photograph in the library:

- the stored "full" image is 1600px on its longest edge, re-encoded at JPEG quality 85;
- the photographer's own pixels are **never written anywhere** and are gone the moment the request ends;
- `photo.blobRef` is classified as an **original** in the blob store (`original/`, inside the backup scope) —
  which is true in the sense that it is the only copy we have, and misleading in the sense that it is not the
  photograph that was taken.

That was a defensible decision while the library existed to serve web pages, and PRD 022 §8.5 recorded it
explicitly. It stops being defensible the moment anything wants the photograph *off* the web:

- a 1600px frame is ~2 MP. A printed programme at 300 dpi gets about 13 × 9 cm out of it; an A4 page needs
  roughly four times the pixels. The event's own 12–24 MP source files would cover both.
- the loss is **silent and permanent**. There is no backfill, no second copy, no "re-render at a larger size"
  — the information left with the HTTP request. Every event archived under the current pipeline is capped
  forever.
- PRD 022 §11 Q2 resolved that these photographs are **never purged**. We are therefore building a permanent
  archive out of downscaled derivatives, which is the one kind of storage decision that cannot be revisited
  later.

**Why now?**

Two things made it visible in the same week:

1. The album zip download (task TBD, shipped alongside this draft) offered a size called "Original". It could not
   honestly be called that, because what it hands over is the 1600px display image. The menu now says
   `XLarge (1600px)` and a test forbids the word "original" appearing there — a guard that documents a gap rather
   than closing it.
2. The maintainer's instruction, 2026-10-01: *"We need to stop scaling input data and store originals in
   /originals folder — fine to create a scale-set 'XLarge (1600px)' but that is not an original."*

- **Evidence.** `albummedia.go:127` (`keepOriginal: false`), `photo/table.sql` ("an admin upload keeps no separate
  original either"), PRD 022 §8.5, PRD 023 §7.9, and the `adminZipSizes` comment in `cmd/api/adminalbumzip.go`.

## 3. Goals

- A photograph handed in to the library is retained **exactly as it was handed in** — same pixels, same metadata,
  same bytes — for as long as the archive exists.
- A curator can download an album, or a single photograph, as the photographer's file.
- Every *reader* keeps being served a scaled, metadata-stripped rendition. The original is a download, never a view.
- The archive's backup scope stays an operator-legible **path** — no manifest, no database lookup — so the thing
  that must be backed up is still answerable with `du`.
- The word "original" in the product means the photographer's file, everywhere, with no exceptions that need
  explaining.
- Nothing already in the library is broken or re-processed by the change.

## 4. Non-Goals

- **Stripping the original's metadata.** Resolved the other way by the maintainer, 2026-10-01: *"full upload
  resolution including metadata — original=original — do not strip exif"*. This **inverts** what `albummedia.go`'s
  header says today ("Never change the pipeline to preserve EXIF because this feature wants a coordinate"), so §6
  R10–R12 carry the replacement rule and the guard that enforces it. The short version: the prohibition was about
  what **readers** are served, and that half does not move an inch. See §8 "The EXIF reversal".
- **Backfilling existing photographs.** It is not possible. There is no source to backfill from. Everything
  uploaded before this ships has no original and never will; the product must say so rather than imply otherwise.
- **Serving originals to viewers.** Resolved by the maintainer, 2026-10-01: *"The viewers should not be burdened
  with a crazy big photo — they should be served one of the scaled sizes stripped of metadata"*. No `srcset`, no
  gallery, no public page, no PWA surface ever names an original. §6 R11.
- **Changing glimt.** `glimtThumbEdges` and the glimt pipeline are PRD 019's storage decision, and §6 R4a now states
  the positive rule: a glimt's display image **is** an original, because it is the most original copy that will ever
  exist, and it stays in the backup scope. What glimt does not get is a *second*, truer copy — there is none to keep.
- **Changing the upload cap.** 32 MB (`maxAdminUpload`) stays as it is, and **it is the gate** — see §6
  Non-Functional. It may be revisited, and when video arrives it likely becomes a per-media-type limit; neither is
  this PRD's business.
- **RAW formats.** `image.Decode` handles JPEG/PNG/GIF, and the *display* pipeline still has to decode the upload
  to produce renditions. A `.CR2` or `.NEF` is refused today and stays refused; photographers hand in JPEGs.
- **Video.** Flagged by the maintainer as coming, and deliberately not designed here. The one thing this PRD should
  avoid is making it harder: see §11 Q5.

## 5. User Stories & Scenarios

- As a **curator**, I want to download an album at full resolution, so that the local paper can print a photograph
  across two columns instead of a thumbnail.
- As a **curator**, I want the archive to hold what the photographer handed in, so that a decision taken in 2026
  does not cap what the 2031 anniversary book can use.
- As an **operator**, I want to know which directory must be backed up, so that a restore is a file copy rather
  than an investigation.

**Happy path.** A photographer drags 300 files from an SD card into the admin uploader. For each file the server
reads the capture time and coordinate into columns, stores the **uploaded bytes unchanged** as an original
(`original/`), then decodes them to produce and store the 1600px display image, the 800px rendition and the 320px
thumbnail as cache (`cache/`) — all three stripped of metadata, as they are today. The contact sheet, the viewer and
the public album page are unchanged, because they all read renditions. The album's zip menu gains a fourth entry
above the scales: **Original**.

**Edge cases.**

- *The upload is already small.* A file the client downscaled to 1024px is still stored as an original, and
  **unlike the portrait path, there is no "only if it has more pixels" condition.** That check exists in
  `imaging.Prepare` because a stripped, same-size original carries *no additional information* — measured in
  production on 2026-08-29 at 1.9× the storage for nothing. Once the original keeps its metadata that premise is
  gone: the capture time, the camera, the lens and the coordinate are information the display image does not have
  and cannot be re-derived from. So every upload gets an original, which also makes `originalRef` mean one thing
  rather than "present, unless…".
- *The format cannot be decoded.* Refused at the door, as today — the decode **is** the validation. Nothing reaches
  the store.
- *Uploaded before this shipped.* No `originalRef`. The Original download falls back to `xlarge`, and the UI says
  which photographs are affected rather than leaving a curator to guess (see §7).
- *Storing the original fails.* The upload fails. The alternative — keep the photograph, silently drop the original
  — leaves one photograph quietly un-recoverable, discovered years later for no visible reason. This mirrors the
  portrait path, which already makes exactly this call (`cmd/api/portrait.go`).
- *A curator passes an original on.* It carries the camera's EXIF, including GPS. That is the point of an archive
  master and it is also a thing a curator can now do by accident, so §7 requires the download to say so. The
  renditions remain the thing to hand to a newspaper.

## 6. Requirements

### Functional

- [x] R1 — The library upload path stores the **uploaded bytes unchanged** — full resolution, metadata intact — as a
      blob store **original** (`blob.Put`, i.e. `<root>/original/`). No re-encode, no strip, no resize, no condition.
      *(Task 479, done 2026-10-01.)*
- [x] R2 — `photo.Uploaded` carries the original as an optional nested object (ref, bytes, width, height, content
      type), mirroring `person.PortraitOriginal`. Absent is a normal value, because every event already in the log
      has no original.
      *(Task 477, done 2026-10-01. One deliberate difference from the portrait: no `Orientation` field — the portrait
      needs one because stripping removes the tag, and PRD 027 does not strip.)*
- [x] R3 — The `photo` projection gains `originalRef`, `originalWidth`, `originalHeight`, `originalBytes`,
      `originalContentType`, all defaulting to empty/zero, so every existing row folds unchanged with no migration.
      *(Task 477, done 2026-10-01. Plus an `original_lookup` index, and — the find of that task — the five columns
      are written as **one group guarded on `originalRef`**, because `photoId` is the hash of the display rendition,
      so a documented re-upload (task 372) of a stripped copy would otherwise have blanked the original.)*
- [x] R4 — The library's 1600px display image is reclassified as **cache** (`blob.PutCache`) for new uploads, because
      it is now derived from something the store holds. Existing display images stay where they are; `locate` finds
      either, and `Put`'s promotion rule already prevents a sole copy ending up in the subtree a backup skips.
      **Scoped to the library path and nowhere else** — see R4a, which is the reason R4 is safe rather than a
      coincidence. *(Task 479, done 2026-10-01. Two existing tests asserted the old classification and were inverted
      in place rather than deleted, so the reversal is findable where the old rule was.)*
- [x] R4a — **Glimt and portrait display images stay `blob.Put` (backed up), and nothing in this PRD may touch
      them.** Maintainer, 2026-10-01: *"portraits and glimt originate from the PWA. These photos are not
      photographer quality, but are taken by users during race — even though the original is not in high quality,
      it's still the most original we have and still needs to be backed up."*
      This is already how they are stored (`glimtmedia.go:205`, `portrait.go:47`), and writing it down turns a
      correct accident into a rule. **The classification tracks "is this the only copy of these pixels", not "was
      this re-encoded"** — exactly as `internal/blob`'s package doc defines the two classes. A glimt's 1600px frame
      is a re-encode *and* the sole surviving copy of a moment during the race, so it is an original. The library is
      the one path where that stops being true, and R4 is a consequence of R1 rather than a tidy-up in its own
      right: the display image may be demoted **only because** a truer copy now exists beside it.
- [x] R5 — The album zip offers a fourth size, `original`, which streams `originalRef` when present and falls back
      to `xlarge` when not. The existing three are unchanged and stay metadata-stripped. *(Task 481.)*
- [x] R6 — `/api/admin/photos/{photoId}/media` accepts `variant=original` for a deliberate single-photograph
      download, behind the same resolve-the-id-then-use-the-row's-ref rule as every other variant, and with **no
      rendition-repair plan** — an original is not rebuildable. *(Task 480.)*
- [x] R7 — The admin tool states, per photograph and per album, whether an original is held — so "can I print
      this?" is answerable before a download rather than after — and says that an original carries the camera's
      metadata, including where it was taken. *(Task 482. The per-photograph half is a **filter preset** rather than a
      per-cell mark: the sheet's marks follow "the ordinary case gets none", and "has an original" is rare now and
      universal later, so no badge for it stays quiet at both ends.)*
- [x] R8 — Deleting a photograph from the library frees the original too, and `photo.RefsInUse` includes
      `originalRef` so that no other table's delete can free bytes this row still needs. *(Task 478, landed before
      479 so no original could exist that a takedown would miss.)*
- [x] R9 — The word "original" is used for nothing else. The 1600px copy is `xlarge` in URLs, labels and code.

#### The reader rule (R10–R12), which is what makes R1 safe

- [x] R10 — **No route outside `requireAdmin` may resolve `originalRef`.** Enforced by a source-walking guard test
      in the manner of `publicprivacy_test.go` and `curatorboundary_test.go`, not by review: this is the single
      invariant the EXIF decision rests on, and its failure mode is silent.
      *(Task 476, done 2026-10-01: `cmd/api/originalboundary_test.go`. Two guards rather than one — a file allowlist
      and a route-to-declaring-file walk — because a permitted file growing a second, unguarded handler is the
      mistake an allowlist alone cannot see.)*
- [x] R11 — **No viewer, `srcset`, gallery, grid, public page or PWA surface references an original.** The viewer's
      candidates stay 800w/1600w (PRD 023 §7.9, task 410). A guard test asserts the viewer's markup names no
      original variant — a 24 MP file in an `srcset` is both a privacy leak and a 10 MB page.
      *(Task 476, done 2026-10-01. Covers `viewer/`, `publicsite.go`, `albumpage.go` and a walk of `vue/src`.)*
- [x] R12 — Every **rendition** remains metadata-stripped, exactly as today. The existing
      `TestStoreAlbumImageReadsTheCoordinateAndStripsIt` keeps passing **unchanged** and becomes the load-bearing
      test of this PRD: it asserts that the bytes a reader is served carry no GPS, which is the half of the old
      prohibition that does not move. *(Task 479. It did pass unchanged. `TestTheRenditionsStillCarryNoMetadata`
      extends the same assertion to the 800px and 320px renditions, which that test does not reach and which are what
      a phone and a grid are actually served.)*

### Non-Functional

- **Storage — accepted by the maintainer, 2026-10-01: 8–15 GB per event is acceptable.** For the record, the
  arithmetic and the gate:
  - **The upload cap is the gate.** `maxAdminUpload` is 32 MB (`cmd/api/adminupload.go`), so no single photograph
    can cost more than that and an event's worst case is bounded rather than open-ended: 2,000 photographs × 32 MB
    = 64 GB absolute ceiling. Realistically a 24 MP camera JPEG is 6–10 MB, giving the accepted 8–15 GB against
    order 1 GB today.
  - The cap may be adjusted over time, and when video arrives it will likely become **per media type**. So nothing
    in this PRD may hard-code 32 MB as a storage assumption: the capacity requirement is a function of the cap, and
    the cap is one constant in one place.
  - Never purged (PRD 022 §11 Q2), all of it inside the backup scope. Task 384's restore spot-check should be run
    against the new trajectory — no longer a blocker, but the number it reports is the one an operator needs.
- **Upload latency.** One extra blob write per photograph, and **no extra decode** — the original is the bytes
  already in memory. Cheaper than a stripped original would have been, since there is no scrubber pass either.
- **Free space.** The upload path's free-space check (`blob.FreeSpacer`) must account for the original as well as
  the renditions, or a full volume is discovered mid-card.
- **Privacy.** The archive now holds EXIF, including GPS, for library photographs. A **deliberate and bounded**
  reversal, bounded by R10–R12, by `requireAdmin`, and by every rendition staying stripped. See §8.
- **Backup legibility.** The split stays a path. No part of this PRD may make "what must be backed up" a question
  that needs a database connection.

## 7. UX / UI Notes

**Nothing in `vue/`.** This is the admin tool and the Go upload path; the PWA neither uploads to the library nor
reads originals. (See `.rules`, "Three surfaces, two frontends".)

In the admin tool (`go/cmd/api/adminui/`, server-rendered — see the `go-server-rendered-pages` skill):

- The album card's zip submenu gains **Original** above `XLarge (1600px)`.
- The submenu is where the absence has to be legible. An album whose photographs predate this change cannot offer
  originals for them, and a download that silently substituted 1600px files would be the exact dishonesty this PRD
  exists to remove. Proposal: the item reads `Original (34 af 180)` when the album is partial, and is disabled with
  a one-line explanation when the count is zero. Wording to be written deliberately, in Danish, as PRD 022 §5
  requires of anything where two similar actions must not be confused.
- The contact sheet's photograph detail says whether an original is held.
- The library counts gain "uden original", so the gap is a number somebody can watch shrink rather than a surprise.
- **The viewer is unchanged, deliberately.** Its `srcset` stays 800w/1600w. A curator viewing a photograph on a
  laptop is served a rendition like everybody else: the original is a *download*, never a *view*. That is the
  maintainer's instruction and it is also the only arrangement in which a 24 MP file cannot end up on a page.

## 8. Technical Considerations

### The EXIF reversal, stated once and plainly

`cmd/api/albummedia.go`'s header currently says: *"**Never** change the pipeline to preserve EXIF because this
feature wants a coordinate."* This PRD changes that, so it owes the next reader a precise account of what was
actually being protected — otherwise the rule looks like it was simply overruled, and the next person overrules the
remainder of it.

The prohibition conflated two things:

1. **No reader may be handed unexamined metadata.** A photograph of a child must not carry where it was taken
   around inside a file nobody has looked at (PRD 003 §6). *This is unchanged and now tested explicitly* — every
   rendition is still re-encoded and stripped, and R10–R12 make "no reader resolves an original" a guard rather
   than a convention.
2. **The archive may not hold metadata.** This was never argued for separately; it followed from the archive and
   the display image being the same object. Once they are two objects the two questions come apart, and the answer
   to the second is different: an archive master that has lost the capture time, the camera and the lens is a worse
   archive, and that information cannot be recovered later.

So the stored original keeps its EXIF **and** every served byte stays stripped. The capture time and coordinate
continue to be read into columns at ingest, because that is where a curator can see, correct and bounds-check
them — the column is a decision somebody made; the file is just a file.

Two consequences to hold on to:

- **`requireAdmin` becomes a privacy boundary, not only an access one.** It was already the case that the admin
  surface sees drafts and deleted photographs; it now also sees coordinates inside files. The guard test for R10 is
  what keeps that from drifting.
- **A curator can now hand on a file containing GPS.** Nothing stops that and nothing should — it is the archive
  master. But it is new, so §7 requires the download to say so, and the renditions remain the thing to send a
  newspaper.

### The rest

- **Frontend (Vue 3 / TS):** N/A — no PWA surface is involved. Stated rather than omitted because the repo has two
  frontends and "no frontend work" is itself worth recording.
- **BFF (Go):**
  - `cmd/api/albummedia.go` — store `raw` with `blobs.Put` **before** preparing renditions; move the display image
    to `blobs.PutCache`; carry the original on `albumMediaPrepared`. Note this does **not** use
    `imaging.Prepare(keepOriginal: true)`: that path strips metadata and declines same-size originals, both of
    which are now wrong here. `keepOriginal` stays as it is for portraits, where stripping is still right.
  - `cmd/api/adminupload.go` — publish the new nested field; extend the free-space estimate.
  - `nathejk/table/photo/` — `Uploaded.Original`, the five columns, fold, `LibraryPhoto` fields, and `RefsInUse`
    must include `originalRef` or a delete elsewhere will free bytes this row still needs (the same class of bug
    task 368 is about).
  - `cmd/api/admindelete.go` — a takedown must free the original too. This is the one path where getting it wrong
    is not recoverable, and it is also the path where "deleted from the library" has to keep meaning what PRD 022
    §5 says it means. **A takedown that left an EXIF-bearing original behind would be a takedown in name only.**
  - `cmd/api/adminalbumzip.go` — a fourth entry in `adminZipSizes`, with the fallback in `adminZipRendition`.
  - `cmd/api/adminlibrary.go` — `variant=original`. **No rendition-repair plan for it:** an original is not
    rebuildable, so the repair machinery (task 430) must not be pointed at it, and `blob.PutAs` already refuses to
    write over an original for the same reason.
  - **Width/height of the original** come from `image.DecodeConfig` on the raw bytes, which is a header read rather
    than a decode, and describe the stored bytes before rotation — as `imaging` already documents for portraits.
- **API endpoints.** Two extended; **both need their OpenAPI annotations updated in the same change**
  (`.rules`, and `TestGlimtRouterAnnotationsMatchTheRegisteredPaths` enforces the block exists):
  - `GET /api/admin/albums/{albumId}/zip` — `size` gains `original`.
  - `GET /api/admin/photos/{photoId}/media` — `variant` gains `original`.
- **Data / storage.** Five default-valued columns and one optional event field; no migration, because the
  projection is replayed and every existing event simply has no original. The blob store needs **no change at all**
  — `original/` and `cache/` already exist (task 429) and `Put`/`PutCache` already choose between them. The
  maintainer's "/originals folder" is `<root>/original/`, already there and already the backup scope (§11 Q4).
- **Dependencies & risks.**
  - *The privacy boundary is now load-bearing.* The biggest risk in this change is no longer capacity, it is a
    future route resolving `originalRef` outside `requireAdmin`. R10's guard test is the mitigation and it should be
    written **before** R1, so the invariant exists before the data does.
  - *Irreversibility.* Once originals are stored they are in the backup scope forever; PRD 022 §11 Q2 forbids a
    retention job. Deciding to stop later does not reclaim anything.
  - *An original that is not verifiable.* Originals keep content addressing — their ref **is** their hash — which is
    what `PutAs` refuses to break. Nothing in this change may weaken that.
  - *Partial archive, permanently.* Two eras of photograph will coexist for the life of the product. R7 exists so
    that is visible rather than discovered.
  - *Video, later.* The shape chosen here — an optional original beside derived renditions, classified by subtree —
    is the shape video will want too. What video will **not** want is a 32 MB cap, which is why the cap must stay
    one constant and not an assumption spread through the storage reasoning.

## 9. Success Metrics

- Every library upload after the change holds an `originalRef` — there are no documented exceptions now that the
  original is the uploaded bytes. Measurable as the "uden original" count going flat.
- A curator can produce a print-resolution file from the admin tool without asking a developer — zero such requests
  after the event following the change.
- The restore spot-check in task 384 completes against the post-change volume, with a recorded duration and size.
  **Half met (task 483):** the spot-check was run and passed; the duration and size were not written into the record.
- **`TestStoreAlbumImageReadsTheCoordinateAndStripsIt` and `cmd/api/publicprivacy_test.go` pass unchanged**, i.e.
  the archive gained metadata and no reader did. If either needed editing, the change went wrong.
- The R10 guard reports zero non-admin resolvers of `originalRef`, on a run that can see every route — the point of
  it being a source walk rather than a list.

## 10. Rollout / Task Breakdown

Sequenced so the **guard comes before the data**: R10's invariant is cheap to write while nothing stores an original
and awkward once something does. No feature flag — the change is per upload, and photographs uploaded during a
partial rollout would be inconsistent in a way nothing could later repair.

1. - [x] Task 476: guard test — no route outside `requireAdmin` resolves `originalRef`, and no viewer/`srcset` names
        an original variant (R10–R11). Written first, against the field before it exists. **Done 2026-10-01** — see
        its log for the one thing it caught immediately: `portrait.go`'s unrelated `originalRef`, which is the
        *portrait* original (task 111) and is stripped. Needed a second list, not a permission.
2. - [x] Task 477: `photo.Uploaded.Original` plus the five projection columns, fold and `LibraryPhoto` fields.
        **Done 2026-10-01** — and it found the rule nobody had written down: the five columns must move as one group,
        or a re-upload blanks the photographer's file.
3. - [x] Task 478: include `originalRef` in `RefsInUse` and in the library takedown's blob-freeing — a takedown that
        left an EXIF-bearing original behind would be a takedown in name only
4. - [x] Task 479: store the uploaded bytes unchanged as an original on library upload (`blobs.Put(raw)`), display
        image to `PutCache` — **the library's only**, never glimt's or a portrait's (R4a) — free-space estimate
        updated, renditions still stripped
5. - [x] Task 480: `variant=original` on the admin media route, with no repair plan, annotations updated
6. - [x] Task 481: `size=original` in the album zip, falling back to `xlarge`, annotations updated
7. - [x] Task 482: say in the admin tool which photographs hold an original — per photograph, per album, a "uden
        original" library count, and that an original carries the camera's metadata
8. - [x] Task 483: run task 384's backup/restore spot-check against the new volume and record the numbers.
        **Done 2026-10-01** by the maintainer — the restore completes, originals resolve by ref, the backup scope is
        still a path. The measured figures were not written into the task and are deliberately not estimated there;
        see its log.
9. - [x] Task 484: documentation sweep — `albummedia.go`'s "never preserve EXIF" header, PRD 022 §8.5 and
        `photo/table.sql`'s header all state something that becomes wrong on the day this ships, and the first of
        them is a rule somebody will otherwise cite back at a future change

## 11. Open Questions

Q1–Q7 are all resolved. They are kept here with their answers, because a question whose answer is only in a chat log
is a question the next person re-opens.

- **Q1 — Is the capacity acceptable?** **Resolved: yes.** 8–15 GB per event, never purged, all backed up. The 32 MB
  upload cap (`maxAdminUpload`) is the gate, and it may be adjusted over time — likely becoming per-media-type when
  video arrives.
- **Q2 — Is "original" the stripped upload, or a ceiling, or the file itself?** **Resolved: the file itself.** Full
  upload resolution, metadata included. *"original=original — do not strip exif"*. The 4000px-ceiling alternative
  was rejected: a ceiling is still scaling, and the word would again be doing unearned work.
- **Q3 — Should an original be downloadable one at a time?** **Resolved: yes, as a download; never as a view.**
  *"The viewers should not be burdened with a crazy big photo — they should be served one of the scaled sizes
  stripped of metadata"*. Hence R6 **and** R11: the route exists, the viewer does not use it.
- **Q4 — Is `<root>/original/` the "/originals folder" that was meant?** **Resolved: yes** — it already exists and
  is already the backup scope (task 429), so this PRD needs no blob-store change.
- **Q6 — Should the 32 MB cap rise now that pixels are being kept?** **Resolved: no.** 32 MB stays. A 24 MP camera
  JPEG fits comfortably, and the cap is the gate the capacity estimate rests on — moving both at once would mean
  neither number had been agreed. Revisit only with a capacity conversation, and expect it to become per-media-type
  when video arrives.
- **Q7 — Does anything need to limit *who* among the curators can download an original?** **Resolved: no, and the
  limit is understood.** The admin surface is one shared credential (PRD 022 §8.2), so "which curator" has no honest
  answer — the same reason `photo` records no uploader. Recorded as a known property of the privacy boundary rather
  than a gap in it; changing it would need real accounts first, which is a decision and not a field.
- **Q5 — Does glimt want the same treatment?** **Resolved: no, and for a sharper reason than "different retention
  promise".** Maintainer, 2026-10-01: a glimt or a portrait comes from a participant's phone during the race; it is
  not photographer quality, *"but it's still the most original we have and still needs to be backed up"*. So there is
  no truer copy to keep — the thing already stored **is** the original, and it already lives in `original/` via
  `blob.Put`. Glimt and portraits are therefore unchanged, and §6 R4a records the rule so that a future reading of
  R4 cannot be mistaken for permission to demote them. All open questions are now closed.
