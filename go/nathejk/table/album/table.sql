-- Curated photo albums for the public frontpage (PRD 011 §6 section 1, task 333).
--
-- # What this file holds after PRD 022
--
-- **Arrangements, not photographs.** An album is a title, a URL, an order and a staging state; the
-- photographs it arranges live in `photo` (PRD 022 §8.3, task 364). Before PRD 022 this file held both,
-- and the coordinate, the caption and the blob refs were columns on `album_item`.
--
-- If you came here looking for those, they are in `photo/table.sql`, along with the reasoning about EXIF
-- and the bounds verdict that used to be in this header. What follows is what is still true of an album.
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
-- # Publication lives here, and only here
--
-- This is now the more important half of the split. `photo` has no `published` column and no visibility
-- flag of any kind, so **a photograph reaches the open web only by being referenced from a published row
-- in this file**. The library cannot publish anything; an album can.
--
-- That makes `published`/`deleted` on `album` the single gate PRD 011 §0b depends on, which is why the
-- querier applies them in SQL rather than leaving them to a caller, and why the curator's draft-visible
-- reads are a separate interface that no public handler can reach (PRD 022 §8.8).
--
-- # Everything here is rebuildable, and none of it is bytes
--
-- These are projections, replayed from the stream on every boot. Unlike before PRD 022, nothing in this
-- file points into the blob store at all — `album_item` holds a `photoId`, and the refs it used to carry
-- are `photo`'s. The blob store is still the one thing in this service that cannot be rebuilt and must be
-- backed up (PRD 008 §8); it is simply no longer reachable from here.
--
-- One consequence for anything that deletes blobs: the question "does anything still reference these
-- bytes?" is no longer asked of this file. It is asked of `photo` and of `glimt`, because content
-- addressing means identical bytes are one object and an organizer curating an album from a photograph a
-- participant also posted publicly is the expected workflow rather than a corner case. See
-- cmd/api/glimtdelete.go and task 368.
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

-- One photograph's place in an album.
--
-- # This table is a membership, and used to be a photograph
--
-- Until PRD 022 it *was* the photograph: it carried `blobRef`, `thumbRef`, `caption`, the dimensions and
-- the coordinate inline. Everything that is a fact about a photograph now lives in `photo` (PRD 022
-- §8.3), and what is left here is the only fact this table was ever the right home for: **which
-- photograph sits at which position in which album.**
--
-- The change was forced by two requirements the old shape cannot express — a photograph in *no* album,
-- and the same photograph in *two*. The second is the one that was actually a bug: two albums meant two
-- rows, each with its own copy of the coordinate and the caption, free to disagree, with nothing to
-- notice when they did. A curator who corrected a location in one album and not the other had made a
-- bug invisible from both. See `photo/table.sql`.
--
-- # `(albumId, ordinal)` is still the key, deliberately
--
-- It would have been tidier to key this on `(albumId, photoId)` — that is what a membership *is*, and it
-- would make "add a photograph already in this album" a natural no-op. It is not done, because the
-- ordinal is in the **public URL**: `/api/public/albums/{albumId}/media/{ordinal}`. Changing the key
-- would change how every curated photograph is addressed, on pages families have already been sent.
--
-- The consequence is that adding the same photograph twice at different ordinals is expressible here,
-- and is prevented above this table rather than by it (see the `album_photo` unique key below).
CREATE TABLE IF NOT EXISTS album_item (
    albumId VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,

    -- The position the curator arranged this item in, and therefore the order it is displayed in.
    -- Explicit rather than implied by insertion order, because the ordering is editorial and an
    -- invisible reordering bug is the kind that is noticed by the person who arranged it.
    ordinal INT NOT NULL,

    -- The photograph. A content hash, and a foreign key into `photo` in every sense except the
    -- declaration — these are projections folded from independent event streams, so a real constraint
    -- would make the arrival order of two messages load-bearing. An item naming a photograph that has
    -- not been folded yet reads as an item with nothing to show, which is what a join produces anyway.
    photoId VARCHAR(64) NOT NULL DEFAULT "",

    addedAt DATETIME NOT NULL,

    -- Soft delete, for the same reason as the parent's.
    --
    -- Note this is a *different act* from deleting the photograph: removing an item takes one photograph
    -- out of one album and leaves it in the library and in every other album. PRD 022 §5 requires the
    -- difference to be obvious in the curator's copy, because one of the two is what an organizer means
    -- when they say "take it down".
    deleted TINYINT(1) NOT NULL DEFAULT 0,

    updatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (albumId, ordinal),

    -- One album's items in order: the album page.
    KEY album_order (albumId, deleted, ordinal),

    -- One photograph in one album, at most once. This is what makes "add this selection to this album"
    -- safe to re-run over photographs that are already in it — the insert collides instead of producing
    -- the same picture twice in one grid.
    --
    -- A unique key rather than the primary key, for the URL reason in the header. It does mean a
    -- *reorder* has to move rows rather than rewrite an ordinal in place, which is the price of keeping
    -- the public address stable.
    UNIQUE KEY album_photo (albumId, photoId),

    -- Which albums a photograph is in: the curator's "not in any album" filter, and the reverse lookup
    -- an undelete would need.
    KEY photo_albums (year, photoId, deleted)
);
