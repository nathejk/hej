# 401 — `?foto={ordinal}` is a server-side deep link, not a fragment

**Status:** done
**Priority:** high
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

PRD 023 §6 ("Functional — the viewer") and §7.8. The album page accepts `?foto={ordinal}`, derives which
`side` contains that item from the ordinal and `albumPageCap`, renders **that** page, and every tile carries
`id="foto-{ordinal}"` so the browser scrolls to it on its own.

**This must land before task 405**, the share button, because the share button's entire output is this link.
Shipping the button first would mean shipping a control that produces addresses the server does not
understand.

**Why a hash would not do**, since it is the obvious cheaper answer and §7.8 asks to be read in this order: a
fragment is client-only. A recipient who opens `#foto-137` gets the album's **first** page, and if the album
is past the cap the photograph they were sent is a "Vis flere" press away — or, with JavaScript off,
unreachable by that link at all. `?foto=137` lets the server do the work it is already holding the data for:
the right page, the right anchor, and the viewer opening on it if it loaded. A shared album link is read days
later on a different device (§2's timing), which is precisely the case app state cannot serve.

Behaviour on bad input is specified and deliberate: an out-of-range or non-numeric `foto` is **ignored, not a
404** (§8). The address still names a real album, and a link that has half-rotted should land on the album
rather than on an error page.

With a 200-item cap the derivation is usually the identity. That is the point rather than an objection: it is
a few lines, and it is what stops a shared link breaking the first day an album grows past the cap.

Scope is `go/cmd/api/albumpage.go` and the item template in `publicsite.go`. No script. The `?foto=` history
handling in the viewer belongs to task 403; this task is only the server side and the anchors. The page
handler's OpenAPI annotation gains the parameter.

## Acceptance Criteria

- [x] `?foto={ordinal}` renders the page containing that item, derived from the ordinal and the cap rather
      than assumed to be page one
- [x] Every tile carries `id="foto-{ordinal}"`, and a `?foto=` load with JavaScript disabled lands scrolled
      on the right tile
- [x] An out-of-range, negative, zero or non-numeric `foto` renders the album's first page rather than a 404
      or a 400
- [x] `?foto=` combines sanely with an explicit `?side=` rather than the two fighting
- [x] The page handler's OpenAPI annotation documents `foto`, including the ignore-rather-than-404 behaviour
- [x] Behavioural Go tests cover the derivation across the cap boundary, not just within page one

## Progress Log

- 2026-09-25 — Task created from PRD 023.
- 2026-09-25 — Picked up. Plan: resolve `foto` to a side by finding the item's **position** in the slice (ordinals are sparse after a removal, so ordinal/cap would be wrong), let `foto` win over an explicit `side`, put `id="foto-{ordinal}"` on every figure, and test across the cap boundary.
- 2026-09-25 — `albumSideHolding` + `albumRequestedSide` in albumpage.go, `id="foto-{ordinal}"` on every figure, OpenAPI annotation updated with `foto` and the ignore-rather-than-404 rule.
- 2026-09-25 — **`foto` wins over `side`** when a link carries both. They are different kinds of request and only one was typed by a human: `side` comes from a "Vis flere" link this page rendered, `foto` comes from somebody pressing share. When they disagree, the photograph is what the sender meant; the window is an implementation detail of how the page is cut up today, and the cut moves when a curator adds photographs.
- 2026-09-25 — **The derivation is a lookup, not a division**, and the reason is worth keeping: an ordinal identifies a *slot in this album*, not a position in it. `album_item` rows are soft-deleted and a photograph deleted from the library stops satisfying `BySlug`'s join — which is how the projection intends a deletion to take effect everywhere at once — so a long-lived album hands back sparse ordinals. `ordinal / albumPageCap + 1` reads correctly and would send a visitor to a page the photograph is not on.
- 2026-09-25 — **The task's premise was wrong about one thing and the fix is in the PRD now.** `?foto=137` does not scroll anybody anywhere: a query string is not a fragment. The query is for the server (which page) and the fragment is for the browser (which tile), a fragment is never sent to a server, and so a shared link has to carry **both** — `?foto=137#foto-137`. Recorded in PRD 023 §7.8 and in task 405's acceptance criteria, since the share button is what builds that URL.
- 2026-09-25 — Caught a worthless guard before trusting it. `TestTheFotoDeepLinkCountsPositionsRatherThanOrdinals` first used three sparsely-numbered photographs and **passed with the division in place**, because `albumPageWindow` returns early for an album at or under the cap and never consults the side. Rewritten with 250 items and one stranded high ordinal near the front, where the two implementations disagree; verified it fails with the division and passes without.
- 2026-09-25 — ✅ All criteria met. `TestTheFotoDeepLinkRendersThePageHoldingIt` covers ordinal 0 (a real ordinal, not a missing parameter), an ordinal past the cap, the last photograph, both directions of the `side`/`foto` fight, and the three kinds of rubbish. `gofmt`, `go vet`, `go test ./cmd/api/` clean.
