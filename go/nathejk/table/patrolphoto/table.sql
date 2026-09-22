-- A patrol's photographs, and which one represents it (task 361).
--
-- # What this is for
--
-- The diploma carries the patrol's photograph, as `diplom`'s version has since 2024, and the same pictures will
-- feed a public gallery shortly. Both need two facts: which photographs a patrol has, and which one an organizer
-- chose to represent it.
--
-- # Two tables, one package — a deliberate divergence from hq
--
-- hq keeps `photo` and `photocover` in separate packages because its `photo` is a **verbatim** copy of foto's,
-- bound for shared-go, and must not be touched. This copy is already narrowed, so that constraint does not
-- apply, and the read this app actually makes — *"which photograph represents patrol 42?"* — needs both tables
-- in one query. One consumer for three verbs is also less wiring than two.
--
-- # What is deliberately absent, and why it matters here more than elsewhere
--
-- **The original.** foto stores the uploaded file byte-identical, metadata and all, and its own read model
-- refuses to serve it: an original can carry the GPS coordinates of where a child was photographed. hq records
-- `originalRef` for provenance. This table has **no column for it at all**, so no query here can name one and no
-- handler can serve one by mistake. What this app can reach is a re-encoded rendition, which cannot carry EXIF
-- because it was rebuilt from pixels.
--
-- Also absent: `sourceUrl`/`sourceKind` (provenance is foto's business and hq's record), `bytes`, and the
-- renditions JSON — of which only the smallest ref is kept, denormalised as `thumbRef`, because that is the one
-- a gallery grid needs and parsing JSON per row to find it is work for nothing.
CREATE TABLE IF NOT EXISTS patrol_photo (
    year VARCHAR(99) NOT NULL DEFAULT "",
    teamId VARCHAR(99) NOT NULL DEFAULT "",

    -- The camera app's category: "start", "finish", … Free text, because it arrives as a query parameter
    -- upstream and an enum here would drop a photograph over a label.
    type VARCHAR(99) NOT NULL DEFAULT "",

    -- The content hash of the display image, and the key this photograph is known by everywhere — including in
    -- this app's own blob store, which is sha256-content-addressed exactly as foto's is. That is what lets the
    -- bytes be fetched once, verified against this value, and stored under the same name.
    ref CHAR(64) NOT NULL,

    -- The smallest rendition, for a grid. Empty for a photograph recorded before rendition sets existed; a
    -- reader must fall back to `ref` rather than treat it as broken.
    thumbRef CHAR(64) NOT NULL DEFAULT "",

    contentType VARCHAR(99) NOT NULL DEFAULT "",
    width INT NOT NULL DEFAULT 0,
    height INT NOT NULL DEFAULT 0,

    -- The crew's "needs a look" flag (the camera app's XXX_ filename prefix). Carried upstream without meaning;
    -- this app gives it one, and it is a conservative one: a flagged photograph is never chosen *automatically*
    -- for a public surface. See querier.Cover for why an explicit choice still wins over it.
    attention TINYINT(1) NOT NULL DEFAULT 0,

    -- When the shutter was pressed, as the camera app reported it. The ordering key for "the newest start
    -- photograph", and not the same thing as when the event arrived.
    capturedAt DATETIME NULL DEFAULT NULL,

    -- A patrol has many photographs, so identity is the content hash. `type` is in the key because the same
    -- bytes filed as both "start" and "finish" are two facts, not one overwriting the other.
    PRIMARY KEY (year, teamId, type, ref),

    -- The read: a patrol's photographs, newest first.
    KEY idx_patrol_photo_team (year, teamId, capturedAt)
);

-- Which photograph represents a patrol, when a human has said.
--
-- Its own table because it is a different fact with a different key: one choice per patrol, where `patrol_photo`
-- has one row per photograph. A row whose `ref` is empty means "explicitly no choice", which is how a selection
-- is undone — and it survives a replay in which the clearing arrives before a later reconsideration, where a
-- deleted row would not.
CREATE TABLE IF NOT EXISTS patrol_photo_cover (
    year VARCHAR(99) NOT NULL DEFAULT "",
    teamId VARCHAR(99) NOT NULL DEFAULT "",
    ref CHAR(64) NOT NULL DEFAULT "",
    selectedAt DATETIME NULL DEFAULT NULL,

    PRIMARY KEY (year, teamId)
);
