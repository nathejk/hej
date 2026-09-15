-- Every QR scan of a team's code (PRD 016).
--
-- One row per scan event. A scan is what happens when someone \u2014 post personnel, typically \u2014 scans the
-- code a patrol carries.
--
-- # A scan does not say which checkpoint it happened at
--
-- That is the central awkwardness of this table, and the reason `checkpersonnel.sql` exists next to
-- it. The event carries *who scanned* and *when*, and nothing else about place. The checkpoint is
-- recovered by asking which post that scanner was on shift at, at that moment.
CREATE TABLE IF NOT EXISTS scan (
    -- (qrId, uts) rather than a scan id: the event carries no id of its own, and one code cannot be
    -- scanned twice in the same second. Same key hq uses.
    qrId VARCHAR(99) NOT NULL,
    uts INT NOT NULL DEFAULT 0,

    year VARCHAR(99) NOT NULL,

    -- The team whose code was scanned. This is what makes a scan "ours" for a patrol: it matches the
    -- user's PatrolID.
    teamId VARCHAR(99) NOT NULL DEFAULT "",
    teamNumber VARCHAR(99) NOT NULL DEFAULT "",

    -- Who scanned it. The only link to a place: joined against checkpersonnel shifts to recover the
    -- checkpoint.
    scannerId VARCHAR(99) NOT NULL DEFAULT "",

    -- Where the scanning device was, as strings \u2014 which is how the event carries them.
    --
    -- Kept as sent rather than parsed on the way in. A scan with an unparseable or absent position is
    -- still a scan that happened, and must still be listed; parsing here would tempt a handler into
    -- dropping the row. The read parses and yields no position when it cannot, which the client
    -- already handles (it lists un-positioned registrations without plotting them).
    latitude VARCHAR(99) NOT NULL DEFAULT "",
    longitude VARCHAR(99) NOT NULL DEFAULT "",

    PRIMARY KEY (qrId, uts),

    -- The only read: a team's scans, newest first.
    KEY year_team (year, teamId, uts)
);
