# 432 — The filmstrip fetched the whole album when the viewer opened

**Status:** done
**Priority:** medium
**Created:** 2026-09-26
**Picked up by:** agent
**Started:** 2026-09-26
**Completed:** 2026-09-26

## Description

Reported from the live site: browsing `2026/album/diplombilleder` left **dozens of
`?variant=thumb` requests pending**, all attributed to `fillStrip` in `viewer.js`. The ordinals were
sparse — 0, 4, 5, 6, 7, 8, 9, 11, 12, 15, 16, 18, 19, 20… — which is the tell: those are exactly the
thumbnails the album grid had *not* already fetched, because the grid's tiles are `loading="lazy"`
and only the ones near the viewport had loaded.

So opening the viewer fired one request per photograph in the page, up to the 200-item page cap, and
they competed with the photograph the visitor was actually waiting for.

**`loading="lazy"` was not broken.** It was the wrong instrument at this density. Its scroll-distance
threshold is thousands of pixels — tuned for a page of full-width images — while a filmstrip button
is `3.5rem` plus a gap, about 62px. So "just off screen" covers something like fifty thumbnails, and
a browser honouring the attribute perfectly still fetches most of the album.

That matters beyond the wasted bytes: PRD 023 §6 budgets **two ahead and one back** for prefetching,
and `prefetchAround` is written carefully to respect it. A filmstrip quietly spending two orders of
magnitude more than the prefetch is the thing that PRD exists to prevent, arriving through a
different door.

**Not a regression from tasks 409/410.** `fillStrip` is unchanged since task 416; `git log` on
`viewer.js` and a diff across the 429/430/409/410 commit confirm the only edits were to the main
image's `srcset` and to `prefetchAround`. The report surfaced a pre-existing problem — the 800px
rendition did not add to it, since the strip only ever requests thumbnails.

## What changed

Thumbnails are now hydrated by an `IntersectionObserver` rooted **on the strip**, rather than by
`loading="lazy"`:

- The URL is staged on `data-src` and promoted to `src` only when the button approaches the strip's
  visible area. Deliberately not both — an `<img>` with a `src` is already a request, whatever an
  observer decides afterwards.
- `root: strip` is the load-bearing part. The strip is the scroll container, so intersection has to
  be measured against it; rooted on the viewport it would treat the whole horizontally-scrolled row
  as visible and we would be back where we started.
- `rootMargin: '0px 300px'` keeps sideways scrolling smooth without going back to a whole-album
  fetch. Roughly five buttons of headroom either side.
- No observer available means load immediately. This is a bandwidth optimisation, not a correctness
  property, and a visible thumbnail beats a clever one. (Baseline is Safari 16.4+ / Chrome 111+, so
  in practice it is always there.)
- The observer is disconnected both when the strip is refilled and on close. Left connected it holds
  every thumbnail `<img>` the viewer has ever shown, which the admin tool would accumulate one album
  at a time.

A pleasant consequence on phones: the strip is `display: none` under 40rem / 34rem (PRD 023 §6), and
a zero-size root intersects nothing, so a phone now fetches **no** strip thumbnails at all rather
than relying on the browser to infer that from a hidden element.

## Acceptance Criteria

- [x] Opening the viewer does not request a thumbnail for every photograph in the album
- [x] The observer is rooted on the strip, not the viewport
- [x] Scrolling the strip still loads thumbnails ahead of the visible edge
- [x] The observer is torn down with the strip and on close
- [x] The mechanism is pinned by test, so `loading="lazy"` cannot quietly come back as the answer

## Progress Log

- 2026-09-26 — Reported with a network panel: many pending `?variant=thumb`, all from `viewer.js`'s
  `fillStrip`.
- 2026-09-26 — Diagnosed the density mismatch rather than blaming `lazy`: 62px buttons against a
  multi-thousand-pixel threshold. Confirmed from the sparse ordinals that the requests were for
  thumbnails the grid had never fetched, and confirmed from `git log`/diff that `fillStrip` is
  untouched by tasks 409/410 — a pre-existing problem, not a regression.
- 2026-09-26 — ✅ Replaced the attribute with a strip-rooted `IntersectionObserver` plus `data-src`
  staging; teardown on refill and on close.
- 2026-09-26 — ✅ Two guards in `viewer_test.go`, and extracted the `jsFunctionBody` helper task 410
  had inlined so both tests share it. Mutation-checked: restoring `img.src` + `loading = 'lazy'`
  fails with both diagnostics.
- 2026-09-26 — `gofmt`, `GOWORK=off go test -timeout 5m ./...` and `go tool staticcheck ./...` clean.

## Worth measuring next

This is the kind of thing task 400 (*measure album page load*) and task 411 (*the viewer QA pass on
real devices*) exist for. The specific number to watch in a network panel: **opening a 200-photograph
album should issue a handful of thumbnail requests, not two hundred.**
