# 320 — Nav slot ordering with Glimt added

**Status:** open
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §7, §11 Q7. `BottomNav` has `MAX_SLOTS = 5` and already overflows into `MoreMenu` for
most roles. Glimt is a **primary** destination for spejdere and must be in the bottom bar for
them — which forces a decision about what moves to "Mere".

A spejder currently sees: maps, rulebook, updates, schedule, faq, privacy (no contacts pane).
Adding Glimt makes seven for five slots.

This task is the decision plus the implementation: pick the per-role ordering, put it in
`vue/src/config/navigation.ts`, and make sure nothing a role needs becomes unreachable.

The Team-section moderation entry (task 309) must not consume a participant slot.

## Acceptance Criteria

- [ ] Per-role ordering decided and recorded in a comment explaining *why*, not just what
- [ ] Glimt in the bottom bar for `spejder` and `bandit`
- [ ] Nothing becomes unreachable — everything demoted is in `MoreMenu`
- [ ] Moderation entry does not take a participant slot
- [ ] Existing `BottomNav` / `MoreMenu` tests updated
- [ ] `npm run test:unit` and `npm run build` pass

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
