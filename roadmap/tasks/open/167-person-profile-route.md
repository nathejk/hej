# 167 — Person profile route

**Status:** open
**Priority:** medium
**Created:** 2026-08-31

## Description

`/contacts/:personId` — a large avatar and the person's details (PRD 007 §7, §11.4).

**The field allow-list, settled 2026-08-31:** avatar, name, group/team, phone, and
function/section for crew. **Excluded:** postal address and guardian number.

Guardian numbers are excluded everywhere by `.rules`, not just here. PRD 003
deliberately treats address and `phoneParent` as own-details never shown to another
member, and everything admitted to this page is cached on other people's devices.

**Project an allow-list in the handler; do not serialise a row.** Every future field
added here is a field cached on other people's devices, so adding one should be a
visible decision rather than a side effect of a schema change. Task 159 is the tripwire
test.

Other requirements:

- Works **offline** for directory members (reads the same synced records as the list).
- For a member reached through a patrol lookup it is **live and not persisted**, like the
  lookup itself (task 168).
- Role-guarded on deep link — a spejder hitting the URL directly must be refused by the
  guard *and* by the endpoint (task 158).
- Night-legible surface, consistent with the list. A `tel:` link is fine; no action bar
  implying sharing, no export.

## Acceptance Criteria

- [ ] Route `/contacts/:personId` renders large avatar, name, group, phone, and crew
      function where applicable.
- [ ] No postal address and no guardian number anywhere on the page or in the payload.
- [ ] Handler projects an explicit allow-list.
- [ ] Works offline for directory members; live for lookup-reached members, persisting
      nothing.
- [ ] Deep link as a spejder is refused.
- [ ] Unknown or non-permitted `personId` renders a neutral not-found, indistinguishable
      from "not allowed".

## Progress Log

- 2026-08-31 — Task created from PRD 007 §7 / §11.4.
