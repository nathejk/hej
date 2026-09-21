# 336 — Recent public glimt on the frontpage

**Status:** done
**Priority:** medium
**Created:** 2026-09-19
**Picked up by:** agent session (Zed)
**Started:** 2026-09-19
**Completed:** 2026-09-19

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

- [x] The frontpage shows the most recent public glimt as a strip, with a link to `/offentligt/glimt`.
- [x] `publiclyVisible` remains the only gate; no new logic decides what is public.
- [x] Retention window and hidden-flag handling are inherited, not re-expressed.
- [x] Attribution via the existing `publicHoldLabel` — hold, never a person.
- [x] No glimt appears on any map.
- [x] Thumbnails, `loading="lazy"`, alt text; works with JavaScript disabled.
- [x] Empty state reads "der er ikke delt nogen offentlige billeder endnu", matching the existing page's
      wording rather than inventing a second phrasing.
- [x] A test asserts a hidden or expired glimt is absent from the frontpage strip as well as from
      `/offentligt/glimt`.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §10 (Phase 1). Answers PRD 019 §11 Q10.
- 2026-09-19 — Picked up. The strip itself already shipped with the frontpage shell (task 332): it reads
  through `publicGlimt`, so `publiclyVisible`, the retention cutoff and the hidden flag are inherited by
  construction rather than reimplemented — which was the task's actual requirement. So this task is
  mostly the tests that would catch somebody *un*-inheriting them later.
- 2026-09-19 — **The gap worth closing was retention.** Task 332 asserted the strip hides group-scoped,
  nathejk-scoped and hidden glimt, but said nothing about **expired** ones — and a zero cutoff serves the
  entire archive to the open web. Added both directions (a cutoff is applied when retention is on, and
  *not* applied when it is off, since zero means "as long as the glimt itself" and not "immediately"),
  mirroring the assertions `glimtpublic_test.go` already makes for the full page.
- 2026-09-19 — Added `TestExpiredGlimtIsAbsentFromBothPublicSurfaces` and its hidden-glimt counterpart,
  which check the strip **and** `/offentligt/glimt` in one loop. Two surfaces, one rule — the shape of the
  test is the argument for sharing the query.
- 2026-09-19 — Asserted the strip is **bounded** (9 items, three complete rows at both widths) and links
  onward, because "a taste with a link" is only true if it cannot grow into the page it links to.
- 2026-09-19 — Asserted attribution positively *and* negatively: the hold label appears, and the
  fixture's author ids do not. `publicHoldLabel` gives a person's name nowhere to come from, but the alt
  text is where one would appear if somebody changed it.
- 2026-09-19 — "No glimt on any map" needed care, because the map endpoint is task 342's and does not
  exist yet. Asserted what is assertable now — no coordinate-shaped field on any of the three public
  glimt surfaces — and wrote the map half so it **logs that it is inert** rather than passing quietly:
  `t.Log("the album map source does not exist yet (task 342); the map half of this test is inert")`. It
  becomes a real guard the moment 342 lands, without anybody having to remember to add one. A test that
  silently asserts nothing is worse than no test.
- 2026-09-19 — `go vet ./...` and `go test ./...` clean. Moving to done.
