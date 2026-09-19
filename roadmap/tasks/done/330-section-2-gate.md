# 330 — Section 2 gate: when a patrol's public page opens

**Status:** done
**Priority:** high
**Created:** 2026-09-19
**Picked up by:** agent session (Zed)
**Started:** 2026-09-19
**Completed:** 2026-09-19

## Description

PRD 011 §0b.3. Nothing else in Phase 2 may be reachable before this exists, so it is the first task
and it is the one with the sharpest failure mode: a gate that opens early publishes a patrol's
positions while that patrol is still racing.

**Two triggers, OR'd, whichever fires first:**

| | trigger | scope | meaning |
|---|---|---|---|
| 1 | the patrol has a scan attributed to a checkpoint in the **last checkgroup** | that patrol | *this patrol has finished* |
| 2 | **the last checkpoint has closed** — greatest `checkpoint.openUntilUts` over non-deleted rows | everything | *the race is over* |

Trigger 2 is a **backstop, not a replacement**: trigger 1 still opens a finishing patrol's page while
the race runs, and trigger 2 catches everyone trigger 1 missed — non-finishers, and patrols whose
finish scan could not be attributed.

**The inputs all exist.** "The last checkgroup" is the non-deleted `checkgroup` with the highest
`sortOrder` (`go/nathejk/table/checkgroup/table.sql`). Attribution runs scan → scanner →
`checkpersonnel` shift → checkpoint → checkgroup, which PRD 016 already built. The closing instant is
`checkpoint.openUntilUts` (`go/nathejk/table/checkpoint/table.sql`).

**Why this is safe, which is not obvious from the code and must be commented at the implementation:**

- The **last checkgroup is the finish line**, so trigger 1 means "has finished". A finished patrol's
  track describes a route it will not walk again.
- It discloses nothing useful to patrols still out: they already know where the finish is, they have
  just not reached it. So PRD 016's possession-grounded reveal model is *satisfied*, not bypassed.
- There is **one route sequence for every patrol** — nothing upstream carries a per-team route
  (PRD 016 §11.9).
- The **last checkpoint always carries absolute opening hours** (maintainer, 2026-09-19), so trigger 2
  always has an instant to fire on.

All four are facts about the *event*, not about the schema. A future event with per-patrol routes, a
post after the finish, or a `relative`-scheme final post invalidates this gate **without changing a
line of this code** — hence the comment requirement below. It is the whole reason this task is not
just a query.

**Do not** put the check in a template. Three routes serve section 2 (the HTML page plus two JSON
endpoints, PRD 011 §8), and a gate applied at render time leaves the JSON open — which is the entire
course in machine-readable form. One shared function, called first in all three.

## Acceptance Criteria

- [x] One shared gate function, called first in every section-2 handler (HTML and JSON alike); no
      handler reaches patrol data without passing it.
- [x] Trigger 1: a scan attributed to a checkpoint in the highest-`sortOrder` non-deleted checkgroup
      opens that patrol's page, and only that patrol's.
- [x] Trigger 2: once the greatest non-deleted `checkpoint.openUntilUts` is in the past, every
      patrol's page is open.
- [x] `openUntilUts` of `0` means "not set", **not** 1970: an absent instant leaves the backstop
      unfired rather than opening every page at the epoch. Asserted by a test, because this is the
      failure that publishes everything at once.
- [x] Fails closed on unreadable input — neither trigger satisfied, or the checkgroup/checkpoint rows
      cannot be read, and the page stays shut.
- [x] **All-or-nothing**: no element has its own reveal rule. No "pins later", no "track later".
- [x] A manual override can open one patrol's page early (for a finished patrol whose scan went
      unattributed). A convenience, not a safety net — the backstop already guarantees a page.
- [x] Before either trigger, the URL answers as **not-yet** (see task 341 for the page itself), and
      the not-yet answer carries **no patrol data at all** — indistinguishable from the answer for a
      patrol number that does not exist, so it cannot be used to enumerate patrols or to read
      finish order live.
- [x] The closing instant is computed once for the event and cached, not joined per request: these are
      unauthenticated routes.
- [x] A comment at the gate records the four event-facts above and that changing any of them
      invalidates it.
- [x] Tests cover: closed, open-by-trigger-1, open-by-trigger-2, override, absent `openUntilUts`, and
      unreadable input.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §0b.3 / §6 / §10 (Phase 0).
- 2026-09-19 — Picked up. Plan: the rule as a package under `internal/`, following `internal/reveal`'s
  shape (narrow local interfaces over the table packages, so the rule is testable without a database),
  plus the HTTP-side decision in `cmd/api`.
- 2026-09-19 — Survey finding that made trigger 1 much cheaper than expected: **`scan.Scan` already
  carries `CheckgroupID`**, populated by the projection from the scanner's `checkpersonnel` shift. So
  no scan → checkpoint → checkgroup join was needed here; the attribution PRD 016 built is already on
  the row. Trigger 1 is a comparison, not a query.
- 2026-09-19 — Trigger 2 needed a new read. Added `checkpoint.Queries.LastCloses(year)`, `MAX(openUntilUts)`
  over non-deleted rows with `openUntilUts > 0`. Noted in the interface doc why this does **not** widen
  the projection's security boundary: every other read there is bounded by ids the caller named, because
  they return positions; this returns one instant, and there is nothing in a timestamp to learn about
  where a post is.
- 2026-09-19 — `Reason` is a four-valued type (`Closed`/`Finished`/`RaceOver`/`Override`) rather than a
  boolean. Decided because a finish-opened page carries a diploma and a backstop-opened page does not
  (task 346) — with a boolean, that distinction would have to be recovered later from a nullable finish
  time, which is the kind of inference that goes wrong once and then looks intentional.
- 2026-09-19 — ✅ **A test caught a real hole.** `TestZeroClosingInstantReportedOkStillDoesNotOpenEverything`
  failed on the first run: the gate trusted `Closing`'s `ok` and compared against `time.Unix(0,0)`, so a
  dependency reporting `ok` with a zero instant opened **every patrol in the event at once**. The querier
  guarded it, but `Closing` is an interface — a guard living in one implementation of a dependency is not
  a guard on the rule. Now checked in both places, with a comment saying why the duplication is
  deliberate.
- 2026-09-19 — Override implemented as `PUBLIC_PAGE_PATROL_OVERRIDE`, a comma-separated list of patrol
  **ids**. Two deliberate properties: it can only ever *open* a page (a config-driven way to withhold one
  would be a second, quieter access-control mechanism), and `splitCSV` drops empty entries so an unset
  variable cannot produce a set containing `""` — which would have made the non-existent patrol of every
  personnel user "overridden". Both asserted.
- 2026-09-19 — Wiring: `publicGate` on `application`, composed only when all three projections exist,
  refused rather than degraded (following the reveal rule's precedent). **A nil gate fails closed rather
  than answering 503**, which is a deliberate divergence from every other nil-projection path in this
  service and is commented as such: a distinguishable "exists but unavailable" on an unauthenticated
  route confirms a patrol number is real and that the race is still running.
- 2026-09-19 — The closed-gate *response* is deliberately not in this task. Every closed path must answer
  identically to "no such patrol", and that answer belongs with the page it mimics (task 341). A note at
  the foot of `cmd/api/publicgate.go` records the property to preserve when 341 lands: one function
  producing it, called from every closed branch.
- 2026-09-19 — **Known gap, deliberate:** `LastCloses`'s NULL/zero path is not covered by a querier test,
  because this repo has no `cqrs.Reader` fake and queriers are not DB-tested anywhere here (see
  `checkpoint/consumer_test.go`, which tests statement text only). The behaviour it protects *is* covered,
  at the gate, where the same guard is duplicated for exactly this reason. Flagging rather than inventing
  a one-off harness.
- 2026-09-19 — All criteria met. `go vet ./...` and `go test ./...` clean across the module. Moving to done.
