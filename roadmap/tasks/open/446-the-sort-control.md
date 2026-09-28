# 446 — The sort control in the album editor card

**Status:** open
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 024 §6 R1/R10 and §7. Needs 442 and 444.

A native `<select>` in the album editor card (`#albumeditor`, `albumeditor.js`) — where the title,
description, cover and publish state already are, because the sort is a property of the album. Saved on
change through the album PATCH the card already uses, then the grid reflects it.

Danish labels: *Manuel*, *Tid, ældste først*, *Tid, nyeste først*, *Filnavn A–Å*, *Filnavn Å–A*.

**This is the server-rendered admin tool, not the PWA** — Go `html/template` with vendored Pico, htmx and
Alpine, no build step. See the `go-server-rendered-pages` skill. A native `<select>` also means no keyboard
handling of our own.

Worth a line of help text under it: what a non-manual mode will do to photographs added later. The curator
choosing the mode is the person who needs to know that, at the moment they choose.

## Acceptance Criteria

- [ ] The control renders the album's current mode
- [ ] Changing it saves and the grid shows the new order without a manual page reload
- [ ] The five labels, in Danish
- [ ] A one-line explanation of what the mode does to photographs added later
- [ ] No Tailwind, no shadcn-vue, no new front-end dependency

## Progress Log

- 2026-09-28 — Task created from PRD 024 §6 R1/R10, §7.
