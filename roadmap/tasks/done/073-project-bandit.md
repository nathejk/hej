# 073 — Project senior/klan as bandit

**Status:** done
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

PRD 006 §8. Consume `NATHEJK.*.senior.*.updated` / `.deleted`,
`NATHEJK.*.bandit.*.armNumber.assigned`, and the klan subjects.

"Bandit" is not a field anywhere: it is the event-role name for a senior in a
klan. The evidence is the subject vocabulary — shared-go's `senior` projector
consumes the `bandit.*.armNumber.assigned` subject and writes `senior.armNumber`.

Note `hq` **also** keeps bandits in its local `personnel` table, so bandit
identity is already split across two projections. Do not add a third notion of it;
derive from the senior/klan events only.

Seniors have **no guardian phone**.

## Acceptance Criteria

- [x] Senior + klan + armNumber subjects consumed
- [x] Classified as the `bandit` app role
- [x] Arm number carried (it is an identification mechanism that needs no photo)
- [x] Guardian phone explicitly "not applicable", not blank
- [x] **`teamName` populated with the klan name** — the login chooser shows it to tell
      two people on one phone apart (task 079), and the column is currently empty for
      bandits
- [x] Idempotent, tested with `cqrstest` fakes

## Progress Log

- 2026-08-25 — Task created from PRD 006.
- 2026-08-25 — Picked up. Added `senior.*.updated`/`.deleted`,
  `bandit.*.armNumber.assigned`, and `klan.*.signedup`/`.updated`, taking the subject
  names from shared-go's own senior projector rather than guessing.
- 2026-08-25 — The arm-number subject has **five** parts
  (`NATHEJK.<year>.bandit.<id>.armNumber.assigned`) and its body
  (`NathejkLokArmNumberAssigned`) carries only the number and a team type — so the member
  id comes from the subject. It is also matched **before** the four-part senior patterns:
  matched after, it would be swallowed and the arm number would silently never be
  recorded. Both facts have tests.
- 2026-08-25 — `handleTeamUpdated` now serves patrulje **and** klan with one decode,
  because `NathejkTeamUpdated` and `NathejkKlanUpdated` agree on the two fields used
  (`teamId`, `name`). Documented at the function, since using the "team" type for a klan
  event otherwise looks like a mistake.
- 2026-08-25 — Seniors get `phoneParent = NULL` explicitly. `NathejkSeniorUpdated` has
  no `PhoneContact` field and shared-go's `senior` table has no `phoneParent` column, so
  this is "not applicable" rather than "missing" — the distinction PRD 005 skips its
  confirmation step on.
- 2026-08-25 — ✅ 9 new unit tests; full suite, vet, staticcheck, gofmt green.

### Verified against the real stream — two findings

- 2026-08-25 — Truncate + full replay: **1530 bandits projected, 1321 with a klan name,
  0 dead letters.** So the klan denormalization task 079 needs is now real data, not just
  a column.
- 2026-08-25 — **Arm numbers: 0 recorded, and that is correct.** Checked the stream
  directly — `nats stream subjects NATHEJK 'NATHEJK.*.bandit.>'` reports **no subjects
  matching**. The events are simply not in this dataset (arm numbers are assigned at the
  event). So the handler is unit-tested but has **never seen a real event**; worth knowing
  before anyone relies on it.
- 2026-08-25 — **Finding worth escalating: only 241 of 1530 bandits (16%) have a phone
  number.** Spejder are at 93% (1610/1727). Since login is phone-only, roughly **1289
  bandits currently cannot sign in at all.**
  - Ruled out my own code as the cause: the stored numbers are correctly normalized
    (`+4530462703`, `+4525231124`, …), and a sampled `senior.updated` event from the
    stream literally carries `"phone":""`.
  - Likely explanation: klan signup collects a contact phone at the **team** level (as
    `patrulje` does with `ContactPhone`, and `signup` does per team) rather than from each
    senior. So for bandits the number on file is often the klan's contact, not the
    individual's.
  - Not something to fix here — it is either an upstream signup-flow gap or a property of
    this constructed dataset, and both are the maintainer's call. Raised in the summary and
    left for PRD 005/006 to decide.
- 2026-08-25 — Moving to done.
