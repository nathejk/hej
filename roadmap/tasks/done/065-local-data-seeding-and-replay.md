# 065 — Local data seeding / replay procedure for developers

**Status:** done
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-26

## Description

PRD 008 §11 Q7. A projection cannot be verified against an empty table, so
developers need realistic data. Options: replay from a real stream, an anonymised
dump, or seeded fixtures.

**Privacy constraint:** the data is minors' names, addresses and guardian phone
numbers. An anonymised or synthetic path is very likely the only acceptable one —
do not document a procedure that copies production personal data onto laptops.

## Acceptance Criteria

- [x] A documented, repeatable way to get usable local data
- [x] No procedure that puts real personal data on a developer machine
- [x] Works from an empty volume (proves replay/rebuild too)
- [x] Referenced from the dev docs so it is discoverable

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
- 2026-08-25 (later) — **Both blockers are gone**, so this is ready to pick up:
  - Docker is running again, and the dev stack comes up clean (db + api + the shared
    `jetstream` broker), so "works from an empty volume" is now testable — and has in
    fact already been demonstrated for the schema (see task 068).
  - PRD 006's `person` projection exists. Its subjects are still empty until tasks
    072-075, though, so the event *shapes* a seeder must publish are still not fixed.
- 2026-08-25 (later) — Revised sequencing: this should follow **task 072** rather than
  wait for all four projectors. Once spejder is projected there is one real subject and
  one real body shape to seed against, which is enough to build the tool and prove the
  round trip; the remaining populations are then additive.
- 2026-08-25 (later) — Unchanged: the privacy constraint. Replaying the real stream onto
  a developer machine stays out of the question, so the seeder publishes **synthetic**
  events with obviously-fake names.
- 2026-08-25 (later) — **Correction from the maintainer: the stream data is constructed
  to look real, not copied from production.** So the alarm I raised above was wrong and
  is withdrawn — no production personal data reaches a developer machine, and nothing
  needs changing about how the dev stack connects to the shared broker.
- 2026-08-25 (later) — Worth keeping the episode on the record, because it makes a point
  this task already predicted. My own earlier note said "realistic-looking fake personal
  data has a habit of being mistaken for real and treated casually". I then did exactly
  the inverse: saw plausible Danish names, addresses and guardian numbers, and reported
  a privacy incident. Convincing fixtures cost something in either direction, and the
  cheap mitigation is to make the fakeness legible — an obviously-synthetic marker
  somewhere in the record, so neither a human nor an agent has to guess.
- 2026-08-25 (later) — Net effect on this task: **scope shrinks back** to what it
  originally was. There is already usable, realistic data in the shared stream, so a
  seeder is no longer needed to make the projection testable — which was its main
  justification. What remains genuinely useful is a way to seed **specific** cases on
  demand: a shared phone number, an unmapped section slug, a member with no birthday.
  Those are the paths that need to exist deliberately rather than by luck.
- 2026-08-26 — Done, in two halves matching the split above.

  ### Half one: the procedure, which turned out to need no tooling

  "How do I get usable local data" has an answer that already worked and was written down
  nowhere: start the stack. Nothing writes to the database directly, so there is no
  migration and no fixture load — the tables fill themselves from the broker. A reset is
  dropping the tables and restarting, which recreates the schema and replays from
  sequence zero in ~2-3 minutes. That satisfies "works from an empty volume", and task
  078 had already proved it end to end from *dropped* tables (not merely truncated), so
  schema creation is on the same path.

  The repo had **no README and no `docs/`**, which is why the criterion asked for
  discoverability and there was nowhere to put it. Added one covering the stack, the
  external networks, the reset/replay loop, `EVENT_YEAR`, how to log in locally, and the
  seeder.

  Two documented commands were **wrong as first written**, and testing them is what
  caught it:
  - `docker compose exec db mysql -p"$MARIADB_ROOT_PASSWORD"` expands the variable on the
    *host*, where it is unset — it fails with "Access denied … using password: NO". Fixed
    to `sh -c '…'` so it expands in the container, and the fix verified.
  - The multi-table `DROP TABLE IF EXISTS a, b, c` form was verified separately against
    throwaway tables rather than assumed.

  Also documented two things that waste time when unknown: `request-pin` returns the same
  response for unknown numbers (deliberate, anti-enumeration), and PIN requests are rate
  limited to 5/min/IP. Both bit me during this task — the rate limiter silently
  invalidated a verification run, including its control, until I noticed.

  ### Half two: `cmd/seed`, for what the real data cannot produce

  Twelve named cases, published as real events under a sentinel year. The justification
  is strongest for the first: task 078 found that **three of the seven app roles have no
  members at all**, so `samarit`, `postmandskab` and `guide` — and therefore the SOS page
  and PRD 007's whole access matrix — could not be exercised by an entitled account.
  Waiting for an organizer to assign someone is not a development strategy.

  Design decisions worth recording:

  - **Publishes events, never SQL.** Inserting rows would test a table shape instead of
    the projector, and would drift from the real message bodies. Publishing real subjects
    means seeded cases travel the same path as production data, including the parts most
    likely to be wrong.
  - **Sentinel year, defaulting to 9999, with real years refused outright.** The broker is
    shared and cannot be un-published, so a mistyped `-year` would otherwise inject
    synthetic members into a live event's projection — visible to `hq` and `tilmelding`
    too. A guard, not a warning.
  - **Legible fakeness**, acting on this task's own lesson: every name starts with `TEST `
    and every phone with `+4599`. A sentinel year is not legible when you are looking at
    one row. Both are enforced by tests — with a minimum-count assertion, because the
    likeliest failure is the extractor finding nothing and the check passing vacuously
    (a trap I fell into earlier in this session).
  - Tests also pin that **no case hardcodes a year**, that subjects are well-formed enough
    for the consumer to match (a too-short subject is silently ignored — the case would
    appear to seed and do nothing), and that case names are unique and sorted.

  ### Verified end to end

  Ran the seeder against the dev broker: 29 events, **0 dead letters**, every case landing
  correctly in the projection.

  | case | outcome |
  |---|---|
  | capability-crew | `samarit`, `postmandskab`, `guide` rows exist — **first time these roles have had members** |
  | out-of-order-crew | assignment published *before* the person and the section label still converged to `samarit` with the label resolved |
  | unmapped-section | `crew` fallback, slug recorded, warning fired for exactly that slug |
  | unusable-phones | `phoneParent` NULL, warnings with digit counts 7 and 16 |
  | bandit | armNumber `999` projected — **the arm-number handler's first real broker event** (task 073 noted it had never seen one) |
  | deleted / no-birthday / no-guardian / duplicate-registration / shared-phone / goegler-signup-only | all as specified |

  Then pointed the API at `EVENT_YEAR=9999` and logged in through the real HTTPS flow:
  `samarit`, `postmandskab`, `guide` and `crew` all authenticated with the correct role.
  The two must-fail cases were re-tested with a working control after the rate limiter
  confused the first attempt: the **deleted member gets no PIN**, and the member with only
  a guardian number gets no PIN, while the control did — confirming the §11 Q13 boundary
  against a purpose-built case rather than an incidental one.

  `EVENT_YEAR` was restored to 2026 afterwards; the seeded year remains on the broker by
  design, inert because the directory reads one year.

  **Residual rough edge, not worth fixing here:** the stream library `log.Printf`s a line
  per publish, so the seeder's own output is interleaved with `Published message
  &jetstream.PubAck{…}` noise. Cosmetic, and in someone else's package.

## Files

- `README.md` (new) — dev stack, replay/reset, `EVENT_YEAR`, local login, seeding
- `go/cmd/seed/main.go` (new) — 12 synthetic edge cases, sentinel year, real-year guard
- `go/cmd/seed/main_test.go` (new) — year guard, no hardcoded years, subject
  well-formedness, legible-fakeness checks with anti-vacuity floors
