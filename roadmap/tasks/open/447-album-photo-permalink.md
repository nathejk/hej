# 447 — A durable link to one photograph in an album

**Status:** open
**Priority:** low
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

## Description

Raised by the maintainer while agreeing PRD 024: *"link today is honest. but we might need to think about a
permalink solution."*

A link into an album addresses a **position**, so after a resort it points at a different photograph. That
is honest rather than broken — "the twelfth photograph" is a different photograph once the album is
re-sorted — and PRD 024 ships on that basis (§4, §11 Q3). But a family who sends somebody *"look at this
one"* means a particular photograph, and position links cannot carry that.

Out of scope for PRD 024 deliberately, and recorded so it is not lost. Not started: it needs a decision
about what a public photograph's address should be before any code. Things to weigh when it is picked up:

- the photograph's id is a **content hash** (PRD 022 §8.3), which is durable and also means the same
  photograph uploaded twice has one address — the right property for a permalink;
- but an id in a public URL is a guessable handle to a photograph in an unpublished album unless the read
  checks publication, which is exactly what `showAdminPhotoMediaHandler`'s projection check exists for on
  the admin side (task 382);
- and PRD 011 §0b.1's privacy posture has to hold for whatever the URL exposes.

## Acceptance Criteria

- [ ] A decision recorded on what a public photograph's address is
- [ ] (then) whatever that decision implies

## Progress Log

- 2026-09-28 — Task created from PRD 024 §11 Q3. Not scoped further on purpose: the decision comes first.
