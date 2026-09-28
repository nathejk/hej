# PRD 024 — Album sort order

**Status:** done
**Author:** agent session (2026-09-28)
**Created:** 2026-09-28
**Last updated:** 2026-09-28 (shipped; tasks 439–446 and 448 all done)
**Approved:** 2026-09-28
**Shipped:** 2026-09-28
**Target users:** organizer (the 2–3 photographers/curators who use `/admin`)

---

## 1. Summary

An album gets a **sort order** the curator chooses — capture time or filename, either direction, or
manual. A non-manual album keeps itself in that order as photographs are added to it. Dragging a
photograph switches the album to manual, after a confirmation if it was not manual already.

## 2. Problem & Motivation

**What problem does this solve?** A curator filing a card into an album gets it in the order the
photographs happened to be added, which is neither capture order nor filename order. Today the only way
to an ordered album is to drag every photograph into place by hand — for a 200-photograph album, from an
order that is close to arbitrary. The common case ("chronological") is the one the tool makes hardest.

**Why now?** The album editor and drag-and-drop reordering have just landed (tasks 396, 435, 436) and the
manual path works. This is the other half: most albums do not want a hand-made order, they want a rule,
and the curator should only be arranging by hand where the arrangement is actually editorial.

**Evidence.** Requested directly by the maintainer (2026-09-28), with the interaction already specified:
five modes, drag switches to manual, confirm if not already manual, and the order is applied when
photographs arrive.

## 3. Goals

- A curator can put an album in capture-time or filename order, either direction, without dragging.
- An album in a chosen order **stays** in it as photographs are added, with no further action.
- A hand-made arrangement is never silently destroyed: switching away from manual is something the
  curator did on purpose, and switching *to* manual is confirmed.
- The public album page shows the same order the curator sees, with no change to any public read.

## 4. Non-Goals

- **Sorting the contact sheet / library.** It is newest-first for a stated reason (`CuratorQueries.Library`
  — "the curator is looking for what they just uploaded") and is not an editorial sequence.
- **Per-album sort on the public frontpage's album list.** That is `album.sortOrder`, an existing and
  unrelated field (the editorial sequence of albums, not of photographs).
- **Sorting by caption, credit, patrol, position or album membership.** No request, and each needs its own
  answer for what an empty value sorts as.
- **A migration that re-sorts existing albums.** See §6 R9.
- **A permalink to a photograph within an album.** A resort changes which photograph is at a position, and
  the maintainer's judgement is that today's link is honest about that (§11 Q3) — a position link means "the
  twelfth photograph", and after a resort that is a different photograph, truthfully. A durable link to *a
  particular photograph* is a real want and a separate piece of work; tracked on the board rather than here.
- **Backfilling capture time or filename for photographs already in the library.** They keep the fallback
  described in §6 R3/R5. A backfill is possible later (the originals are gone, but see §11 Q4) and is not
  needed for the feature to be useful on the next event.

## 5. User Stories & Scenarios

- As a **curator**, I want an album in the order the photographs were taken, so that it reads as the night
  did rather than as the order I happened to click.
- As a **curator**, I want to drop a second card into an album that is already in time order and have it
  interleave correctly, so that "add more photographs" is not "re-sort the album by hand".
- As a **curator**, I want to hand-arrange an album whose order is editorial, and be sure that adding one
  more photograph later will not throw my arrangement away.

**Happy path.** The curator opens an album (`/admin/albums/{id}`), picks *"Tid, ældste først"* from the
sort control in the editor card. The grid reorders immediately and the album is saved in that mode. They go
to the library, select 40 photographs from a second card, "Læg i album". Back on the album, the 40 are
interleaved in capture order among the ones that were already there. The public album page shows the same
sequence.

**Switching to manual.** The album is in *"Tid, ældste først"*. The curator drags one photograph three
places to the left. A confirmation appears: this album is sorted by time; moving a photograph by hand
switches it to manual order, and new photographs will then be added at the end. They confirm; the move
applies and the control reads *"Manuel"*. A second drag needs no confirmation.

**Declining.** They cancel the confirmation. Nothing moves, the mode is unchanged, and the grid is exactly
as it was — the drop is abandoned, not applied-then-reverted.

**Edge cases**

- **A photograph with no capture time.** Sorts as if taken at its upload time (§6 R3), which for a card
  uploaded the same day puts it among its neighbours rather than at one end.
- **A photograph with no filename** (uploaded through the raw-body path, which the drop zone does not
  use): sorts as `""`, which is first ascending. Rare and documented rather than special-cased.
- **Ties.** Two photographs from a burst can share a capture time to the second, and two cards can hold
  the same filename. Ties break on `photoId` — the same rule the library read already uses, and the
  property that makes the order deterministic rather than "whatever MariaDB did this time".
- **An album whose every photograph lacks both keys** sorts stably and looks unchanged. Correct, and worth
  saying out loud so it is not reported as a bug.
- **Removing a photograph** from a non-manual album leaves a gap in the ordinals and no re-sort. Removal
  does not change relative order, so there is nothing to recompute.

## 6. Requirements

### Functional

- [x] **R1 — Five modes.** `manual`, `time-asc`, `time-desc`, `filename-asc`, `filename-desc`. Stored on
      the album. Danish labels: *Manuel*, *Tid, ældste først*, *Tid, nyeste først*, *Filnavn A–Å*,
      *Filnavn Å–A*.
- [x] **R2 — The sort produces ordinals; it is not a read-time order.** Choosing a mode, or adding to a
      non-manual album, recomputes `album_item.ordinal` for the album's live items and publishes one
      `itemsreordered` event — the mechanism the manual reorder already uses. See §8 D1 for why.
- [x] **R3 — Capture time.** A new `shotAt` on the photograph, read from EXIF `DateTimeOriginal` **before**
      re-encoding, exactly as the GPS fix already is. Null when absent; the sort then falls back to
      `uploadedAt` for that photograph.
- [x] **R4 — `time-asc` / `time-desc`** sort on capture time (with the R3 fallback), ties on `photoId`.
- [x] **R5 — Filename.** `photo.fileName`, taken from the multipart part's filename, which the upload used to
      read and discard. Empty on the raw-body path. Stored as given, never parsed for meaning, normalised by
      `photo.NormalizeFileName` (basename, valid UTF-8, printable, 255 runes) both at publication and in the
      fold.

      **It required a decision, taken in task 448.** PRD 022 §6 admits no personal data to the library
      projection, and two privacy guards refused the column when task 441 first wrote it — correctly, since a
      filename can carry a name. The maintainer's decision (2026-09-28): keep it. A photographer sorts a card
      on their own computer and the album should keep that order; the field is behind the admin credential, it
      is a sort key, and "name will never be used". PRD 022 §6 now records this as its second written-down
      exception, with the bounds it rests on.

      The bound that is **enforced rather than promised**: `isPersonShaped` still flags the word, so any
      public response that grows a filename fails the public guard. The exception is scoped to the
      authenticated surface and `TestTheFilenameExceptionStopsAtTheAdminSurface` fails if it is widened.
- [x] **R6 — `filename-asc` / `filename-desc`** sort case-insensitively on the filename, ties on
      `photoId`. Case-insensitive because `IMG_*.JPG` and `img_*.jpg` from two cameras in one album
      otherwise separate into two blocks.
- [x] **R7 — Adding photographs applies the mode.** When items are added to an album whose mode is not
      `manual`, the album's whole live order is recomputed. (The maintainer's "applied when uploading new
      photos" — an upload lands in the *library*, and an album gains photographs through "Læg i album",
      which is the moment this has to happen. See §11 Q1.)
- [x] **R8 — A hand move switches the album to manual.** If the mode is not `manual`, the admin tool
      confirms first, and the server **refuses a move on a non-manual album** (409) rather than trusting
      the client to have asked. Declining changes nothing.
- [x] **R9 — New albums are `time-asc`; albums that already exist stay `manual`.** The maintainer's answer
      to §11 Q2 was "default to time-asc, do not think about old albums", and the cheapest way to make that
      true is to put the default in **two different places**, which is worth spelling out because it looks
      redundant:
      - the **column** defaults to `manual`, the value that means "never recompute";
      - the **create handler** sets `time-asc` on the `Created` event.

      So a new album is time-ordered because somebody's code chose that, and every album that exists today
      keeps its arrangement because the column's default is the inert value. Defaulting the column itself to
      `time-asc` would have been one line shorter and would have re-sorted every existing album the first
      time anybody added a photograph to it — and, because there is no backfill (§11 Q4), re-sorted it on the
      `uploadedAt` fallback, i.e. shuffled a hand-arranged album into roughly upload order. That is the one
      irreversible thing this feature could do, and this is what makes it impossible rather than unlikely.
      No migration, and nobody has to think about old albums.
- [x] **R10 — The mode is visible** in the album editor card, and the grid reflects a change immediately
      rather than on the next reload.

### Non-Functional

- **Privacy — the filename needed a decision, and got one.** A camera filename is usually `IMG_0123.JPG`, but
  it is free text from somebody's computer and *can* name a person (`mor-og-far.jpg`). This PRD originally
  noted only that it must never reach a **public** read; that was an understatement, because PRD 022 §6
  forbids personal data **in the projection at all** — enforced by
  `TestNoStructInTheLibraryOrTheAdminToolNamesAPerson` and
  `TestTheLibraryTablesDeclareNoPersonShapedColumn`, which is where task 441's first attempt stopped.
  Task 448 decided to keep it, PRD 022 §6 now records it as a second written-down exception with its bounds,
  and the exception is **scoped to the authenticated surface** so that the public guard still refuses it. The
  lesson worth keeping: the privacy posture here is about the **column**, not the response, because a column
  is in every backup taken since it was added.
- **Order stability is part of the contract.** Every mode must be a total order, ties included, so that two
  recomputations of an unchanged album produce an unchanged sequence. Without that, an add would reshuffle
  unrelated photographs and the public page would change for no reason a curator could explain.
- **Cost.** A recompute is one event carrying the album's live order, at most a few hundred ids — the same
  shape and size as a manual full reorder, which is already the normal case for drag-and-drop.
- **Accessibility.** The sort control is a native `<select>`, so it needs no keyboard handling of its own.
  The confirmation must be reachable and dismissable by keyboard (Escape), like the tool's other overlays.

## 7. UX / UI Notes

**Not the Vue PWA.** This is entirely in the server-rendered admin tool (`go/cmd/api/adminui/`) — Go
`html/template` with vendored Pico, htmx and Alpine, no build step. See the `go-server-rendered-pages`
skill. Nothing in `vue/` changes.

- **The control** sits in the album editor card (`#albumeditor`, `albumeditor.js`), with the title,
  description, cover and publish state — it is a property of the album, and that card is where an album's
  properties are. A native `<select>` with the five options, saved on change through the existing album
  PATCH, followed by the sheet reload the card already does.
- **The confirmation** is the tool's existing overlay shape (`sheetshell.js`), not `window.confirm`. Its
  wording is the feature, not decoration — PRD 022 §5's standard. It has to say three things: this album is
  sorted by *X*; moving a photograph by hand switches it to manual; new photographs will then be added at
  the end. Two buttons: *"Flyt og skift til manuel"* and *"Annuller"*.
- **The confirmation appears at the end of the drag, not the start.** Interrupting a pointer gesture with a
  dialog is how a drag gets abandoned by accident, and the curator may well drop where they started. The
  drag's gap and floating stack (tasks 435, 436) behave exactly as now; the confirmation replaces the
  request that would otherwise have gone out on release.
- **After a mode change the grid reorders on screen**, via the sheet reload the editor card already
  triggers — with the projection wait from task 438, since a recompute is a publish like any other.

## 8. Technical Considerations

**BFF (Go)** — all of it. No frontend (`vue/`) work.

**D1 — Why the sort materialises into ordinals instead of being a read-time `ORDER BY`.** This is the
central decision.

- `album_item.ordinal` is already the order **both** surfaces read: the curator's grid
  (`Library` with `AlbumID`) and the public album page (`KEY album_order`). A read-time sort would have to
  be taught to both, and PRD 023's paging and filmstrip read positions too — three places to keep in step,
  where one of them being wrong shows up as a public page in a different order from the curator's.
- `VerbItemsReordered` already exists and already means "here is the album's whole order", because
  `album_item` is keyed on `(albumId, ordinal)` and a reorder has to move rows. Sorting is exactly that
  event with the list computed by a rule instead of by a pointer.
- It makes `manual` **cost nothing and change nothing**: the mode is a rule for *when* to recompute, and
  manual means never.
- The trade is that the order is only as fresh as the last recompute. That is the right trade here: the
  inputs (capture time, filename) are immutable once a photograph is in the library, so the only thing that
  can invalidate an order is a membership change — which is precisely when R7 recomputes.

**Data / storage**

- `album.sortMode VARCHAR(16) NOT NULL DEFAULT 'manual'` — with the value list in the column comment, as
  `boundsVerdict` does.
- `photo.shotAt DATETIME NULL DEFAULT NULL` — null means "the file did not say".
- `photo.fileName VARCHAR(255) NOT NULL DEFAULT ""`.
- Event fields to match: `album.Created.SortMode`, `album.Updated.SortMode *string`,
  `photo.Uploaded.ShotAt` / `.FileName`. Additive and optional, so replaying an older log is unaffected —
  the projections fold to the defaults above.
- **A new index.** Sorting reads the album's items joined to their photographs; the existing
  `photo_albums (year, photoId, deleted)` covers the lookup direction, and `album_order` covers the album's
  items. Worth checking the plan for the recompute read before adding anything — a few hundred rows may not
  need it.

**EXIF.** The repo reads EXIF with its own code (`internal/imaging`: `ReadOrientation`, `ReadGPS`,
`findExifSegment`, `ifdEntry`, `ifdLongTag`) and has **no EXIF dependency**. `DateTimeOriginal` is tag
`0x9003` in the Exif sub-IFD at `0x8769` — the same sub-IFD-pointer shape `ReadGPS` already walks, so this
is a `ReadShotAt` next to it rather than a new dependency. It is an ASCII `YYYY:MM:DD HH:MM:SS` with no
timezone; parsed as the event's local time, since a Nathejk photograph was taken at Nathejk. Malformed or
absent → not ok, and R3's fallback applies. `internal/imaging` has a fuzz test (`fuzz_test.go`) that the new
reader must be added to: this is attacker-adjacent parsing of bytes from a file somebody handed us.

**API endpoints** — no new routes. Three changed, all of which **already have OpenAPI annotations that must
be updated**:

- `PATCH /api/admin/albums/{albumId}` — accepts `sortMode`. Rejects an unknown value (400) rather than
  ignoring it. Setting a non-manual mode recomputes the order.
- `POST /api/admin/albums/{albumId}/items` (add) — applies R7. The response should say whether it
  re-sorted, so the tool's note can say so.
- `PATCH /api/admin/albums/{albumId}/move` — **409 on a non-manual album** (R8). This is the endpoint that
  makes the rule true regardless of the client.

`GET /api/admin/albums/{albumId}` and the album fragments gain `sortMode` so the card can render it.

**Dependencies & risks**

- **Ordinals change under a resort, and that is accepted** (§11 Q3): a position link means "the twelfth
  photograph", and after a resort that is a different photograph — which is true rather than broken. What
  must still be checked before this ships is that nothing *stores* an ordinal expecting it to be stable;
  PRD 023's paging reads by offset, which is fine. A durable per-photograph permalink is separate work.
- **A recompute is a publish**, so the tool must wait for the projection before reloading, exactly as the
  uploader now does (task 438). Adding a third caller makes the shared `ctx.settled(ids)` helper that
  tasks 437/438 both noted worth doing first.
- **Two curators, one album.** Curator A re-sorts while B adds: both publish a full order, last write wins,
  and the loser's intent is not lost because both orders are derived from the same rule. Only a manual
  arrangement can actually be clobbered, and only by another manual arrangement — which is true today.
- **No migration risk.** All three columns default to what the current behaviour already is.

## 9. Success Metrics

This is a tool used by three people, so the honest metrics are observational rather than numeric:

- A curator can produce a chronological album of a card without dragging anything. Verifiable directly.
- No album's order changes as a result of shipping this. Verifiable before/after on the 2026 data.
- A hand-arranged album that later gains photographs keeps its arrangement (they append). Verifiable.
- Nobody reports "the album re-shuffled itself" — the failure this feature can plausibly introduce.

## 10. Rollout / Task Breakdown

Sequenced so that nothing user-visible ships before the data it needs exists. Tasks 1–3 are independently
useful and carry no interaction change; a curator sees nothing until task 5.

Created on the board 2026-09-28, in dependency order:

- [ ] Task 439: `ctx.settled(ids)` on the admin context, with the uploader moved onto it *(prerequisite for
      444 — a resort's reload has the same race, and this stops it being a third copy)*
- [x] Task 440: read `DateTimeOriginal` in `internal/imaging`, with fuzz coverage (`ReadShotAt`) — **done**
- [x] Task 441: capture `shotAt` on upload — event field, column, fold, curator read. The `fileName` half was
      refused by the privacy guards and became task 448, which **decided to keep it**; both fields now ship
- [x] Task 442: `album.sortMode` — column, event fields, fold, and the curator read — **done**
- [x] Task 443: the sort itself — one function from (items, photographs, mode) to an order, ties on
      `photoId`, table-driven tests per mode including empty keys
- [x] Task 444: apply the mode on `PATCH album` (mode change) and on add-to-album; OpenAPI updated
- [x] Task 445: refuse a move on a non-manual album (409) and the confirm-then-switch flow in the tool
- [x] Task 446: the sort control in the album editor card
- [x] Task 448: decide whether the library may store a filename — **decided: yes, with bounds** (PRD 022 §6)

Out of scope, on the board because PRD 024 is what raised it:

- [ ] Task 447: a durable link to one photograph in an album (§4, §11 Q3) — needs a decision about what a
      public photograph's address is before any code

## 11. Open Questions

All five were answered by the maintainer on 2026-09-28; kept with their answers, because the reasoning is
what the requirements above rest on.

1. **"Applied when uploading new photos" — confirming the reading.** ✅ *"I meant 'applied when added to
   album'."* §6 R7 as written. No uploader on the album view.
2. **What should a new album default to?** ✅ *"default to time-asc, do not think about old albums."* See R9
   — new albums are created `time-asc`, and the column's default stays `manual` so that "do not think about
   old albums" is safe rather than merely unexamined.
3. **Does anything persist an ordinal as an address?** ✅ *"link today is honest. but we might need to think
   about a permalink solution."* Accepted as-is; a per-photograph permalink is out of scope (§4) and on the
   board as its own task.
4. **Is a backfill wanted later?** ✅ *"don't care about backfill."* Photographs already in the library sort
   on the `uploadedAt` fallback forever. No backfill task.
5. **Should `filename-*` be offered in year one,** given Q4? Kept — reopened by task 441 as a privacy
   question and settled by task 448: the library may hold the filename, behind the credential and as a sort
   key only. ✅ *"do persist filename, path is not public, it's protected by authentication — filename is used
   for sorting on local computer and we want to be able to keep that sorting after upload. this interface is
   intended for photographers — name will never be used."* That last clause is the bound the exception rests
   on, and it is why the modes exist at all: the photographer's own ordering is the thing being preserved.

---

## 12. What shipping it taught us

Added on completion, because four of these were found by building it and none of them were in the spec.

1. **The privacy posture here is about the column, not the response.** §6's first draft said the filename must
   never reach a *public read*. Both privacy guards refused the column anyway, and they were right: PRD 022 §6
   forbids personal data in the projection, because a column is in every backup taken since it was added. The
   decision (task 448) kept the field with bounds, and the bound that matters is **enforced by scoping the
   exception** to the authenticated surface rather than by promising it.

2. **An unrecognised sort mode must leave the album alone.** `sortMode` is a column, so a binary will meet a
   value a newer one wrote. The first draft gave unknown modes an empty sort key, which made every photograph
   compare equal and left the `photoId` tiebreak to reorder a curated album **by content hash** on rollback.

3. **"Nothing stated" cannot mean "sorts itself".** `a.SortMode != SortModeManual` reads correctly and makes
   `""` automatic, so an album with no mode would refuse a hand move and re-sort itself on every addition — the
   safest input producing the least safe behaviour. `album.SortModeOr` is the single answer now.

4. **Waiting for photographs is not waiting for an order.** `ctx.settled` (task 438) proves photographs exist;
   after a re-sort they already did, so a grid reloaded on that signal shows the old order. The server had to
   answer with the order it published (`resortedOrder`) before the tool could wait for anything meaningful.

5. **A shared template makes a missing field a silent truncation.** The editor card is rendered from both the
   album page's data and the list fragment's. Adding `SortMode` to one produced a page that stopped executing
   mid-card and returned 200. Caught by an existing view test, not by any of the new ones.
