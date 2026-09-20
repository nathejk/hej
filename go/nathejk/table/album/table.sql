-- Curated photo albums for the public frontpage (PRD 011 §6 section 1, task 333).
--
-- # What makes these different from glimt
--
-- A glimt is something a participant shared. An album is something an **organizer chose**. That is
-- not a shade of the same thing: it is where the whole safety argument of this feature lives.
--
-- PRD 011 §0b.2 records that permission to publish these photographs was obtained upstream — "if we
-- do not have permission to show a picture it will not be there". So there is deliberately **no
-- rights model here**: no per-subject consent flag, no release register, no approval state. The safety
-- property is *the set* — a photograph is in an album because somebody decided it may be shown — and
-- modelling consent in this schema would imply a check that does not happen here and invite somebody
-- to trust it.
--
-- What the schema does owe that decision is the ability to **take something back out**, because
-- permission is withdrawable and mistakes happen. Hence `deleted` on both tables (task 335).
--
-- # The coordinate column is the point of this table
--
-- `latitude`/`longitude` on an item is the one field here that does not exist for glimt, and it exists
-- for one narrow reason: a curated photograph may be plotted on the public map.
--
-- The media pipeline destroys EXIF — including GPS — by re-encoding, and **that does not change**
-- (`internal/imaging`, PRD 003 §6). The coordinate is read from the original bytes *before* they are
-- re-encoded and written here instead. The distinction is worth stating because it is exactly the kind
-- of thing a later reader will try to simplify away:
--
--   * a coordinate in a column is a decision somebody made, which a curator can see, correct, and
--     delete;
--   * a coordinate inside a stored file is a leak waiting to happen.
--
-- Never "fix" the pipeline to keep EXIF because this column needs a value.
--
-- # Everything here is rebuildable except the blobs
--
-- These are projections, replayed from the stream on every boot. `blobRef`/`thumbRef` point into the
-- content-addressed store, which is the one thing in this service that cannot be rebuilt and therefore
-- the one thing that must be backed up (PRD 008 §8).
--
-- **And those objects may be shared with a glimt.** Content addressing means identical bytes are one
-- object, and an organizer curating an album from a photograph a participant also posted publicly is
-- not a corner case — it is the expected workflow. Anything that deletes blobs must check both tables;
-- see the note in cmd/api/glimtdelete.go.
CREATE TABLE IF NOT EXISTS album (
    albumId VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,

    -- The URL segment: /offentligt/album/<slug>. Stored rather than derived from the title so that
    -- retitling an album does not break a link somebody already shared, which on a public page is a
    -- link in a family's chat history.
    slug VARCHAR(99) NOT NULL DEFAULT "",

    title VARCHAR(255) NOT NULL DEFAULT "",
    description TEXT NOT NULL,

    -- Curator-set order on the frontpage. Explicit, because "3-5 albums" is an editorial sequence and
    -- ordering by creation time would put the one they finished last first.
    sortOrder INT NOT NULL DEFAULT 0,

    -- Unpublished albums are invisible to every public read. A staging state, so a curator can
    -- assemble an album over several sittings without it appearing half-finished on the open web.
    published TINYINT(1) NOT NULL DEFAULT 0,

    createdAt DATETIME NOT NULL,

    -- Soft delete, as in `checkgroup`, `checkpoint` and `person`: the last event wins, and a flag
    -- keeps a re-add expressible. Soft rather than destructive for a reason specific to this table —
    -- a removal here honours somebody's objection, and an accidental one should be recoverable
    -- without republishing a photograph that was taken down on purpose (task 335).
    deleted TINYINT(1) NOT NULL DEFAULT 0,

    updatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (albumId),

    -- The public read: a year's published albums in curator order.
    KEY year_sort (year, deleted, published, sortOrder),
    -- The slug lookup, which is how every album page is addressed.
    UNIQUE KEY year_slug (year, slug)
);

-- One photograph in an album.
--
-- Its own table rather than a JSON column on `album`, for the reason the glimt media table gives: the
-- map reads *located items across every album* and never wants the parent rows, and the frontpage
-- reads one cover per album and never wants the rest.
CREATE TABLE IF NOT EXISTS album_item (
    albumId VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,

    -- The position the curator arranged this item in, and therefore the order it is displayed in.
    -- Explicit rather than implied by insertion order, because the ordering is editorial and an
    -- invisible reordering bug is the kind that is noticed by the person who arranged it.
    ordinal INT NOT NULL,

    -- Content hashes into the blob store. Validated with blob.Ref.Valid before writing, as the glimt
    -- and portrait folds do: a ref is the one string here that could otherwise become a filesystem
    -- path.
    blobRef VARCHAR(64) NOT NULL DEFAULT "",
    thumbRef VARCHAR(64) NOT NULL DEFAULT "",

    caption TEXT NOT NULL,

    width INT NOT NULL DEFAULT 0,
    height INT NOT NULL DEFAULT 0,
    bytes INT NOT NULL DEFAULT 0,

    -- Where the photograph was taken, read from EXIF before the bytes were re-encoded. See the
    -- header. NULL is the normal case — most photographs have no usable fix, and a curator may also
    -- have removed one deliberately.
    latitude DOUBLE NULL DEFAULT NULL,
    longitude DOUBLE NULL DEFAULT NULL,

    -- What the race-area check made of that coordinate: inside | outside | none | unknown.
    --
    -- Stored rather than recomputed on read, and the four values are not three:
    --
    --   * `none`    — the file carried no usable coordinate. Nothing to plot, nothing wrong.
    --   * `inside`  — plottable.
    --   * `outside` — a coordinate that is not in the race area: a phone with no fix, a photograph
    --                 taken at home, a mistyped edit. **Kept, not discarded**, and not plotted. A
    --                 curator has to be able to see that an item was rejected rather than wonder why
    --                 it is missing from the map — and discarding the value would destroy the evidence
    --                 that anything happened.
    --   * `unknown` — there was a coordinate but no race area to judge it against (no positioned
    --                 checkpoints yet). Distinct from `outside` on purpose: `outside` is a statement
    --                 about the photograph, `unknown` is a statement about us, and conflating them
    --                 would silently condemn every early-season upload.
    boundsVerdict VARCHAR(16) NOT NULL DEFAULT "none",

    addedAt DATETIME NOT NULL,

    -- Soft delete, for the same reason as the parent's.
    deleted TINYINT(1) NOT NULL DEFAULT 0,

    updatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (albumId, ordinal),

    -- One album's items in order: the album page.
    KEY album_order (albumId, deleted, ordinal),
    -- Every plottable item in the year, across albums: the map. `boundsVerdict` is in the key so the
    -- read does not have to filter a scan of every photograph in the event.
    KEY year_plottable (year, deleted, boundsVerdict),
    -- The shared-blob check, which must be able to ask "does any album item reference this ref?"
    -- cheaply — it runs inside the glimt delete path.
    KEY ref_lookup (blobRef),
    KEY thumb_lookup (thumbRef)
);
