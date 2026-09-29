# 465 — The shareCard view model, and the tags in layout-head

**Status:** open
**Priority:** high
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

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

## Acceptance Criteria

- [ ] One template block emits every tag, from data
- [ ] A page that sets nothing still previews correctly, with the branded card
- [ ] `og:image` is absolute (task 466)
- [ ] No tag is emitted empty — an empty `og:description` is worse than none
- [ ] A test that walks every public page and asserts each one carries a complete card

## Progress Log

- 2026-09-28 — Created from PRD 026 §10 on approval.
