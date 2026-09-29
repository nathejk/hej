# 464 — The branded share card

**Status:** done
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

PRD 026's first piece of work, and its open question §11.1: the frontpage's share preview is a neutral branded card,
and there was no such artwork in the repo. The poster is portrait A4 and the app icons are square; `og:image` wants
1200×630.

The maintainer settled it:

> I don't have a branded artwork at hand - create a black image, with logo moon and title (NATHEJK) taking up most
> of the space. Then i might come with a better solution later on, but this is it for now

So this is explicitly a placeholder, built to be replaced.

## What shipped

`internal/sharecard.Render(poster, w, h)` — a black 1200×630 PNG carrying the crescent and the wordmark across most
of its width — plus `GET /{year}/share-card.png` with full OpenAPI annotations, rendered once behind a `sync.Once`
like the diploma thumbnail.

### It lifts the marks off the event poster rather than typesetting them

Drawing "NATHEJK" needs a font rasteriser — `golang.org/x/image`, which `internal/imaging` refuses for reasons that
apply here too — or seven hand-built letterforms that would look hand-built. And the obvious font is the one already
embedded next door: `impact.ttf`, whose licence permits **embedding a subset in a document** and says nothing
friendly about anything else. Rasterising it into a PNG served over HTTP is at best an argument, and I would rather
not make it in a placeholder.

The poster already has both marks, in the right shapes, as pixels we ship. They separate on colour alone: saturated
yellow is the moon, flat black is the wordmark, everything else is parchment and is dropped. A side effect worth
having is that the card cannot drift from the year's artwork — the same property `Thumbnail` has.

### Everything about it was got wrong once, and looking is what found it

This is the part worth reading. Every step of the extraction was written, tested green, rendered, and **wrong**:

| Attempt | What the card showed | Why |
|---|---|---|
| Hand-measured crop | `NATUEIK` | The band I measured by eye took the top half of the letters |
| Projection profile at 0.4% ink | a black rectangle, 0% ink | The parchment's cracks and the dark smudge along the poster's top edge tripped the row test, so the band began at y=0 |
| `inkMaxLuma = 70` | a bright arc ringing the moon | The moon's drop shadow is luma 53–64 and was repainted white |
| Loose moon test | brown dots in the corner | Tan specks in the parchment passed as "yellow" |

Every one of those produced a valid PNG of the right size. The fix in each case came from **measuring the poster
instead of guessing at it** — the letters are `0,0,0`, the moon is `255,255,1`, its shadow is `58,52,52`, the
parchment is `214,196,152` — and those numbers are now in the code and in
`TestTheThresholdsMatchThePoster`, so the next person re-measures rather than guesses.

The band is **located** by a projection profile rather than hardcoded, which is what makes next year's artwork a
non-event.

### The guard that makes the approach acceptable

The whole thing rests on an assumption about one JPEG, and the failure mode is silent: a wrong crop is a valid image,
and the only place anybody would notice is a Facebook post days later, after Facebook has cached it. So `Render`
**checks its own output** — a band yielding implausibly little or implausibly much ink is an error, not a card — and
the tests assert the result by pixel proportions (mostly black, some yellow, a wordmark spanning 85% of the width and
28% of the height) rather than against a golden image that would fail on every harmless change and tell nobody
whether the result was still right.

Mutation-checked both ways: clipping the search band fails three tests with the right messages, and loosening the ink
threshold fails on the moon's shadow.

## Acceptance Criteria

- [x] A black 1200×630 card with the moon and NATHEJK across most of it
- [x] No new dependency, no font licence question
- [x] Served at its own route, unauthenticated, cacheable, `noindex`
- [x] Rendered once per process
- [x] OpenAPI annotations
- [x] A card that cannot be produced is an error rather than a plausible wrong card
- [x] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-09-28 — Four rendered-and-looked-at iterations, each one green before I looked. Recorded in full above,
  because the lesson is not about this card: **an image-producing function cannot be tested by asserting that it
  produced an image.**
- 2026-09-28 — Not wired into any page yet. The `og:` tags are the rest of PRD 026, which is still in `draft/`
  awaiting approval.
