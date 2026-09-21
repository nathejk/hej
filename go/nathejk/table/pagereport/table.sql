-- Reports from the open web that a public page should come down (PRD 011 §6, task 343).
--
-- # What this table is for
--
-- A patrol's public page can be *wrong*: a scan attributed to the wrong patrol, a track that is not
-- theirs, a name that changed. And a patrol may have a reason nobody anticipated. A public page about a
-- group of children with no way to write to anyone is how a small problem becomes an angry one — so this
-- is the sink behind the footer's promise ("Skriv til os, så tager vi det ned"), which until now was a
-- promise with nothing behind it.
--
-- # Why it is its own table rather than a column on public_patrol
--
-- Two reasons. A report is an **event that happened**, not a property of the patrol, so several reports
-- about one page must be able to coexist; and `public_patrol` is a projection rebuilt from the upstream
-- team events on every boot, where a locally-written flag would be erased on the next replay.
--
-- It is also not `glimt_report`, even though a report is the same *shape* of thing (PRD 019): that table
-- is keyed by glimt id, and the subject here is a page.
--
-- # What is deliberately absent
--
-- **The reporter's IP address.** It is the obvious identifier and it is not here, for the reason PRD 019
-- recorded: it would put a personal identifier of somebody outside the app into an append-only table, to
-- solve a duplicate-counting problem that does not matter. `reporterPersonId` carries the sentinel
-- `public` instead, so several anonymous reports of the same page are several rows that cannot be told
-- apart by reporter — which is the honest cost and a small one.
--
-- No user agent, no referrer, no session. Somebody reporting a page about children should not have to
-- trade identifying data for it.
--
-- # Who reads it
--
-- Organizers, out of band. There is deliberately **no in-app moderation queue and no auto-hide** — see
-- the handler in cmd/api/patrolreport.go for why a single anonymous report must not take a patrol's page
-- down.
CREATE TABLE IF NOT EXISTS public_page_report (
    reportId VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL DEFAULT "",

    -- What kind of page, and which one. Two columns rather than one URL, so a report survives a change
    -- of URL scheme and so the pair can be indexed: `page` is a closed set (see events.go), `ref` is the
    -- page's own identifier — a patrol *number*, which is how the page is addressed.
    page VARCHAR(32) NOT NULL DEFAULT "",
    ref VARCHAR(99) NOT NULL DEFAULT "",

    -- What the reporter wrote. Optional: a report with no words is still a report, and demanding an
    -- explanation is a way of getting fewer of them.
    reason TEXT,

    -- The sentinel, never an IP. See the header.
    reporterPersonId VARCHAR(99) NOT NULL DEFAULT "",

    reportedAt TIMESTAMP NULL DEFAULT NULL,
    updatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (reportId),

    -- The query an organizer actually runs: "what has been reported about this page?", and "what came in
    -- this year?".
    KEY page_ref (year, page, ref),
    KEY reported (year, reportedAt)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
