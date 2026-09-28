# 451 — The credit resolver: crew only, name only, within the year

**Status:** open
**Priority:** high
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 025 §6 R3, and **the load-bearing task of that PRD.** Everything else is plumbing; this is the part that
decides whether "the public site can credit a photographer" stays distinct from "the public site can render any
person's name".

One function, in one place: *given a photograph's year and a credit id, return a crew member's name or `""`.*

- **`person.RoleCrew` only** (`classify.go`). A spejder's or bandit's id — mistyped, stale or malicious —
  resolves to nothing.
- **The name field only.** Nothing else on that row is read, ever.
- **The photograph's own year**, with no fallback to another. Somebody who was crew in 2026 and is not in 2027
  still has their 2026 photographs credited, because those resolve against 2026.
- **`""` when it finds nothing**, and PRD 025 §6 R5: that renders as no credit line at all — no placeholder, no
  empty label, no gap.

## The guards this replaces

`TestACreditIsOnlyEverTypedNeverDerived` asserts the opposite of this feature and must be **rewritten, not
deleted**. PRD 022 §6's claim was "the credit is never derived from person records"; the new claim is narrower
and still worth enforcing:

- the resolver reads `name` and no other column;
- it yields "" for anybody who is not `RoleCrew`;
- it is scoped to one year;
- it is the **only** path from the person projection to a public page, and it has no second caller.

Also: except `creditpersonid` in `libraryPersonShapedExceptions` (task 448's pattern) so the id may exist in the
library while a **public** response carrying it still fails.

## Acceptance Criteria

- [ ] One resolver, one place, with the four bounds above
- [ ] A test proving a participant's id resolves to ""
- [ ] A test proving no column but `name` is read
- [ ] `TestACreditIsOnlyEverTypedNeverDerived` rewritten to the new claim, not removed
- [ ] `creditpersonid` excepted for the library scope only, with its argument written where the exception lives
- [ ] Mutation-checked: dropping the `RoleCrew` check must fail a test

## Progress Log

- 2026-09-28 — Task created from PRD 025 §6 R3, §8.
