# 401 — `?foto={ordinal}` is a server-side deep link, not a fragment

**Status:** open
**Priority:** high
**Created:** 2026-09-25
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `?foto={ordinal}` renders the page containing that item, derived from the ordinal and the cap rather
      than assumed to be page one
- [ ] Every tile carries `id="foto-{ordinal}"`, and a `?foto=` load with JavaScript disabled lands scrolled
      on the right tile
- [ ] An out-of-range, negative, zero or non-numeric `foto` renders the album's first page rather than a 404
      or a 400
- [ ] `?foto=` combines sanely with an explicit `?side=` rather than the two fighting
- [ ] The page handler's OpenAPI annotation documents `foto`, including the ignore-rather-than-404 behaviour
- [ ] Behavioural Go tests cover the derivation across the cap boundary, not just within page one

## Progress Log

- 2026-09-25 — Task created from PRD 023.
