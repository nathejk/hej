# 386 — An album item added before a reorder deadletters on every replay

**Status:** done
**Priority:** high
**Created:** 2026-09-23
**Picked up by:** agent
**Started:** 2026-09-23
**Completed:** 2026-09-23

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

---

## What was actually wrong: three bugs, one confusion

The reported deadletter was the visible one. Reproducing it in SQL against the dev database turned up two
more, and all three are the same mistake: **`album_item` puts the ordinal in its primary key, so a position
looks like an identity. It is not one.** A position is a slot in one album; the photograph is what somebody
chose; and the two stop agreeing the moment the album is reordered.

| | Bug | Severity |
|---|---|---|
| 1 | `handleItemAdded` deadletters on replay after a reorder | loud, no data loss |
| 2 | `handleItemRemoved` is keyed on the ordinal, so a replay after a reorder **soft-deletes whichever photograph has since moved into that slot** | **silent, takes a live photograph off a public album** |
| 3 | `handleItemsReordered`'s vacate collides with itself once a row is stranded in the offset range | loud, and the curator's reorder silently does not happen |

**Bug 2 is why bug 1 could not be fixed on its own.** Today's deadletter is load-bearing by accident: the
failing `itemAdded` aborts before anything is written, and the stale `itemRemoved` that follows it in the log
is *already* being applied to the post-reorder table. Fixing the add without fixing the removal would have
converted a loud no-op into a silent removal of the wrong photograph — strictly worse. Verified by
constructing the sequence in SQL before writing any Go.

**Bug 3 needs no replay at all.** Remove an item, then reorder twice. The first reorder strands the removed
row at `ordinal + reorderOffset` (it is not named in the order, so nothing places it back down); the second
reorder's vacate then tries to move ordinal 0 onto 1000000, which the stranded row holds:

```
ERROR 1062 (23000): Duplicate entry 'al-1000000' for key 'PRIMARY'
```

## The fix

**`handleItemAdded` — two statements instead of one upsert.**

1. `UPDATE … SET deleted=0, addedAt=… WHERE albumId=… AND year=… AND photoId=…` — reinstates an existing
   membership **where it is**, found by photograph whatever ordinal it now holds.
2. `INSERT … SELECT … FROM DUAL WHERE NOT EXISTS (… photoId=…)` — creates the row only when the photograph
   is not already in the album.

`WHERE NOT EXISTS` rather than `INSERT IGNORE`, and the difference is the whole point: `IGNORE` would also
swallow a **primary key** clash — a different photograph already at this ordinal — and that case must stay
loud, because it means a membership was not recorded. A dead letter is a far better outcome for that than
silence. The condition suppresses exactly one conflict, the one statement 1 has already handled. (Confirmed
against the dev database that MariaDB permits the target table in the subquery, and that a genuine PK clash
still raises 1062 through it.)

**`ItemRemoved` gains `PhotoID`**, and the fold matches on it, falling back to the ordinal only when the
field is absent. Both publishers now send it — `albumremove.go` resolves it from the item it already looks up
for the 404, and `admindelete.go` had it in hand already.

**The reorder's vacate gains `ORDER BY ordinal DESC`.** Moving rows highest-first means none is ever written
onto a position another still occupies — the same reason a shift-right of an array walks backwards.

### The semantics, stated, with the alternative named

`ItemAdded` means **"this photograph is a member of this album, at this position if it is not here yet"** —
and nothing about the position of a photograph that already is. That is not a convenient reading chosen to
dodge the bug; it is what the writer means. `addAdminAlbumItemsHandler` never publishes the event for a live
member, and for a **removed** one it publishes the ordinal that row already holds — so "keep the existing
position and clear the removal" is exactly what it asked for. A later reorder is the newer truth about
position, and a replayed older add must not argue with it.

**Rejected: mirroring the reorder** — vacate, then place. It converges too, because the reorder replays
afterwards and fixes the order either way. Rejected because a replayed *old* add would transiently override a
*newer* reorder, which is the wrong direction for a stale event to push; and because it costs two extra
statements per photograph on the common path, where the common path is a photographer filing three hundred of
them.

### What the ordinal fallback costs, and why it is not "skip"

A legacy `itemadded` is skipped. A legacy `itemremoved` is **applied**, by ordinal, and the asymmetry is
deliberate: skipping an add costs an album an item, while skipping a removal puts a photograph somebody asked
to have taken down back on a public page. That is the one direction this projection must never fail in. An
imperfect removal beats a silently reversed one.

## The guard

`nathejk/table/album/membershipsafety_test.go` — the family rule rather than three separate patches:

- no membership fold locates a row by position alone;
- a removal prefers the photograph and only falls back to the ordinal, guarded on its absence;
- the vacate is descending;
- the add is a conditional insert, and specifically **not** `photoId=VALUES(photoId)` (the clause that
  actually caused the deadletter) and **not** `INSERT IGNORE`.

It reads the source, for the reason `querysafety_test.go` states at length: these properties live in SQL
text, `cqrs.Writer` is `Consume(string) error`, and there is no way to execute a statement against a fake.
Blunt, and honest about checking the presence of a clause rather than the behaviour of a database — which is
why the behaviour was verified against a real one.

**One trap worth recording:** the first draft of `TestTheAddDoesNotOverwriteWhateverHoldsThePosition` failed
against *its own comment* explaining why `INSERT IGNORE` was rejected. `foldBody` now strips comment lines. A
guard that searches for forbidden SQL will always find it in the prose forbidding it.

## Verified by breaking it

Each sabotage reverted, each failing:

| Sabotage | Caught by |
|---|---|
| removal back to the ordinal (`if false`) | the family guard, plus both removal tests |
| `ORDER BY ordinal DESC` deleted | `TestTheReorderVacatesInDescendingOrder` |
| the add restored to the old `ON DUPLICATE KEY UPDATE photoId=VALUES(photoId)` | the family guard *and* `TestItemAddedWritesTheMembership` |

## Verified live

Against the dev stack, with `album_item` read directly rather than a 204 trusted.

**The deadletter is gone.** The reproduction was still in the table — `0=f929, 1=1783` against a log that
adds `0=1783, 1=f929` — and a full replay with the fix produced **no `album_item` dead letter at all**. The
only unresolved row in `deadletter` is the pre-existing `postalCode` one from task 352.

**Bug 3, end to end:** added a third photograph, reordered, saw the removed row strand at 1000000 exactly as
documented, then **reordered again** — 204, the order flipped correctly, the stranded row moved to 2000000,
no 1062. That second reorder is the operation that failed before the fix.

**Bug 2's path:** removed a photograph by id (the row went `deleted=1` at its own ordinal, not somebody
else's), then re-added it — **reinstated in place at the same ordinal**, not appended.

**Convergence, both directions**, which is the criterion that matters:

| | result |
|---|---|
| Restart, folding the whole log onto the existing table | `album_item` byte-identical before and after |
| `TRUNCATE album_item`, restart, rebuild from the log | **identical to the incrementally folded table** — verified with a `UNION ALL … HAVING COUNT(*) <> 2` diff returning no rows |

The stranded row at ordinal 2000000 came back at 2000000 in the clean rebuild too, which is the right answer:
it is derived from the log like everything else.

**The public surface, with `PUBLIC_ALBUMS=true`:** two images on the album page, ordinals 0 and 1 serving 200,
the stranded removed row answering **404**, and the frontpage counting "2 billeder".

## Acceptance Criteria

- [x] A replay against a database that already holds a post-reorder album produces no deadletter
- [x] The resulting membership is the same whether the events are folded onto an empty table or onto one
      already holding the final state — asserted, with a test that would fail today
- [x] The chosen semantics for "added a photograph already in this album" is stated in a comment, with the
      alternative named
- [x] Task 367's reorder offset reasoning is referenced rather than restated
- [x] Verified against the dev stack: the two deadletters are gone after a restart, and the album still
      renders in the curator's order
- [x] The existing deadlettered rows are either replayed or explained

## Notes

- **The two deadlettered rows needed no replay.** Each failing statement would have written what the table
  already held — the membership was correct throughout, which is why this was noise rather than data loss.
  They are gone from `deadletter` and the replay no longer produces them.
- **The stranded offset row compounds** by one `reorderOffset` per reorder, so an album with a removed
  membership reordered a few thousand times would overflow the signed INT column. Left as is and documented:
  at that point the write fails loudly rather than corrupting, and compacting the offset range would need to
  enumerate rows the event does not name — a read this fold does not have.
- **This is a symptom of task 350, not a replacement for it.** All three bugs exist because a replay folds old
  events onto a table that already reflects newer ones, and nothing truncates it. Option 2 there — truncating
  on boot — would dissolve this whole class. That decision is still open and is worth more than these three
  fixes; what this task does is make the fold correct *without* depending on it.
- **Found and not fixed here:** the frontpage renders `{{.Count}} billeder` unconditionally, so an album with
  one photograph reads "1 billeder". Filed as task 387 rather than folded into this commit.
