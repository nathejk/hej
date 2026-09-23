# 377 — Tag a patrulje by the number on its sign, confirmed by name

**Status:** done
**Priority:** medium
**Created:** 2026-09-22
**Picked up by:** agent session (Zed)
**Started:** 2026-09-23
**Completed:** 2026-09-23

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

## What landed

The three endpoints and the tag panel. **No roster and no picker**, as the task required — the curator types the
number from the sign, the tool resolves it and shows the patrol back, and nothing is written until they confirm.

Worth setting beside task 376: this is the **opposite** decision from the checkpoint picker, and the difference is
the point. There, enumerating the course is something the curator legitimately needs and no scraper can reach, so
a separate interface was worth adding. Here the thing enumerated would be *every patrol in the event*, the curator
already knows the one number they want, and the confirmation covers the only thing a list would have added. Two
similar-looking asks, two different answers, for a reason that is written down in both places.

**The number is sent on the tag request, never a team id**, and the server re-resolves it. The confirmation showed
the curator a name; the request that follows must not be able to name a *different* patrol, and the only way to
guarantee that is for the server to do the lookup both times. `tagAdminPhotosRequest` has no `teamId` field, and a
test asserts it never gains one.

The panel clears its confirmation on **any** edit to the number box, so the button can never act on a patrol the
curator has stopped looking at.

`normalizePatrolNumber` is reused from the public page rather than reimplemented, so `042` and `42` are one patrol
— which is what the number on the sign means.

## Guards

Two protect the design rather than the code, and both were verified by breaking them:

- `TestThePublicPatrolInterfaceGainsNoListRead` — asserts `publicpatrol.Queries` still has exactly `ByNumber`.
- `TestThereIsNoRouteThatListsPatrols` — walks the route table and fails on any collection route ending in
  `patrols`. Verified by adding one: it failed with the reasoning attached.

Plus `TestThePatrolTagSurfaceNamesNoPerson`, which checks both the types *and* a real response body for
person-shaped content — so a field renamed to something innocuous would still be caught by what it carries.

## Verified live, against real dev patrol data

- patrol `1` resolved to *Skjoldungerne 2 · Skjoldungerne, Kongslejre division · Det Danske Spejderkorps* — the
  korps **label**, not the `dds` slug, and no person anywhere in the payload;
- `99999` → 404; `/api/admin/patrols` → 404, because it does not exist;
- both photographs tagged with patrol 1, then one also with patrol 10 → **two patrols on one photograph**, each
  keyed on its own `teamId`;
- untagging patrol 10 by its team id left patrol 1 intact on both photographs;
- the public map read still answers 404 — **a tag has no public effect in v1** (§11 Q1);
- the curator's counts moved to `tagged: 2`, with `tagCount: 1` on each photograph after the untag.

## Acceptance Criteria

- [x] Typing a number shows the patrol's name back, and nothing is saved until the curator confirms
- [x] An unknown number is refused with a plain Danish reason and no tag is written
- [x] Tagging works on a multi-selection in one request; a photograph may carry several tags — verified live
- [x] The stored tag carries `teamId` **and** the number, and a test walks a renumbering end to end: the original
      tag still names the original patrol, and the untag still finds it
- [x] `publicpatrol.Queries` gains no method and no list read — asserted on the interface, and on the route table
- [x] No response names a person, and no type has a field that could — asserted structurally and on a real body
- [x] Untagging removes exactly one tag, scoped by year, photograph and team
- [x] OpenAPI annotations with `@Failure 401` on all three endpoints

## One behaviour worth knowing

Untagging a tag that does not exist answers **204**, not 404. The event is published, the fold's `UPDATE` matches
no row, and nothing happens. That is ordinary idempotent-DELETE behaviour and is the right choice here — a curator
clicking twice should not see an error — but it does mean the endpoint cannot be used to probe which tags exist,
which is a small bonus rather than an accident.
