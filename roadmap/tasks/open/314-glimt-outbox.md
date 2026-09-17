# 314 — glimtOutbox.ts — IndexedDB outbox for pending posts

**Status:** open
**Priority:** high
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `glimtOutbox.ts` — raw IDB store for drafts + blobs, with a comment on why no wrapper
- [ ] Enqueue, list, mark-uploaded, remove, and a drain routine
- [ ] Drain runs on foreground (`visibilitychange`) and on `online`
- [ ] Partial progress survives an app close — already-uploaded items are not re-uploaded
- [ ] A failed drain leaves the draft queued, never silently dropped
- [ ] Unit tests with a fake IDB, including resume-after-partial-upload
- [ ] `npm run test:unit` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
