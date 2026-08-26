CREATE TABLE IF NOT EXISTS checkpoint (
    -- Identity. Keyed per year like every other projection here, because the same
    -- checkpoint id is not reused across events and reads are always year-scoped.
    checkpointId VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,

    name VARCHAR(199) NOT NULL DEFAULT "",

    -- Position. NULL-able, and that is the normal case for some checkpoints rather
    -- than an error: organizers add posts before deciding exactly where they go, and
    -- 3 of 12 had no position a month before the 2026 event. The race-area derivation
    -- tolerates it (PRD 002 §11.2) — a 3 km buffer around the ones that do have
    -- positions absorbs the rest — so "no position yet" must be storable and
    -- distinguishable from 0,0 off the coast of Africa.
    latitude DOUBLE NULL DEFAULT NULL,
    longitude DOUBLE NULL DEFAULT NULL,

    -- Soft delete, for the same reason as `person`: the last event about a checkpoint
    -- wins, and a flag keeps a re-add expressible.
    deleted TINYINT(1) NOT NULL DEFAULT 0,

    updatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (checkpointId, year),

    -- The only read this table serves: "every positioned checkpoint for a year".
    KEY year_deleted (year, deleted)
);
