# 379 — Delete from the library, and say plainly how that differs from removing from an album

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent
**Started:** 2026-09-23
**Completed:** 2026-09-23

## Description

`DELETE /api/admin/photos/:photoId` — a **soft** delete with an optional reason, publishing
`photo.deleted`. The photograph then disappears from every album and every public read, and its blobs
are purged unless another row, **including a glimt**, still references them (task 368's shared purge
helper).

Soft rather than destructive, for the reason `album/table.sql` already records: a removal here honours
somebody's objection, and an accidental one should be recoverable without republishing a photograph
that was taken down on purpose. The capability exists in the log even though v1 ships no undelete
button (PRD 022 §6, §11 Q6) — if a curator deletes 40 photographs with a mis-aimed "select all", the
only recovery is a maintainer, and that is a choice being made rather than an oversight.

**The copy is the substance of this task, not decoration.** PRD 022 §5 and §7 both single it out:
removing a photograph from an album leaves it in the library and in every other album, while deleting
it from the library removes it from all of them — and *one of those two is the thing an organizer means
when they say "take it down"*. If the two buttons read alike, the wrong one gets pressed under exactly
the pressure that makes it matter. Write these sentences in Danish, carefully, rather than generating
them. This also carries more weight than it looks because §11 Q2 resolved that photographs are **never
purged automatically** — the soft delete is now the *only* way a photograph ever leaves.

Note the second door: the existing in-app takedown endpoints in `go/cmd/api/albumremove.go`, gated on
`isGlimtModerator`, **stay exactly as they are**. They answer a different question — "somebody
complained and I am at a barbecue with my phone" — and two doors to the same removal is correct here.
What must not happen is the two disagreeing about blob purging (PRD 022 §8.9).

## Acceptance Criteria

- [x] A delete is soft, records the optional reason on the event, and survives a replay
- [x] The photograph vanishes from every album and every public read immediately after the fold
- [x] Blobs are purged only when nothing else — album, photo or glimt — references them, and any error
      means nothing is deleted
- [x] Delete works on a multi-selection from the contact sheet
- [x] The two Danish sentences distinguishing "fjern fra album" from "slet fra biblioteket" are written
      and reviewed, and the destructive action is the visually distinct one
- [x] The in-app removal endpoints are unchanged and share the purge helper — tested from both sides
- [x] A re-upload of deleted bytes does not resurrect the row (task 372)
- [x] OpenAPI annotations with `@Failure 401`

## What was done

Two endpoints, deliberately separate, plus the shared purge that task 368 deferred here.

**`DELETE /api/admin/albums/:albumId/items/:photoId`** — takes one photograph out of one album and
**purges nothing**. Addressed by photograph rather than by ordinal even though `album_item` is keyed on
the ordinal: the curator selected a photograph, and a stale ordinal in a browser tab would remove
whatever now occupies that slot. The handler resolves photograph → ordinal itself.

**`DELETE /api/admin/photos/:photoId`** — the takedown. Soft, optional reason on the event, then the
blob purge. The photograph's own row is **excluded** from the sharing check, because the fold is
asynchronous and the row is still live at the moment the question is asked; without the exclusion the
check looks entirely correct and never frees a byte. Event first, bytes second — a failed publish after
a purge would leave a row whose objects are gone and anything sharing them blank permanently.

**`blobpurge.go` is new, and is the whole point of the task.** PRD 022 §8.9's requirement is that two
doors is correct but the two disagreeing about purging is not, and the way two copies disagree is not by
being written differently — it is by one of them not being updated when a *third* owner of the blob
store appears. So the union of owners is asked in exactly one place (`blobRefsInUse`), `glimtdelete.go`
now delegates to it, and `TestBothDeletePathsShareOnePurgeHelper` fails if either path grows its own
loop or calls `app.blobs.Delete` directly.

The rule that matters more than the mechanism, recorded in the file header: **if any owner cannot be
asked, nothing is deleted.** A failed query is not "assume unused". Leaking disk is recoverable and
visible in a graph; blanking somebody else's photograph is neither. Nil and error are therefore
different answers — a nil projection has nothing to protect, a failing read means we cannot tell.

### The copy

> **Fjern fra et album** — Billederne bliver taget ud af det album du vælger. **De bliver liggende i
> arkivet** og i de andre album de ligger i, og du kan lægge dem tilbage når som helst.
>
> **Slet fra arkivet** — **Billederne forsvinder helt.** De bliver fjernet fra alle album, de kan ikke
> ses offentligt, og filerne bliver slettet. Det er det du skal bruge, hvis nogen har bedt om at få et
> billede taget ned. Det kan **ikke** fortrydes herfra — kun en udvikler kan hente et slettet billede
> frem igen.

The destructive one is the visually distinct one, and the confirmation says how many — because v1 ships
no undelete button (§11 Q6) and a mis-aimed "select all" is only recoverable by a maintainer.

## Verified by breaking it

Each guard sabotaged, the failure watched, the edit reverted:

- **Dropped the library owner** from `blobRefsInUse` (`if app.models.Photos != nil` → `if false`): four
  glimt-side tests failed, including `TestGlimtDeleteKeepsBytesOfAPhotographInNoAlbum`. This is the
  break that matters most, because it is exactly the shape of "a third owner appeared and one path was
  not updated".
- **Dropped the exclusion** (`exclude.PhotoIDs` → `nil`):
  `TestAdminDeleteExcludesThePhotographBeingDeleted` failed. Without the test this is invisible — the
  feature works, it just silently never frees disk.
- **Made a failed sharing check proceed** (`return` → `inUse = map[string]bool{}`): both fail-closed
  tests failed, from both doors.
- **Unwrapped `requireAdmin`** on the new album-item route: task 371's AST walk named the file and line.

## Verified live

Against the dev stack, with the blob store inspected directly rather than trusting a 204:

1. Removed `f929bed6…` from album `8d858f83…` → `album_item.deleted = 1`, **both blobs still on disk**,
   `photo` row untouched.
2. Deleted the same photograph from the library → `photo.deleted = 1`, and `/blobs/f9` and `/blobs/b1`
   are now empty. Image and thumbnail both freed.
3. `/admin` serves both sentences and the enabled `data-act="delete"` control.

The glimt-shared case could not be exercised live: this dev database is replayed from the production
log, so its `glimt_media` rows reference blobs that were never copied down. Covered by the unit tests
and by break-test 1 above, which is the stronger evidence anyway.

## Notes

- **A cosmetic repair, incidental:** literal `\u2014` escape sequences had leaked into `//` comments and
  a few Markdown files across the repo from earlier authoring. Replaced with real em dashes in comments
  and prose only — the ones inside Go string literals are valid escapes and were left alone.
- **The gates wedge the dev loop and it does not look like that.** A rebuild here returned a 500 from
  the new endpoint; the cause was the container still running gosec/staticcheck/test, serving the old
  binary. Sixty seconds later the same request answered 204. Worth remembering before debugging a
  handler that is fine.
