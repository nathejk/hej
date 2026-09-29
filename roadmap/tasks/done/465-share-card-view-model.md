# 465 — The shareCard view model, and the tags in layout-head

**Status:** done
**Priority:** high
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

**PRD:** 026

## Description

PRD 026's core. Every public page must state its own preview — title, description, image, alt — and the tags must be
emitted from one place so a page cannot half-declare itself.

`publicPageData` gains a `shareCard` (title, description, image URL, alt) and `layout-head` renders `og:type`,
`og:site_name`, `og:locale` (`da_DK`), `og:url`, `og:title`, `og:description`, `og:image`, `og:image:alt`, plus
`twitter:card` and `twitter:image`.

**The defaulting is the load-bearing part.** A page that says nothing must get the frontpage's branded card and a
sensible title, never an absent or broken `og:image` — the same fail-safe shape `RobotsPolicy()` has, and for the
same reason: a field every handler must remember to fill fails in the direction nobody notices.

#