# 456 — Public album media is cached `immutable` at a URL that is no longer immutable

**Status:** open
**Priority:** high
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

## Description

**A live defect introduced by PRD 024, found while scoping task 447.** Not reported from use yet, which is the
only reason it is not urgent: the symptom appears days later and looks like something else.

`GET /api/public/albums/{albumId}/media/{ordinal}` serves

```go
const publicGlimtMediaCacheControl = "public, max-age=31536000, immutable"
```

— a year, `immutable` — at a URL whose last segment is **an ordinal**. PRD 024 made ordinals *mutable*: a
non-manual album re-sorts itself every time photographs are added to it, and a manual reorder rewrites them too.

So the promise is now false. `immutable` tells every cache — browser, CDN, ISP proxy — **not to revalidate at
all**, for a year. After a re-sort:

- `…/media/3` is photograph Y on the server and photograph X in every cache that has it;
- the album **page** is `max-age=60`, so it refreshes within a minute and then references the same ordinal URLs
  — which caches answer from their stale copies. The page updates, the images do not;
- the result is a published album showing the right captions under the wrong photographs, for up to a year, with
  nothing a curator can do about it and no way to tell from the server that it is happening.

This was survivable before PRD 024, when ordinals only moved on an occasional manual reorder. It is not now: the
common case — dropping a second card into a time-sorted album — rewrites every ordinal in it.

## The fix is the same change task 447 wants

Address public media by **ref** rather than by ordinal: `…/media/{ref}` (or the ref as a query), resolved within
the named album exactly as the ordinal is today. Then the URL is content-addressed, `immutable` becomes true
again, and the whole class of problem goes away rather than being mitigated.

This is the same insight as the permalink (task 447): **a photograph's position is not its identity.** Doing both
together is one story and one set of tests.

The ref is not a secret and cannot be made one — it is the SHA of the bytes the route itself serves, so any
visitor who downloads a photograph can compute it. What must stay true is that a ref is **not a capability**: it
addresses bytes only *within an album that is published*, and the route keeps checking that, exactly as it checks
the ordinal's album today.

## What must not be lost

- **404 for every reason** — unknown album, unpublished album, missing item, removed item — so a prober cannot
  tell which. That rule is already written at the handler and must survive the change.
- **The `publicAlbums` switch** still gates the bytes (task 359/382), not only the pages.
- **Old ordinal URLs keep working**, or every album page in every cache and chat history starts 404ing. They are
  the ones that are wrong under a re-sort, so they should keep resolving as they do today — the point is that the
  *page* stops minting them.

## Acceptance Criteria

- [ ] Public album media is addressable by ref, within a published album
- [ ] The album page's own image URLs use it
- [ ] `immutable` is only sent for a content-addressed URL
- [ ] Ordinal URLs still resolve (they are what is already cached and shared)
- [ ] 404 for every failure reason, unchanged
- [ ] A test that a re-sort cannot change what a given media URL serves

## Progress Log

- 2026-09-28 — Found while scoping task 447, by checking what the album page's image URLs are addressed by. The
  cache header and the ordinal are each defensible alone; together, after PRD 024, they are a promise the server
  cannot keep.
