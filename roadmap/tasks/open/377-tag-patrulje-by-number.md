# 377 — Tag a patrulje by the number on its sign, confirmed by name

**Status:** open
**Priority:** medium
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

## Description

`GET /api/admin/patrols/:number` resolves a number to a patrol for confirmation,
`POST /api/admin/photos/tags` tags a whole selection, and
`DELETE /api/admin/photos/:photoId/tags/:teamId` untags one. Writes the events from task 367.

**No roster read, and no picker.** PRD 022 §8.6: `publicpatrol.Queries` has exactly one method,
`ByNumber(year, number)`, and its doc comment says the absence of a list read is deliberate — "a list
read is what a scraper would ask for". There is no patrol list anywhere in the service at any layer. An
autocomplete picker would need one, so v1 does not build one: the curator types the number **from the
patrol's sign**, which is how they know it, the tool resolves it through the existing `ByNumber`, and the
name is shown back for confirmation before the tag is saved. An unknown number is refused with a plain
reason. This gets the feature with no new read, no new enumerable surface, and no widening of a type
unauthenticated handlers read through. If a picker is later wanted it must be a **separate
authenticated interface**, not a method added to `publicpatrol.Queries`.

**Hazard:** `teamNumber` is not unique per year — `ByNumber` does `ORDER BY teamId LIMIT 1` over a
deliberately non-unique index. The tag therefore stores the resolved `teamId` as well as the number
(PRD 022 §8.6, §11 Q1) so it survives a renumbering rather than silently pointing at a different
patrol. The confirmation step is what makes the `LIMIT 1` defensible: a human sees which patrol they
got.

A tag names **a patrol, never a person.** No field for a member, and none may be added — PRD 022 §4
repeats this specifically because a tagging UI is where somebody reaches for a name. And per §11 Q1 the
tag has **no public effect in v1**: it is curator metadata, so tagging can be used in anger and
corrected before a mistag can put a photograph on the wrong family's page.

## Acceptance Criteria

- [ ] Typing a number shows the patrol's name back, and nothing is saved until the curator confirms
- [ ] An unknown number is refused with a plain Danish reason and no tag is written
- [ ] Tagging works on a multi-selection in one request; a photograph may carry several tags
- [ ] The stored tag carries `teamId` and the number; a test proves it still resolves after a
      renumbering
- [ ] `publicpatrol.Queries` gains no method and no list read — asserted by a test on the interface
- [ ] No response from these endpoints names a person, and there is no field that could
- [ ] Untagging removes exactly one tag
- [ ] OpenAPI annotations with `@Failure 401`
