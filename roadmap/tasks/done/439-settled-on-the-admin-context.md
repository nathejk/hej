# 439 — ctx.settled(ids): one place that waits for the projection

**Status:** done
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

From PRD 024 §8, and noted as the right shape by both task 437 and task 438.

Every write in the admin tool publishes an event and then calls `ctx.reloadSheet()` in the same tick,
reading a projection that is folded downstream. The uploader now waits properly (task 438: poll
`GET /api/admin/photos?ids=…` until the ids come back), and PRD 024 adds a third caller — a resort is a
publish like any other, and its reload has the same race.

Eight files have the bug today: `albumaction`, `deleteaction`, `patrolaction`, `positionaction`,
`captionaction`, `creditaction`, `albumorder`, `vieweredit`. They are less likely to be *noticed* than
the upload was — one row folded rather than three hundred, and the visible change is an attribute on a
cell that is already there rather than cells that are absent — but the shape is identical.

Do this **before** PRD 024's apply-the-sort tasks, so the third caller is a call and not a third copy.

## Acceptance Criteria

- [x] `ctx.settled(ids)` on the admin context: resolves when the library can name every id, or when a
      bounded wait gives up; reports which happened
- [x] The uploader is moved onto it and `waitForPhotos` leaves `upload.js`
- [x] The uploader's behaviour and its guard are unchanged in substance
- [x] `TestEveryAdminContextMemberIsProvided` covers the new member (it will, by construction)
- [x] Callers that reload after a write but have no ids to wait for are left alone, not given a fake wait

## Progress Log

- 2026-09-28 — Task created from PRD 024 §8.
- 2026-09-28 — Moved `waitForPhotos` and `seen` out of `upload.js` and onto the context as `ctx.settled`,
  together with `SETTLE_MS` and `IDS_PER_ASK`. The loop is unchanged line for line: chunking below
  `maxAdminLibraryIDs`, dropping ids already seen so a large batch converges, 10-second bound, 150 ms
  doubling to 1 s, a failed read ending the wait rather than retrying forever. `upload.js` keeps only the
  decision to call it and the fallback sentence.
- 2026-09-28 — Decided the return stays a **plain boolean** rather than becoming `{ settled, missing }`.
  Both ways of not getting there — the deadline passed, or the read itself broke — leave a caller with
  exactly one thing to do: reload anyway and say it may be behind. A caller that genuinely needs to name
  the stragglers should have that added here rather than polling on its own; recorded in the comment so
  the next caller does not read the boolean as an oversight.
- 2026-09-28 — `seen` is a local `const` in `initAdminTool` rather than a second context member. It is the
  wait's mechanism, not shared surface, and putting it on `ctx` would invite a caller to treat an absent
  id as "does not exist" — the reading task 438 warns against under "Worth knowing".
- 2026-09-28 — Left the eight reload-after-write callers alone, per the criteria. `settled([])` would
  answer `true` immediately, which is honest, but a caller that cannot name what it wrote needs its own
  answer to "which ids prove this landed" — and for the removals the proof is an id *disappearing*, which
  is a different read.
- 2026-09-28 — ✅ Guard updated. Each needle is now asserted against the asset that owes it: the upload's
  two (`tally('stored', out.photoId)`, `const caught = await ctx.settled(b.stored)`) against `upload.js`,
  the wait's five plus the presence URL and the provider line against `main.js`. Asserting all of them
  against one file would pass on a tool where the wait had been dropped from the other. Comments are
  stripped from both, since both now carry prose about the polling.
- 2026-09-28 — Mutation-checked. Replacing `const caught = await ctx.settled(b.stored);` with
  `const caught = true;` fails on the upload needle; deleting the bound and the converge line from
  `main.js` fails on both of those. Worth recording a mutation that did **not** fail: inserting an early
  `return true;` at the top of `ctx.settled` while leaving the loop below it, because the needles still
  match unreachable code. That is the known ceiling of a source-read guard on this surface rather than
  something more needles fix, and it is why the behaviour is written down in both files.
- 2026-09-28 — `gofmt -l .` silent, `go vet ./...` and `go tool staticcheck ./...` clean,
  `go test ./cmd/api/ -count=1` green (57 s).
