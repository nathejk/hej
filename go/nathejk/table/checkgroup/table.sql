-- The event's checkgroups — the "postlinje" groups a checkpoint belongs to (PRD 016).
--
-- # Why this app needs them
--
-- Two of the three reveal rules are expressed in terms of a *group*, not a checkpoint: scanning any
-- checkpoint reveals its whole checkgroup, and a sheet handed out at a post reveals its checkpoints
-- once that group is reached. And the on-time verdict needs the group's `scheme`, because a relative
-- window is anchored on the patrol's scan at another group.
CREATE TABLE IF NOT EXISTS checkgroup (
    checkgroupId VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,

    name VARCHAR(199) NOT NULL DEFAULT "",

    -- Route order. The outer half of the sequence the map's next-checkpoint arrows follow; the inner
    -- half is checkpoint.sortOrder. One sequence for every patrol — nothing upstream carries a
    -- per-team route (PRD 016 §11.9).
    sortOrder INT NOT NULL DEFAULT 0,

    -- How this group's checkpoints' open windows are expressed: fixed | relative | none.
    --
    --   * fixed    — the checkpoint's openFromUts/openUntilUts are absolute instants.
    --   * relative — the window opens at the patrol's own scan at relativeCheckgroupId and lasts the
    --                checkpoint's openDuration minutes. Per patrol, so it cannot be precomputed.
    --   * none     — no window, and therefore no verdict. Rendering "for sent" here would be a
    --                fabrication.
    --
    -- Empty until an update has been seen, which reads as "we do not know yet" and yields no verdict —
    -- the safe direction, since a wrong "for sent" is worse than a missing one.
    scheme VARCHAR(20) NOT NULL DEFAULT "",

    -- With scheme=relative, the group whose scan starts the window.
    --
    -- Stored unresolved: a group that has since been deleted is handled on read, not by rewriting
    -- this. Nothing re-publishes a checkgroup when another one is deleted, so a write-time cascade
    -- would depend on replay order between two independent projections.
    relativeCheckgroupId VARCHAR(99) NOT NULL DEFAULT "",

    -- Soft delete, as in `checkpoint` and `person`: the last event wins, and a flag keeps a re-add
    -- expressible.
    deleted TINYINT(1) NOT NULL DEFAULT 0,

    updatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (checkgroupId, year),

    -- The read: a year's groups in route order.
    KEY year_sort (year, deleted, sortOrder)
);

-- `showOnMap` is deliberately absent.
--
-- It exists upstream on this entity, it is written by an organizer toggle in HQ's postlinje screen,
-- and — as far as the code goes — it is read by nothing at all: no map, no export, no
-- participant-facing surface. Its intent is therefore unverified, and both ways of guessing are bad.
-- Treating it as a gate would hide every checkpoint if organizers never set it; treating it as
-- permission would leak if it actually means "show on the planning map".
--
-- The deeper reason not to store it is that our rule does not need it. Reveal here is grounded in
-- **physical possession**: a checkpoint drawn on the paper sheet in a patrol's hand, or on a post they
-- have already stood at, cannot be a secret from that patrol. A rule anchored in what someone already
-- holds cannot over-reveal, whatever a flag says.
--
-- If organizers confirm it is participant-facing it becomes an *additional* filter, never a
-- replacement. Until then, not consuming it is the honest position (PRD 016 §11.3). Do not "fix" this
-- omission without that answer.
