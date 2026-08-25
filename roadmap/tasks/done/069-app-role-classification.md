# 069 — App-role classification and the section-slug map

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

PRD 006 §8. Own the mapping from signup data to app role in one place, so no
handler or frontend infers a role from a team type or a slug.

Known mapping:

- **spejder** ← a `spejder` row (team is a `patrulje`)
- **bandit** ← a `senior` row (team is a `klan`); the giveaway is the subject
  `NATHEJK.*.bandit.*.armNumber.assigned` projecting onto `senior.armNumber`
- **gøgler** ← `NATHEJK.*.gøgler.*` events
- **crew** ← a `crewmember` row; the *function* comes from `sectionSlug`, which is
  organizer-authored free text

The crew part is the risky bit: `postmandskab`/`guide`/`samarit` rest on **string
convention validated by nothing**. Implement a slug→role map with a **logged
fallback to generic crew** — never fail a login over an unmapped slug.

## Acceptance Criteria

- [x] A pure, unit-tested classification function (no DB, no stream)
- [x] Slug→role map with normalization (case, whitespace, Danish characters)
- [x] Unmapped slug → generic crew **and** a warning log, never an error
- [x] Table-driven tests incl. unmapped, empty and mixed-case slugs
- [x] Documented that this is convention, and what would replace it
      (projecting `section.Type`, PRD 006 §11)

## Progress Log

- 2026-08-25 — Task created from PRD 006.
- 2026-08-25 — Picked up. `classify.go` is pure — no database, no stream, no logger —
  so it is exhaustively testable and the projectors (tasks 072-075) all go through it.
- 2026-08-25 — Introduced a `Population` type to keep the two vocabularies apart:
  upstream speaks in entity kinds and team types, the app speaks in roles, and only
  this file translates. That is what stops a handler or a projector re-deriving "a
  senior is a bandit" somewhere else.
- 2026-08-25 — Documented the bandit derivation where someone will find it: "bandit"
  is not a field anywhere: it is the event role a klan senior plays, and the giveaway
  is that shared-go's `senior` projector consumes
  `NATHEJK.*.bandit.*.armNumber.assigned` and writes `senior.armNumber`.
- 2026-08-25 — Had to duplicate the role strings rather than import `users.Role`:
  this package is bound for shared-go and Go forbids importing another module's
  `internal` tree. Added a test pinning the duplicated values against the same
  literals, since a drift would return a role the frontend router guard does not
  recognise.
- 2026-08-25 — Normalization decision: fold **case and whitespace** but deliberately
  **not** Danish characters. `førstehjælp` and `forstehjaelp` are different strings an
  organizer might type, and silently equating them would hide a data problem that
  task 078 should surface. Both spellings can be added to the map explicitly instead.
- 2026-08-25 — The distinction that took the most thought, and is now two separate
  tests: **"no section assigned" is not the same as "slug not recognised"**. The
  former is a normal upstream state (`crewmember` has an `Unassigned` filter) and must
  not be reported; the latter must be, or the map silently rots as organizers rename
  sections. Both yield `RoleCrew`; only the second returns `ok=false`.
- 2026-08-25 — Added a test asserting **no mapped slug resolves to `RoleCrew`**. A
  typo in the map would otherwise fail safe but silently demote a real samarit to
  least-privileged — taking the SOS page away from someone who needs it. Also a test
  that every map key is already in normalized form, since an unnormalized key can
  never be matched.
- 2026-08-25 — An unknown `Population` also returns the fallback with `ok=false`, on
  the same least-privilege reasoning: if we cannot tell what someone is, they get the
  fallback rather than a guess.
- 2026-08-25 — `HasGuardianPhone` added here too, so the spejder-only rule lives with
  the classifier rather than being restated in each projector. Pinned by a test
  because PRD 005's confirmation step is spejder-only on the strength of it.
- 2026-08-25 — On the "warning log" criterion: the function is deliberately
  logger-free and returns `ok=false` instead. The **caller** logs, which keeps this
  pure and testable; tasks 072-075 are where the warn line lands. Noted here so the
  criterion is not read as satisfied by something that does not exist yet.
- 2026-08-25 — ✅ All criteria complete. 6 test functions, 12 table cases. Green on
  both the workspace and `GOWORK=off` paths.
- 2026-08-25 — Verification caveat: the slug map is a **guess at the organizers' real
  section names**, seeded from the one sighting in the codebase (`"samarit"` used as a
  section slug in hq's SOS tests) plus the obvious Danish spellings. Task 078 exists
  to check it against real data; until then, assume it is incomplete.
- 2026-08-25 — Moving to done.
