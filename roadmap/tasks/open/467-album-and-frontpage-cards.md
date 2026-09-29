# 467 — The album's and the frontpage's share cards

**Status:** open
**Priority:** high
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

**PRD:** 026

## Description

The frontpage's card is the branded one (task 464). An album's is **its cover**, addressed by ref with
`?variant=medium` — 800px, the rendition nearest the size a card is rendered at and already what the cover grid uses
(task 461).

Title is the album's title; description is the album's own description plus the photograph count. **Never a caption
and never a credit**: those are the two free-text fields on this surface and a credit is the one that names somebody
(task 393).

An album with **no cover emits no `og:image`** and falls back to the branded card — never a URL that 404s.

## Acceptance Criteria

- [ ] The album card is the cover, by ref, at `variant=medium`
- [ ] The description is the album's own text plus the count
- [ ] No caption or credit reaches any tag
- [ ] An empty album previews with the branded card, not a broken image
- [ ] `PUBLIC_ALBUMS=false` is unaffected (album pages 404 already)

## Progress Log

- 2026-09-28 — Created from PRD 026 §10 on approval.
