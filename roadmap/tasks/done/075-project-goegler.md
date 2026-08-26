# 075 — Project gøgler

**Status:** done
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:** agent
**Started:** 2026-08-26
**Completed:** 2026-08-26

## Description

PRD 006 §8. Consume `NATHEJK.*.gøgler.*.signedup` / `.updated` /
`.status.changed`.

Gøgler people do **not** exist in shared-go: they live in `hq`'s local
`personnel` table. Projecting them here is therefore a second projection of the
same population in a second repo, which can disagree with hq's — PRD 006 §11 Q4
asks whether hq's slice should be promoted to shared-go instead. Proceed, but
record the duplication.

No guardian phone.

## Acceptance Criteria

- [x] Gøgler subjects consumed and classified as the `gøgler` app role — `.signedup`
      and `.updated`. **`.status.changed` does not exist on the stream** and is not
      implemented; see the progress log
- [x] Guardian phone explicitly "not applicable" (`phoneParent` NULL)
- [x] Idempotent, tested with `cqrstest` fakes
- [x] The duplication with hq's `personnel` noted in the package docs and §11 Q4

## Progress Log

- 2026-08-25 — Task created from PRD 006.
- 2026-08-26 — Done. 224 gøglere projected (125 in 2025, 99 in 2026), 0 dead letters.

  **No shared-go contract exists for this population.** Not just no table — no
  messages either. So the wire shapes in `goegler.go` were read off the live stream and
  are an *observation*, not an agreement: nothing upstream breaks if tilmelding changes
  them. Worth knowing before anyone relies on them.

  **`signedup` is load-bearing here, unlike the crew equivalent.** Task 074 had just
  established that `crew.*.signedup` is a strict subset of `crewmember.*.updated` and
  deliberately skipped it. The gøgler pair looks structurally identical — same two
  verbs, and the same `teamId`-vs-`userId` split in the body — so reusing that
  conclusion was the obvious move and would have been wrong. Counted first:

  | year | signedup | updated | signedup with **no** updated |
  |---|---|---|---|
  | 2025 | 125 | 99 | 26 |
  | 2026 | 99 | 68 | 31 |

  Roughly a third of gøglere exist only in the signup event. Skipping it would have
  quietly excluded them from the directory and therefore from logging in at all. The
  final row counts (125 / 99) match the distinct signup ids exactly, confirming nobody
  was dropped.

  **Identity.** One person, two spellings: `teamId` on signup, `userId` on update, both
  equal to the subject's entity id (verified on a person carrying both events). Neither
  handler trusts the body alone — both fall back to the subject.

  **`.status.changed` and `.deleted` do not exist.** Both were named in the task/PRD;
  neither appears on the stream. Left unimplemented rather than written blind against a
  shape nobody has seen. The consequence is security-relevant and is now PRD 006 §11
  Q10: **a gøgler who withdraws keeps a working login indefinitely**, because there is
  no deletion event to project. Every other population has one. Feeds into task 076.

  **Scout group stored in `teamName` — a deliberate stretch, with one guard.** A gøgler
  has no nathejk team and no section, so both columns the login chooser reads to tell
  two people on one phone apart (task 079) are empty for them. Their scout group is the
  only affiliation the events carry, so it goes in `teamName`. `teamId` is left empty,
  which is the property that keeps this safe: PRD 002's patrol-scoped reads key on
  `teamId`, so this can never make a gøgler appear to be in a patrulje. Verified:
  `teamId` non-empty for 0 of 224. Written only when non-empty, so the thin `signedup`
  event cannot blank a group that `updated` supplied (covered by a test).

  **Verified against the real stream** (truncate, restart, full replay):

  | year | gøglere | with phone | with group | with teamId | phoneParent NULL |
  |---|---|---|---|---|---|
  | 2025 | 125 | 124 | 77 | 0 | 125 |
  | 2026 | 99 | **99 (100%)** | 58 | 0 | 99 |

  0 dead letters. Other populations unchanged (spejder 1294, bandit 1433, crew 20 live,
  reconciling with task 074's run).

  **Finding: duplicate gøgler registrations, which the chooser cannot solve.** 100%
  phone coverage looked like good news until the shared numbers were inspected. Of the
  12 shared phones among 2026 gøglere, **8 carry the same name on every row** — one
  human who submitted the signup form repeatedly, each time getting a fresh `userId`
  and so becoming a separate person. "Rikke Banke Peytz" is 5 rows on one number;
  "Klaus" is 4. **30 of 99 gøglere sit on a shared number**, so about a third of them
  hit the login chooser, and for most the alternatives are indistinguishable because
  they are the same person. Only 4 of the 12 are genuine sharing (siblings, e.g. Anders
  and Sofie Rossén Fensløv — same number, same group), which is the case 079 handles
  correctly.

  Deliberately **not** patched in the projection: de-duplicating means picking a
  winner, which is a product decision, and would hide an upstream data problem. Raised
  as PRD 006 §11 Q9.

  **Also recorded (not this task's work):** the user's decision that sign-in stays
  phone-only despite only 17% of bandits having a number — the inaccessibility is an
  intentional forcing function. Written up as PRD 006 §11 Q11, including the
  consequence that bandit onboarding must assume a first login very close to the event.

## Files

- `go/nathejk/table/person/goegler.go` (new) — message shapes + the duplication note
- `go/nathejk/table/person/consumer.go` — gøgler subjects and handlers
- `go/nathejk/table/person/goegler_test.go` (new)
- `roadmap/prd/doing/006-member-directory.md` — §11 Q4 answered; Q9/Q10/Q11 added;
  task checklist brought up to date
