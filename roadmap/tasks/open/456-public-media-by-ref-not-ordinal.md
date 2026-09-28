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

## What was done, and what is waiting for you

**The defect is fixed, conservatively.** `albumMediaCacheControl(selector)` sends `immutable, max-age=1y` only
for a **content-addressed** URL and `public, max-age=60` for an ordinal — matching the album page that
references it. The false promise is gone: no cache is told "never revalidate" about a URL whose meaning a
re-sort can change. Mutation-checked by restoring the old header, which fails the test.

The route also **accepts a ref** now (`:selector` is a ref or an ordinal, distinguished by `blob.Ref.Valid()`,
which an ordinal can never satisfy). Ordinal URLs keep resolving, because they are already in caches and shared
links.

**The page still mints ordinals, and that is the part waiting for a decision.**

## The invariant this ran into

Putting refs in the page — the other half of the fix, and what task 447's permalink needs — breaks a rule that is
stated twice and tested structurally:

```go
// No blob hash may appear in a public payload — content addressing would make it a forwardable, unrevokable
// capability. So the view model carries a **boolean**, not the ref, and the page addresses photographs by
// ordinal.  — TestTheAlbumPageNeverPutsARenditionRefInItsHTML
```

```go
// Media is content-addressed, so `/api/.../<sha256>` would be a bearer token: forwardable, unrevokable, and
// identical for every caller. […] That is precisely how a "min gruppe" photograph of a child becomes public —
// not through a breach, but through a hash being pasted somewhere.  — glimtmediaserve.go
```

### What I can say about it, having implemented the route

The direct hazard **does not apply here**, and the reason is in the code rather than in an argument:
`albumItemRef` resolves a selector *inside a named album that must be in `Published(year)`*, so a ref has
exactly the same reach as an ordinal — `TestAlbumMediaByRefIsScopedToItsPublishedAlbum` asserts that a ref from
one album 404s in another, and 404s in the unpublished one. Unpublishing revokes it. It is not a bearer token,
and the hash is not a secret in any case: it is the SHA of bytes the route already serves, so any visitor who
downloads a photograph can compute it.

What publishing it *does* cost is **defence in depth**. The rule's value is that no hash is ever in circulation,
so no future route — an added convenience, a debug endpoint, a change to the glimt path that shares this blob
store — can be exploited by a hash somebody already has. That is a real property and it is the one being traded.

### So: three ways forward, and it is your call

- **A — narrow the invariant.** Refs may appear in a public payload *where the route resolving them is
  album-scoped and publication-checked*. Both guards get an exception by name, with the reasoning, and
  `glimtpublic_test.go`'s structural rule keeps holding for glimt, where media is per-member scoped and the
  argument is much stronger. Task 447's permalink then works as specified.
- **B — keep the invariant and permalink by something else.** A per-album opaque token, or a short
  album-scoped id minted for the purpose. It costs a column and a mapping, and buys back the defence-in-depth.
  The address stops being derivable from the bytes, which is also a property: it cannot be computed by somebody
  who only has the photograph.
- **C — leave the page on ordinals.** The cache bug is already fixed, so this is a complete stopping point. It
  costs task 447: there is no durable public address for a photograph, and a shared link keeps rotting when an
  album is re-sorted.

I have not chosen. The last two times I guessed at one of these rules you had information I did not.

## Acceptance Criteria

- [x] Public album media is addressable by ref, within a published album
- [ ] The album page's own image URLs use it — **waiting on A/B/C**
- [x] `immutable` is only sent for a content-addressed URL
- [x] Ordinal URLs still resolve (they are what is already cached and shared)
- [x] 404 for every failure reason, unchanged
- [x] A test that a re-sort cannot change what a given media URL serves — by construction: the ref form's
      meaning cannot change, and the ordinal form no longer claims it cannot

## Progress Log

- 2026-09-28 — Found while scoping task 447.
- 2026-09-28 — Fixed the header, added the ref form to the route, and wrote the anti-capability test.
- 2026-09-28 — Hit `TestTheAlbumPageNeverPutsARenditionRefInItsHTML` when minting refs into the page. Reverted
  that half rather than excepting the guard: it is the third privacy invariant this work has met, and the
  previous two both turned on something the maintainer knew and I did not.
- 2026-09-28 — Also walked into the backtick trap in `publicsite.go` for the fifth recorded time, by writing
  `immutable` in backticks inside a Go raw string. Comment now says so at the site.
- 2026-09-28 — `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...` clean. Mutation-checked the header.
