# 076 — Handle deletions and phone changes

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent
**Started:** 2026-08-26
**Completed:** 2026-08-26

## Description

PRD 006 §5. Two mutations that are easy to miss and both have security weight:

- **Deletion** (`spejder.deleted`, `senior.deleted`, `crewmember.deleted`): a
  deleted member must lose their login. A projection that only ever inserts leaves
  a working credential behind.
- **Phone change**: the old number must stop resolving, or two numbers log in as
  one person and a reassigned number logs in as the wrong one.

A phone change should also invalidate PRD 005's verification if the *guardian*
number changed — the acknowledged number is stored precisely so that is decidable.

## Acceptance Criteria

- [x] Delete events remove the person from the directory
- [x] A changed phone stops resolving at the old value
- [x] Guardian-number change clears `verifiedAt`
- [x] Tests for delete, phone change and re-add
- [x] Idempotent under replay

## Progress Log

- 2026-08-25 — Task created from PRD 006.
- 2026-08-26 — Done. 0 dead letters; all population counts identical to task 075's run.

  **Two of the criteria were already met**, by tasks 068/070/074, and it is worth
  recording *why* rather than just ticking them:

  - Deletion: `handleMemberDeleted` / `handleCrewMemberDeleted` set `deleted=1`, and the
    filter lives in `querier.Lookup`/`Get` (`deleted = 0`) rather than at the call sites
    — so no future caller can forget it. Verified on real data: 433 spejder delete
    events, 433 rows still deleted; 101 senior, 1 crewmember.
  - Phone change: the upsert writes `phone` unconditionally, so the column is
    *overwritten* rather than accumulated, and the old value stops matching. Both are
    now pinned by tests, because "also keep the previous number" is a plausible-sounding
    change that would silently let two numbers log in as one person.

  **Deletion is not sticky, and that is deliberate.** An `updated` after a `deleted`
  sets `deleted=0`, because upstream re-adding a member is real and they should get
  their login back; stream order is the truth and the last event wins. While confirming
  this I found `table.sql` claimed the *opposite* — that a flag makes ordering harmless
  because "a `deleted` arriving before a late `updated` still leaves the person
  excluded". That was simply false against the code. Comment corrected and both
  orderings pinned by tests.

  **The new work: `invalidateVerification`.** PRD 005 asks a member to confirm a
  *specific* number — "this number can be contacted during Nathejk" — so once that
  number changes the confirmation is about a number nobody agreed to, and the app would
  show staff a green tick for a phone that may not answer. Three design points:

  - **A second statement, not part of the upsert.** The upsert must write `phoneParent`
    unconditionally; this must fire only when the number actually changed. Spejder
    details are re-published on any edit and re-delivered on every replay, so clearing
    unconditionally would destroy a valid verification the first time an organizer fixed
    an address.
  - **The comparison is in SQL**, because a projector cannot read (`cqrs.Writer` takes
    statements, not queries).
  - **`NOT (acknowledgedPhone <=> ?)`** — MariaDB's null-safe inequality. This matters at
    both ends: a guardian number being *removed* must invalidate just as much as one
    being changed, and a plain `<>` against NULL yields NULL and would quietly skip it.
    Verified directly against MariaDB on a temp table, all five cases: unchanged → kept,
    changed → cleared, removed → cleared, never-verified → no-op, both-NULL → kept.

  `acknowledgedPhone` is left in place, so the pair reads as "they did verify, and it was
  for a number that is no longer current" — and changing the number back does not
  resurrect the old consent.

  Nothing writes `verifiedAt` yet (that is PRD 005), so this is a proven no-op on current
  data — which is the point of doing it now: verification cannot ship without its
  invalidation already in place.

  **`Person.IsVerified()`** is the read-side half: `verifiedAt` **and** the acknowledged
  number still matching. Redundant in a healthy row, deliberately. The two guards fail
  differently — the projector's clear depends on the change arriving through
  `handleSpejderUpdated`, while this holds whatever event path moved the number,
  including one nobody has written yet. Cost is a string compare; cost of being wrong is
  telling staff a guardian consented to a number they never saw, during an emergency.

  ### Finding: the projection was silently discarding emergency contact numbers

  `normalizeOrEmpty`'s comment claimed a bad number was "visible (that person cannot log
  in) and fixable upstream". It was not visible at all — nothing logged it. For
  `phoneParent` that is worse than a lost login: staff are told "no number on file" for a
  member whose parents did supply one, and nobody finds out until they need to ring it.

  Added `ReportUnusablePhone`, reusing 074's callback pattern. It fired for **48 distinct
  people** across both years:

  | field | pattern | people |
  |---|---|---|
  | phoneParent | 7 digits — one short | 27 |
  | phoneParent | non-empty, no digits at all | 4 |
  | phoneParent | free text: "Mor: 24281097 eller Far: 22239313" | 3 |
  | phoneParent | 10 digits, not a country code | 2 |
  | phoneParent | 9 digits — one long | 1 |
  | phone | various | 10 |

  The sink receives a **digit count, not the number**. These are third parties' phone
  numbers, usually a child's parent, and a log line is the wrong place for them; the
  person id is enough to find the record upstream. Making that structural beats making it
  a rule someone has to remember.

  One class was recoverable and is fixed: **a `45` country code typed without the `+`**.
  `internal/phone` required exactly 8 bare digits, so `4530756173` was rejected — an
  unambiguous case, since Danish subscriber numbers are exactly 8 digits. Recovered 2
  numbers on replay (1 own, 1 guardian). Worth noting how it hid: the person package's
  *test double* had accepted `45`+8 for some time, so the tests believed a case worked
  that production silently dropped.

  The remaining classes are deliberately **not** repaired. Guessing a digit for a number
  the app may have to ring in an emergency is worse than admitting it is unusable, and
  picking one of "Mor eller Far" is a guess about who to call. Recorded as PRD 006 §11
  Q12, which asks the question this task cannot answer: **who watches that log, and
  when?** These are only fixable by someone contacting the family.

  ### Finding: 36 spejder have a guardian number but no way to log in

  Of 557 live 2026 spejder: 499 have an own phone, 59 do not — and **36 of those 59 do
  have a guardian number on file**. Under phone-only sign-in they cannot log in at all,
  though the data holds a working number for their household. `Lookup` searches `phone`
  only, never `phoneParent`. 23 have no number of any kind.

  Distinct from the bandit gap (§11 Q11): that is lazy data entry, this is young scouts
  who genuinely have no phone. Raised as §11 Q13 rather than decided here — allowing it
  means a parent's handset logs in *as the child*.

## Files

- `go/nathejk/table/person/consumer.go` — `invalidateVerification`, `normalizePhone`,
  guardian number computed once per event
- `go/nathejk/table/person/querier.go` — `Person.IsVerified()`
- `go/nathejk/table/person/table.go` — `ReportUnusablePhone` option
- `go/nathejk/table/person/interfaces.go` — `countDigits`, corrected comment
- `go/nathejk/table/person/table.sql` — corrected the soft-delete comment
- `go/nathejk/table/person/mutation_test.go` (new)
- `go/nathejk/table/person/consumer_test.go` — `upsertStatement` helper
- `go/internal/phone/normalize.go` + test — accept a bare `45` country code
- `go/cmd/api/main.go` — wire the unusable-phone warning
- `roadmap/prd/doing/006-member-directory.md` — §11 Q12/Q13 added; 076 ticked
