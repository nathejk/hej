# PRD 006 — Member directory for the app (person lookup by phone, app roles)

**Status:** doing
**Author:** agent session (Zed)
**Created:** 2026-08-25
**Last updated:** 2026-08-25
**Approved:** 2026-08-25
**Shipped:**
**Target users:** indirectly all app users (spejder, bandit, gøgler, crew); directly the `hej` BFF

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 1. Summary

Replace `hej`'s mocked user directory with a real one: a single person-grained
read model that resolves a **normalized phone number** to a person, their **app
role** (spejder, bandit, gøgler, crew function) and their team, plus the profile
fields the app needs. Every other feature in the app — login, role-based
navigation, the profile page, onboarding verification — is currently sitting on
top of a seeded mock and cannot be truthful until this exists.

## 2. Problem & Motivation

- **What problem does this solve?** `go/internal/users/directory.go` is a mock: a
  seeded phone → role map. PRD 001 shipped it deliberately as a skeleton, with the
  note that "the real Nathejk-records lookup replaces it later without touching
  handlers". That replacement is now the blocker under four separate PRDs:
  - **PRD 003** (profile page) needs name, address, own phone and guardian phone.
  - **PRD 005** (onboarding) needs the guardian phone to confirm, plus a
    per-person verification flag, plus the portrait.
  - **PRD 002** (map) needs patrol identity.
- **PRD 007** (portrait identification) needs the caller's role *and* team to
  decide who may see whose face — including the rule that spejdere and banditter
  are mutually invisible. That authorization is only as correct as this directory's
  role classification, which makes §11's "are these the right app roles?" question
  load-bearing rather than cosmetic.

- **PRD 009** (offline-first client data layer) caches the part of this directory
  the user may see, so name/team lookups work with no signal. Note the constraint it
  raises: caching the *whole* directory would put every participant's address and
  guardian number on every device, which is a far larger disclosure than portraits.
  Only the permitted subset, and only the fields the UI needs, should ever leave the
  server.

  Each has been written as "extend the directory"; none can proceed until the
  directory is real.
- **Why now?** Three PRDs are queued behind it, and every one of them would
  otherwise ship against seeded data that looks fine in dev and is empty in
  production.
- **Evidence.** A survey of `shared-go`, `hq` and `tilmelding` (2026-08-25)
  established that the lookup `hej` needs **does not exist anywhere** and cannot
  be composed from what does. Details in §8; the four findings that decide the
  design are:
  1. **No phone lookup exists at all.** No `ByPhone`-style query in any of the
     three repos, no `Filter` accepts a phone, and no `phone` column is indexed.
  2. **Gøgler people are not in shared-go.** They live in `hq`'s local
     `personnel` table, which `hej` cannot import (another module's `internal`-ish
     domain tree).
  3. **Crew function is unenforced free text.** `crewmember.sectionSlug` →
     `section.label`, organizer-authored. `section.Type` is carried on the wire
     (`messages/crew.go`) but dropped by the projector.
  4. **The obvious fan-out read is not viable.** `spejder.GetByID` is a stub
     returning `nil, nil`; `senior.GetByID`'s SQL is broken (`s,phone` instead of
     `s.phone`); `spejder.GetAll` joins `spejderstatus`, which shared-go does not
     own; `senior.GetAll` requires a `klan` join. Four unindexed table scans per
     login attempt, two of them through non-functional code paths.

## 3. Goals

- One lookup, `phone → person`, fast and indexed, usable on the login hot path.
- A single place that owns the **app-role classification**, so no handler and no
  frontend ever infers a role from a team type or a section slug.
- Cover all four participant populations: spejder, bandit, gøgler, crew.
- Keep `hej`'s handlers unchanged: `users.Directory` stays the seam, the mock
  stays as the test double.
- Provide the fields PRDs 002, 003 and 005 need, without each one growing its own
  lookup path.

## 4. Non-Goals

- **Becoming the system of record.** This is a projection. Corrections happen in
  `tilmelding`, not here.
- **Writing member data.** Read-only, with one exception in scope elsewhere: PRD
  005's verification flag, whose write path this PRD must accommodate but does not
  design.
- **Portrait storage.** The portrait (PRD 005 / PRD 003) is app-owned data with
  its own consent and retention story, and PRD 007 owns viewing it. This PRD
  carries the *reference*, not the bytes or the policy.
- **Unifying the role taxonomies across repos.** See §11 — this PRD defines
  `hej`'s app roles and owns the mapping; it does not refactor `hq`'s
  `personnel`, `TeamType` or `UserType`.
- **Historical years.** Current event year only, though the projection is keyed by
  year so history is not destroyed.
- **Replacing `hq`'s `personnel` table.** Tempting (see §11) but a separate,
  larger decision.

## 5. User Stories & Scenarios

Mostly a technical enabler, so the stories are about what it unblocks:

- As a **spejder**, I want to log in with my own phone number and see my patrol's
  pages, so that the app knows who I am without me typing anything else.
- As a **crew member on the samarit team**, I want the app to show me the SOS
  pages, so that role-gated navigation reflects my actual function.
- As a **bandit**, I want to log in even though I am registered as a senior in a
  klan, so that my event role and my signup category do not have to match.
- As a **developer**, I want one interface returning a fully classified person, so
  that no handler has to know that bandits are seniors or that crew function is a
  section slug.

### Primary path

1. A user submits a phone number at login.
2. `hej` normalizes it (`internal/phone`) and calls `Directory.Lookup(phone)`.
3. One indexed read returns the person, their app role, their team, and the
   profile fields.
4. The session is established; role-gated routes and nav follow from the role.
5. An established session resolves via `Directory.Get(id)` on every request.

### Edge cases

- **Number not recognized.** `found=false`. Handlers must already treat this
  identically to a recognized number (anti-enumeration, per the existing
  `Directory` contract) — the PIN simply never verifies.
- **Same number on two people.** Realistic: siblings sharing a phone, or a parent
  number entered as the scout's own. The unique key makes this a **collision that
  must be resolved deliberately**, not silently. See §11 — this is the biggest
  open design question.
- **A person changes phone number.** The projection updates; any session keyed on
  person id keeps working.
- **Crew member with no section assigned** (`Unassigned` is a real filter in
  `crewmember`). They are a legitimate user with no specific function — classify as
  the generic `crew` role, not as an error. They get the pages every signed-in role
  sees, and **not** the elevated cross-population access PRD 007 gives to identified
  crew functions.
- **Section slug that maps to no known function.** Organizer-authored slugs will
  drift. Fall back to generic crew and **log it loudly**; never fail the login.
- **Person deleted** (`crewmember.deleted`, `senior.deleted`, `spejder.deleted`).
  Must remove them from the directory, or a deleted member keeps a working login.
- **Two events / year rollover.** Keyed by year; the app reads the current year.

## 6. Requirements

### Functional

- [ ] A person-grained read model with one row per person per year, carrying at
      minimum: person id, year, app role, name, normalized phone, team id, team
      name.
- [ ] `UNIQUE` on (year, normalized phone) — or an explicit, documented collision
      policy if uniqueness cannot hold (§11).
- [ ] `GetByPhone(year, normalizedPhone)` and `GetByID(id)`, both index-backed.
- [ ] **Extend the app-role enum.** `gøgler` and a generic `crew` value do not
      exist in `users.Role` (Go) or `session.store.ts` (TS) today, yet this PRD
      covers both populations — a gøgler login is currently untypeable. Add them in
      both places, and state what navigation each gets in
      `vue/src/config/navigation.ts`.
- [ ] Cover **spejder** (from `spejder`), **bandit** (from `senior` + `klan`),
      **crew** (from `crewmember` + `section`) and **gøgler**.
- [ ] Classify crew into `postmandskab` / `guide` / `samarit` where determinable,
      and a named generic `crew` role otherwise. **The generic role must be treated
      as least-privileged, not unrestricted** — it is a fallback for unclassifiable
      data, so it must not inherit the full crew access PRD 007 grants to
      samarit/guide/postmandskab. (Corrected 2026-08-25: this previously said "show
      them the unrestricted pages", which combined with PRD 007's matrix would have
      given an unclassified account every portrait in the event.)
- [ ] Carry the member's **lifecycle status** (`types.MemberStatus`), which PRD
      005's "has started the event" skip rule reads (`racing` onwards). Without it
      that rule is unimplementable. **Task 080** owns this: the status arrives on the
      lifecycle/`spejderstatus` subjects, not on the member-detail events task 072
      projects, so the column exists but stays empty until then.
- [ ] Carry `phoneParent` for spejder. **Only spejder have a guardian number** —
      the model must make that explicit rather than leaving an empty string that
      reads as "missing data" for everyone else.
- [ ] Carry the fields PRD 003 needs: name, address, postal code, city, email,
      own phone, guardian phone, birthday.
- [ ] Carry patrol/team identity for PRD 002.
- [ ] Accommodate PRD 005's `verified_at` + acknowledged number, and a portrait
      reference.
- [ ] Own **phone normalization** consistently, so a number stored by the
      projector and a number typed at login normalize identically.
- [ ] `hej`'s existing `users.Directory` interface is preserved; the mock remains
      for tests.
- [ ] Deletions and role changes are reflected, not just insertions.

### Non-Functional

- **Login latency.** A single indexed lookup. No fan-out across four tables.
- **Correctness over completeness.** An unclassifiable person must still be able
  to log in, with a **least-privileged** default role — never locked out, and never
  silently over-privileged.
- **Observability.** Unmapped section slugs, phone collisions and
  classification fallbacks must be visible in logs — these are data problems that
  will only surface in production data.
- **Privacy.** The directory holds minors' names, addresses and guardian phone
  numbers. It must not be exposed through any unauthenticated endpoint, and the
  app must only ever return the *caller's own* row (cross-person reads belong to a
  separate, access-controlled feature).

## 7. UX / UI Notes

N/A — no user-facing surface of its own. It is what makes PRD 002's patrol
identity, PRD 003's profile page and PRD 005's confirmation step show real data
instead of seeds. The only user-visible effect is that role-gated navigation
becomes correct for crew.

## 8. Technical Considerations

### Where it lives

Per the user's direction: **start the projection in `hej`, lift it to shared-go
once the design settles.** That is the right sequencing — the classification rules
(especially crew function) are the uncertain part, and iterating on them inside one
repo is much cheaper than the commit → push → version-bump loop across shared-go
consumers described in `go-bff-layout`.

Two constraints on that plan:

- Put it under `go/nathejk/table/person/` from the start, not `internal/`. The
  `go-bff-layout` skill is explicit that `internal/` is not importable from
  another module and that such an import "blocks the move". A projection destined
  for shared-go must not be born in `internal/`.
- Take `(cqrs.Publisher, cqrs.Writer, cqrs.Reader)` in the constructor and
  declare any application dependency in an `interfaces.go`. Never a `*sql.DB` or a
  concrete stream. This is what makes the later lift mechanical.

### Infrastructure: see PRD 008

`hej` is a stateless service today — `data.Models` is a facade over mocks and
`commands.Commands` is an empty struct — so this projection cannot exist until the
binary has a database, a broker and the cqrs seam. That work is **PRD 008**
(persistence and event-stream infrastructure), which this PRD depends on and which
must be sequenced first.

It was originally described here, which was the wrong home for it: three other
PRDs (003, 005, 007) need the same persistence, two of them for app-owned writes
unrelated to the directory. One correction to what this PRD first claimed:
**dev already provisions MariaDB 10.8** as the `db` service (plus phpMyAdmin at
`sql.hej.local.nathejk.dk`); what is missing in dev is a DSN, a driver and a NATS
service. Production, by contrast, has neither a database nor a broker. See PRD 008
§2 for the verified inventory.

What this PRD relies on 008 for: the cqrs triple, an `xstream.Mux`, shared-go in
`go.mod` with a proven `GOWORK=off` build, and the constructor → projections →
`data.NewModels(...)` registration pattern.

### Subjects to consume

All already published (verified in `shared-go/tables/*/consumer.go` and
`hq/.../personnel/consumer.go`):

| App role | Subjects |
|---|---|
| spejder | `NATHEJK.*.spejder.*.updated`, `…deleted`, `NATHEJK:*.patrulje.*.signedup`, `…updated`, `…started` |
| bandit | `NATHEJK.*.senior.*.updated`, `…deleted`, `NATHEJK.*.bandit.*.armNumber.assigned`, `NATHEJK:*.klan.*.signedup`, `…updated` |
| crew | `NATHEJK.*.crewmember.*.registered`, `…updated`, `…deleted`, `NATHEJK.*.crewmember.*.section.assigned`, `NATHEJK.*.crew.*.signedup`, plus `NATHEJK.*.section.*.added`/`moved`/`deleted` for function labels |
| gøgler | `NATHEJK.*.gøgler.*.signedup`, `…updated`, `…status.changed` |

Consuming events directly — rather than reading the existing projections — also
sidesteps blockers 2 and 4 in §2: `hej` never needs `hq`'s `personnel` table, and
never depends on the broken `GetByID` implementations.

### Role classification, and why it is the risky part

The three taxonomies in play do not line up, and nothing in the code reconciles
them:

| Layer | Values |
|---|---|
| `hej` app role (today) | `spejder`, `bandit`, `postmandskab`, `guide`, `samarit` — **no `gøgler`, no generic `crew`**; this PRD adds both |
| shared-go `TeamType` | `patrulje`, `klan`, `crew`, `gøgler` |
| shared-go `UserType` | `gøgler`, `crew` |

- **spejder** ← a `spejder` row (team is a `patrulje`).
- **bandit** ← a `senior` row (team is a `klan`). Not a field: the event
  vocabulary gives it away, since `NATHEJK.*.bandit.*.armNumber.assigned` is
  projected onto `senior.armNumber`. Note `hq` *also* keeps bandits in its local
  `personnel` table, so bandit identity is currently split across two
  projections — do not add a third notion of it.
- **gøgler** ← `NATHEJK.*.gøgler.*` events. No shared-go table exists.
- **crew** ← a `crewmember` row; the *function* comes from `sectionSlug`, which is
  organizer-authored free text. `tilmelding`'s crew signup UI treats first-level
  `section` rows as "crew functions", which is the only place that intent is
  written down. Mapping those slugs to `postmandskab`/`guide`/`samarit` is
  therefore **string convention, validated by nothing**.

Two candidate fixes for the crew problem, both worth pricing before building:

1. **Project `section.Type`.** The `NathejkSectionAdded` message already carries
   `Type` and `SelfAssignable` (`messages/crew.go`) and the projector drops both.
   Adding the column makes crew function a modelled fact rather than a slug
   match. This is the better answer if `Type` means what the app needs.
2. **A slug → role map in `hej`**, with a logged fallback to generic crew.
   Cheaper, honest about being a convention, and survives slug drift without
   locking anyone out.

Recommendation: (2) to ship, (1) as a follow-up in shared-go — but note the
`hej` app roles are also not obviously the right long-term taxonomy (§11).

### API endpoints

**None of its own.** This PRD changes what existing handlers read. Any endpoint
introduced by PRDs 003/005/007 carries its own OpenAPI annotations, per `.rules`.
Note that `hej`'s api is **strictly a BFF**: it serves this app's frontend and does
not expose the directory to other services, which get their data from the event
stream instead.

### Data / storage

- New table (working name `person`): `(year, personId)` primary key,
  `UNIQUE KEY (year, phone)`, plus indexes for team-scoped reads. Columns for the
  fields in §6, `verifiedAt`, and a portrait reference.
- Schema drift via `cqrs.EnsureColumn` / `cqrs.EnsureIndex` from the constructor,
  per `go-bff-layout`.
- Rebuildable from the event log by design — that is what makes starting it in
  `hej` and lifting it later safe.

### Dependencies & risks

- **Risk: this PRD is a prerequisite for four others** (002, 003, 005, 007) and
  supplies PRD 009's directory dataset. It is larger than any of them. Do not let the
  app PRDs quietly start against the mock and diverge.
- **Risk: PRD 007's access matrix is only as correct as this classification.** A
  crew member misclassified upward would see portraits across the spejder/bandit
  divide, which 007 treats as a correctness requirement for the game rather than a
  privacy nicety. That makes §11's "are these the right app roles?" question
  load-bearing, not cosmetic.
- **Blocked by PRD 008**, with one deliberate exception so the two do not
  deadlock: **008 is blocked by nothing**; this PRD's *schema slice and first
  projection* serve as 008's acceptance test; the **rest** of this PRD is blocked by
  008. (Stated explicitly 2026-08-25 — previously both documents said the other
  should come first, which reads as a cycle.) The infrastructure lift is larger than
  this projection and is where the schedule will actually go.
- **Risk: gøgler coverage may need write-side work elsewhere.** They are only in
  `hq`'s local table today; projecting them fresh in `hej` is possible but means
  two projections of the same population, in two repos, that can disagree.
- **Risk: unmapped or renamed section slugs** silently misclassify crew. Mitigated
  by the logged fallback, not eliminated.
- **Risk: phone collisions** (§11) are a data reality, not an edge case.
- **Risk: the lift to shared-go never happens** and `hej` keeps a domain
  projection that three other repos would benefit from. Give the lift an owner and
  a trigger condition when this PRD is approved, not later.

## 9. Success Metrics

- Every registered participant across all four populations can log in with their
  registered number; zero "unrecognized number" support reports traceable to the
  directory.
- Login lookup is a single indexed query (verified by inspection, not timing).
- Crew members see the pages matching their function, with zero unmapped-slug
  fallbacks in the final pre-event data.
- PRDs 002, 003 and 005 ship against real data, with no seeded fallbacks left in
  production code.

## 10. Rollout / Task Breakdown

Sequence PRD 008's infrastructure first — nothing here can be tested without it —
then the projection, then classification, then the swap. Keep the mock swappable
throughout so `hej` stays runnable at every step.

Proposed tasks for `roadmap/tasks/open/` (created 2026-08-25 as tasks **067–078**):

- [x] 067 — extend the app-role enum with `gøgler` + generic `crew`
- [x] 068 — `nathejk/table/person` skeleton: schema, constructor, `EnsureColumn`
- [x] 069 — app-role classification + slug→function map with logged fallback
- [x] 070 — `GetByPhone` / `GetByID` queriers + phone-normalization consistency
- [x] 071 — phone-collision policy (§11 Q1)
- [x] 072 — project spejder (+ patrulje team names)
- [x] 073 — project senior/klan as bandit (incl. armNumber subject)
- [x] 074 — project crewmember + section
- [x] 075 — project gøgler
- [x] 076 — handle deletions and phone changes
- [x] 077 — swap `users.Directory` to the projection; keep the mock as test double
- [x] 078 — backfill/replay verification against real event data

Task 068's schema slice doubles as PRD 008's acceptance test (see PRD 008 §8), which
is why the two were approved together with 008 first.

## 11. Open Questions

1. ~~**Phone collisions.**~~ *Answered 2026-08-25 (task 071): **disambiguate after
   PIN verification.*** The PIN proves control of the *number*, not which of its
   owners is holding it, so the flow verifies first and then asks "which of you is
   this?". Refusing the login was rejected as locking out real participants;
   last-write-wins was rejected outright, since it would show one sibling the
   other's data.

   Implemented in the directory contract rather than as a policy buried in a
   handler: `LookupAll` returns every owner, and `Lookup` returns **not found** when
   a number is shared. That asymmetry is the safety property — a caller who has not
   thought about collisions gets a refused login (visible, fixable) instead of
   silently authenticating someone as their sibling.

   **Still to build:** the chooser itself (a short-lived post-verification token, a
   candidate list, and the UI) is **task 079** and belongs to the auth/PRD 005
   surface. Until it ships, a shared number cannot log in at all — which is why 079
   must land before, or with, task 077's swap to the real projection. The schema
   keeps `year_phone` non-unique so the projector never fails on this data (task
   068).

   How common it actually is is now **measured, and it is not rare**: a full replay of
   the real event stream (2026-08-25, task 072) produced **213 phone numbers shared by
   more than one person**, against ~1,610 members with a number at all — roughly one in
   eight. That settles two things. The "refuse the login" option would have locked out
   hundreds of members, so rejecting it was correct; and **task 079 is not optional
   polish** — without the chooser, that whole cohort cannot log in.

   **Task 078 then found the premise was wrong.** The collisions are overwhelmingly *not*
   siblings. Restricted to the 2026 event and its 827 live participants:

   | shared numbers where… | count | meaning |
   |---|---|---|
   | every row has the **same name** | **70** | duplicate registrations |
   | names differ | 15 | genuine sharing |

   316 rows sit on a shared number, and the largest cluster is **9 rows for one person**:
   "Cæcilie Bæk Lahoz", same patrulje, nine distinct `personId`s. Verified upstream —
   three of the nine were read back off the stream as three separate
   `spejder.*.updated` events with three separate member ids (and two different
   `teamId`s), so the projection is not inventing them.

   This does not invalidate the chooser, which is still required for the 15 genuine cases.
   It does mean the chooser **cannot do the job alone**: 82% of the time it will offer
   several identical names, which is not a question a user can answer. See Q9, which now
   covers spejder as well as gøglere.
2. ~~**Is `section.Type` the right source for crew function?**~~ *Answered 2026-08-26
   (task 078): **no — it does not exist in the data.*** Every section event on the broker
   was read back: **0 of 14 carry a `type`**, and 0 carry `selfAssignable`. They contain
   `slug` and `label` and nothing else. The field is declared on `NathejkSectionAdded`
   and never populated by the producer, so "project `section.Type` instead" was never an
   available option.

   That changes the slug map's status from stopgap to mechanism, and the work from
   "replace it" to "keep its coverage honest". `classify.go` now lists the **whole** 2026
   tree, split into sections that grant a capability and sections that are real but grant
   nothing (kitchen, HQ, PR, …) mapped explicitly to `crew`. Absence from the map now
   means exactly one thing — "nobody has classified this section yet" — which is what
   makes the unmapped-slug warning worth logging. Before, it fired on every replay for
   three well-known sections and was pure noise. **Replay warnings are now 0.**
3. **Are `postmandskab`/`guide`/`samarit` the right app roles at all?** They were
   invented in PRD 001 for the skeleton nav. The real crew organisation is a
   section tree, which is richer and organizer-editable. Continuing to hardcode
   roles in a frontend enum may be the wrong shape — and this is now **blocking for
   PRD 007**, whose access matrix must enumerate every role that can log in
   (including `gøgler` and generic `crew`). Settle it before more features gate on
   it.

   **Task 078 sharpens this considerably: three of the seven app roles have no members
   at all.** A from-scratch replay of the real stream produces exactly four roles —
   `spejder` (557), `bandit` (151), `gøgler` (99) and generic `crew` (20). Not one person
   is classified `postmandskab`, `guide` or `samarit`, because every 2026 crew assignment
   points at `hoensegaard` (8), `team` (6), `goeglerledelse` (5) or nothing (1) — none of
   the three capability sections has anyone in it.

   So PRD 007's access matrix is currently being designed around three roles that exist
   only in the mock directory, and the `samarit` SOS page has no one who can reach it.
   Either the capability sections fill up closer to the event (in which case the map is
   ready and this resolves itself), or crew function is not modelled the way PRD 001
   assumed and the three roles should collapse. **Worth answering before PRD 007 builds on
   them.**
4. ~~**Gøgler: project fresh in `hej`, or promote `hq`'s `personnel` slice into
   shared-go?**~~ *Answered 2026-08-25 (task 075): **project fresh here, and record the
   duplication.*** Reading hq's projection was never available — that would be one
   service calling another's API, which the architecture forbids — and promoting hq's
   slice would have blocked this app's login on another repo's release. So `hej` now
   holds the **second** projection of this population, derived from the same events by
   different code, and the two can silently disagree. Promotion to shared-go remains the
   intended end state; the trade is documented at the top of
   `go/nathejk/table/person/goegler.go` so it cannot be mistaken for an oversight.

   Two things the implementation had to discover, because gøglere have **no message
   types in shared-go at all** — the wire shapes were read off the live stream and are
   an observation, not an agreement:

   - `gøgler.*.signedup` is **not** redundant with `.updated`, unlike the crew pair.
     31 of 99 gøglere in 2026 (26 of 125 in 2025) have a signup and no update at all.
     The two families looked similar enough that reusing the crew reasoning would have
     silently dropped a third of the population from the directory, and therefore from
     logging in.
   - `.status.changed` and `.deleted`, both named in §8, **do not exist** on the stream.
     Left unimplemented rather than written blind. **A gøgler therefore has no deletion
     path** — see Q9.
5. **When does this lift to shared-go**, and who owns it? What is the trigger —
   the first other consumer, or a fixed date?
6. ~~**Does `hej` need the full member set or only the current year's
   participants?**~~ *Answered 2026-08-26 (task 078): **project everything, read one
   year.*** The projection holds both years (3,278 rows: 2,451 for 2025, 827 live for
   2026) and the directory reads only `EVENT_YEAR`, so last year's participants are
   inert — they cannot log in. Filtering at projection time was rejected as a
   false economy: the whole table is a few thousand rows, the projector would need to
   learn the current year (a second place for the year to be wrong), and a replay after
   the year rolls over would discard data that is cheap to keep and awkward to recover.

   It also decides "recognized number" for someone who attended last year but not this
   one: **not recognized**, because the lookup is year-scoped. They get the same response
   as a stranger, which is the anti-enumeration behaviour we want anyway.

   One consequence to keep in view: the two years disagree sharply on data quality (see
   Q11), so any statistic computed over the whole table rather than one year is
   misleading. That mistake was made and corrected during this task.
7. ~~**Which year does the app read?**~~ *Answered 2026-08-26 (task 077): **configurable,
   defaulting to the current year.*** `EVENT_YEAR` in `cmd/api/env.go`, pinned to `2026`
   in the dev compose file. Not derived from `time.Now()` the way `hq` does it, because
   the clock is wrong for this in two ordinary situations: a test event held outside its
   nominal year, and the days around new year, when the app would stop recognising every
   participant of an event that has not happened yet. The year is fixed once at directory
   construction rather than read per call, so every lookup within a request agrees about
   which event is running.
8. **Portrait reference:** does the directory carry it, or does the app own a
   separate table keyed by person id? The latter keeps app-owned data out of a
   projection that will be rebuilt from events — and a rebuild must not drop
   portraits.
9. **Duplicate registrations — the chooser cannot solve this one.** Found in task 075 among
   gøglere and confirmed in task 078 to be **far larger and mostly a spejder problem**.

   Across the 2026 event, **70 of 85 shared phone numbers carry the same name on every
   row**: one human who submitted the signup form repeatedly, each time getting a fresh
   id and so becoming a separate person. By population, rows sitting on a shared number:
   spejder 274, gøgler 30, bandit 9, crew 3. The worst single case is **9 rows for one
   spejder** — "Cæcilie Bæk Lahoz", same patrulje, nine ids; among gøglere, "Rikke Banke
   Peytz" is 5 and "Klaus" is 4.

   **Not a projection bug.** Three of Cæcilie's nine ids were read back off the stream as
   three separate `spejder.*.updated` events with three separate member ids — and two
   different `teamId`s, so the patrulje record appears to be duplicated too.

   Consequence for login: the task 079 chooser will offer several *identical* names, which
   is not a question a user can answer, and picking wrong yields an account with the wrong
   team, no portrait and no history. Deliberately **not** patched in the projection:
   de-duplicating means choosing a winner, which is a product decision, and would hide the
   upstream problem.

   **One caveat before acting on the scale of this:** the dev broker's dataset is
   constructed to look real rather than copied from production. These duplicates are
   genuinely in the events, but whether they reflect real signup behaviour or an artifact
   of how the dataset was generated is something only the data's author can say. If it is
   real, it needs fixing upstream (or a merge step) before an event; if it is generator
   noise, the chooser's real-world job is the 15 genuine cases and this is a false alarm.
10. **Gøglere cannot be deleted.** No `gøgler.*.deleted` exists on the stream, so a
    gøgler who withdraws keeps a working login indefinitely. Every other population has
    a deletion path. This is security-relevant and feeds directly into **task 076**;
    resolving it may require a new event upstream rather than anything in `hej`.
11. ~~**Should sign-in stay phone-only** given how many bandits lack a number?~~
    *Answered 2026-08-26: **yes, phone-only.*** The gap is accepted as a **deliberate
    forcing function**. Bandits reserve seats before they know who will actually attend,
    and are "quite lazy when it comes to entering data"; making the app inaccessible
    without a number is what will get the numbers supplied. No klan-level fallback
    contact, and no alternative sign-in method, is to be added.

    **Correction (task 078): the gap I reported was overstated, and the error was mine.**
    The figure given when this question was raised — "only 239 of 1433 live 2026 bandits
    (17%) have a phone" — counted **both years**, and 2025 dominates it. Per year:

    | year | role | live | with phone | |
    |---|---|---|---|---|
    | 2025 | bandit | 1282 | 122 | 10% |
    | **2026** | **bandit** | **151** | **117** | **77%** |
    | 2025 | spejder | 737 | 691 | 94% |
    | 2026 | spejder | 557 | 499 | 90% |

    For the event that is actually coming, bandit coverage is **77%, not 17%** — 34 people
    short of a number, not ~1,200. The bandit phone problem is a 2025 artifact. Verified
    not to be a projection bug either way: the numbers that exist are stored correctly
    normalized, and the 2025 source `senior.updated` events genuinely carry
    `"phone": ""`.

    The decision stands and is now better supported than when it was made. What changes is
    the planning consequence: the earlier note that "bandit onboarding must assume a first
    login very close to the event" was written for a cohort of 1,200 stragglers. At 34 it
    is an ordinary trickle, and PRDs 005/007 do not need to be shaped around it — though
    they still cannot assume a bandit has had weeks to verify a profile or take a
    portrait.
12. ~~**38 people have an emergency contact number the app cannot use.**~~ *Answered
    2026-08-26: **the check-in counter is the backstop — nobody starts without full
    details.*** Found in task 076 by making the projection's silent phone-number drops
    visible. Distinct people across both years, all previously indistinguishable from "no
    number on file":

    | pattern | people | recoverable? |
    |---|---|---|
    | 7 digits — one short | 27 | no |
    | free text naming two numbers ("Mor: ... eller Far: ...") | 3 | no, ambiguous |
    | 10 digits, not a country code | 2 | no |
    | 9 digits — one long | 1 | no |
    | non-empty but no digits at all | 4 | no |

    Plus 10 people with an unusable *own* number, who therefore cannot log in.

    One class **was** recoverable and is now fixed: a `45` country code typed without the
    `+`. `internal/phone` rejected those, silently. Notably the projection's test double
    had accepted them for some time, so the tests believed a case worked that production
    dropped — the fake was ahead of the real implementation.

    The rest are deliberately **not** repaired. Guessing a digit for a number the app may
    have to ring in an emergency is worse than admitting it is unusable, and choosing one
    of "Mor eller Far" is a guess about who to call. They are now logged with the person
    id (and a digit count — never the number itself, since these are third parties'
    phones), so they can be corrected upstream before an event.

    **The answer to "who fixes these" is the check-in counter**, which already refuses to
    start anyone whose details are incomplete. So a bad number is not a silent hole in the
    emergency-contact chain after all — it is caught in person, by the same procedure that
    catches everything else. The log is therefore an *early-warning* convenience, not a
    safety-critical queue: useful for shortening queues at the counter, not something an
    event depends on. That also reframes PRD 005's value: the members whose data is
    already clean self-verify and walk through, leaving the counter free for the ~48 who
    genuinely need a human.
13. ~~**Can a spejder log in with their guardian's number?**~~ *Answered 2026-08-26:
    **no — nobody logs in with a guardian's number.*** Not the guardian, not the member it
    belongs to. A participant is not required to bring a phone; without one they have no
    number and no app, and that is an accepted outcome rather than a gap to engineer
    around.

    The measurements that prompted the question: of 557 live 2026 spejder, 59 have no own
    phone and **36 of those do have a guardian number on file**; 23 have no number of any
    kind. `Lookup` searches `phone` only, never `phoneParent`.

    This is now an enforced boundary rather than a note, because the pressure to break it
    is specific and recurring: `OR phoneParent = ?` looks like a one-line rescue of 36
    locked-out children, and is really a way for a parent's handset to authenticate **as
    the child** with no audit trail — and where siblings share the number, as an arbitrary
    one of them. `TestLoginNeverMatchesOnTheGuardianNumber` inspects the WHERE clause of
    every lookup and fails if the guardian column appears; it was verified to fail when
    the fallback is added.
