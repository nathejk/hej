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

    -- PRD 005's verification. All three are projections of the verification event —
    -- nothing writes them directly.
    --
    -- Three columns rather than two, because the member may acknowledge a number that is
    -- NOT the one on file: if they cannot recognise phoneParent they are asked to supply
    -- the right number and confirm that instead (task 148).
    --
    --   acknowledgedPhone    the number the member says can be reached. AUTHORITATIVE for
    --                        contacting a guardian during the event.
    --   verifiedAgainstPhone what phoneParent held at the moment of acknowledgement.
    --
    -- The pair keeps two questions apart that call for opposite responses:
    --
    --   phoneParent != verifiedAgainstPhone   the register changed since → ask again
    --   acknowledgedPhone != verifiedAgainstPhone   the member corrected us → fix the register
    --
    -- With one column those states are indistinguishable, and a member who corrected us
    -- would be re-asked forever while the register stayed wrong.
    verifiedAt TIMESTAMP NULL DEFAULT NULL,
    acknowledgedPhone VARCHAR(99) NULL DEFAULT NULL,
    verifiedAgainstPhone VARCHAR(99) NULL DEFAULT NULL,

    -- Content hash of the portrait in the blob store (internal/blob). The bytes
    -- never live in this row: a projection rebuild truncates and refills, and
    -- portraits are the one thing that cannot be rebuilt from the log.
    portraitRef VARCHAR(64) NOT NULL DEFAULT "",

    -- Content hash of the *default* thumbnail — the smallest rendition — denormalized
    -- so the common "serve this person's thumbnail" read needs no JSON parsing. Same
    -- reasoning as teamName above. Empty for a portrait captured before thumbnails
    -- existed; readers fall back to the full image rather than treating that as broken.
    portraitThumbRef VARCHAR(64) NOT NULL DEFAULT "",

    -- Every thumbnail rendition, as JSON: name, ref, contentType, bytes, width,
    -- height. A list because more sizes are expected (an identification grid wants a
    -- different size from an avatar), and each carries its own dimensions and byte
    -- count so PRD 007 can budget an offline cache without downloading anything.
    --
    -- JSON rather than a side table because the set is small, always read with its
    -- person, and written only by the portrait event. If something ever needs to query
    -- *across* renditions, that is the moment to normalize.
    portraitThumbs TEXT NULL DEFAULT NULL,

    -- Content hash of the ORIGINAL upload (task 111), stored at its own resolution with
    -- all metadata stripped. Kept so renditions can be produced again later: portraitRef
    -- is a 1024px re-encode, and no sharper crop or new thumbnail size can come from it.
    --
    -- Empty when no original was kept (an upload in a format with no metadata scrubber,
    -- or a portrait captured before this column existed).
    portraitOriginalRef VARCHAR(64) NOT NULL DEFAULT "",

    -- The EXIF orientation the upload declared (1-8, 0 = unknown/none).
    --
    -- Stripping metadata removes the tag from the stored original, so this is the one
    -- piece of it worth keeping: without it a re-render from the original would not know
    -- which way up the face goes. The display renditions already have the rotation
    -- applied, so nothing reading portraitRef needs this.
    portraitOrientation TINYINT NOT NULL DEFAULT 0,

    -- When that portrait was captured. The retention job (task 109) works from this:
    -- "the portrait does not outlive the event" needs an age on the row, and the
    -- message's delivery time is not usable because it changes on every replay. NULL
    -- means "unknown age", which the purge treats as purgeable rather than immortal.
    portraitCapturedAt TIMESTAMP NULL DEFAULT NULL,

    -- Soft delete. A hard DELETE would work too, but a flag keeps the row available
    -- as evidence and makes "was this person removed?" answerable without consulting
    -- the log.
    --
    -- Deletion is NOT sticky, deliberately: an `updated` arriving after a `deleted`
    -- sets this back to 0, because upstream re-adding a member is a real thing that
    -- happens and they should get their login back. Stream order is the truth, and the
    -- last event about a person wins. On the current data this never fires — 433
    -- spejder delete events, 433 rows still deleted (task 076).
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
