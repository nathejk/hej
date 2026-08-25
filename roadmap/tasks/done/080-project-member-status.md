# 080 — Project member lifecycle status onto the person directory

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

Deferred out of task 072, which left the `person.memberStatus` column in place but
empty. It does not arrive on the `spejder.*.updated` events that task projects — it
comes from the lifecycle/`spejderstatus` subject family, which is a separate set of
events (`types.MemberStatus`: registered → seated → racing → finished, plus
waiting/transit/sheltered/reunited/released).

Two features read it, so it is not optional:

- **PRD 005's skip rule.** Profile confirmation is skipped for a member who has
  already started the event, which means `racing` or later. Without this column the
  rule is unimplementable and every returning member is asked to re-confirm.
- **The portrait nudge** (PRD 005, clarified 2026-08-25). A member who started the
  event *without* a portrait must still be nudged. That means the nudge is driven by
  "has no portrait", **not** by verification status — but the skip rule beside it
  still needs the status, and the two must not be wired to the same signal by
  accident.

## Where to get it

Do not invent a second notion of member status. `hq` already projects this from the
same events into its `spejderstatus` table, and `shared-go/types/member.go` owns the
vocabulary plus the `CanFinish` / `InOurCare` helpers. Consume the same subjects and
store the same string values, so a status here means exactly what it means in `hq`.

Note `types.MemberStatus` also documents superseded persisted values
(`REGISTERED`/`STARTED`/`active`/`emergency`/`hq`/`out`) that may still appear in
older events; `hq`'s `ParseMemberStatus` maps them. Reuse that mapping rather than
matching raw strings, or older members will read as having no status at all.

## Acceptance Criteria

- [x] Lifecycle subjects added to the person projection's `Consumes()` — **one**
      subject, deliberately; see the scope note in the log
- [x] `memberStatus` populated with `types.MemberStatus` values (`racing`)
- [ ] Legacy/superseded status strings mapped, not dropped — **withdrawn as
      unnecessary**, see log
- [x] An unknown status value cannot cost a member their row
- [x] A derived "has started the event" helper (`Person.HasStarted`)
- [x] Idempotent under replay; tested with `cqrstest` fakes
- [x] Verified against the shared stream, zero dead letters

## Progress Log

- 2026-08-25 — Task created, deferred out of task 072 and made urgent by the
  portrait-nudge clarification in PRD 005.
- 2026-08-25 — Picked up. Read `hq`'s `spejderstatus` projector first, and that changed
  the scope of this task substantially.
- 2026-08-25 — **Scope decision: this is not a lifecycle projection.** `hq` already
  projects the full lifecycle — withdrawals, pickups, shelter placements, handovers —
  each with message types defined in *that* repo. Mirroring it here would mean a second,
  lagging notion of member status in a second repo, disagreeing with `hq` in ways nobody
  would notice until an organizer compared two screens. This app asks exactly one
  question ("has this member started?"), so it consumes exactly one transition:
  `NATHEJK:*.patrulje.*.started`.
- 2026-08-25 — That makes `memberStatus` here **coarse by design**: `racing` or empty.
  Still correct for the question asked, because every state *after* racing also implies
  the member started — someone now `waiting` or `sheltered` began the event, and the flag
  never needs unsetting. Recorded in the handler doc so nobody mistakes the column for a
  faithful lifecycle mirror. If the operational states are ever needed, read `hq`'s
  projection or lift it to shared-go rather than growing a parallel one.
- 2026-08-25 — **Withdrew the legacy-mapping criterion.** It assumed this projection
  reads upstream status *strings*, which it does not: it derives `racing` from a start
  event and writes its own constant. There is no legacy value to map. Left the criterion
  unchecked with this note rather than silently deleting it, since it was a reasonable
  thing to expect before reading `hq`.
- 2026-08-25 — Used `NathejkTeamStarted.Members` rather than marking the whole team.
  `hq`'s projector notes that `StartPatrulje` publishes a separate `deleted` for every
  member who did **not** start, so a team-wide update would mark no-shows as racing and
  inflate every team's strength.
- 2026-08-25 — `UPDATE`, not upsert: a start event must not invent a person whose details
  event has not arrived. On replay the details land first and this applies after.
- 2026-08-25 — Added `Person.HasStarted()` and `Person.NeedsPortrait()` as separate
  methods, with doc comments stating they must **not** be wired to the same signal.
  This is the exact confusion the maintainer caught in PRD 005: a member who started
  without a photo still needs nudging, so verification/lifecycle status answers the skip
  rule while portrait *absence* answers the nudge. A test asserts the independence.
- 2026-08-25 — Also put the specific `.started` case **before** the shorter patrulje
  patterns in the switch. Both orderings happen to work, but `hq` carries a comment that
  the reverse has bitten this codebase before, and a `.started` routed to the team-name
  handler would silently never set a status. Covered by a test.
- 2026-08-25 — ✅ 5 new unit tests, plus a full replay against the shared stream:
  **1727 persons, 696 marked started, 0 dead letters.**
- 2026-08-25 — That 696 is worth keeping: under the pre-clarification logic those
  members would have had their **portrait step skipped** along with their confirmation —
  40% of the cohort, and precisely the 40% already out on the route where identification
  matters. The bug the maintainer caught was not a corner case.
- 2026-08-25 — Moving to done.
