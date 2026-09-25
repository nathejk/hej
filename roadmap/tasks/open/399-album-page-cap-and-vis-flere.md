# 399 — Cap the album page at 200 items, with a plain "Vis flere" link and no JavaScript

**Status:** open
**Priority:** high
**Created:** 2026-09-25
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] An album larger than `albumPageCap` renders at most that many items per response, and `?side=N`
      renders the following window with the same layout
- [ ] The "Vis flere" control is a real `<a href="?side=2">` and appears only when there is a next page —
      verified with JavaScript disabled
- [ ] `?side=` out of range, non-numeric or `0` lands on a sensible page rather than an error or an empty grid
- [ ] `loading="lazy"`, `decoding="async"` and the intrinsic `width`/`height` are still on every tile
- [ ] The page handler's OpenAPI annotation documents `side`
- [ ] The existing album-page tests stay green, plus a new one per new behaviour (the cap, the link, the
      second page)

## Progress Log

- 2026-09-25 — Task created from PRD 023.
