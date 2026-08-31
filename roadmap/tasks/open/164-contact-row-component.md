# 164 — Contact row component

**Status:** open
**Priority:** high
**Created:** 2026-08-31

## Description

The directory row (PRD 007 §7, maintainer direction 2026-08-31):

- **avatar on the left**;
- **member name**, with the **team/group name in smaller grey print below it** where
  applicable;
- **phone number on the right**;
- tapping the row (not the number) opens the person's profile (task 167).

Details that matter:

- The favourite toggle lives on the row too, but **keep it clear of the phone number** —
  a thumb aiming for "favourite" must not start a call.
- A `tel:` link on the number is fine. No share, no copy-all, no export.
- Missing portrait → neutral placeholder with initials, visibly "no photo" rather than a
  failed load (`avatar` fallback).
- A withdrawn member shows their status marking and offers no call action (task 160).
- Compose the existing `avatar` primitive — the same one PRD 003's `ProfilePhoto.vue`
  composes. Do not fork PRD 003's capture component.

## Acceptance Criteria

- [ ] Row matches the specified layout at mobile widths without truncating the name
      awkwardly.
- [ ] Group line is visually secondary (smaller, grey) and omitted when absent.
- [ ] Phone number right-aligned, tappable as `tel:`, with a hit area separated from the
      favourite toggle.
- [ ] Row tap opens the profile; number tap does not.
- [ ] Initials placeholder when no portrait.
- [ ] Withdrawn state renders a marking and no call action.
- [ ] Component tests cover: no portrait, no group, long name, withdrawn.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §7.
