# 467 — The album's and the frontpage's share cards

**Status:** done
**Priority:** high
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

**PRD:** 026

## Description

The frontpage's card is the branded one (task 464). An album's is **its cover**, addressed by ref with
`?variant=medium` — 800px, the rendition nearest the size a card is rendered at and already what the cover grid uses
(task 461).

Title is the album's title; description is the album's own description plus the photograph count. **Never a caption
and never a credit**: those are the two free-text fields on this surface and a credit is the one that names somebody
(task 393).

An album with **no cover emits no `og:image`** and falls back to the branded card — never a URL that 404s.

#