# 381 — Extend the structural privacy walk to the library and the tag

**Status:** open
**Priority:** high
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

## Description

The structural privacy test (task 337, in `go/cmd/api/publicprivacy_test.go` —
`TestPublicAlbumReadModelHasNowhereToPutAPerson`, `TestPublicViewTypesHaveNowhereToPutAPerson` and
their siblings) must walk the new types too: the `photo` projection's row and event structs, the
`photo_patrol` tag, and every admin response type that reaches a public surface.

PRD 022 §6 Non-Functional states the requirement flatly: the library row has **no uploader, no curator,
no person id, no name, no phone number, and emphatically no `phoneParent`**. §9's last metric is that
this is enforced "by extending the task 337 structural test rather than by review", and that phrasing is
the whole point of this task. Review catches it the first time and not the fifth; a walk over the types
catches it on the compile after somebody adds a convenient field. Task 361's record shows this working
exactly that way — the structural guard fired on the first compile, which is what it was for.

The repo rule this defends is unusually hard: guardian phone numbers may exist on exactly one surface in
this project, a user confirming their **own** guardian's number, and nowhere else — not a directory, not
a contact list, not a cached manifest. A photo library with a `phoneParent` would be absurd, which is
precisely why nobody would notice a struct embedding that carried one.

There is an additional reason the absence is structural here rather than a policy: with a **shared
credential** (PRD 022 §8.2) the tool cannot honestly attribute anything to a person, so a curator or
uploader field would be a lie in a column as well as a privacy hazard.

## Acceptance Criteria

- [ ] The walk covers the `photo` row type, all four/five `photo` event types, `photo_patrol`, and the
      admin response types
- [ ] Adding a `name`, `phone`, `phoneParent`, `personId`, `uploadedBy` or `curator` field to any of
      them fails the suite — verified by adding one and removing it
- [ ] The walk recurses through embedded structs, slices and maps, not just top-level fields
- [ ] A SQL-level check asserts the new tables declare no person-shaped column
- [ ] The test's failure message explains *why*, referencing task 337 and PRD 022 §6
- [ ] No existing assertion from task 337 is weakened to accommodate the new types
