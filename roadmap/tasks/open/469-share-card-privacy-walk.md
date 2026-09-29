# 469 — Extend the public privacy walk over the share card

**Status:** open
**Priority:** high
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

**PRD:** 026

## Description

The share card adds person-shaped *risk* to the public view model: a title, a description and an alt text, all
free-ish strings rendered into `<head>`. PRD 026's non-functional requirement is that none of them may carry a
person's name.

`isPersonShaped` already walks the public page types. The new fields must be walked rather than excepted, and the
guard that matters is the negative one: **no album caption or credit may reach a tag**, because those are the two
strings on this surface that could contain a name.

## Acceptance Criteria

- [ ] The walk covers the new view-model fields
- [ ] A test that a fixture credit — `fixtureCredit`, the one name the public surface may carry — appears nowhere in
      any page's meta tags
- [ ] No new exception added to `libraryPersonShapedExceptions` or the public list without reasoning at the line

## Progress Log

- 2026-09-28 — Created from PRD 026 §10 on approval.
