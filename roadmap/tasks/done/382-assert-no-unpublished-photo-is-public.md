# 382 — Assert no unpublished photograph is reachable on any public surface

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent
**Started:** 2026-09-23
**Completed:** 2026-09-23

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

- [x] Every case in the list above is in the fixture and absent from every public response
- [x] The public media route refuses an `(albumId, ordinal)` whose photo is deleted or whose album is
      unpublished — by row absence, not by a filter in the handler
- [x] No coordinate with a verdict other than `inside` appears in the map data
- [x] The route enumeration is self-checking: an unlisted public route that serves photographs fails the
      suite
- [x] Every assertion is verified by breaking it once
- [x] The test names PRD 022 §9 and PRD 011 §0b as the reason it exists
- [x] Runs in the normal suite, not behind a tag

## What was done

`go/cmd/api/publicvisibility_test.go`. Eight photographs, three albums, every awkward case from PRD 022
§9's list, walked across every public GET route and every media address the fixture can express.

### The fixture holds rows, not answers

This is the decision the whole file rests on. Every public album read funnels through `Published`,
`BySlug` and `Plottable`, and each applies its filter **in SQL** — the clauses task 365's
`album/querysafety_test.go` guards. A stub returning prepared lists would have made this file pass by
construction: it would assert that a fixture containing no draft photographs contains no draft
photographs.

So `visibilityStore` holds albums, memberships and photographs as separate rows and applies the same joins
the querier does. The join lives in one method, `live()`, for the same reason the real SQL is guarded
against the count and the cover disagreeing with the page — three reads that each decide what "live" means
are three chances to differ.

The two halves are complementary and neither is sufficient: `querysafety_test.go` proves the clauses are in
the statement; this proves a read obeying those clauses leaks nothing through any handler, template, or
cache header on the way out.

Bytes are real, too. A 404 caused by an absent blob looks exactly like a 404 caused by the control under
test, and would pass this file while the control was gone.

### The distinction the first draft got wrong

The initial marker list forbade the `outside` and `unknown` photographs everywhere, and the album page
failed. The test was wrong, not the page. **A bad coordinate is not a reason to hide a photograph** — it is
a reason not to plot it, and PRD 022 §9's two sentences are separate for exactly that reason. The curator
published those two; only their positions are withheld.

So their *coordinates* are forbidden and their captions are not — and
`TestThePublishedPhotographIsStillShown` now asserts both photographs are **present**, because otherwise
"withhold the coordinate" could be implemented as "drop the photograph" and every absence-assertion here
would still go green. Same reason the frontpage count is asserted: three live items out of five
memberships, so "5 billeder" over three photographs fails.

## A real hole, found by the walk

`TestNothingAlbumShapedSurvivesTheSectionBeingHidden` failed on its first run:

```
/api/public/albums/al-open/media/0 answered 200 with the section hidden; want 404
```

**`albumMediaHandler` ignored `PUBLIC_ALBUMS`.** With the albums section switched off, the pages 404'd and
the bytes did not — so anybody holding an album id and an ordinal, from a shared link, a crawler's index or
a browser history, could still fetch the photographs an organizer had just decided to hide. Fixed, with the
reasoning in place and the OpenAPI description updated.

This is the *same hole in the same feature* task 376 found in `/api/public/albums` two tasks ago. Worth
noting as a pattern rather than as two incidents: a media route serves no HTML and appears in no
navigation, so it is the surface a feature flag gets forgotten on. The general lesson is the one this test
encodes — hiding a feature has to be asserted on *every* surface, and a section whose links keep serving is
not hidden, it is unadvertised.

It is also why the gate test uses a **published, live, `inside`** photograph. Everything else in the file
asks whether a control lapsed for a photograph nobody cleared; this asks whether a photograph that *is*
cleared still disappears when the section is off. Opposite questions, and passing the first says nothing
about the second.

## Verified by breaking it

Six sabotages, each reverted, each hitting a different mechanism:

| Sabotage | What failed |
|---|---|
| `live()` stops checking `p.deleted` | the deleted-photograph marker, on the page and the map |
| `live()` stops checking `m.removed` | the removed-membership marker |
| `Published`/`BySlug` stop checking `published`/`deleted` | the draft *and* the taken-down album, by slug, title and id |
| `Plottable` stops checking the verdict | both forbidden coordinates, on the map endpoint and in the walk |
| **production:** `albumItemRef` stops comparing the ordinal | three media addresses answered 200 where 404 is required, including "no such ordinal" |
| **production:** a new `/api/public/albums/:albumId/original/:ordinal` route | the self-check named the route and said what to do |

The first four sabotage the fixture rather than the product, deliberately: those controls live in SQL,
which `cmd/api`'s stubs cannot execute, so what is being proved here is that the *assertions* bite on a
read that stops filtering. Task 365's guards prove the clauses are present in the statement. Neither
substitutes for the other.

## Verified live

The gate was checked against the dev stack rather than only in the suite, because "404" is a weak
observation — a route can 404 for a dozen reasons and look correct. With `PUBLIC_ALBUMS=false` (dev's
setting) both `/api/public/albums` and the media route answer 404; with a container started at
`PUBLIC_ALBUMS=true`, both answer 200. That pair is what makes the 404 attributable to the gate.

## Notes

- The glimt media route is skipped by the coverage self-check **by name**, not by a loose match, so a new
  album-side route cannot slip through the same gap. Glimt visibility is PRD 019's, with its own tests.
- The diploma and the patrol pages are in the walk via the route enumeration; they carry no album
  photographs today, and if one ever does, the markers are already looking.
