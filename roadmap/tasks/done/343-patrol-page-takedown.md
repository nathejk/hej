# 343 — Takedown affordance on the patrol page

**Status:** done
**Priority:** medium
**Created:** 2026-09-19
**Picked up by:** agent
**Started:** 2026-09-21
**Completed:** 2026-09-21

## Description

PRD 011 §6. A visible route to "take this down" on a patrol's public page.

**Not because the design is in doubt** — PRD 011 §0b.1 settled that a patrol's merged route may be public,
and the right to publish the tracks has been obtained. This exists because a page can be *wrong* (a scan
attributed to the wrong patrol, a track that is not theirs, a name that changed), and because a patrol may
have a reason nobody anticipated. A public page about a group of children with no way to write to anyone is
the kind of thing that turns a small problem into an angry one.

The public glimt page already has this affordance (`glimtpublic.go`'s footer: *"Er der et billede, der ikke
skal ligge her? Skriv til os, så tager vi det ned."*). Match its tone and mechanism rather than inventing a
second reporting path — and note PRD 019 already built an anonymous report endpoint with a by-IP rate limit
and a `publicReporterSentinel` for the audit trail. Reuse that thinking; a patrol-page report is the same
shape of thing.

Two details worth carrying from PRD 019's reasoning:

- **Do not record the reporter's IP** as an identifier. PRD 019 chose a sentinel instead, accepting that
  several anonymous reports collapse into one row, because recording an IP would put a personal identifier
  of someone outside the app into an append-only audit table to solve a duplicate-counting problem that does
  not matter.
- **The cost of a spurious report is a moderator's glance; the cost of a throttled one is a page somebody
  objected to staying up.** So be generous with the limit.

Whether a report auto-hides a patrol page (as it does a glimt) or only notifies is a judgement for this
task: a patrol page is not a photograph, and hiding one on a single anonymous report is easier to abuse.
Recommend notify-only, and record the reasoning either way.

## Acceptance Criteria

- [x] A visible, plainly-worded takedown route on every patrol page, matching the glimt page's tone.
- [x] Reuses PRD 019's anonymous-report shape rather than a second mechanism.
- [x] No reporter IP stored as an identifier; sentinel used, following `publicReporterSentinel`.
- [x] By-IP rate limited, generously (see task 347).
- [x] The auto-hide-vs-notify decision is made and its reasoning recorded in this task's log.
- [x] Reachable and usable with JavaScript disabled.
- [x] Consistent with the copy written in task 331, which also names a route to ask for removal.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §10 (Phase 2).
- 2026-09-21 — **Done.** `POST /offentligt/patrulje/{number}/anmeld` (`cmd/api/patrolreport.go`), a
  `<details>` form on the patrol page, and a new projection `nathejk/table/pagereport` writing
  `public_page_report`.

  **Decision: notify, not auto-hide.** A reported glimt is hidden before any human looks; a reported
  patrol page is not. Four reasons, in order of weight:

  1. *The subject differs.* A glimt is one photograph from one member. A patrol page is the record of
     eight people's night, and the page their grandparents were sent a link to.
  2. *The abuse is trivial and the cost asymmetric.* Anyone can POST here with no account. Auto-hide
     would let one anonymous stranger remove any patrol's page from the open web, repeatedly, and the
     patrol would have no way to tell it was not us.
  3. *There is no undo path.* Un-hiding a glimt is a moderation action with a surface; un-hiding a patrol
     page is not — auto-hide would be a one-way door with an anonymous handle on it.
  4. *The urgency is lower than it looks.* The page names no person (task 337); the likeliest complaint
     is a wrong attribution, which is a correction, and a correction that takes an hour is fine.

  **The condition that makes this defensible is that somebody reads the table.** If nobody does, the
  decision stops holding and auto-hide becomes the lesser evil. That is a process condition, recorded in
  the handler so whoever revisits it finds the reasoning rather than re-deriving it.

  **A form POST, not JSON**, so it works with JavaScript off — and POST-redirect-GET (303) back to the
  page, so a reload does not file the report twice and the acknowledgement lands where the visitor was
  reading. `Reported` comes off `?anmeldt=1`, never from stored state: "others have complained about your
  patrol's page" is not something a public page should tell a visitor.

  **Same gate as the page.** Reporting is only possible where the page is open. Otherwise the route would
  answer differently for a real closed patrol than for a number that does not exist and become the
  discovery oracle the whole surface avoids being (task 330).

  **No CSRF token, deliberately.** There is no session here, so there is no authority to borrow: a forged
  cross-site POST files a report anyone could file by hand. A token would also break the JS-off property
  for no gain.

  **The reporter column is normalised twice.** The handler sends the sentinel, and the *projection* also
  replaces anything address-shaped (`looksLikeAddress`) before it writes. The second lock is the one that
  matters: the projection is the last point before something is written into an append-only table, and
  what must never be written there is an identifier for somebody outside the app.

  **Why its own table.** Not a column on `public_patrol` — that projection is rebuilt from upstream events
  on every boot, so a locally-written flag would be erased — and not `glimt_report`, which is keyed by
  glimt id. Deliberately **no querier**: there is no in-app moderation surface, organizers read the table
  out of band, and a query nothing calls invites somebody to build the surface it implies.

  **Two false positives my own tests caught, both mine:**
  - `TestAClosedPageLeaksNothingAboutARealPatrol` greps the whole document for `"43"` — and my
    thank-you box's border colour `#16653433` contains it. Colour changed, and the sharp edge is now
    documented in the test rather than waiting for the next person.
  - The same shape again: `TestPatrolPageOmitsAnUnspecifiedKorpsAndGroup` forbids the word *andet*, which
    my copy used ("eller noget helt andet"). Reworded to *tredje*.
  - And `TestIslandAssetsAreVendored` from task 342 failed **in the api dev container**, which mounts only
    `go/` — so there is no `vue/public` to stat. It now skips where the frontend tree is absent, which is
    the honest scope: it bites on a full checkout and in CI.

  **Verified end to end in the dev stack**, not just in tests: the form POST answered 303 with
  `Location: /offentligt/patrulje/71?anmeldt=1`, and the row appeared in `public_page_report` with
  `page=patrol`, `ref=71`, `reporterPersonId=public` — so the publish, the subject, the consumer and the
  schema all agree.

  **One thing left undone on purpose:** the footer's promise (task 331) is now backed on patrol pages and
  on the glimt page, but the frontpage and album pages still say "skriv til os" with no form. Album
  takedown is the curator's path (task 335) and the frontpage has nothing of its own to take down, so
  adding a form there would be a route to report nothing in particular.

  `gofmt`, `go vet`, `go test ./...` clean.
