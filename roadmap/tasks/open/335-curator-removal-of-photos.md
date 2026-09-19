# 335 — Curator removal of a photograph or an album

**Status:** open
**Priority:** high
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 011 §0b.2, §6. Permission to publish the album photographs has been obtained upstream — *"if we do
not have permission to show a picture it will not be there"* (maintainer, 2026-09-19) — so this feature
builds **no rights model, no per-subject flag and no consent register**. The safety property is *the
set*: a photograph is in an album because somebody decided it may be shown.

What that leaves is narrow and non-negotiable: **permission is withdrawable, and mistakes happen.** So
a curator must be able to remove a photograph, or a whole album, and have it leave the public page
promptly.

"Promptly" means within the page's cache window, not "at the next deploy". The public pages set
`Cache-Control: public, max-age=60` (see `glimtpublic.go`, where the same short window is chosen so a
takedown lands quickly) — removal must be bounded by that, and the bound should be stated in the code
rather than inferred.

Note this is the one operation on this surface that is **irreversible in effect** even if the row
survives: once a photograph is down, somebody's objection has been honoured, and re-publishing it
should require the same deliberate act as publishing it did. Prefer a soft delete with a visible
state (the `deleted` flag pattern in `checkgroup`, `checkpoint` and `person`) over a destructive one,
so a re-add stays expressible and an accidental removal is recoverable.

Curation tooling as a whole is PRD 011 §11 Q5 and undecided; this task is only the removal path, which
is needed regardless of where the tool eventually lives.

## Acceptance Criteria

- [ ] A curator can remove a single photograph from an album, and a whole album, by a documented
      mechanism (whatever §11 Q5 settles on — do not block on the full tool).
- [ ] Removal is soft, with a visible state, following the `deleted` flag pattern already used in
      `checkgroup` / `checkpoint` / `person`.
- [ ] A removed photograph disappears from every public surface — frontpage cover, album page, map
      markers (task 342), and the media endpoint — within the stated cache window.
- [ ] The media endpoint stops serving the bytes too: a removed photograph must not remain fetchable
      by a URL somebody already has.
- [ ] The bound on "promptly" is stated in the code, not left to be inferred from cache headers.
- [ ] A test asserts a removed item is absent from every public read path, not just the album page.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §0b.2 / §6 / §10 (Phase 1).
