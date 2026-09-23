# 391 — There is no album list, so publish and unpublish are unreachable

**Status:** open
**Priority:** high
**Created:** 2026-09-23
**Picked up by:**
**Started:**
**Completed:**

## Description

Maintainer, using the tool:

> *"I have no album list, so I can't work on albums publish/unpublish."*

Correct, and it is a gap rather than a bug. `#albumlist` exists **only inside the "Tilføj til album" panel**,
as assignment checkboxes. The album editor — `/admin/album/:slug`, which is where **Udgiv på forsiden** and
**Fjern fra forsiden** live, along with title, description, sort order and captions (task 378) — has no link
from anywhere in the tool. It is reachable only by typing a slug into the address bar.

So the whole publish half of PRD 022 §5 is built and unusable.

## What this needs

An album list on `/admin` that is navigation, not a picker:

- Every album in the selected year, **including unpublished and deleted ones** — `listAdminAlbumsHandler`
  already returns those, which is why it is on the curator interface (task 366).
- Its state visible at a glance: published / draft, and the item count. A curator's first question is "what
  have I not published yet".
- A cover thumbnail, since that is how a human recognises an album.
- A link to the editor for each one.
- Publish and unpublish **from the list**, so the common case does not need the editor at all. The endpoint
  exists: `PATCH /api/admin/albums/:albumId` with `{"published": true|false}`.
- Creating an album from here too, not only from inside the add-to-album panel.

## Notes for whoever picks this up

- `PATCH` with `published` **alone** is deliberate (task 378): a curator pressing publish has said one thing
  and must not also commit a half-typed title. Keep that — send publication by itself.
- Publication takes up to a minute to reach the frontpage. The editor says so; the list should too, or the
  curator presses it twice.
- `itemCount` counts **live** items only, so an album whose photographs were all deleted reads 0 and cannot
  have a cover. That is correct and worth rendering honestly rather than hiding.
- Depends on task 390: build this in the overlay shell rather than as a fifth inline card.
- Guard it with task 381's structural walk in mind — the list is a curator-only read and must not grow a
  person-shaped field.

## Acceptance Criteria

- [ ] Every album in the year is listed, drafts and deleted ones included and visibly marked
- [ ] Each album links to its editor
- [ ] Publish and unpublish work from the list, sending publication alone
- [ ] The item count and cover come from the same read the album page uses, so they cannot disagree
- [ ] An album with no live items renders honestly rather than being omitted
- [ ] The delay before the frontpage updates is stated
- [ ] A new album can be created from the list
