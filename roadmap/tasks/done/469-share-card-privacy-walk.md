# 469 — Extend the public privacy walk over the share card

**Status:** done
**Priority:** high
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

**PRD:** 026

## Description

The share card adds person-shaped *risk* to the public view model: a title, a description and an alt text, all
free-ish strings rendered into `<head>`. PRD 026's non-functional requirement is that none of them may carry a
person's name.

`isPersonShaped` already walks the public page types. The new fields must be walked rather than excepted, and the
guard that matters is the negative one: **no album caption or credit may reach a tag**, because those are the two
strings on this surface that could contain a name.

#