# 382 — Assert no unpublished photograph is reachable on any public surface

**Status:** open
**Priority:** high
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

## Description

A test, not an inspection. PRD 022 §9: *"No photograph on a public page that a curator did not publish.
Verified by test, not observed; zero is the only acceptable number."* Same for the map: *"No coordinate
on the public map with a verdict other than `inside`."*

The surface to enumerate: the public frontpage, `/{year}/album/{slug}`, the public media route
`/api/public/albums/{albumId}/media/{ordinal}`, the map data read, the glimt strip, the patrol pages and
the diploma. Build a fixture containing the awkward cases and assert none of them appears anywhere:

- a photograph in the library and in **no** album;
- a photograph in an **unpublished** album;
- a photograph in a **deleted** album;
- a **removed item** in a published album;
- a **soft-deleted photograph** that is still a member of a published album;
- a photograph with verdict `outside`, and one with `unknown`;
- a photograph whose album is hidden by `PUBLIC_ALBUMS=false` (task 359).

This matters more after PRD 022 than before it, and the reason is the model change: a photograph now
exists **before** anybody decides it may be shown. Under the old model existence and publication were
nearly the same act, so the class of bug "a row leaked before curation" had nowhere to live. The whole
bulk is now that class of bug's habitat, and PRD 011 §0b leans on publication as a **safety control**
rather than a convenience — "a photograph is in an album because somebody decided it may be shown".

The test must also verify the enumeration itself, the way
`TestPublicRouteEnumerationCoversTheKnownSurface` already does: a new public route that serves media and
is not in the list would make this test pass by not looking.

## Acceptance Criteria

- [ ] Every case in the list above is in the fixture and absent from every public response
- [ ] The public media route refuses an `(albumId, ordinal)` whose photo is deleted or whose album is
      unpublished — by row absence, not by a filter in the handler
- [ ] No coordinate with a verdict other than `inside` appears in the map data
- [ ] The route enumeration is self-checking: an unlisted public route that serves photographs fails the
      suite
- [ ] Every assertion is verified by breaking it once
- [ ] The test names PRD 022 §9 and PRD 011 §0b as the reason it exists
- [ ] Runs in the normal suite, not behind a tag
