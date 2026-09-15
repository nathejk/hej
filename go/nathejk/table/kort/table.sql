-- The printed map sheets HQ defines, mirrored here so this app can tell a patrol which sheets it
-- holds and which checkpoints they reveal (PRD 016).
--
-- A row is one *sheet* — one thing physically handed to a patrol, carrying one QR code, causing
-- one reveal. That is why a double-sided A3 is a single row with two extents rather than two rows:
-- splitting it to record its geometry would double-count the handover, which is exactly the thing
-- this app reasons about when it decides what a patrol may see.
--
-- # This is a copy of HQ's schema, not a shared one
--
-- Deliberately (PRD 016 §8). HQ's read models answer an organizer's questions and ours answer a
-- patrol's, and they diverge immediately — most sharply over the successor team that now holds a
-- reassigned sheet, which is a field HQ shows and we are forbidden to. A copy makes that
-- divergence a normal edit instead of a negotiation, and means an HQ refactor cannot break an app
-- running during an event.
--
-- The comments below are copied along with the columns, on purpose: they record hazards learned
-- the hard way upstream, and a copy that drops them re-learns them during a race.
CREATE TABLE IF NOT EXISTS kort (
    id VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,

    -- The set this sheet belongs to. Exactly one: a sheet is printed for one audience.
    --
    -- Not a foreign key — no projection in this codebase declares one, and here it would be
    -- actively wrong: replay delivers events in stream order, so a sheet may legitimately be
    -- materialised before its set. An unknown kortsaetId is tolerated, never dropped.
    kortsaetId VARCHAR(99) NOT NULL DEFAULT "",

    name VARCHAR(199) NOT NULL DEFAULT "",

    -- a4 | a3 | skitse | andet. A skitse is the interesting one: a hand-drawn slip with no QR
    -- code and usually no extent, whose only trace in the system is its checkpoint list — which
    -- is why a sheet with no area still matters here.
    --
    -- Stored as sent, without validation: a fifth format added upstream must not cost us a sheet.
    format VARCHAR(20) NOT NULL DEFAULT "",

    note TEXT NOT NULL DEFAULT "",

    -- Handout order along the route, within a set.
    sortOrder INT NOT NULL DEFAULT 0,

    -- Where this sheet is handed out: the id of the checkgroup whose post gives it to the patrol,
    -- or "" for "at the QR scan". This is the sheet's *reveal trigger*, and the whole reason this
    -- projection exists rather than just the handout one.
    --
    -- "" is the default and means the QR rule, which is the behaviour that existed before this
    -- column upstream: an unconfigured sheet therefore keeps working exactly as it did. It is a
    -- value, never "unknown".
    --
    -- Not a foreign key, and a checkgroup that has since been deleted is treated as "" on read
    -- (see querier.Sheets). That is the safe direction: the alternative is a reveal keyed to a
    -- post that will never be reached, so those checkpoints would never appear at all.
    handoutCheckgroupId VARCHAR(99) NOT NULL DEFAULT "",

    -- JSON array of checkpoint ids drawn on this sheet.
    --
    -- An array rather than a join table because the relation is read in exactly one direction —
    -- given a sheet, which checkpoints — and we never ask which sheets contain a given checkpoint.
    -- A join table would cost a row per assignment and a join on every read to answer a question
    -- nobody asks.
    --
    -- The price is paid on deletion: removing a vanished checkpoint from every sheet would be JSON
    -- surgery. We do not pay it — ids are resolved against our own checkpoint projection on read
    -- and unresolvable ones dropped (task 256). At ~15 sheets a year that filter is free, and the
    -- read model rebuilds from the stream, so a bug there is fixed by replay rather than by
    -- migration.
    --
    -- `[]` and not NULL when empty: a pure overview map legitimately has no checkpoints, and every
    -- reader should decode an array rather than branch on NULL.
    --
    -- TEXT rather than MariaDB's JSON type, following the established spelling in this codebase:
    -- JSON is an alias for LONGTEXT there anyway and the JSON functions work on TEXT.
    checkpointIds TEXT NOT NULL DEFAULT "[]",

    -- JSON array of 0-2 {northWest:{latitude,longitude}, southEast:{...}} rectangles.
    --
    -- A list because a double-sided sheet shows two different areas. Nothing records which is the
    -- front, and the checkpoints are not split per side — both sides are handed over at once, so
    -- the distinction has no consumer. Empty is normal: a skitse has no area worth recording.
    extents TEXT NOT NULL DEFAULT "[]",

    -- Soft delete, for the same reason as `checkpoint` and `person`: the last event about a sheet
    -- wins, and a flag keeps a re-add expressible.
    deleted TINYINT(1) NOT NULL DEFAULT 0,

    updatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    -- The only read: a year's sheets, grouped by set, in handout order.
    KEY year_set_sort (year, kortsaetId, sortOrder)
);
