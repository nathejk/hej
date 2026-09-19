# 330 — Section 2 gate: when a patrol's public page opens

**Status:** open
**Priority:** high
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] One shared gate function, called first in every section-2 handler (HTML and JSON alike); no
      handler reaches patrol data without passing it.
- [ ] Trigger 1: a scan attributed to a checkpoint in the highest-`sortOrder` non-deleted checkgroup
      opens that patrol's page, and only that patrol's.
- [ ] Trigger 2: once the greatest non-deleted `checkpoint.openUntilUts` is in the past, every
      patrol's page is open.
- [ ] `openUntilUts` of `0` means "not set", **not** 1970: an absent instant leaves the backstop
      unfired rather than opening every page at the epoch. Asserted by a test, because this is the
      failure that publishes everything at once.
- [ ] Fails closed on unreadable input — neither trigger satisfied, or the checkgroup/checkpoint rows
      cannot be read, and the page stays shut.
- [ ] **All-or-nothing**: no element has its own reveal rule. No "pins later", no "track later".
- [ ] A manual override can open one patrol's page early (for a finished patrol whose scan went
      unattributed). A convenience, not a safety net — the backstop already guarantees a page.
- [ ] Before either trigger, the URL answers as **not-yet** (see task 341 for the page itself), and
      the not-yet answer carries **no patrol data at all** — indistinguishable from the answer for a
      patrol number that does not exist, so it cannot be used to enumerate patrols or to read
      finish order live.
- [ ] The closing instant is computed once for the event and cached, not joined per request: these are
      unauthenticated routes.
- [ ] A comment at the gate records the four event-facts above and that changing any of them
      invalidates it.
- [ ] Tests cover: closed, open-by-trigger-1, open-by-trigger-2, override, absent `openUntilUts`, and
      unreadable input.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §0b.3 / §6 / §10 (Phase 0).
