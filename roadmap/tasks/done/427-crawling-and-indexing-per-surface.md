# 427 — The frontpage should be findable; individual photographs never should

**Status:** done
**Priority:** high
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

> *"It's ok that the frontpage is indexed and if people search for 'nathejk' and 'fotos' or 'video' that they can
> find us, that's very much intended, but we do not want the individual photos indexed."* … *"for now option 1 — if
> we at a later point adds some storytelling to an album, that should be indexable too. Glimt should never be
> indexable."*

Recorded as **PRD 011 §0c**, which this task implements.

### What the uniform rule was hiding

Every public page answered `noindex, nofollow`. That read as a careful privacy posture and was, in one respect,
the opposite of one:

- the page the event **wants** found was the one explicitly blocked;
- the photographs it wants protected carried **no policy of their own at all** — they were covered only by their
  hosting page being unindexed, which does not reach a photograph fetched by its own URL;
- and `/robots.txt` fell through to the SPA handler, so a crawler asking for the rules got `index.html` with a 200.

Indexing the frontpage made the second point urgent rather than theoretical: the frontpage *shows* album covers
and the glimt strip, so making it findable would have offered those photographs to image search.

### Two policies, and no third

| | |
|---|---|
| `noindex, nofollow` | the default, and the **zero value means it** |
| `index, follow, noimageindex` | the only way a public page enters the index |

There is no policy that indexes a page together with its photographs, which is a stronger guarantee than a
per-page setting: the failure nobody would notice cannot be reached by filling in a field wrongly. It would take
adding a third constant. `TestNoPolicyIndexesAPageWithItsImages` is the tripwire.

### "Never" is a method, not an omission

`publicGlimtPageData` and `publicPatrolPageData` **override** `RobotsPolicy` to return the unindexed policy
whatever their field says. Today an override and an unset field look identical; they are not the same promise. An
unset field is one assignment away from being set — by somebody wiring an unrelated feature, who would have no
reason to know — while an override has to be deleted, past a comment saying not to. The test sets the field to the
indexable policy and requires the answer to be unchanged.

### The `robots.txt` trap

`Disallow` blocks *crawling*, not indexing, so a blocked URL is one a crawler can never read a `noindex` from —
and can still be listed as a bare URL, unremovably. A well-meaning `Disallow: /2026/glimt` would make the glimt
page **more** likely to appear in results. So: crawling allowed everywhere, indexing refused per response, and no
path named in the file — including not `/admin`, since a public file naming a door advertises it.

## Acceptance Criteria

- [x] The frontpage is indexable as text, with its images excluded
- [x] Album, privacy, glimt and patrol pages are not indexed
- [x] Glimt and patrol pages cannot be *made* indexable by setting a field
- [x] Every photograph, and the diploma, carries `X-Robots-Tag: noindex` on its own bytes — including on a 304
- [x] `robots.txt` is served, allows crawling, and carries no `Disallow`
- [x] The header and the meta tag come from one place and cannot disagree
- [x] PRD 011 records the policy and the reasoning

## Progress Log

- 2026-09-25 — The zero value is the safe policy, reached through `RobotsPolicy()` rather than a field read: a page
  that says nothing — including one somebody adds next year without reading any of this — stays out of the index. A
  field every handler must remember to fill fails in the direction that cannot be undone, since a photograph in a
  search index is not recalled by fixing the header afterwards.
- 2026-09-25 — The media header is set inside `streamGlimtMedia`, the one function every photograph on every
  surface passes through, rather than at the four call sites. "The curator forgot one route" is exactly how this
  guarantee would be lost. On the 304 path too: a crawler that already holds the bytes still reads those headers.
- 2026-09-25 — `TestPublicSitePagesAreNotIndexed` became `TestTheCrawlingPolicyIsPerSurface`, a table with a
  reason per surface. Its failure was the first thing that happened after the policy change, which is the test
  doing its job — and the old shape, one rule for the whole site, is precisely what made the inversion above
  invisible for as long as it was.
- 2026-09-25 — Found while writing that table: the album route answers **503 through the shared JSON helper** when
  the projection is missing, and that path carries none of these headers. Not fixed here — it is an error path
  rather than a surface, and a JSON 503 is not indexable content — but noted, because the same is true of every
  JSON error the public site can emit.
- 2026-09-25 — The `robots.txt` guard strips comment lines before searching, because the file's own comment
  explains `Disallow` at length. That is the seventh guard in this repo that would have matched its own reasoning.
- 2026-09-25 — ✅ All criteria met. `gofmt`, `go vet`, `go test ./...` across every package clean.
