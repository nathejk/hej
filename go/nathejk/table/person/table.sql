CREATE TABLE IF NOT EXISTS person (
    -- Identity. personId is the upstream member/user id, so it survives a phone
    -- change and is what a session cookie carries.
    personId VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,

    -- The app role this person plays (users.Role). Owned by this projection's
    -- classifier, never inferred downstream from a team type or a section slug.
    appRole VARCHAR(32) NOT NULL DEFAULT "",

    -- Contact. phone is stored NORMALIZED (internal/phone) because it is the login
    -- lookup key and a lookup on an unnormalized column silently misses.
    name VARCHAR(199) NOT NULL DEFAULT "",
    phone VARCHAR(99) NOT NULL DEFAULT "",

    -- Guardian/emergency contact. Only spejder have one (confirmed against
    -- shared-go: PhoneParent exists on `spejder` and on no other population), so
    -- "not applicable" must be distinguishable from "missing" — hence NULL rather
    -- than an empty-string default. PRD 005's confirmation step reads this, and
    -- rendering a blank for a bandit as though data were absent would be wrong.
    phoneParent VARCHAR(99) NULL DEFAULT NULL,

    -- Profile details for PRD 003.
    address VARCHAR(199) NOT NULL DEFAULT "",
    postalCode VARCHAR(32) NOT NULL DEFAULT "",
    city VARCHAR(99) NOT NULL DEFAULT "",
    email VARCHAR(199) NOT NULL DEFAULT "",
    birthday DATE NULL DEFAULT NULL,

    -- Team identity for PRD 002's patrol-scoped reads.
    --
    -- For a spejder this is their patrulje and for a bandit their klan; crew and
    -- gøglere have no team, which is why the section columns below exist. Both are
    -- carried because the login chooser has to tell two people on one phone apart,
    -- and "which patrulje" or "which section" is often the only difference between
    -- them (task 079).
    teamId VARCHAR(99) NOT NULL DEFAULT "",
    teamName VARCHAR(199) NOT NULL DEFAULT "",

    -- Crew affiliation. sectionSlug is the organizer-authored key the app classifies
    -- a crew function from (see classify.go); sectionName is its human label, stored
    -- denormalized for the same reason teamName is — the login path reads one row and
    -- must not join.
    sectionSlug VARCHAR(99) NOT NULL DEFAULT "",
    sectionName VARCHAR(199) NOT NULL DEFAULT "",

    -- Where the member is in the event lifecycle (types.MemberStatus). PRD 005's
    -- skip rule reads it: `racing` onwards means they have started, so the
    -- confirmation step is skipped.
    memberStatus VARCHAR(32) NOT NULL DEFAULT "",

    -- Bandits are identified in the field by arm number rather than by face, which
    -- is why it is carried here (PRD 007 §4 keeps it as the non-photo fallback).
    armNumber VARCHAR(32) NOT NULL DEFAULT "",

    -- PRD 005's verification. verifiedAt is a projection of the verification event
    -- (nothing writes it directly), and acknowledgedPhone records WHICH number was
    -- confirmed so a later guardian-number change can invalidate it (task 076).
    verifiedAt TIMESTAMP NULL DEFAULT NULL,
    acknowledgedPhone VARCHAR(99) NULL DEFAULT NULL,

    -- Content hash of the portrait in the blob store (internal/blob). The bytes
    -- never live in this row: a projection rebuild truncates and refills, and
    -- portraits are the one thing that cannot be rebuilt from the log.
    portraitRef VARCHAR(64) NOT NULL DEFAULT "",

    -- Soft delete. A hard DELETE would work too, but a flag keeps a replay's
    -- ordering harmless: a `deleted` arriving before a late `updated` still leaves
    -- the person excluded, whereas a delete-then-insert would resurrect them.
    deleted TINYINT(1) NOT NULL DEFAULT 0,

    updatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (personId, year),

    -- The login lookup. Deliberately a plain KEY, not UNIQUE: two people can share
    -- a number (siblings, or a guardian's number entered as the scout's own) and a
    -- UNIQUE constraint would make the projector fail on real data rather than let
    -- the collision policy decide. See task 071.
    KEY year_phone (year, phone),
    KEY year_team (year, teamId),
    KEY year_role (year, appRole),
    KEY year_section (year, sectionSlug)
);
