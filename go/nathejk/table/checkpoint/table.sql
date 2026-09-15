CREATE TABLE IF NOT EXISTS checkpoint (
    -- Identity. Keyed per year like every other projection here, because the same
    -- checkpoint id is not reused across events and reads are always year-scoped.
    checkpointId VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,

    name VARCHAR(199) NOT NULL DEFAULT "",

    -- The checkgroup (postlinje group) this checkpoint belongs to.
    --
    -- Load-bearing for the reveal rule (PRD 016): scanning any checkpoint reveals its whole
    -- checkgroup, and a sheet handed out at a post reveals its checkpoints once that *group* is
    -- reached. Without this column neither rule can be evaluated.
    --
    -- Arrives on `.created`, which is why this projection now consumes an event family it used to
    -- skip. Empty until a create has been seen for the checkpoint, which is normal during a partial
    -- replay rather than an error.
    checkgroupId VARCHAR(99) NOT NULL DEFAULT "",

    -- Position within the checkgroup, from `checkpoints_sorted`.
    --
    -- The inner half of route order; the outer half is checkgroup.sortOrder. Together they are the
    -- sequence the next-checkpoint arrows follow, and it is one sequence for every patrol — nothing
    -- upstream carries a per-team route (PRD 016 §11.9).
    sortOrder INT NOT NULL DEFAULT 0,

    -- When the post is open, and therefore whether a patrol reached it on time.
    --
    -- Three schemes upstream, and all three have to be storable:
    --
    --   * fixed    — openFromUts/openUntilUts are absolute instants.
    --   * relative — openDuration minutes, counted from the patrol's own scan at another
    --                checkgroup. The anchor is per patrol, so it cannot live here; only the
    --                duration can.
    --   * none     — no window at all, and therefore no verdict. Zero in all three columns.
    --
    -- openDuration is in **minutes**, matching how the upstream event's duration is stored. A
    -- duration in one unit and instants in another is asking for a bug, but changing it here would
    -- put us out of step with the organizers' own screens, which is worse.
    --
    -- Zero means "not set" rather than "midnight 1970": a real window on this event is always far
    -- from the epoch, so the ambiguity is theoretical, and NULL columns would make the on-time
    -- comparison three-valued for no gain.
    openFromUts INT NOT NULL DEFAULT 0,
    openUntilUts INT NOT NULL DEFAULT 0,
    openDuration INT NOT NULL DEFAULT 0,

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

    -- "Every positioned checkpoint for a year", which the race area is derived from.
    KEY year_deleted (year, deleted),

    -- The reveal rule's read: given the checkgroups a patrol has reached, which checkpoints. Also
    -- serves route order, which is (checkgroup.sortOrder, checkpoint.sortOrder).
    KEY year_checkgroup_sort (year, checkgroupId, sortOrder)
);
