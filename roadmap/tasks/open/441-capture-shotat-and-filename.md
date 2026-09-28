# 441 — Capture the capture time and the filename on upload

**Status:** open
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 024 §6 R3/R5. Needs task 440 first.

Two new fields on a photograph, both of which the upload already has and throws away:

- `shotAt` — from `imaging.ReadShotAt`, read before re-encoding. `DATETIME NULL`; null means the file did
  not say, and the sort falls back to `uploadedAt` for that photograph.
- `fileName` — from the multipart part's filename. `r.FormFile("photo")` reads the header today and
  discards it (`file, _, err :=`). `VARCHAR(255) NOT NULL DEFAULT ""`, stored as given and never parsed
  for meaning. Empty on the raw-body path, which the drop zone does not use.

**A filename is person-shaped and the privacy guard will not catch it.** Usually `IMG_0123.JPG`, but it is
free text from somebody's computer and can name a person (`mor-og-far.jpg`). The public site names no human
being with one written-down exception (PRD 011 §0b.1, task 393), and `isPersonShaped` in
`publicprivacy_test.go` does **not** flag a field called `fileName` today. So:

- add `filename` to that guard's needles, and
- except it by name for the admin surface with the reasoning, exactly as `credit` was excepted in task 393
  — same treatment, opposite conclusion.

Event fields are additive and optional, so replaying an older log folds to the defaults above.

## Acceptance Criteria

- [ ] `photo.Uploaded.ShotAt` and `.FileName`; columns and fold to match
- [ ] Read before re-encoding, alongside the GPS fix
- [ ] The multipart filename is captured; the raw-body path stores ""
- [ ] `filename` is in `isPersonShaped`'s needles and excepted by name with reasoning
- [ ] No public response carries either field
- [ ] Replaying a log without the fields folds to null / ""

## Progress Log

- 2026-09-28 — Task created from PRD 024 §6 R3/R5 and its privacy note.
