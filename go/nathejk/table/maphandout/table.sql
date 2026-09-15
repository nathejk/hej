-- Every map sheet ever handed to a team, kept as history rather than as a current binding.
--
-- # Why history and not "which sheet does this team hold"
--
-- The physical map ↔ QR code ↔ team binding is made in the skan app, whose own projection stores
-- only the *current* holder (one row per year+code, overwritten on re-bind). That is right for skan
-- — it hands the next sheet to whoever holds the code now — but it cannot answer the question this
-- app asks: which maps has *this* patrol been given? When a team is discontinued its remaining
-- scouts, and their sheets, are reassigned to another team, so a sheet legitimately changes hands
-- and skan's row moves with it.
--
-- So this is keyed by (year, code, team) rather than (year, code): one row per team a code was ever
-- bound to. Whether the patrol still holds a sheet is derived on read, as the binding with the
-- newest lastUts for that code.
--
-- # What this app does NOT keep, unlike HQ's equivalent
--
-- HQ's copy resolves and returns the *successor* team — the one holding a reassigned sheet now — so
-- an organizer can chase it. A participant may not see that: it names another team to a patrol who
-- has no business knowing. Our read derives only a boolean ("do we still hold it") and never selects
-- the successor's id, number or name, so there is nothing for a handler to forget to strip. This
-- single field is the clearest reason this projection is a copy of HQ's rather than shared code
-- (PRD 016 §8).
--
-- QR codes and teams are identified as the qr.registered event carries them: the code as a per-year
-- id (stickers restart at 1 each event and may be reused, so the id is only unique within a year —
-- hence year in the key), the team by its id.
CREATE TABLE IF NOT EXISTS maphandout (
    year VARCHAR(99) NOT NULL,
    qrId VARCHAR(99) NOT NULL,
    teamId VARCHAR(99) NOT NULL,

    -- The kort sheet handed over.
    --
    -- "" for a code registered before its sheet was recorded, which means **unknown sheet** rather
    -- than **no sheet** — and must never be erased by a later empty value, the same rule skan's own
    -- projection follows. A patrol holding a sheet nobody wrote down still holds a sheet, and the
    -- app lists it as "Ukendt kort".
    mapId VARCHAR(99) NOT NULL DEFAULT "",

    -- Who scanned the code to bind it, and when this team first and last held it.
    --
    -- Unix **seconds**, as the event carries them — not milliseconds. Anything rendering these has
    -- to scale them, and forgetting to is how a handout appears to have happened in 1970.
    registeredBy VARCHAR(99) NOT NULL DEFAULT "",
    firstUts INT NOT NULL DEFAULT 0,
    lastUts INT NOT NULL DEFAULT 0,

    PRIMARY KEY (year, qrId, teamId),
    -- The two reads: a team's whole history, and every binding of one code (to find its current
    -- holder, which is how "do we still hold it" is answered).
    KEY year_team (year, teamId),
    KEY year_qr (year, qrId, lastUts)
);
