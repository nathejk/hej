# 321 — Document Glimt in PrivacyView

**Status:** open
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §6. `/privatliv` must document Glimt honestly, in Danish, in language a 12-year-old and
their parent can both read:

- **What is stored** — the photos and videos, the caption, which hold posted it, when. Not the
  location: GPS/EXIF is stripped on upload.
- **Who can see it** — the three audiences, in plain words, and that **the Team section can see
  everything posted whatever the audience**. This is the disclosure the PRD insists on: a
  participant choosing "Min gruppe" is entitled to know that is not the same as "only my
  gruppe" (PRD 019 §6). It must be stated, not discoverable.
- **That no names are attached** — a glimt is signed with the patrulje, not a person.
- **How long it lives** — the **real configured retention** from `/api/config` (task 310), not
  a hard-coded sentence. A page that says "90 dage" while the deployment is set to 30 is worse
  than saying nothing.
- **How to get something removed** — the author can delete; anyone can report; reporting hides
  it from the public feed immediately.

## Acceptance Criteria

- [ ] `PrivacyView.vue` has a Glimt section covering all five points
- [ ] Team-section reach explicitly stated
- [ ] Retention figure read from `/api/config`, not hard-coded
- [ ] Danish, plain language, no legalese
- [ ] `npm run test:unit` and `npm run build` pass

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
