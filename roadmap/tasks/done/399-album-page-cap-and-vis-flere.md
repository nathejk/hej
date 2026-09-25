# 399 — Cap the album page at 200 items, with a plain "Vis flere" link and no JavaScript

**Status:** done
**Priority:** high
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

PRD 023 §2a.3 and §6 ("Functional — the album page"). `albumPageHandler` renders every item `BySlug`
returns, on every request, and an album is now as large as an SD card (PRD 022's bulk hand-in). This task
bounds the response: at most `albumPageCap` items, proposed **200**, with `?side=N` for the rest and a real
`<a href="?side=2">Vis flere</a>` when there are more.

**Be clear about what this is and is not.** §2a is honest that paging buys approximately nothing on
transferred bytes — `loading="lazy"` already does that work and stays. What the cap buys is bounded server
work per request, a bounded DOM on a weak device and a bounded viewer list. And 200 is deliberately
**generous**: this is insurance against somebody emptying a 2000-photograph memory card into one album, not
a pagination feature, and nobody should tune it downward for aesthetics. If 200 turns out to be the wrong
number, task 400 is what says so.

It is about twenty lines in `go/cmd/api/albumpage.go` plus the template in `publicsite.go` — `HasMore` and
`NextSide` on the view, the offset applied to the item slice, and the link rendered when `HasMore`. No
script is involved at any point, which is what makes each page a normal, shareable address that works with
JavaScript off (PRD 023 §3, PRD 011 §8).

`?side=` rather than `?page=`, per §8: the surrounding copy is Danish and the query string is part of what a
visitor sees and sends. The handler's existing OpenAPI annotation covers a page that took no parameters and
now takes one, so it needs updating (`.rules`: every endpoint, including the HTML ones).

Independent of task 398's tile size — they touch the same template but neither blocks the other.

## Acceptance Criteria

- [x] An album larger than `albumPageCap` renders at most that many items per response, and `?side=N`
      renders the following window with the same layout
- [x] The "Vis flere" control is a real `<a href="?side=2">` and appears only when there is a next page —
      verified with JavaScript disabled
- [x] `?side=` out of range, non-numeric or `0` lands on a sensible page rather than an error or an empty grid
- [x] `loading="lazy"`, `decoding="async"` and the intrinsic `width`/`height` are still on every tile
- [x] The page handler's OpenAPI annotation documents `side`
- [x] The existing album-page tests stay green, plus a new one per new behaviour (the cap, the link, the
      second page)

## Progress Log

- 2026-09-25 — Task created from PRD 023.
- 2026-09-25 — Picked up. Plan: `albumPageCap` and a `side` window in `albumPageHandler`, `HasMore`/`NextSide` on the view, the link in the template, then tests for the cap, the link, the second page and the bad-input cases.
- 2026-09-25 — `albumPageCap = 200` and `albumPageWindow` in albumpage.go; `HasMore`/`NextSide` on the view; the link in the album template. The window function returns the **resolved** side as well as the slice, because the first cut re-derived it by calling `albumSide` twice and that is precisely how the link ends up one page off the window beside it.
- 2026-09-25 — Decided every bad `side` lands on a page rather than an error: nonsense, `0` and negatives are page one, past-the-end clamps to the **last** page. A visitor never typed this query string — a link did, possibly one that was right when it was sent and is not now because the curator removed photographs. An empty grid under a real album's title reads as "the photographs are gone", which is a worse lie than "here is the end of the album".
- 2026-09-25 — An album at or under the cap returns before any arithmetic, so `?side=7` on a 40-photograph album is simply the album, with no control to press. `TestAnOrdinaryAlbumHasNoPagingControl` holds that: the cap is insurance against a memory-card dump, and a "Vis flere" under a two-screen album would make a complete page look truncated.
- 2026-09-25 — ✅ Tests: `TestTheAlbumWindowCapsAndClamps` (11 cases, table-driven, away from HTTP — an off-by-one reaches the page as "the link is missing", a long way from the line that caused it), plus `TestABigAlbumIsCappedWithAPlainLink` end-to-end. The fixtures number their captions so a test can say *which* window it is looking at; counting tiles cannot tell page 2 from a second copy of page 1, which is the bug an offset error actually produces.
- 2026-09-25 — The end-to-end test also asserts no `<script` on the page. PRD 011 §8 requires this page to work without JavaScript, and it is about to grow a JavaScript viewer (task 402) — if the route to photograph 201 ever becomes an event listener, that assertion is what should stop it.
- 2026-09-25 — OpenAPI annotation updated with the `side` parameter and the clamping rule. `gofmt`, `go vet`, `go test ./cmd/api/` clean.
