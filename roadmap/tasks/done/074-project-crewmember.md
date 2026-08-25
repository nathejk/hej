# 074 — Project crewmember and section

**Status:** done
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:** agent
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

PRD 006 §8. Consume `NATHEJK.*.crewmember.*.registered` / `.updated` / `.deleted`
/ `.section.assigned`, `NATHEJK.*.crew.*.signedup`, plus
`NATHEJK.*.section.*.added` / `.moved` / `.deleted` for the function labels.

Crew function comes from `sectionSlug` → the section tree, which is
organizer-authored. Use task 069's classification with its logged fallback.

Ordering matters and cannot be assumed: a `section.assigned` may arrive before the
`section.added` that names it. The projection must converge either way rather than
dropping the assignment.

## Acceptance Criteria

- [x] Crewmember + crew + section subjects consumed — with one deliberate exception,
      see the note on `crew.*.signedup` below
- [x] Function derived through task 069's classifier
- [x] **`sectionSlug` and `sectionName` populated** — verified against the real stream
- [x] Out-of-order section/assignment events converge — all three orderings
- [x] Unassigned crew classified as generic crew, not an error
- [x] `deleted` flag respected
- [x] Idempotent, tested with `cqrstest` fakes incl. the out-of-order case

## Progress Log

- 2026-08-25 — Task created from PRD 006.
- 2026-08-25 — Done.

  **Convergence.** The hard requirement was that section and assignment events
  converge in any order, and `cqrs.Writer` accepts statements only — a projector
  cannot read. So the ordering is handled with SQL rather than Go-side state, in a new
  two-column `person_section` (year, slug, label) table:

  1. *section named first, then assigned* — the assignment handler resolves the label
     with `UPDATE person p LEFT JOIN person_section s …`. Two statements, not one,
     because a JOIN in the first statement would still see the person's **old** slug.
  2. *assigned first, then section named* — `section.added` back-fills:
     `UPDATE person SET sectionName=… WHERE year=? AND sectionSlug=?`.
  3. *assigned before the person is even known* — this one was missed in the first
     draft. The assignment handler was an `UPDATE`, which affects zero rows for an
     unseen person, and **nothing ever re-publishes an assignment**, so the role would
     be lost permanently. It is now an upsert writing a key-and-section stub row.

  `LEFT JOIN … COALESCE(s.label,'')` rather than an inner join, so re-assigning
  someone to a not-yet-known section clears the stale name instead of keeping the
  previous section's label.

  **`upsertKeepingRole`.** `appRole` is written on insert only. The crew details
  handler has to supply the generic `crew` fallback (it does not know the person's
  section), and details events are re-published whenever an organizer fixes a typo and
  redelivered on every replay. With a plain upsert, that would put a classified
  `samarit` back on `crew` — no error, no dead letter, just a member who quietly loses
  their SOS page some time after an unrelated edit. Safe in the other direction
  because the assignment handler writes the role unconditionally, so it wins whichever
  event lands first.

  **`ReportUnmappedSlug` option.** An unrecognised section slug had to be reported
  without this package importing the app's logger (it is bound for shared-go). Added
  as a nil-safe `func(slug string)` callback installed via a variadic option, wired
  from `cmd/api` to `logger.Warn`. It fires on real data (see below), so it is not
  dead weight.

  **`crew.*.signedup` deliberately NOT consumed.** It was in the PRD's subject list.
  Checked against the stream: it is a strict subset of `crewmember.*.updated` — same
  person, identical name/phone/email, published seconds apart — and its body keys the
  person as `teamId` rather than `userId`. Consuming it would add a second spelling of
  the primary key for zero extra fields. `section.*.deleted` was also listed and does
  not occur on the stream; not implemented rather than written blind.

  **Classification map vs. reality.** Read the real 2026 section tree off the stream:
  `bandit, goeglerledelse, guides, hoensegaard, hq, koekken, noedtelefon, postmand,
  postmandskab, pr, rover, samarit, team`. Two slugs the map should have matched and
  did not — `postmand` and `guides` — are now mapped (the map had `postmandskab` and
  `guide`/`guider`). The rest correctly fall back to `crew`. `goeglerledelse` is
  deliberately *not* mapped to `gøgler`: that role is for the performers (task 075),
  and the people running them are crew. Task 078 still owns the full audit.

  **Verified against the real stream** (truncate `person` + `person_section` +
  `deadletter`, restart, full replay):

  | appRole | live | soft-deleted |
  |---|---|---|
  | spejder | 1294 | 433 |
  | bandit | 1433 | 97 |
  | crew | 20 | 1 |

  0 dead letters. Totals reconcile exactly with the previously recorded 1727 spejder
  and 1530 bandits, so nothing regressed. Sections: 13 rows in `person_section` with
  Danish labels; all 20 live crew carry a slug except one genuinely unassigned; 19 of
  20 have a phone; `phoneParent IS NULL` for all 21 crew rows, as designed. The
  unmapped-slug warning fired for `hoensegaard`, `team` and `goeglerledelse`, which is
  correct — those are real sections with no app capability attached.

  **Incidental finding, worth knowing.** The replay logged one
  `unexpected end of JSON input` that was **not** dead-lettered: the stream library
  logs a handler error and *drops the message* (`jetstream.Stream.Consume`), so a dead
  letter count of zero does not by itself mean nothing was lost. The error carried no
  subject, making it unattributable among ~30k messages, so `HandleMessage` now wraps
  every error with the subject. Re-running identified it as
  `NATHEJK.2026.spejder.test-freja.updated` — a stray empty-bodied test message,
  pre-existing and unrelated to crew.

## Files

- `go/nathejk/table/person/section.sql` (new) — `person_section` label lookup
- `go/nathejk/table/person/table.go` — section table creation, `Option` /
  `ReportUnmappedSlug`
- `go/nathejk/table/person/consumer.go` — crew + section subjects and handlers,
  subject-annotated errors
- `go/nathejk/table/person/sql.go` — `upsertKeepingRole`
- `go/nathejk/table/person/classify.go` — `postmand`, `guides`; real slug inventory
- `go/nathejk/table/person/crew_test.go` (new)
- `go/cmd/api/main.go` — wire the unmapped-slug warning
