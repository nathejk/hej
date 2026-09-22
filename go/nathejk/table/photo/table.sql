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
-- A projection, replayed from the stream on every boot. `blobRef`/`thumbRef` point into the
-- content-addressed store, which is the one thing in this service that cannot be rebuilt and therefore
-- the one thing that must be backed up (PRD 008 §8).
--
-- **PRD 022 resolved that these blobs are never purged** (§11 Q2): unlike glimt, nobody was told these
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
    -- `thumbRef` may be "" when one could not be produced; readers fall back to the full image rather
    -- than rendering a gap, as the glimt grid does.
    blobRef VARCHAR(64) NOT NULL DEFAULT "",
    thumbRef VARCHAR(64) NOT NULL DEFAULT "",

    -- The curator's words for this photograph, shared by every album it appears in.
    --
    -- One caption rather than one per album membership. A photograph in "Natten" and in "Postmandskabet"
    -- could conceivably want different words, and that override is deliberately **not** built (PRD 022
    -- §11 Q3) — because the cost of guessing wrong here is the exact bug this table was created to
    -- remove: two copies of one fact, drifting.
    caption TEXT NOT NULL,

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
    KEY ref_lookup (blobRef),
    KEY thumb_lookup (thumbRef)
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
