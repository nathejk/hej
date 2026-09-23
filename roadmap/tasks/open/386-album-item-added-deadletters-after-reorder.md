# 386 — An album item added before a reorder deadletters on every replay

**Status:** open
**Priority:** high
**Created:** 2026-09-23
**Picked up by:**
**Started:**
**Completed:**

## Description

Found incidentally while verifying task 383 against the dev stack. Every boot produces two deadlettered
statements:

```
Error 1062 (23000): Duplicate entry '8d858f83-…-1783442ddb4d…' for key 'album_photo'
Error 1062 (23000): Duplicate entry '8d858f83-…-f929bed6d3d4…' for key 'album_photo'
```

with the captured query being an ordinary `handleItemAdded` upsert:

```sql
INSERT INTO album_item SET albumId="8d858f83-…", year="2026", ordinal=1,
  photoId="f929bed6…", addedAt="…", deleted=0
ON DUPLICATE KEY UPDATE year=VALUES(year), photoId=VALUES(photoId), addedAt=VALUES(addedAt), deleted=0
```

## The mechanism

`album_item` carries **two** unique constraints: `PRIMARY KEY (albumId, ordinal)` and
`UNIQUE KEY album_photo (albumId, photoId)`. That pairing is deliberate and correct — task 367's
`handleItemsReordered` exists entirely because of it, and its offset trick is the standard way to renumber a
unique-keyed sequence.

`ON DUPLICATE KEY UPDATE` copes with a violation of *one* unique key. It does not cope with a row that
conflicts with **two different existing rows** on two different keys: MariaDB updates the first conflicting
row it finds, and that update then violates the other key. That is exactly the shape here.

The history that produces it:

1. `itemAdded(ordinal=0, photo=A)` and `itemAdded(ordinal=1, photo=B)`.
2. `itemsReordered` swaps them, so the table holds `(0, B)` and `(1, A)`.
3. The projection is **not truncated between boots**, so a replay re-applies step 1 against the
   post-reorder table. `itemAdded(ordinal=1, photo=B)` now collides with `(1, A)` on the primary key *and*
   with `(0, B)` on `album_photo` — two rows, two keys, 1062.

So the fold is idempotent on a fresh database (where the adds precede the reorder) and **not** idempotent on
a database that has already seen the reorder. Replay-order sensitivity of exactly the kind the offset
technique was introduced to remove — it was applied to the reorder fold and not to the add fold.

## Why this is worth fixing rather than tolerating

The end state today happens to be correct: the failed statement would have written what the table already
held, so nothing is lost and no surface is wrong. That is luck, not design, and it is not worth relying on:

- **It is noise that trains people to ignore deadletters.** Two per boot, forever, for a healthy album. The
  next deadletter that means something arrives next to them.
- **The convergence argument is the projection's whole warranty.** `.rules` and PRD 022 §8.7 both lean on
  "the log is the truth and the table is derived"; a fold that fails on replay makes that false in a way
  that will eventually matter — a reorder followed by a re-add, or a rebuild that reaches the adds after a
  partial fold, could genuinely land wrong.
- It will get worse as albums get reordered more, which is what shipping the curator's tool makes likely.

## Approach (not prescriptive)

The add fold needs the same two-step shape the reorder fold uses: vacate, then place. Something like —
within one `Consume` so a boot cannot be interrupted between the halves — move any row in this album that
already holds this `photoId` out of the way by the same `reorderOffset`, then upsert by
`(albumId, ordinal)`, then collapse any leftover offset rows.

Worth checking whether the simpler reading is defensible instead: **an `itemAdded` whose photograph is
already in this album is a no-op**, because the reorder is the later truth and the add has nothing to say
about position that the reorder has not overridden. If that holds, the fold is a conditional insert and the
whole problem disappears. Decide it deliberately and write down which it is — the two differ when an event
stream is replayed out of order, and that difference is the bug.

## Acceptance Criteria

- [ ] A replay against a database that already holds a post-reorder album produces no deadletter
- [ ] The resulting membership is the same whether the events are folded onto an empty table or onto one
      already holding the final state — asserted, with a test that would fail today
- [ ] The chosen semantics for "added a photograph already in this album" is stated in a comment, with the
      alternative named
- [ ] Task 367's reorder offset reasoning is referenced rather than restated
- [ ] Verified against the dev stack: the two deadletters are gone after a restart, and the album still
      renders in the curator's order
- [ ] The existing deadlettered rows are either replayed or explained
