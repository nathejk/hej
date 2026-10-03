-- The year's photograph library — "the bulk" (PRD 022 §8.3, task 363).
--
-- # Why this table exists at all
--
-- Until now a photograph came into existence *by being put in an album*: `album_item` carried the blob
-- refs, the caption, the coordinate and the verdict inline, keyed `(albumId, ordinal)`. For the feature
-- PRD 011 described — a curator assembling three to five albums — that was the right shape, and it is
-- worth being clear that this table is not a correction of a mistake.
--
-- It is a response to a workflow PRD 011 did not describe and PRD 022 does: **photographers hand in a
-- card, and the editing happens afterwards.** That breaks the old shape in three specific places:
--
--   * a photograph must be able to exist in **no** album, which `album_item` cannot express;
--   * the same photograph belongs in **several** albums, which under the old shape meant several rows,
--     each with its own copy of the coordinate, the caption and the verdict — free to disagree, with
--     nothing to notice when they did. A curator who corrects a location in one album and not the other
--     has created a bug that is invisible from both;
--   * a location is set on **forty photographs at once**, which is a statement about forty photographs,
--     not about forty album positions.
--
-- So the photograph is the thing, and an album is an arrangement *of* things. `album_item` becomes a
-- membership row (task 364) and everything that is true of the photograph itself lives here.
--
-- # The id is the content hash, and that is load-bearing
--
-- `photoId` is derived from the stored bytes (PRD 022 §8.5). A photographer re-dragging a folder is
-- routine, not a corner case, and content addressing means those bytes are already one object in the
-- blob store — so the row must be one row too. Deriving the id rather than minting one makes the whole
-- upload path idempotent for free: the same file republishes the same event and the fold converges.
--
-- The consequence to hold on to is in `deleted` below.
--
-- # Everything here is rebuildable except the blobs
--
-- A projection, replayed from the stream on every boot. `blobRef`/`thumbRef`/`mediumRef`/`originalRef` point into
-- the content-addressed store. Since task 429 that store has two halves, and only one of them must be backed up
-- (PRD 008 section 8). **PRD 027 changed which column sits in which half**, so the current arrangement is:
--
--   * `originalRef` is an **original** -- the photographer's own file, kept unchanged since PRD 027, and the only
--     copy of those pixels there will ever be. Backed up.
--   * `blobRef`, `thumbRef` and `mediumRef` are **cache** -- all three are derivable from the original and are
--     rebuilt on demand (task 430).
--
-- `blobRef` used to be an original, and the reason is worth keeping: before PRD 027 the photographer's file was
-- discarded, so the 1600px re-encode **was** the only copy. The classification tracks "is this the only copy of
-- these pixels", not "was this re-encoded" -- which is also why a glimt's display image is still an original
-- (PRD 027 R4a): for a participant's photograph taken during the race there is no truer copy, however modest the
-- quality. This demotion is the library's alone.
--
-- **PRD 022 resolved that these blobs are never purged** (section 11 Q2): unlike glimt, nobody was told these
-- photographs would disappear, so there is no retention job and the store grows by one event per year.
-- That makes capacity an operational requirement rather than a footnote — see task 384.
--
-- **And these objects may be shared with a glimt, or with each other.** Identical bytes are one object,
-- so anything that deletes blobs must ask every table that could name them. See
-- `cmd/api/glimtdelete.go` and task 368.
--
-- # What it deliberately does not record
--
-- **Who uploaded it, and who curated it.** No uploader, no curator, no person, no id of one. This is
-- the same omission `album` makes and it is inherited for the same reason plus a new one:
--
--   * `album`'s reason — every album read is served to an unauthenticated page, and a `curatorPersonId`
--     would be a personal identifier on the one surface that must name no person (task 337);
--   * this table's own reason — PRD 022 §8.2 takes a **shared credential**, so there is no honest
--     answer to "who". A column that could only ever hold a guess is worse than no column, because the
--     next reader will believe it.
--
-- If per-curator attribution is ever wanted it needs real accounts first, and that is a decision, not a
-- field added quietly.
CREATE TABLE IF NOT EXISTS photo (
    -- The content hash of the stored full rendition. See the header: derived, not minted, so that
    -- re-uploading a file lands on the row it already has.
    photoId VARCHAR(64) NOT NULL,

    year VARCHAR(99) NOT NULL,

    -- Content hashes into the blob store. Validated with a ref check before writing, as the glimt,
    -- portrait and album folds do: a ref is the one string here that could otherwise become a
    -- filesystem path.
    --
    -- `thumbRef` and `mediumRef` **may both be ""**, and readers fall back to the full image rather than
    -- rendering a gap, as the glimt grid does. That rule is not a nicety, it is what removes the need for
    -- a migration: every photograph uploaded before a rendition existed simply has "" and renders from
    -- `blobRef`. So it is written down here rather than left to be inferred from the code.
    blobRef VARCHAR(64) NOT NULL DEFAULT "",
    thumbRef VARCHAR(64) NOT NULL DEFAULT "",

    -- The 800px rendition (task 409, PRD 023 §7.9).
    --
    -- Between the 320px thumbnail and the 1600px display image, and it exists for one measured reason: a
    -- 390pt phone shows about 800px of a photograph at 2x, so serving it the 1600px display image is four
    -- times the pixels for no visible gain — times however many photographs somebody swipes through.
    --
    -- Produced **at upload**, not resized on request. `imaging.Prepare` already takes a list of edges, so
    -- this is one more entry in that list; there is no image-resizing endpoint in this service and this is
    -- not the feature that should introduce one.
    --
    -- Since task 429 these bytes are stored in the blob store's **cache** class: they are derivable from
    -- `blobRef`, so they are outside the backup scope and task 430 rebuilds them on a miss. That is what
    -- makes a third rendition cheap enough to be worth having.
    mediumRef VARCHAR(64) NOT NULL DEFAULT "",

    -- The photographer's file as it was handed in (PRD 027).
    --
    -- # This is the only column group here that is not derived from another
    --
    -- `blobRef` above is a 1600px re-encode with its metadata stripped. Until PRD 027 it was the only copy
    -- this service kept, which is why the header further up says "an admin upload keeps no separate original
    -- either" and why `blobRef` was classified an original in the blob store. That was true in the sense that
    -- nothing truer existed, and misleading in the sense that it is not the photograph that was taken: a
    -- 2 MP frame, so a printed programme gets about 13 x 9 cm out of it and an A4 page cannot be made at all.
    --
    -- These columns name the photograph that was taken. The uploaded bytes, unchanged: not re-encoded, not
    -- resized, and **not stripped of metadata** -- EXIF including the GPS coordinate is retained.
    --
    -- # Why the metadata stays here when it is stripped everywhere else
    --
    -- Because PRD 003 section 6's rule was protecting *readers*, and it still does: every rendition is decoded
    -- and re-encoded, which removes everything, and `TestStoreAlbumImageReadsTheCoordinateAndStripsIt` pins
    -- that. What changed is that the archive master and the bytes a reader is served are now two different
    -- objects, so "no reader may be handed unexamined metadata" and "the archive may not hold metadata" stopped
    -- being one question. PRD 027 answers them differently, because an archive that has lost the capture time,
    -- the camera and the lens is a worse archive and none of it can be recovered later.
    --
    -- **So `originalRef` is the most sensitive column in this projection**, and the invariant that makes it
    -- acceptable is enforced rather than documented: `cmd/api/originalboundary_test.go` fails if any route
    -- outside `requireAdmin`, or any viewer/public/PWA surface, resolves it. A portrait original is stripped
    -- (`person.portraitOriginalRef`) and that is not an inconsistency -- it is the same question asked about a
    -- photograph *of a person*.
    --
    -- # Backup class, which is the reason PRD 027 exists at all
    --
    -- An **original** in the blob store's sense: the only copy of these pixels, so inside the backup scope,
    -- never purged (PRD 022 section 11 Q2), and its ref is always its true content hash -- `blob.PutAs` refuses
    -- to write over one, because this is the data that cannot be rebuilt and therefore must stay verifiable.
    --
    -- Since PRD 027, `blobRef` is written to the **cache** class for new library uploads: it is now derivable
    -- from this. That demotion is scoped to the library and must not be generalised -- a glimt's or a portrait's
    -- display image stays an original, because for those it is the most original copy that will ever exist
    -- (PRD 027 R4a).
    --
    -- # All five are "" / 0 together, and that is permanent for old rows
    --
    -- Empty means no original is held, which is the state of every photograph uploaded before PRD 027 shipped.
    -- **No backfill is possible** -- the bytes left with the HTTP request -- so readers fall back to `blobRef`,
    -- that photograph's most original surviving form, exactly as they already fall back for a missing rendition.
    -- Two eras of photograph therefore coexist for the life of the product, which is why the admin tool says
    -- which is which (PRD 027 R7) rather than letting a curator find out after a download.
    --
    -- The fold writes the five as one group guarded on `originalRef`, so a later event either replaces the whole
    -- file or touches none of it. See handleUploaded: `photoId` is the hash of the *display* rendition, so a
    -- re-upload of a stripped copy produces the same id and must not be able to blank the original.
    originalRef VARCHAR(64) NOT NULL DEFAULT "",

    -- The upload's own format, since these bytes were not re-encoded -- so it may differ from the renditions,
    -- which are always image/jpeg.
    originalContentType VARCHAR(80) NOT NULL DEFAULT "",

    -- The stored size. Carried so the archive's growth is answerable without opening objects: PRD 027's
    -- capacity argument is made of this number, and the 32 MB upload cap is what bounds it.
    originalBytes INT NOT NULL DEFAULT 0,

    -- The stored bytes' dimensions **before** rotation is applied, so for a photograph taken sideways they are
    -- swapped relative to `width`/`height` above.
    --
    -- Not a defect to correct: these describe the file, and the orientation needed to display it is still
    -- inside the file, because the metadata was not stripped. That is one concrete way this differs from a
    -- portrait original, where the tag is removed and so has to be recorded in a column of its own.
    originalWidth INT NOT NULL DEFAULT 0,
    originalHeight INT NOT NULL DEFAULT 0,

    -- # Video (PRD 029)
    --
    -- A library item is a photograph or a video, and both live in this table so albums, ordering (PRD 024),
    -- credit (PRD 025) and patrol tags need no second path (PRD 029 §8, option A). Every existing row is a
    -- photograph that is ready, which is what the defaults say, so no backfill is needed.
    --
    -- For a video, `thumbRef`/`mediumRef` hold the **poster** at the usual sizes, `originalRef` the uploaded
    -- file (PRD 027 rules: backed up, admin-only), and `blobRef` the 720p MP4 -- so every reader that serves
    -- "the display rendition" serves something playable. `videoSdRef` is the 480p MP4, held only for clips
    -- over five minutes. Both MP4s are cache class: rebuilt from the original on a miss.
    kind VARCHAR(8) NOT NULL DEFAULT "photo",

    -- `processing` until the transcode worker reports, then `ready` or `failed`. Only `ready` may appear in a
    -- published album (task 503). Photographs are always `ready`.
    status VARCHAR(12) NOT NULL DEFAULT "ready",

    durationMs INT NOT NULL DEFAULT 0,

    -- The 720p rendition is `blobRef` (above); this column exists so the delete walks can name it as a video
    -- rendition rather than infer it. Equal to `blobRef` for a ready video, "" otherwise.
    videoRef VARCHAR(64) NOT NULL DEFAULT "",
    videoSdRef VARCHAR(64) NOT NULL DEFAULT "",

    -- ffmpeg's reason, shown to the curator on a failed item. Never public.
    failReason VARCHAR(255) NOT NULL DEFAULT "",

    -- The curator's words for this photograph, shared by every album it appears in.
    --
    -- One caption rather than one per album membership. A photograph in "Natten" and in "Postmandskabet"
    -- could conceivably want different words, and that override is deliberately **not** built (PRD 022
    -- §11 Q3) — because the cost of guessing wrong here is the exact bug this table was created to
    -- remove: two copies of one fact, drifting.
    caption TEXT NOT NULL,

    -- The photographer's credit line, e.g. "Foto: Anne Sørensen" (task 393). Empty when there is none.
    --
    -- # This is the one column in this table that names a person, and it is deliberate
    --
    -- Everything else here is arranged so the library names nobody — no uploader, no curator, no person id
    -- (PRD 022 §6), enforced by a structural walk over these types rather than by review. This column is the
    -- exception, and the bounds are what make it acceptable:
    --
    --   * it names a **consenting adult volunteer in a professional capacity**, because they asked to be
    --     credited — not a participant, not a minor, not somebody who never agreed to be in this app;
    --   * it is **free text a curator typed**. It is never derived, never looked up, and never joined to the
    --     `person` projection. That is the property that matters: the hazard was never that a name appears on
    --     a page, it is a system that starts deriving names from its person records and publishing them. A
    --     string somebody typed cannot do that.
    --
    -- So it is a `credit`, not a `photographer`, and emphatically not a `creditPersonId`. If crediting the
    -- same six people every year becomes tedious, the browser remembering the last one typed is the fix; a
    -- roster would mean holding a list of volunteers' names in this service, which is what §6 is arranged
    -- against.
    --
    -- VARCHAR(160), not TEXT like the caption: a credit line that can hold prose will eventually hold prose,
    -- and the caption is where prose goes.
    credit VARCHAR(160) NOT NULL DEFAULT "",

    -- The crew member credited for the photograph, or "" (PRD 025, task 450).
    --
    -- **A reference, where `credit` above is a name.** Either one may be set and never both: the writer clears
    -- the other, and the fold does too. A row with both would be a photograph with two answers about who took
    -- it, and the read would have to pick one — which is a decision belonging to the curator, not to a
    -- COALESCE.
    --
    -- Stored as a reference so that erasure works by deletion: the name lives in the person projection, so a
    -- crew member who asks to be removed is removed in one place and is gone from every photograph. A name
    -- copied into this table would also be on the append-only log, where it could never be erased — which is
    -- why PRD 022 §6's original "never a creditPersonId" was reversed rather than worked around (task 455).
    --
    -- Not person-shaped by the guard's reckoning only because it is excepted by name for this surface, exactly
    -- as `fileName` is; a **public** response carrying it still fails. It resolves to a name through one
    -- function (task 451) which reads the name column and nothing else, for crew and nobody else, within this
    -- photograph's own year.
    creditCrewId VARCHAR(99) NOT NULL DEFAULT "",

    width INT NOT NULL DEFAULT 0,
    height INT NOT NULL DEFAULT 0,
    bytes INT NOT NULL DEFAULT 0,

    -- Where the photograph was taken.
    --
    -- Either read from EXIF before the bytes were re-encoded, or placed deliberately by a curator. The
    -- two are not distinguished, and that is on purpose: PRD 011 §6 requires the location to be a
    -- reviewable field rather than metadata that happens to survive, and a curator correcting a bad fix
    -- produces exactly as authoritative a value as the camera did. What matters is that *somebody or
    -- something decided*, and that it can be seen, corrected and removed.
    --
    -- NULL is the normal case — most photographs have no usable fix.
    --
    -- The media pipeline destroys EXIF, including GPS, by re-encoding, and **that does not change**
    -- (`internal/imaging`, PRD 011 §6, PRD 019). The coordinate is read from the original bytes
    -- *before* they are re-encoded and written here instead. Stated again because it is exactly the kind
    -- of thing a later reader will try to simplify away:
    --
    --   * a coordinate in a column is a decision somebody made, which a curator can see, correct, and
    --     delete;
    --   * a coordinate inside a stored file is a leak waiting to happen.
    --
    -- Never "fix" the pipeline to keep EXIF because this column needs a value.
    latitude DOUBLE NULL DEFAULT NULL,
    longitude DOUBLE NULL DEFAULT NULL,

    -- What the race-area check made of that coordinate: inside | outside | none | unknown.
    --
    -- Stored rather than recomputed on read, and the four values are not three:
    --
    --   * `none`    — there is no coordinate. Nothing to plot, nothing wrong.
    --   * `inside`  — plottable.
    --   * `outside` — a coordinate that is not in the race area: a camera with no fix, a photograph
    --                 taken at home, a misplaced click. **Kept, not discarded**, and never plotted. A
    --                 curator has to be able to see that a photograph was rejected rather than wonder
    --                 why it is missing from the map — and discarding the value would destroy the
    --                 evidence that anything happened.
    --   * `unknown` — there was a coordinate but no race area to judge it against (no positioned
    --                 checkpoints yet). Distinct from `outside` on purpose: `outside` is a statement
    --                 about the photograph, `unknown` is a statement about us, and conflating them would
    --                 silently condemn every early-season upload.
    --
    -- Recomputed whenever the coordinate changes, and never otherwise: the race area grows as organizers
    -- site checkpoints, so a verdict recomputed on read would be a different verdict next week for a
    -- photograph nobody touched.
    boundsVerdict VARCHAR(16) NOT NULL DEFAULT "none",

    -- When the camera says the photograph was taken, from EXIF `DateTimeOriginal`, read before the bytes
    -- were re-encoded (PRD 024 §6 R3, task 440).
    --
    -- NULL when the file did not say — no EXIF, a format that carries none, or a camera whose clock had
    -- never been set. That is a large minority of files, so every reader needs the fallback rather than
    -- treating NULL as exceptional: the album sort uses `uploadedAt` for those.
    --
    -- Not the same thing as `uploadedAt` and not derivable from it. The admin uploader runs three requests
    -- at a time, so arrival order is not even file order, let alone exposure order.
    shotAt DATETIME NULL DEFAULT NULL,

    -- The name the photographer's file had, so an album can be sorted the way the card was sorted on their
    -- own computer (PRD 024 §6 R5, task 448).
    --
    -- **The second written-down exception to PRD 022 §6's "no personal data in this projection"**, after
    -- `credit`. A filename is usually `IMG_0123.JPG` and could in principle be `mor-og-far.jpg`, so the
    -- exception rests on bounds rather than on the value being harmless: the field lives behind the admin
    -- credential, never appears on a public read, and is never rendered as an attribution or used as
    -- anybody's name. It is a photographer's own filing, kept so that filing survives the upload.
    -- `isPersonShaped` still flags the word, and `libraryprivacy_test.go` excepts it here by name with that
    -- reasoning; a public response that grows a filename still fails the guard.
    --
    -- Capped at 255 characters **by the writer, before it is published**, not here. A value longer than the
    -- column is a write MariaDB either truncates or refuses depending on its mode, and a projection that
    -- quietly stores less than it was told is the failure task 352 shipped and task 350 exists to prevent.
    -- The basename only: a path component would be storing somebody's directory layout.
    fileName VARCHAR(255) NOT NULL DEFAULT "",

    uploadedAt DATETIME NOT NULL,

    -- Soft delete, as in `album`, `checkgroup`, `checkpoint` and `person`: the last event wins, and a
    -- flag keeps a re-add expressible.
    --
    -- Soft rather than destructive for two reasons specific to this table. The first is `album`'s: a
    -- removal here may honour somebody's objection, and an accidental one should be recoverable without
    -- republishing a photograph that was taken down on purpose.
    --
    -- The second is new and is the consequence of the content-addressed id (see the header). Because a
    -- re-upload of the same file produces the *same* `photoId`, a destructive delete would make a
    -- deletion silently undoable by a photographer re-dragging a folder — the objection honoured on
    -- Tuesday quietly reversed on Wednesday by somebody who was not told. Hence the rule the `album`
    -- create fold already follows and which the upload fold must follow here: **the upsert does not
    -- touch `deleted`.**
    deleted TINYINT(1) NOT NULL DEFAULT 0,

    updatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (photoId),

    -- The library read: a year's photographs, newest first. The contact sheet's only ordering.
    KEY year_uploaded (year, deleted, uploadedAt),
    -- Every located photograph in the year: the curator's map, and the public one via `album_item`.
    -- `boundsVerdict` is in the key so the read does not have to scan every photograph in the event.
    KEY year_plottable (year, deleted, boundsVerdict),
    -- The shared-blob check, which must be able to ask "does any live photograph reference this ref?"
    -- cheaply — it runs inside the glimt and album delete paths.
    --
    -- **One key per ref column, and every ref column needs one.** A rendition whose column has no index
    -- still gets asked about by `RefsInUse`, so the answer would come from a scan of the year's
    -- photographs inside a delete path.
    KEY ref_lookup (blobRef),
    KEY thumb_lookup (thumbRef),
    KEY medium_lookup (mediumRef),
    -- The original's ref (PRD 027). Indexed for the same reason as the three above, and with more at stake:
    -- `RefsInUse` consults it inside the **library takedown** (task 478), where freeing too much destroys the
    -- only copy of somebody's file and freeing too little leaves an EXIF-bearing photograph on disk after it was
    -- taken down. Neither outcome should also be slow.
    KEY original_lookup (originalRef),
    -- The two video renditions (PRD 029). Same reason as every other ref: `RefsInUse` asks about them inside
    -- a delete path.
    KEY video_lookup (videoRef),
    KEY video_sd_lookup (videoSdRef)
);

-- Which patrols a photograph shows (PRD 022 §8.6, task 367).
--
-- # Why a table rather than a column
--
-- Because two patrols in one frame is ordinary, not a corner case. A `teamId` column on `photo` would
-- make the common case of a group shot inexpressible, and the workaround — the curator picking whichever
-- patrol is more prominent — is a silent editorial decision made by a schema.
--
-- # Both the id and the number, and this is not redundancy
--
-- `public_patrol.teamNumber` is **not unique per year**: that index is deliberately non-unique and the
-- only read does `ORDER BY teamId LIMIT 1`. So a tag keyed on the number would be free to start pointing
-- at a different patrol after a renumbering, and a tag holding only the id would be unreadable to a
-- curator, who knows patrols by the number printed on the sign.
--
-- `teamId` is therefore the identity and `teamNumber` is what it was resolved *from*, captured at the
-- moment a curator confirmed the name back. Keeping the number is what makes the row legible a year
-- later; keeping the id is what makes it correct.
--
-- # A patrol, never a person
--
-- There is no column here for a member and none may be added. A photograph is attributed to a patrulje,
-- which is the rule the public patrol page and the public glimt page already work under (PRD 011 §4,
-- `.rules`). This is stated rather than left to be noticed because a tagging feature is precisely where
-- somebody would reach for a name — and because task 381's structural walk will fail on one.
--
-- # In v1 nothing public reads this
--
-- PRD 022 §11 Q1 resolved that tags *will* surface a photograph on that patrol's own public page, but
-- **not in v1**: for now a tag is curator metadata with no public effect, so the tagging can be used in
-- anger and corrected before a mistag can put a photograph on the wrong family's page. No public read may
-- be written against this table until that second decision is taken.
CREATE TABLE IF NOT EXISTS photo_patrol (
    year VARCHAR(99) NOT NULL,
    photoId VARCHAR(64) NOT NULL,

    -- The resolved patrol. The identity of the tag; see the header.
    teamId VARCHAR(99) NOT NULL,
    -- The number the curator typed, kept for display and for the record.
    teamNumber VARCHAR(99) NOT NULL DEFAULT "",

    taggedAt DATETIME NOT NULL,

    -- Soft delete, matching every other removal in this projection: a re-tag stays expressible and an
    -- accidental untag is recoverable from the log.
    deleted TINYINT(1) NOT NULL DEFAULT 0,

    updatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    -- One tag per patrol per photograph. This is what makes the bulk tag action safe to re-run over a
    -- selection that partly overlaps what is already tagged: the upsert converges instead of duplicating.
    PRIMARY KEY (year, photoId, teamId),

    -- One photograph's tags: the curator's detail view.
    KEY photo_tags (year, photoId, deleted),
    -- One patrol's photographs. Unused in v1 by design (see the header) and indexed anyway, because it
    -- is the read the deferred patrol-page feature is built on and an index is not a disclosure.
    KEY patrol_photos (year, teamId, deleted)
);
