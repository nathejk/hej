# 414 — The frontpage's album list reads as a list of links, not a shelf of albums

**Status:** done
**Priority:** medium
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

The public frontpage's albums section (PRD 011 §6, task 333/334) was too rustic to carry the event's
memory. Reported by the maintainer against `/2026`, with a photo-library screenshot as the reference:
a year header with a count, then a row of uniform square covers, each with its title, a middle line and
a count stacked beneath it.

What was actually wrong, found by reading the markup rather than the screenshot:

- The card's title, description and count were **inline `<span>`s**, so `margin-top: .35rem` on `.name`
  did nothing and all three ran together on one line.
- The cover was rendered at its own aspect ratio, so no two cards in a row agreed on the height of the
  picture or the baseline of the title.
- An album with no cover rendered **no element at all**, so its card was shorter than its neighbours.
- `repeat(auto-fit, minmax(14rem, 1fr))` stretched two albums to half the page each.

Constraints: this is a server-rendered page (`go-server-rendered-pages`) — no Tailwind, no build step, CSS
inline in a Go **raw string**, so no backtick may appear anywhere in the markup, CSS or comments.

## Acceptance Criteria

- [x] The heading carries the number of albums, through the `albumCount` helper rather than by hand
- [x] Covers are a uniform square crop, and a missing cover occupies the same footprint
- [x] Title, description and count are three block lines in a fixed order
- [x] Two albums do not stretch to fill the row
- [x] No test needle about the album cards breaks, including the `1 billede` singular from task 387

## Progress Log

- 2026-09-25 — Task created, after the change, to give the code comments something to point at. The work
  was done in one pass: it is a styling fix on one section, so the board entry is a record rather than a
  plan.
- 2026-09-25 — `publicsite.go`: `.grid`/`.card` replaced with `.albumgrid`/`.album`; square crop on a
  `.cover` wrapper so the grey box matches a photograph's footprint; `auto-fill` instead of `auto-fit`;
  Lucide `image` as inline SVG for a coverless album; `albumCount` registered as the `albums` template
  function beside `photos`.
- 2026-09-25 — The cover's `alt` is now empty. It repeated the title, which is the next thing inside the
  same link, so a screen reader announced every album twice.
- 2026-09-25 — Walked into the documented backtick trap: `` `margin-top` `` inside a CSS comment ended the
  Go raw string and the compiler pointed at a line of CSS. Removed, and the comment now says out loud that
  no backtick may go in there — the fifth time this has cost somebody a build.
- 2026-09-25 — ✅ All criteria met. `go build ./...` and
  `go test ./cmd/api/ -run 'Frontpage|Album|Public|Plural|Danish'` pass.
- 2026-09-25 — **Renumbered from 397 to 414 before committing.** ID 397 was already taken by shipped work —
  the Fototilladelse consent gate and the diploma album (`photoconsent.go`, `patrolphoto`, commits `da8f952`
  and `40695df`) — but **its task file was never committed**, so `roadmap/tasks/` looked empty at 397 while the
  git log was not. IDs are never reused (TASKS.md), so this file moved rather than the older claim.
  For the next person taking a number: **`git log --format=%s | grep -oE 'task\([0-9]+\)'` as well as the
  three folders.** The folders are not the whole board when a task file has gone missing, and the missing 397
  file is a real gap in the audit trail that somebody who knows that work should fill.
- 2026-09-25 — Follow-up, deliberately not in scope here: the **album** page has the same rustic problem at
  a larger size (100+ photographs, tiles wider than the 320px thumbnail behind them, and no way to view one
  properly). That is a feature, not a polish pass, and is specced as PRD 023.
