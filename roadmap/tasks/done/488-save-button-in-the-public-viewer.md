# 488 — A save button in the public photo viewer

**Status:** done
**Priority:** medium
**Created:** 2026-10-01
**Picked up by:** agent session
**Started:** 2026-10-01
**Completed:** 2026-10-01

**Follows:** task 405 (share), PRD 023 §7.7 (the action registry), PRD 027 (what an original is)

## Description

Maintainer: *"in the public album viewer, next to share button top right corner, there should be a download button that
allows user to download a fair size version of current photo."*

A family looking at a photograph of their own patrol wants to keep it. Until now the only way was long-press or
right-click, which works and which nobody thinks of while holding a phone in one hand at a prize ceremony.

**"A fair size version" is the display image** — `item.full`, the 1600px rendition already on screen. A few hundred
kilobytes, enough for a postcard-sized print or an attachment, already stripped of metadata by the ingest pipeline,
and already in the browser's cache, so on a cache hit the save costs no transfer at all.

**Not the photographer's file.** That copy keeps the camera's metadata including where the photograph was taken, and
PRD 027 R10/R11 put it behind the curator's credential. `viewer.js` is deliberately **one copy** shared by the public
album page and the admin tool (PRD 023 §9), so a variant added here for a curator's convenience would ship to the open
web the same afternoon — which is why `originalboundary_test.go` fails if this file so much as names one.

## The thing worth a second thought, and it was raised rather than assumed

The share control's own comment takes a position this appears to contradict:

> `navigator.share` accepts `files`, and this deliberately never passes any. […] a shared **link** stops working when a
> photograph is taken down, and a shared **JPEG** does not. […] Somebody who wants the file can save it from the
> photograph; we are simply not the ones handing out copies that outlive a takedown.

That rule stands, and this is why the save button is a **separate control** rather than files added to the share sheet.
The distinction:

- **Share** is us handing a copy to a third party on somebody's behalf, into a group chat. Still a link, never bytes.
- **Save** is the visitor acting on the photograph in front of them — which the same comment already conceded is
  possible ("somebody who wants the file can save it from the photograph"). This removes friction from an existing
  capability; it does not create one.

A takedown still cannot recall a copy somebody saved, and it never could. What a takedown governs is what we **serve**,
and that is unchanged. The share comment has been amended so it does not read as contradicted by the control beside it.

## Requirements

- [x] R1 — A save control in the viewer's action row, **immediately after share**, before fullscreen. Icon:
      Lucide `cloud-download` (changed from `download` on 2026-10-02; see the log).
- [x] R2 — It saves `item.full`, the display image. Never a thumbnail, never the 800px rendition, and never the
      photographer's file.
- [x] R3 — Declared by the **public album page only**. The curator's tool does not get it: it already has the album as
      a zip at four sizes and the photographer's file besides, so this would be the least useful of five ways out of
      that page.
- [x] R4 — An `<a download>`, not a fetch-into-a-blob: the browser reuses the cached response the `<img>` already has.
- [x] R5 — The filename names the album, derived from the permalink's album segment. **Never `data-filename`**, which
      is the photographer's own filename and exists only behind the credential (task 475) — on the public page it is
      empty, so a filename built from it would work in the admin tool and vanish in production.
- [x] R6 — Nothing about what the server serves changes. No new route, no new query parameter, no new cache entry.

## Acceptance Criteria

- [x] The public album page declares `share,download,fullscreen` in that order
- [x] The viewer registers the control before it can open (task 415's bug, which applies to every new action)
- [x] A guard asserts it saves the display image and not a smaller rendition
- [x] A guard asserts it issues no request of its own
- [x] A guard asserts the anchor is connected before it is clicked — Firefox ignores a click on a disconnected one
- [x] A guard asserts the filename ignores the photographer's filename
- [x] A guard asserts the curator's tool does not declare it
- [x] The public action allowlist is widened **deliberately**, with the reason recorded
- [x] Full gate clean: `gofmt`, `go vet`, `GOWORK=off go test ./...`

## Progress Log

- 2026-10-01 — Task created and implemented.
- 2026-10-01 — Icon is Lucide `download`, deliberately the mirror of the existing `share` glyph: same box, arrow
  reversed. The two sit side by side and the pair reads as "out of here" and "onto my phone" without either label
  being read.
- 2026-10-01 — Registered **unconditionally**, unlike share. Share sits behind a feature gate because
  `navigator.share` and `navigator.clipboard` may both be missing and an inert button is worse than none; an
  `<a download>` has nothing to detect, and a browser that ignored the attribute would *open* the photograph — worse
  than a save, still not broken.
- 2026-10-01 — Filename is `<album>-<ordinal>.jpg`. The ordinal is **a label on a copy, not an address**: it moves
  when the album is re-sorted, which mattered for the permalink (task 447) and does not matter here, because nobody
  resolves a filename — and it is the number the visitor can see, so two saved files are told apart the way they were
  seen on screen. Falls back to a crudely slugified album title when the host page supplies no permalink; deliberately
  cruder than the server's slug rules, because a second slug implementation that *nearly* matches is worse than an
  obviously different one.
- 2026-10-01 — **Walked straight into the backtick trap** the `go-server-rendered-pages` skill warns about:
  `publicsite.go` holds its template in a Go raw string, and a backtick in my new template comment terminated the
  literal. The symptom is exactly as documented — a Go syntax error pointing at a line of HTML. Four incidents in one
  session before task 394 extracted the admin tool; this is the fifth, in the one file that was not extracted.
- 2026-10-01 — The public page's **action allowlist caught the change**, which is what it is for: adding `download` to
  `data-viewer-actions` failed `TestTheAlbumPageWiresTheViewer` until the allowlist was widened on purpose. Widened
  with a note that the three permitted actions are all **reads**, and that anything which writes still belongs behind
  the credential — so the next widening has to say which of the two it is.
- 2026-10-01 — ✅ All five new guards checked against their own failure: pointed the download at `item.medium`, removed
  the appendChild, and declared `download` on the admin page. Each failed with its own explanation, then reverted.
- 2026-10-01 — ✅ `gofmt`, `go vet`, full `go test ./...` green, including `originalboundary_test.go` — the viewer still
  names no original.
- 2026-10-02 — **Icon changed to Lucide `cloud-download` on the maintainer's instruction**, replacing the `download`
  tray glyph the entry above argues for. The earlier reasoning is left standing because this log is append-only, and
  because it was wrong in an interesting way: I picked the tray because it *mirrors* `share` — same box, arrow
  reversed — and treated that as the virtue. It is the defect. Two controls differing only in the direction of a small
  arrow, side by side, on a phone, at arm's length, and the cost of confusing them is **sharing a photograph of
  somebody's child when you meant to keep it**. That is not a symmetrical mistake, so the pair should differ by
  silhouette rather than by arrow. A cloud does. It also says where the bytes come from, which is what the action
  does.
- 2026-10-02 — Path data copied from `lucide-icons/lucide` rather than written from memory: `cloud-download` was last
  changed in 0.421.0 and the older drawing is still what most snippets show, so a remembered version would have been
  a different icon wearing the right name.
- 2026-10-02 — Added `TestEveryRegisteredIconExistsInTheIconTable`, because this change created a hazard that did not
  exist before. `icon()` is string concatenation on `ICONS[name]`, so a name that is not a key yields the literal
  "undefined" inside an `<svg>` — which renders as **an empty button**: correctly sized, correctly labelled,
  focusable, invisible, and silent in every log and every Go test. That was theoretical while every key was one
  lowercase word; `cloud-download` is the first key with a hyphen, so it must be quoted in the table *and* in the
  registration, which is two spellings that have to agree where there was one. Verified by misspelling it.
- 2026-10-02 — ✅ Full gate this time, **including `staticcheck`**, which I had been omitting across this and the four
  preceding tasks — see the container-log diagnosis in the session: the dev loop runs it as a gate before `go build`,
  so a finding there leaves the API not running at all. It is clean, but that was luck rather than process.
