-- Recorded position points, for the post-race patrol route (PRD 011 §6, task 340).
--
-- # This projection exists against the PRD's advice, and the advice was unmeasured
--
-- PRD 011 §8.4 asked for the track to be **read from the stream on demand** rather than projected,
-- reasoning that projecting would mean "millions of points into MariaDB (827 participants × ~1,440
-- points) to serve a view opened once per patrol". Two things turned out to be wrong with that, and both
-- were found by checking rather than by thinking harder:
--
--  1. **The library cannot do it.** `stream.Stream` offers `Subscribe` (push) and `LastMessage`. There is
--     no bounded fetch-by-subject, so "read the last 12 hours of one person's subject" is not expressible
--     without dropping to nats.go underneath the abstraction every other read in this service goes
--     through. That would be a second, undocumented path to the broker for one page.
--
--  2. **The volume is ordinary.** 827 participants × ~1,440 points is 1.19M rows of six small columns —
--     roughly 60–100 MB. The `scan` and `person` projections are already the same order of magnitude in
--     bytes. "Millions of points" is true and is not the same as ruinous.
--
-- So the points are projected, and the *expensive* part — merging, gap-breaking and simplifying — happens
-- on read and is cached (internal/patroltrack). That keeps the PRD's actual goal, which was that a public
-- page must not do bulk work per request.
--
-- # Deduplication is the primary key
--
-- Task 083 established the contract that a point is identified by `(person, timestamp)`, because a retry
-- after a timeout can legitimately publish the same point twice, and **the reader is the only place a
-- duplicate can be removed**.
--
-- Here that contract is the PK. An upsert cannot create a second row for the same point, so the dedup
-- requirement is satisfied by construction rather than by a `DISTINCT` somebody has to remember. This is
-- the single strongest argument for projecting rather than reading the stream: on-demand reading would
-- have meant deduplicating a million points in memory, per request, correctly, forever.
--
-- # What is deliberately absent
--
-- No user type, no role, no name. The published event carries `userType` (stamped at publish time,
-- because roles change and the stream is retained indefinitely) and it is not projected: the public page
-- shows one merged patrol route with **no attribution to a person** (PRD 011 §0b.1), so a column that
-- distinguished members would be a column somebody could group by.
--
-- `personId` *is* here, and has to be: the merge is a union of per-person segments, and interleaving
-- points from different people produces a zig-zag between members walking ten metres apart. It never
-- leaves the reader \u2014 `patroltrack.Segment` has no field for it.
CREATE TABLE IF NOT EXISTS track_point (
    year VARCHAR(99) NOT NULL,

    -- Whose phone recorded it. Needed to keep one person's segments together during the merge, and
    -- never returned to a caller. See the header.
    personId VARCHAR(99) NOT NULL,

    -- Epoch **milliseconds**, as the client stores it and the event carries it.
    --
    -- Milliseconds rather than a DATETIME, and this is not laziness: `(person, timestamp)` is the identity
    -- of a point, so the timestamp is a key. An integer cannot be re-serialised into a different-but-equal
    -- form, while a formatted time can — a consumer comparing "…16.954Z" with "…16.954000Z" would see two
    -- points where there is one, and the PK would stop deduplicating.
    ts BIGINT NOT NULL,

    lat DOUBLE NOT NULL,
    lng DOUBLE NOT NULL,

    -- The radius the browser reported, in metres.
    --
    -- Kept because it is the only evidence of how much to trust a point: task 082 measured 10.5 m median
    -- on an iPhone against 35 m on a Wi-Fi-only iPad, and a future decision to drop poor fixes from the
    -- drawn route needs this to make it. Not used by the first version of the map, deliberately — a filter
    -- nobody has calibrated is worse than none.
    accuracy DOUBLE NOT NULL DEFAULT 0,

    -- The dedup contract, as a constraint. See the header.
    PRIMARY KEY (year, personId, ts),

    -- The only read: one person's points in time order, for the merge.
    KEY year_person_ts (year, personId, ts)
);
