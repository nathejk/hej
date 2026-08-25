# 065 — Local data seeding / replay procedure for developers

**Status:** doing
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:**

## Description

PRD 008 §11 Q7. A projection cannot be verified against an empty table, so
developers need realistic data. Options: replay from a real stream, an anonymised
dump, or seeded fixtures.

**Privacy constraint:** the data is minors' names, addresses and guardian phone
numbers. An anonymised or synthetic path is very likely the only acceptable one —
do not document a procedure that copies production personal data onto laptops.

## Acceptance Criteria

- [ ] A documented, repeatable way to get usable local data
- [ ] No procedure that puts real personal data on a developer machine
- [ ] Works from an empty volume (proves replay/rebuild too)
- [ ] Referenced from the dev docs so it is discoverable

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Picked up. **Blocked**, and worth being precise about why, because two
  of the three options are closed and the third depends on work that has not happened.
- 2026-08-25 — Ruling out the options:
  - **Replay from the real stream** — closed on privacy grounds. The stream carries
    minors' names, addresses and guardian phone numbers. A documented procedure that
    copies that onto developer laptops is not something to write, and the task's own
    criteria forbid it.
  - **Anonymised dump** — needs a production database to dump *from*, which does not
    exist yet (task 061), and an anonymisation step nobody has specified. Also
    circular: the dump would be of projections, which are supposed to be rebuilt from
    events rather than transported.
  - **Synthetic fixtures** — the only viable route, and the one to build. But it has
    a hard prerequisite: fixtures must be *published events*, not SQL inserts, because
    under this architecture nothing writes to the database directly (PRD 008 §8). So
    the seeding tool has to publish `NATHEJK.*` messages that real projectors consume,
    and **there are no projectors yet** — PRD 006 lands the first one. Writing a
    seeder now would mean inventing event bodies for a projection that does not exist
    and will not match.
- 2026-08-25 — Second blocker regardless of approach: the criterion "works from an
  empty volume (proves replay/rebuild too)" cannot be verified because the **Docker
  daemon on this machine is not responding**. The verification is the substance of
  this task, not a formality.
- 2026-08-25 — Recommendation for when it is unblocked: build it as a small
  `cmd/seed`-style tool (or a `go test` helper) that publishes synthetic events to the
  dev broker, and derive the fixture shape from PRD 006's person projection so the two
  cannot drift. Keep the names obviously fake — realistic-looking fake personal data
  has a habit of being mistaken for real and treated casually.
- 2026-08-25 — **Blocked on:** PRD 006's first projection (for the event shapes), and
  a working Docker daemon.
