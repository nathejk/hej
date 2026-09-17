# 314 — glimtOutbox.ts — IndexedDB outbox for pending posts

**Status:** done
**Priority:** high
**Created:** 2026-09-17
**Picked up by:** agent session (Zed)
**Started:** 2026-09-18
**Completed:** 2026-09-18

## Description

PRD 019 §5, §8. **The app must not lose a photo it accepted.** A spejder posts from a field
with no signal; the glimt is accepted into a local outbox and uploaded when the network comes
back.

Modelled on `vue/src/helpers/trackDb.ts` — raw IndexedDB, no wrapper library — holding draft
metadata plus the media blobs until each item is uploaded and the glimt is created.

Drained on **foreground and on `online`**, never by Background Sync: it is unavailable on iOS
and a backgrounded web app does not run (PRD 002 measured 2% background coverage). The UI must
never imply otherwise — a pending glimt shows *venter på nettet*, and resumes on next
foreground if the app was closed mid-upload.

The outbox is **never evicted** by the offline quota logic while it holds unsent media
(task 315).

## Acceptance Criteria

- [x] `glimtOutbox.ts` — raw IDB store for drafts + blobs, with a comment on why no wrapper
- [x] Enqueue, list, mark-uploaded, remove, and a drain routine
- [x] Drain runs on foreground (`visibilitychange`) and on `online`
- [x] Partial progress survives an app close — already-uploaded items are not re-uploaded
- [x] A failed drain leaves the draft queued, never silently dropped
- [x] Unit tests including resume-after-partial-upload
- [x] `npm run test:unit` (834 tests), `type-check` and `build` pass

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-18 — Picked up. This is the last criterion standing between the composer (317) and PRD 019
  §5's promise: "the app must not lose a photo it accepted."
- 2026-09-18 — **Two stores, not one.** Drafts and media are separate object stores keyed
  `[draftId, ordinal]`, so the author's arrangement is a constraint the database enforces rather than a
  convention the drain has to remember — and a re-put of an item is an overwrite, not a duplicate.
  Enqueue writes both in **one transaction**: a draft with no media is a card that can never be
  posted, and media with no draft is quota nobody will ever reclaim.
- 2026-09-18 — **An uploaded item's Blob is dropped and its refs kept.** This is the mechanism behind
  two separate requirements at once. It is how a draft that failed on item four *resumes* at item four
  instead of starting over — which on rural mobile data is the difference between a post that
  eventually lands and one that never does. And it is how a waiting draft stops costing quota for
  bytes the server already has, which matters because a phone short of space is exactly where an
  outbox turns into the reason the *next* glimt cannot be queued.
- 2026-09-18 — The drain **stops at the first draft that fails** rather than working through the queue.
  Continuing would burn a data budget re-failing on the same dead connection, and the queue is ordered
  because the member posted in an order.
- 2026-09-18 — Overlap-guarded. Foreground and `online` fire together often enough — unlock a phone in
  a coverage hole and both arrive within a second — and two concurrent drains would upload everything
  twice. Tested with `Promise.all`.
- 2026-09-18 — Wired into **`useSyncLoop`'s `glimt` dispatch entry**, not a loop of its own. That check
  already runs on precisely the moments a queued post could go out — foreground, reconnect, manual
  refresh — and adding a second loop is the mistake `useFreshnessLoop.ts` documents at length. Drained
  *before* the version check, since a post that lands changes the feed.
- 2026-09-18 — **No Background Sync, and the copy reflects that.** It is unavailable on iOS and a
  backgrounded web app does not run there at all (PRD 002 measured 2% coverage over 22 hours). So the
  notice says "venter på nettet" rather than "sender i baggrunden" — claiming otherwise would misplace
  somebody's photographs, which is a worse failure than the delay itself.
- 2026-09-18 — 🐞 **A test caught a real inconsistency.** `createFromRefs` set the store's `error`, so a
  *background* drain failure raised a banner — directly contradicting the comment I had just written
  saying it must not. Fixed by making it return `{ok, reason}` and touch no UI state: `error` is a
  banner, and its two callers want opposite things from a failure. A member watching the composer
  should be told; a background drain must stay quiet, because a queued post that has not gone yet is a
  normal state on this network and a banner on every foreground trains people to ignore banners. The
  deep helper should never have owned that decision.
- 2026-09-18 — Tests mock the outbox at its module boundary rather than adding `fake-indexeddb`. That
  follows the repo's existing split — `trackDb.ts` has no spec either, and `track.store.spec.ts` tests
  the logic above it — and it is the right one here: the IndexedDB plumbing needs a browser, while
  everything worth getting wrong is in the drain. 13 tests.
- 2026-09-18 — ✅ All criteria met. `type-check`, 834 tests and `build` clean. **This closes task 317**,
  whose last outstanding criterion was this.

### ⚠️ Not verified, and cannot be from here

The suite mocks IndexedDB away, so **`glimtOutbox.ts` itself has never run**. Nothing here proves a Blob
survives a real transaction, that the `[draftId, ordinal]` key path behaves, or that quota failures
surface as expected in Safari.

That is what **task 325** is for, and its five scenarios are exactly the gap: post offline, close the
app, reopen offline, restore the network foregrounded and closed, and kill the app mid-upload. Until
that runs on a real phone, the promise in PRD 019 §5 is implemented but not demonstrated.
