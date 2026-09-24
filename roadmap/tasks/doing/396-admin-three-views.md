# 396 — Split /admin into an album list, an album view and an all-photos view

**Status:** doing
**Priority:** high
**Created:** 2026-09-24
**Picked up by:** Claude
**Started:** 2026-09-24
**Completed:**

## Description

Maintainer:

> *"In the /admin route we have everything in one page, this will very soon grow out of hand. We will split it
> into 3 views: an album list; a single album view, including all photos (maybe folded away for not loading
> everything); an all photos view. I would expect each year carries 10-20 albums, with 100-200 photos per
> album. And on top of that some images that does not make it into an album."*

## Scale this has to hold

| | per year |
|---|---|
| albums | 10–20 |
| photographs per album | 100–200 |
| photographs in the library | ~2,000–4,000+, since albums overlap and some are in none |

The album list is small and needs no paging. The all-photos view is the only one that must page, and the
contact sheet already does (`limit=120`, "Hent flere" as an out-of-band offset — task 395). A single album at
200 thumbnails is fine with `loading="lazy"`, which the editor already uses, so "folded away" is a paging
question, not a correctness one — see *Open questions*.

## What exists already

- **View 2 half-exists.** `/admin/album/:slug` (`adminalbumpage.go`, `adminui/album.html`, task 378) edits
  title, description, sort order, publication, item order and captions. It is a separate template with its own
  CSS and none of the contact sheet's selection or action sheets.
- **View 1 is a fragment.** `/admin/fragments/albums` already renders the list (task 391); it just lives on
  the same page as everything else.
- **View 3 is the rest of today's `/admin`**: upload, filters, contact sheet, action bar, the five sheets.

So this is mostly a re-arrangement of existing fragments across three pages, plus one new filter.

## The constraint that shapes it

PRD 022 §7: **navigating away loses the selection**, which is why the action sheets are overlays and not
routes (task 390). Three pages are now three places a selection can be lost, so:

- A selection belongs to one view and is never expected to survive navigation between them. The action bar
  says nothing that implies otherwise.
- The action bar and the five sheets must work in **both** view 2 and view 3 — a curator sorting an album
  wants "Sæt position", "Tag patrulje", "Fotokredit" and "Fjern fra albummet" there, not a trip back to the
  library. That means the sheets and `sheetshell.js` / `contactsheet.js` become shared between two pages
  instead of being duplicated into album.html. The `ctx` surface from main.js is what makes that feasible.
- In the album view, "Fjern eller slet" defaults its album picker to the album being viewed.

## Proposed shape

**Decided (2026-09-24, maintainer): the curator pages move out of `/admin` and sit beside the public pages
under the year.**

| Route | Auth | View / contents |
|---|---|---|
| `GET /<year>` | public | frontpage (unchanged) |
| `GET /<year>/album/:slug` | public | album page (unchanged) |
| `GET /<year>/patrulje/:number` | public | patrol page (unchanged) |
| `GET /<year>/albums` | admin | album list: counts, the album list fragment, "Opret nyt album", "N billeder uden album" linking to `photos?album=none` |
| `GET /<year>/album/:slug/edit` | admin | today's editor fields, then the album's photographs as a contact sheet with selection and actions; reorder and captions stay |
| `GET /<year>/photos` | admin | upload, filters, contact sheet, action bar — today's page minus the album list |

`/<year>` is the existing literal `publicRoot` prefix, so only `EVENT_YEAR` is served until task 392 and its
PRD land. httprouter accepts all of these: `/album/:slug/edit` shares the public route's wildcard name, and
`albums` / `photos` are new static siblings. The year in the **path** also settles 392's "year in the URL"
requirement for the pages, instead of `?year=`.

**Only the pages move.** `/admin/fragments/*`, `/admin/vendor/*` and `/api/admin/*` stay where they are.
They are not addresses anyone reads, and moving them would redo the OpenAPI scope (task 380) for nothing.

### Security: good enough until role-based auth replaces the shared credential

The maintainer has said basic auth is replaced by role-based authentication before next year's race, so this
only has to hold until then. What it costs to put authenticated pages under the public prefix:

1. **The guards classify by path prefix, and that stops being true.** `isAdminPath` (`admin_test.go`) treats
   `/admin*` and `/api/admin*` as the admin surface, and the privacy and visibility walks (tasks 337, 351, 382)
   treat `publicRoot + "…"` as public. `/2026/photos` would count as **public** and fail the unpublished-photo
   walk, or worse, get quietly excluded. The fix is to classify by **wrapper** (`requireAdmin` present in the
   handler expression, which `routeUsesAdminWrapper` already reads from the AST) rather than by path, and to
   do that **before** adding the routes. Role-based auth will need wrapper-based classification anyway.
2. **The browser sends the credential to the public pages as well.** Basic auth credentials are sent
   preemptively to everything at or below the challenged URL's directory. A challenge at `/2026/albums` covers
   `/2026/`, so a curator's browser sends `Authorization` on the public frontpage and album pages too. Public
   handlers ignore it, and nothing shared caches those pages with the header, so this is acceptable for now.
   It must not become a reason for a public handler to behave differently when it sees the header.
3. **`no-store` and `X-Robots-Tag` come from `requireAdmin`**, so they follow the wrapper and need no change.
   The public pages keep their own caching.
4. **No link to an admin page from a public one.** A visitor typing `/2026/photos` gets a credential prompt,
   which is fine. A visible link would be an invitation. The reverse link (edit → public) is fine and already
   exists.

- **Filters in the URL** on view 3 (`/admin/photos?album=none`), so the album list can link into a filter and a
  reload keeps it. Same reasoning task 392 gives for the year.
- **New filter `album=<id>`** on `PhotoCurator.Library` / the photos fragment, so the album view's grid is the
  same fragment as view 3 rather than a second renderer. Today only `album=none` is accepted
  (`adminlibrary.go:214`). Ordering is the album's ordinal, not newest-first, when filtered to one album.
- **Upload lives in view 3.** Uploading into an album directly is a reasonable follow-up, not this task.
- **A shared header/nav** across the three pages. adminalbumpage.go's comment already says *"if a third admin
  page appears, that is the moment to extract a shared head"* — this is that moment.
- Keep the one-script-per-page injection (task 395): each page injects only the `init…` files it uses, and
  `TestEveryAdminContextMemberIsProvided` must hold per page, not just for the union.

## Captions moved to the action bar

The editor's one caption field per photograph did not survive the move to the shared grid, and would not have
scaled to 200 items anyway. Captions are now a **"Billedtekst"** action on the selection, in both views; with one
photograph selected the field is prefilled with its caption. The ↑/↓ reorder buttons went with that list, so the
album view has **no reordering until drag-and-drop lands** — the next piece of this task.

## Deleting an album

**Added (2026-09-24, maintainer):** *"it should be possible to delete an album (remember a confirm prompt)"*.

From the album list, behind `hx-confirm`. It publishes the existing `album.Deleted` (the Team section's in-app
takedown uses the same one), which takes the album off the frontpage and marks its items removed so the map
drops them. No photograph is deleted, and the prompt says so.

## Interaction with task 392 (year selector)

392 puts the year in the URL (`?year=`). All three views and every link between them must carry it. Whichever
lands second pays for it; landing this first means 392 has three pages to thread instead of one, but each is
simpler. Either order works — just not both at once.

## Open questions

**All three resolved (2026-09-24, maintainer):**

- **Infinite scroll**, in both the album view and the all-photos view. The last cell of each page carries
  `hx-trigger="revealed"` and fetches the next one, which replaces the "Hent flere" button. The next offset
  still comes from the server (task 395), and "Vælg alle der matcher filteret" still pages the JSON endpoint
  for ids, so selecting everything does not depend on what has been scrolled into view.
- **An explicit cover, "Gør til forsidebillede", on any photograph in the album.** It no longer has to be the
  first one. This **reverses** today's rule that the cover is the first live item (`adminalbum.go:255`,
  `adminAlbumPageHandler`, and the public read), so it needs a cover field on the album and an event that sets
  it, rather than a reorder. If the chosen cover is removed from the album or deleted from the library, fall
  back to the first live item so a published album never loses its cover image. The public album page and the
  frontpage cover must use the same rule, in one place.
- **Reordering is drag and drop of a multi-selection.** The ↑/↓ buttons go. The curator selects one or more
  photographs (click, shift-click range, which the contact sheet already has) and drags them to a new position,
  and they land there together in their current relative order. The existing
  `PATCH /api/admin/albums/:albumId/items` already takes the whole new order as one event, so the server side
  does not change: the page computes the new list and sends it. With infinite scroll, a drop target may be
  beyond what is loaded, so the order sent must be the album's **full** live order, not just the loaded cells
  (fetch the ids first, or have the server take "move these ids before this id" instead). Decide which in
  implementation, and test a move from the end of a 200-item album to the start.

The original questions, for the record:

1. **"Folded away" in the album view** — paged like view 3 (120 + "Hent flere"), or all items rendered with
   lazy thumbnails? Recommendation: page it with the same fragment. 200 lazy thumbnails is fine; 200 caption
   inputs and move buttons in one DOM is where it starts to feel heavy, and reuse is free.
2. **Reordering at 200 items** — the ↑/↓ buttons do not scale to moving a photo from position 180 to 1. Is a
   "gør til forsidebillede" (move to first) action enough for now, with drag-and-drop later?
3. ~~Does `/admin` land on the album list?~~ **Resolved:** the pages move to `/<year>/…`. No redirects from `/admin`:
   the tool has never been live, so there are no bookmarks to keep (maintainer, 2026-09-24).
   Task 385's half-page should still be updated to say `/<year>/photos` for uploading.

## Acceptance Criteria

- [x] Route guards classify admin vs public by the `requireAdmin` wrapper, not by path — landed first
- [x] `/<year>/albums` shows the album list, the counts, and a link to photographs without an album
- [x] `/<year>/album/:slug/edit` shows the editor and the album's photographs with selection and all actions
- [x] `/<year>/photos` shows upload, filters and the contact sheet; filters are in the URL
- [x] An album can be deleted from the list, behind a confirm prompt that names it and says the photographs stay
- [ ] No public page links to an admin page
- [x] `album=<id>` filter on the library read and the photos fragment, ordered by ordinal, with tests
- [x] Action sheets and selection are shared code between views 2 and 3, not copies
- [x] A shared head/nav across the three pages
- [ ] Verified with a year-sized fixture (~20 albums, ~3,000 photos): no view loads more than one page of
      thumbnails up front
- [ ] PRD 022 §7 and task 385's half-page updated for the new URLs
- [x] The open questions above are answered in this file before implementation
- [ ] Infinite scroll in the album and all-photos views; "select all matching" still selects unloaded ones
- [ ] An explicit album cover settable on any item, falling back to the first live item, used by every read
- [ ] Multi-select drag-and-drop reordering replaces ↑/↓, verified moving items from position ~180 to 1
