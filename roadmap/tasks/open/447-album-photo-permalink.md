# 447 — A durable link to one photograph in an album

**Status:** open
**Priority:** medium
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

## The decision (maintainer, 2026-09-28)

> all public photos exist in an album, permalink should reflect this: `/<year>/album/<slug>/<ref>`

So a photograph's public address is **its album plus its ref**, and the two facts that makes true are the point:
every public photograph is in an album, and a ref does not move when the album is re-sorted.

### Why the ref is safe to put in a public URL

It is the **SHA of the bytes the media route already serves**: `photoId = stored.Ref = blobs.Put(prepared.Full
.Bytes)`, and the public full variant is those same bytes. Any visitor who downloads a photograph can compute it.
It is not a secret, cannot be made one, and publishing it therefore costs nothing in confidentiality.

What must stay true is that a ref is **not a capability**. It addresses a photograph *within a named, published
album*, and the read checks that — the same check the ordinal route makes today. A ref that addressed bytes
directly would mean a photograph seen once in a published album stayed reachable after it was unpublished, which
is precisely what `showAdminPhotoMediaHandler`'s projection check exists to prevent on the other side.

### What it replaces, and what it must not break

Today's share link is `?foto=<ordinal>` (task 405), and the ordinal is exactly the thing PRD 024 made unstable.
The established rule around it must carry over, because it is better than the obvious alternative:

> **Not found is not an error.** […] a link that has half-rotted — because the photograph it pointed at was taken
> down — should land on the album rather than on an error page. — `albumSideHolding`, PRD 023 §8

So `/<year>/album/<slug>/<ref>` where the ref is not (or is no longer) in that album **lands on the album**, not
on a 404. A takedown must not make an album look deleted. With a path segment rather than a query parameter, that
is a redirect to `/<year>/album/<slug>` rather than rendering the album at an address that no longer names a
photograph.

`?foto=<ordinal>` keeps resolving — those links are already in people's chat histories — but the page stops
minting them.

### And it fixes a bug: see task 456

The same insight applies to the media route, which is addressed by ordinal *and* served
`immutable, max-age=1y`. After PRD 024 that is a promise the server cannot keep. Task 456 has the detail; the two
should be done together, because they are one change of mind about what identifies a photograph.

## Acceptance Criteria

- [x] A decision recorded on what a public photograph's address is
- [ ] `/<year>/album/<slug>/<ref>` lands on the window holding that photograph
- [ ] A ref not in the album redirects to the album, never 404 (PRD 023 §8)
- [ ] The viewer's share action mints the new form
- [ ] `?foto=<ordinal>` still resolves
- [ ] A test that re-sorting an album does not change a photograph's address

## Progress Log

- 2026-09-28 — Task created from PRD 024 §11 Q3. Not scoped further on purpose: the decision comes first.
- 2026-09-28 — Decision recorded. Scoping it turned up task 456, which is the same mistake in the media route and
  is a live defect rather than a missing feature.
