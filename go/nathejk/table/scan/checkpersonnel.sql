-- Who was on shift at which checkpoint, and when (PRD 016).
--
-- # Why this table is in the `scan` package
--
-- It has no consumer of its own. It exists solely so a scan can be attributed to a checkpoint, and the
-- join is the interesting part — splitting it into its own package would put the one query that
-- matters in neither. Same arrangement as `person`, which owns a second `section` table for the same
-- kind of reason.
--
-- # This table is load-bearing, and it is fed from outside this repo
--
-- A scan counts for a post if the scanner was on a registered shift there at that moment. So with no
-- shifts recorded, **no scan can be attributed**: every patrol's checkpoint scans read as ordinary
-- unattributed scans, no on-time verdict is computed, and reveal rule 3 never fires. hq's own code
-- says as much about its equivalent — "with no shifts recorded, no scan can be attributed and every
-- team reads as missing".
--
-- That failure is invisible from inside this repo, which is why task 260 counts unattributed scans:
-- the postmandskab rota being incomplete is an upstream data problem that presents here as a feature
-- that quietly does nothing.
CREATE TABLE IF NOT EXISTS checkpersonnel (
    -- The shift's own id, from the subject. A user may have several shifts, at different posts.
    id VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,

    userId VARCHAR(99) NOT NULL DEFAULT "",
    checkpointId VARCHAR(99) NOT NULL DEFAULT "",

    -- The shift window, in unix seconds.
    --
    -- Zero when the shift was added without one, which is a real upstream state rather than an error.
    -- A zero window matches nothing in the attribution join, so such a shift attributes no scans —
    -- the safe direction: attributing a scan to the wrong post would put a wrong post name and a
    -- wrong on-time verdict in front of a patrol.
    startUts INT NOT NULL DEFAULT 0,
    endUts INT NOT NULL DEFAULT 0,

    PRIMARY KEY (id),

    -- The attribution join: given a scanner and an instant, which checkpoint.
    KEY year_user_window (year, userId, startUts, endUts)
);
