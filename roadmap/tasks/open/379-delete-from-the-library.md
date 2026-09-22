# 379 — Delete from the library, and say plainly how that differs from removing from an album

**Status:** open
**Priority:** high
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] A delete is soft, records the optional reason on the event, and survives a replay
- [ ] The photograph vanishes from every album and every public read immediately after the fold
- [ ] Blobs are purged only when nothing else — album, photo or glimt — references them, and any error
      means nothing is deleted
- [ ] Delete works on a multi-selection from the contact sheet
- [ ] The two Danish sentences distinguishing "fjern fra album" from "slet fra biblioteket" are written
      and reviewed, and the destructive action is the visually distinct one
- [ ] The in-app removal endpoints are unchanged and share the purge helper — tested from both sides
- [ ] A re-upload of deleted bytes does not resurrect the row (task 372)
- [ ] OpenAPI annotations with `@Failure 401`
