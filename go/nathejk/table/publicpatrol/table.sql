-- What a patrol's public page is allowed to say about it (PRD 011 §8, task 338).
--
-- # Why this table exists when shared-go already projects `patrulje`
--
-- It is not a cache and it is not a convenience. It is the privacy boundary of the public surface,
-- expressed as a schema.
--
-- shared-go's `patrulje` table carries what the organizers need: `contactName`, `contactPhone`,
-- `contactEmail`, `contactRole`. Its `Queries` returns all of them in one struct. Reading the public
-- page through that would mean a leader's name and mobile number sitting in memory, inside the handler
-- that renders an unauthenticated page, one field access away from a template — on the one surface whose
-- entire claim is that it **names no person** (PRD 011 §0b.1, task 337).
--
-- So this projection folds the *same events* and deliberately writes **only** the four fields the page
-- shows. `NathejkTeamUpdated` carries `contactName` and the rest; this consumer reads that event and
-- throws them away. The difference between "we do not select it" and "it is not here" is the difference
-- between a rule somebody can break and one they cannot.
--
-- # The cost, stated plainly
--
-- A second consumer of events shared-go already consumes. That is duplication, and it is the price of the
-- property above. Two things keep it small: the fold is four columns wide, and the events are a published
-- contract rather than shared-go's internals, so this does not couple us to their schema — only to the
-- same messages they read.
--
-- **Do not "fix" this by reading shared-go's table instead.** That is not a refactor, it is a removal of
-- the boundary.
--
-- # What is deliberately absent
--
-- Every contact field. Also `liga`, `memberCount`, `signupStatus` and `paidAmount` — not because they are
-- sensitive, but because the page does not show them, and a column nobody reads is a column somebody
-- will eventually render.
CREATE TABLE IF NOT EXISTS public_patrol (
    teamId VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL DEFAULT "",

    -- The number on the patrol's sign, and the address of its public page.
    --
    -- Empty until a number is assigned, which is a normal state for months: a patrol signs up long
    -- before the numbers are handed out. A patrol with no number has no page, because there is no URL
    -- to reach it by.
    teamNumber VARCHAR(99) NOT NULL DEFAULT "",

    -- The patrol's own name — "Ørnene". A *patrol's* name, never a person's. This is the field the
    -- table's header comment is about: the same event carries `contactName`, and it is not here.
    name VARCHAR(255) NOT NULL DEFAULT "",

    -- The scout group and the korps, for the header line beneath the name.
    --
    -- `korps` is a **slug** (`dds`, `kfuk`, …), not a label. The label comes from
    -- shared-go's `types.CorpsSlug.Label()` at render time rather than being stored, so a corrected
    -- label reaches every page without a replay — and so the stored value stays the one the upstream
    -- event actually said.
    groupName VARCHAR(255) NOT NULL DEFAULT "",
    korps VARCHAR(16) NOT NULL DEFAULT "",

    updatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (teamId),

    -- The only read: one patrol, by the number in its URL. Not unique, deliberately — nothing upstream
    -- guarantees one number per year, and a duplicate must not make the read fail. The querier takes the
    -- first and the reasoning is there.
    KEY year_number (year, teamNumber)
);
