# 336 — Recent public glimt on the frontpage

**Status:** open
**Priority:** medium
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 011 §6 (section 3). The frontpage's third section: the most recent public glimt, as a strip, linking
to the full page at `/offentligt/glimt`.

This is PRD 019's feed **surfaced**, not reimplemented. That distinction is the whole task:
`publiclyVisible` in `go/cmd/api/glimtpublic.go` stays the single gate deciding what is publicly
visible, and the retention window, the hidden-flag handling and the `publicGlimt` query are reused as
they are. A second implementation of "which glimt are public" is how the two come to disagree, and the
disagreement would be a photograph somebody took down still appearing on the frontpage.

Attribution is to the **hold**, never to a person — PRD 019's rule, unchanged. `publicHoldLabel` already
renders it; reuse that too.

**Public glimt carry no location and are not on the map.** Their EXIF is stripped and nothing replaces
it, and more importantly a member tapping "Offentligt" agreed to share a *photograph*, not a *position*.
Only curated album photographs are plotted (task 342). If located glimt are ever wanted it needs a
deliberate opt-in in the composer and its own copy — PRD 011 §11 Q7 — not a quiet pipeline change here.

This answers PRD 019 §11 Q10 ("does the public feed show everything, or a curated selection?") with
**both**: the albums are the curated face, this strip is the raw feed, and they do different jobs.

## Acceptance Criteria

- [ ] The frontpage shows the most recent public glimt as a strip, with a link to `/offentligt/glimt`.
- [ ] `publiclyVisible` remains the only gate; no new logic decides what is public.
- [ ] Retention window and hidden-flag handling are inherited, not re-expressed.
- [ ] Attribution via the existing `publicHoldLabel` — hold, never a person.
- [ ] No glimt appears on any map.
- [ ] Thumbnails, `loading="lazy"`, alt text; works with JavaScript disabled.
- [ ] Empty state reads "der er ikke delt nogen offentlige billeder endnu", matching the existing page's
      wording rather than inventing a second phrasing.
- [ ] A test asserts a hidden or expired glimt is absent from the frontpage strip as well as from
      `/offentligt/glimt`.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §10 (Phase 1). Answers PRD 019 §11 Q10.
