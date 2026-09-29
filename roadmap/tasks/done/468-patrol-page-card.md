# 468 — The patrol page's share card is its diploma

**Status:** done
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

**PRD:** 026

## Description

A parent sharing their patrol's page is the most likely share on the site, and the diploma is what they are proud of.
`og:image` is `/api/public/patrol/{number}/diploma/thumb`, which after task 463 looks like a diploma.

The title is the patrol as the page already names it; the description is the finish or participation sentence, which
is the same `diploma.sentences` distinction the certificate makes — a patrol the backstop opened must not be
described as having finished.

Note what the thumbnail does **not** carry: any photograph of anybody. The stand-in pictures an empty backdrop, so
this card puts no child's face on Facebook's CDN even though the album cards deliberately do.

#