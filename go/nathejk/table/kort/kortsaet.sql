-- A set of map sheets: most often "Patruljer" and one for everybody else (PRD 016).
--
-- # Why this is a table and not a string on kort
--
-- The team type is a property of the set as a whole. Stored per sheet, five sheets in one set
-- would each carry a copy that can disagree, and "which set is the patrol set?" becomes a question
-- with five possibly-conflicting answers — while the thing we actually do with it is filter the
-- patrol's handout list. It also lets "Patruljer" and "patruljer" drift into two sets.
--
-- Sets stay fully dynamic: the operator creates them, so a year with three sets needs no code
-- change here.
CREATE TABLE IF NOT EXISTS kortsaet (
    id VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,

    name VARCHAR(199) NOT NULL DEFAULT "",

    -- Which team type this set is *specifically for* — not "who may use it".
    --
    -- NULL is the ordinary case, not a missing value: the crew set is for gøglere, banditter and
    -- crew, who are not one team type, and klaner normally draw from it too. Forcing a value would
    -- mean inventing a fictional one.
    --
    -- Deliberately **not unique**. Several sets may carry the same team type: it is a filter that
    -- yields the candidate sheets for a team type, which is what the handout list needs, not a key.
    -- A uniqueness constraint would buy a property nobody consumes and would block a year that
    -- splits its patrol maps into two sets.
    --
    -- The value we filter on is `patrulje`. Not `spejder` — that is the domain's word for a
    -- person, HQ refuses it on write, and a filter against it would match nothing and silently
    -- reveal nothing (PRD 016 §11.1). See kort.PatrolTeamType.
    --
    -- NULL-able, and the consumer must be able to write NULL: a set's updated event carries the
    -- whole record, so an absent teamType means the set has none rather than "unchanged", and an
    -- operator un-marking the patrol set has to be expressible.
    teamType VARCHAR(20) NULL DEFAULT NULL,

    sortOrder INT NOT NULL DEFAULT 0,

    deleted TINYINT(1) NOT NULL DEFAULT 0,

    updatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    KEY year_sort (year, sortOrder),
    -- The read that matters to this app: which sets are for patrols.
    KEY year_teamtype (year, teamType)
);
