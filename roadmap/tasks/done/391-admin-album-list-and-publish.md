# 391 — There is no album list, so publish and unpublish are unreachable

**Status:** done
**Priority:** high
**Created:** 2026-09-23
**Picked up by:** agent
**Started:** 2026-09-23
**Completed:** 2026-09-23

## Description

Maintainer, using the tool:

> *"I have no album list, so I can't work on albums publish/unpublish."*

Correct, and a gap rather than a bug. `#albumlist` existed **only inside the "Tilføj til album" sheet**, as
assignment checkboxes. The album editor — `/admin/album/{slug}`, where **Udgiv på forsiden**, the title, the
description, the sort order and the captions live (task 378) — had **no link from anywhere in the tool**. It was
reachable only by typing a slug into the address bar.

So the whole publish half of PRD 022 §5 was built and unusable.

**Nothing in the suite noticed**, and the reason is worth keeping: every endpoint the feature needs was tested
directly, and tested well. A feature with no route to it is indistinguishable from a missing feature, and no
amount of endpoint coverage says anything about whether a human can reach it.

## What landed

An album list above the contact sheet — above, because it answers the question a curator opens the page with
("what have I not published yet?") while the sheet answers "what have I not sorted yet?".

Each album is a card with:

- **A cover**, because that is how a human recognises an album.
- **Its state as a badge** — Udgivet / Kladde / Slettet — and the live item count.
- **A link to the editor.** This is the point of the task; without it the list is decoration.
- **Publish and unpublish from the list**, so the common case never needs the editor. "Publish this" requires no
  editing, and making a curator open a page to press one button — when they are looking at a list that already
  says which albums are drafts — is the friction that makes a tool feel like a form.
- **A link to the public page** for a published album, so "did that work?" is one click rather than a guess.

The note above the list counts what a curator actually wants: not "5 album" but *"2 album er ikke udgivet
endnu."*

### Drafts and deleted albums are shown, not filtered

That is the entire difference between `album.CuratorQueries` and `album.Queries` (task 366): the public read
hides them so drafts cannot be enumerated, and this one shows them because hiding them is what the *other* read
is for. A test fails if the list filters either flag.

A **deleted** album gets no publish button. Restoring one is not built, and a button that would publish
something taken down is the wrong thing to offer.

### Covers go through the admin media route

Not the public one. The list shows unpublished albums, and the public route would — correctly — refuse their
photographs (task 382). A list whose draft covers were all broken images is a list a curator stops trusting.

An album with **no live items** has no cover and gets a dashed placeholder rather than a blank gap, so the row
reads as "empty album" instead of "image failed to load". `itemCount` counts live items whose photograph is
also live, so an album whose photographs were all deleted honestly reads 0 — rendered, not hidden, because "why
is this album empty" is a question with an answer the curator needs.

## Under it

`album.CuratorAlbum` gained `CoverPhotoID`, from a subquery mirroring the one `Queries.Published` already uses —
the **lowest live ordinal**, so the curator's cover and the frontpage's are the same photograph. A list whose
cover differed from the frontpage's would make a curator distrust the list.

It carries the **photograph**, where the public summary carries an ordinal. Not an inconsistency: the curator's
surfaces address photographs and the admin media route takes a photo id, while the public media route is
addressed by position within a published album. Same photograph, reached the two legitimate different ways, and
noted as such at the field.

`sql.NullString` rather than `COALESCE`, so "no cover" stays distinguishable from a photo id that is somehow
empty.

## Verified by breaking it

| Sabotage | Caught |
|---|---|
| the editor link replaced with `#` | *"must link to its editor"* |
| publication bundled with the title | *"must be sent by itself"* (task 378's rule) |
| deleted albums filtered out of the list | *"must not hide deleted albums"* |
| covers switched to the public media route | both halves of that guard |

## Verified live

- `/api/admin/albums` returns five albums with `coverPhotoId` resolved on the four that have live items, and
  **absent** on the one with `itemCount: 0`.
- Publish → unpublish round trip on the draft `test` album: 200, `published: true`, then 200 and back to draft.
- All three editor pages reachable from the list: `test` and `kladde` offer *Udgiv på forsiden*, `ved-maalet`
  offers *Fjern fra forsiden*.
- A draft album's cover serves 200 through the admin media route.
- The page renders `<h2>Album</h2>`, the container with `data-year="2026"`, and the editor links.

## Acceptance Criteria

- [x] Every album in the year is listed, drafts and deleted ones included and visibly marked
- [x] Each album links to its editor
- [x] Publish and unpublish work from the list, sending publication alone
- [x] The item count and cover come from the same read the album page uses, so they cannot disagree
- [x] An album with no live items renders honestly rather than being omitted
- [x] The delay before the frontpage updates is stated
- [x] A new album can be created from the list

## Notes

- **Task 381's privacy walk fired on `CoverPhotoID`** (the blunt `photo` needle) and was excepted by name with
  reasoning, per that file's own rule — never loosen the needle. Deliberately listed as `coverphotoid` rather
  than turning the `photoid` case into a prefix match, because a prefix would let `PhotoIdentityOf` through.
  Third time this guard has caught a legitimate field and made somebody write down why it is legitimate; that is
  the guard working, not friction.
- **Creating an album from the list uses `window.prompt`.** Crude, and deliberate: the add-to-album sheet
  already has a proper inline create form for the case that matters (creating an album *in order to* file the
  current selection into it), and a second sheet for a one-field form is not worth the markup. Worth replacing
  if the list grows any other create-shaped action.
- The count uses a singular (`1 billede`), so this card does not have the bug task 387 records on the public
  frontpage. That task is still open for the public side and the delete confirmation.
