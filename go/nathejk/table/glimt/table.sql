-- Glimt: a moment a participant shared, and the media it carries (PRD 019).
--
-- # Three tables, not one
--
-- A glimt is one row plus an ordered list of media plus an unordered list of reports. The media
-- could have been a JSON column on `glimt` — the event carries it as a list — but the post-race
-- browse (PRD 019 §0a) reads *thumbnails by the thousand* and never wants the parent row, so the
-- media get their own table and their own key. Reports are separate for a different reason: they
-- are the audit trail behind a takedown, and audit trails are append-only while `glimt` is folded.
--
-- # The vocabulary trap
--
-- `teamNumber` and `teamName` here are the **hold's** — a patrulje or a klan (PRD 019 §0b). They
-- are named to match `person`, from which they are copied, and they have nothing to do with the
-- Team *section* that moderates. Anything gating on moderation reads `person.sectionSlug`, never a
-- column in this file.
--
-- # Everything here is rebuildable except the blobs
--
-- These tables are projections and are replayed from the stream on every boot. `blobRef`/`thumbRef`
-- point into the content-addressed blob store, which is the one thing in this service that cannot
-- be rebuilt and therefore the one thing that must be backed up (PRD 008 §8).
CREATE TABLE IF NOT EXISTS glimt (
    glimtId VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,

    -- The owner. An ownership and moderation column, NOT a display column: only DELETE
    -- authorization and the moderation queue may read it, and every other response projects it
    -- out (PRD 019 §6, task 302). The same discipline `person.phoneParent` gets.
    authorPersonId VARCHAR(99) NOT NULL DEFAULT "",

    -- Who the author was posting as, frozen at creation: spejder, bandit or crew. Decides who a
    -- `group`-scoped glimt reaches, so it must not be re-derived from the author's current role —
    -- a crew member who was out as a bandit stays a bandit for the glimt they posted then.
    authorGroup VARCHAR(32) NOT NULL DEFAULT "",

    -- The hold's number and name, frozen at creation for the same reason: a glimt keeps saying
    -- what it said even if the hold is renamed or the author moves. Number is "" for crew, who
    -- have a section rather than a numbered hold — a normal state, not missing data.
    teamNumber VARCHAR(32) NOT NULL DEFAULT "",
    teamName VARCHAR(255) NOT NULL DEFAULT "",

    -- group | nathejk | public. Immutable after creation: widening it later would retroactively
    -- expose a photo shared under a narrower promise, so there is no update path (PRD 019 §6).
    audience VARCHAR(32) NOT NULL DEFAULT "group",

    caption TEXT NOT NULL,

    -- When the author posted it. The clock retention works from (task 310), which is why it is
    -- stored rather than derived from message delivery time — delivery time changes on replay.
    createdAt DATETIME NOT NULL,

    mediaCount INT NOT NULL DEFAULT 0,

    -- Taken down: by a report (automatically, before any human looks) or by the Team section.
    -- NULL means visible.
    --
    -- Hiding is NOT deleting. The row and the blobs survive so an unhide is possible, and the
    -- author and moderators can still see it. Only the author's DELETE destroys media.
    hiddenAt DATETIME NULL DEFAULT NULL,
    hiddenBy VARCHAR(99) NOT NULL DEFAULT "",

    -- How many reports it has had, for the moderation queue's ordering. Cheaper than counting
    -- glimt_report on every read of a queue someone refreshes at 03:00.
    reportCount INT NOT NULL DEFAULT 0,

    -- Tombstone. A deleted glimt keeps its row so a replay cannot resurrect it and so the feed
    -- can tell "gone" from "never existed".
    deleted TINYINT(1) NOT NULL DEFAULT 0,

    PRIMARY KEY (glimtId, year),

    -- The feed: newest first, filtered by audience.
    KEY year_audience_created (year, audience, createdAt),
    -- The hold collection (PRD 019 §0a.1), the post-race read: one hold, oldest first.
    KEY year_team_created (year, teamNumber, createdAt),
    -- The author's own glimt, for their feed and for DELETE authorization.
    KEY year_author (year, authorPersonId),
    -- The moderation queue: reported and not yet reviewed, first.
    KEY year_reported (year, reportCount, hiddenAt)
);

-- One media item. Ordered within its glimt by `ordinal`, which is the order the author arranged
-- them in and therefore the order they must be shown in.
CREATE TABLE IF NOT EXISTS glimt_media (
    glimtId VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,
    ordinal INT NOT NULL,

    -- Content hashes in the blob store. `thumbRef` may be "" for an item whose thumbnail could
    -- not be produced; the client falls back to the full item rather than showing a gap.
    blobRef VARCHAR(128) NOT NULL,
    thumbRef VARCHAR(128) NOT NULL DEFAULT "",

    -- image | video.
    kind VARCHAR(16) NOT NULL DEFAULT "image",
    contentType VARCHAR(64) NOT NULL DEFAULT "",
    bytes INT NOT NULL DEFAULT 0,
    width INT NOT NULL DEFAULT 0,
    height INT NOT NULL DEFAULT 0,

    -- 0 for a still. Enforced server-side against the parsed container for video (task 322) —
    -- a recorder UI's own limit is not a limit.
    durationMs INT NOT NULL DEFAULT 0,

    PRIMARY KEY (glimtId, year, ordinal)
);

-- Why a glimt was taken down, kept as history.
--
-- Append-only and keyed by reporter, so one person cannot inflate the count by tapping twice while
-- two people reporting the same thing still register as two.
CREATE TABLE IF NOT EXISTS glimt_report (
    glimtId VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,
    reporterPersonId VARCHAR(99) NOT NULL,

    -- Free text, for a human reading the queue. Deliberately not an enum: the useful reasons are
    -- not knowable in advance, and a dropdown would invite picking the nearest wrong one.
    reason TEXT NOT NULL,
    createdAt DATETIME NOT NULL,

    PRIMARY KEY (glimtId, year, reporterPersonId),
    KEY year_created (year, createdAt)
);
