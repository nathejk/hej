-- What this app knows about an event year (task 357).
--
-- # Why it exists
--
-- One thing, today: the two cities. A diploma says *"har gennemført Nathejk 2026 / fra Lundby til Glumsø"*,
-- and until this projection existed there was nowhere in this repo to learn those two place names — the
-- sibling `diplom` service had 2024's hardcoded in its source, which is exactly the arrangement that moving
-- the diploma here was meant to end (`internal/diploma`, task 345).
--
-- # Copied from hq, narrowed
--
-- hq's `nathejk/table/year` folds the same events into seven columns: headline, description, both cities,
-- both dates. This keeps the **cities** and drops the rest, the same way `public_patrol` keeps four fields
-- of a team event and drops the contact details.
--
-- The dropped columns are not sensitive — an editorial headline, a description, the event's dates — so the
-- reason is narrower than that table's: nothing on this surface renders them, and a column nobody reads is
-- a column somebody eventually renders. Adding one back is a line in the fold and a line here, if a page
-- ever wants it.
--
-- # The slug is the key, and it is the year
--
-- `2026`, matching the `year` column everywhere else in this schema and the `EVENT_YEAR` the app is
-- configured with. There is one row per event, so this table is tiny and stays tiny.
CREATE TABLE IF NOT EXISTS event_year (
    slug VARCHAR(99) NOT NULL,

    -- Where the walk starts and where it ends, as the organizers typed them: "Lundby", "Glumsø".
    --
    -- Empty is the normal state until somebody fills them in, and empty must stay renderable as *nothing*:
    -- a diploma whose route line reads "fra  til " is worse than one with no route line, so the read
    -- requires both to be present before it reports a route at all (see querier.Route).
    cityDeparture VARCHAR(99) NOT NULL DEFAULT "",
    cityDestination VARCHAR(99) NOT NULL DEFAULT "",

    updatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (slug)
);
