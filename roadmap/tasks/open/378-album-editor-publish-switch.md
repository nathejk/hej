# 378 — The album editor: fields, order, captions, and the publish switch

**Status:** open
**Priority:** high
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

## Description

`GET /admin/album/:slug` (HTML), `GET /api/admin/albums`, `POST /api/admin/albums`,
`PATCH /api/admin/albums/:albumId` and `PATCH /api/admin/albums/:albumId/items`: an album's own page
with its title, description, sort order, publish toggle, and its items in order with caption fields and
an ordinal.

**Create is always unpublished** (PRD 022 §6). Not a default the curator can override at creation: an
album is assembled over several sittings, and a create that could publish would put the first
photograph on the open web before the second was chosen. Publishing is a `PATCH`, expressed by the
absence of `Published` on `album.Created` — the reasoning `album/events.go` already records.

**The slug is frozen at creation.** Retitling must never break a link already shared, which on a public
page is a link in a family's chat history. The existing `Updated` event deliberately has no slug field;
keep it that way.

The editor reads through `album.CuratorQueries` (task 366) so it can see unpublished and deleted albums
and removed items. No public read may gain that ability — `album.Queries` stays publication-filtered in
SQL, and `BySlug` keeps returning the same "not found" for unknown, unpublished and deleted so drafts
cannot be enumerated (PRD 022 §8.8). Note the admin page is addressed by **slug**, like the public page,
so a draft's slug is guessable from the public 404 only if the public read leaks — hence the test.

Captions live on the **photo**, not the item (PRD 022 §8.3): one place to edit, shared by every album it
is in. A per-album caption override is imaginable and is deliberately deferred (§11 Q3) rather than
built speculatively. The **cover is the first live item** — no cover column. Ordinals are editable
numbers; drag-to-reorder is explicitly not a launch requirement (PRD 022 §4).

Unpublishing or deleting must drop the album from the frontpage within the page's 60-second cache
window and no longer (PRD 022 §6) — "we will take it down" has to be true.

## Acceptance Criteria

- [ ] A created album is unpublished, and there is no request shape that creates a published one
- [ ] The slug is set at creation and no endpoint can change it
- [ ] Title, description, sort order and published are editable; pointer semantics mean editing one does
      not wipe another
- [ ] Editing a caption changes it in every album the photograph is in
- [ ] Reordering by ordinal works and the cover follows the first live item
- [ ] Unpublishing removes the album from the frontpage within 60 seconds — tested against the cache
- [ ] The public `BySlug` still answers an identical "not found" for unknown, unpublished and deleted
- [ ] Every endpoint carries OpenAPI annotations with `@Failure 401`
