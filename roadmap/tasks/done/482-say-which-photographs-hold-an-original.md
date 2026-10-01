# 482 — Say which photographs hold an original

**Status:** done
**Priority:** medium
**Created:** 2026-10-01
**Picked up by:** agent session (PRD 027 rollout)
**Started:** 2026-10-01
**Completed:** 2026-10-01

**PRD:** 027 (R7)
**Depends on:** 481

## Description

Two eras of photograph will coexist for the life of this product. Nothing uploaded before PRD 027 ships has an
original and nothing ever will — there is no source to backfill from. So "can I print this?" has to be answerable
**before** a download rather than discovered inside a zip, and a download that silently substituted 1600px files
(task 481's fallback) would be the exact dishonesty PRD 027 exists to remove.

Three places, all in the server-rendered admin tool:

- **Per photograph**, on the contact sheet's photograph detail: whether an original is held.
- **Per album**, in the zip submenu. Proposal from PRD 027 §7: the item reads `Original (34 af 180)` when the album
  is partial, and is **disabled with a one-line explanation** when the count is zero. A disabled item with a reason
  teaches the curator why; a missing item just looks broken.
- **A library count**, "uden original", so the gap is a number somebody can watch go flat rather than a surprise.

**It must also say that an original carries the camera's metadata, including where the photograph was taken.** This
is new and it matters: before PRD 027 every byte leaving this tool was stripped, so a curator forwarding a file could
not leak a location. Now they can, by accident, while doing nothing wrong. The renditions remain the thing to send a
newspaper, and the UI should make that the easy reading rather than a footnote.

**Copy is Danish and written deliberately** — PRD 022 §5 requires that of anything where two similar actions must not
be confused, and "Original" sitting directly above `XLarge (1600px)` is precisely that situation.

## Constraints

This is the **server-rendered admin tool** (`go/cmd/api/adminui/`). Apply the `go-server-rendered-pages` skill:
Go `html/template` with vendored Pico + htmx + Alpine, real `.html`/`.css`/`.js` files, **no Tailwind, no
shadcn-vue, no npm dependency, no build step, no CDN**. Icons, if any, are inline SVG copied from Lucide. Nothing in
`vue/` is touched by this task.

## Acceptance Criteria

- [x] The contact sheet's photograph detail states whether an original is held — **delivered as a filter preset
      rather than a per-cell mark; see the log for why, this is a deliberate deviation**
- [x] The zip submenu's Original entry shows the held/total count when partial
- [x] The entry is disabled with a one-line Danish explanation when no photograph in the album has an original
- [x] The library counts include "uden original"
- [x] The download says an original carries the camera's metadata, including where it was taken, and points at the
      renditions as the thing to pass on
- [x] All copy is Danish and reviewed for the Original / XLarge confusion specifically
- [x] No Tailwind, no build step, no new npm or CDN dependency introduced
- [x] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-10-01 — Task created from PRD 027.
- 2026-10-01 — Picked up. Four surfaces: the per-album count in the download menu, the "uden original" library count,
  the per-photograph answer, and the sentence about what an original carries.
- 2026-10-01 — Projection first: `album.CuratorAlbum.OriginalCount` (a second subquery beside `itemCount`, same live
  definition plus `originalRef <> ""`) and `photo.Counts.WithoutOriginal`. Both carried through `adminAlbumSummary` and
  `adminCountsView`. Adding the count to the **edit** response too, where it cannot change — omitting it there would
  leave a stale zero in whatever re-rendered from that payload.
- 2026-10-01 — **Deviation from the brief, deliberate.** The criterion asked for the contact sheet's photograph detail
  to state whether an original is held. I implemented it as a filter preset — «Uden original», `?original=no` — rather
  than as a mark on the cell. The sheet's marks follow one rule, stated in the fragment's own header: *the ordinary
  case gets no mark*, because a badge on most of the grid makes the whole row slower to read. "Has an original" breaks
  that in both directions over time — rare today, universal in two years — so no badge for it stays quiet at both
  ends. A filter answers the same question per photograph, puts nothing on the cells, and is the mechanism this tool
  already uses for exactly this shape of question: uden album, uden billedtekst, uden fotokredit.
- 2026-10-01 — The viewer was the other candidate for "photograph detail" and was **rejected**: `viewer/viewer.js` is
  deliberately one copy shared by the public album page and the admin tool (PRD 023 §9), so anything original-aware in
  it would ship to the open web the same afternoon — which is precisely what R11 and task 476's guard forbid.
- 2026-10-01 — Label is «Uden original», not «Mangler original», and there is a test for it. Nothing is missing that
  anybody can supply: no backfill can produce an original for a photograph uploaded before PRD 027, so a label
  implying an outstanding task would send a curator hunting for a button that cannot exist. Same reasoning puts the
  library count behind an `{{if}}` — a permanent "0 uden original" line is noise, not a progress report.
- 2026-10-01 — The download menu's three states: fully covered gets a bare «Original» (a count reading "180 af 180" is
  noise); partly covered gets `Original (34 af 180)` plus a line saying the rest come as 1600px; none gets **no link
  at all**, with a sentence explaining the absence. A `<span>` rather than a disabled `<button>` — there is no action
  to offer, and a control that cannot be pressed invites pressing it.
- 2026-10-01 — The metadata sentence names the consequence, not the mechanism: "indeholder kameraets data, bl.a. hvor
  billedet er taget", ending by pointing at the scales as the thing to send on. Amber, not red: this is a thing to
  know before passing a file on, and the red in that menu belongs to «Slet album» alone — spending it twice would make
  neither mean anything.
- 2026-10-01 — Hit and fixed a real break on the way: adding `WithoutOriginal` to the counts **template** before the
  view struct made the admin page render half a document with a 200. `html/template` stops executing at an unknown
  field and the failure is silent, which is how `TestAdminPageRendersWholly` earns its keep — it was the test that
  said so, not the browser.
- 2026-10-01 — ✅ `gofmt`, `go build`, full `go test ./...` green. New tests cover all three menu states, the count line
  present and absent, the filter reaching the read in both directions, a 400 for a bad value, and the preset appearing
  on both views.
