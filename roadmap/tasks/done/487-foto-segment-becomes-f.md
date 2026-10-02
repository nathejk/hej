# 487 — `/foto/` becomes `/f/` in the photograph permalink

**Status:** done
**Priority:** low
**Created:** 2026-10-01
**Picked up by:** agent session
**Started:** 2026-10-01
**Completed:** 2026-10-01

**Follows:** task 447 (the permalink and its shape), task 486 (the 12-character ref)

## Description

The last three characters, taken after task 486 had already done the significant work. Offered in 486's write-up as
"five characters at no risk" and then asked for.

```
447   114  …/2026/album/loerdag-morgen/foto/766ec78fe2c398816215e583258353a5903e33d53a712e5d37d05a2abecb9e34
486    62  …/2026/album/loerdag-morgen/foto/766ec78fe2c3
487    59  …/2026/album/loerdag-morgen/f/766ec78fe2c3
```

**48% shorter than the original across the two tasks**, of which task 486 is 46 points and this is 2.

## What this costs, stated because it is not nothing

`foto` was never arbitrary. `albumPhotoPermalink`'s doc gave it two justifications and this task keeps one and
spends the other:

- **Kept, because it is mechanical.** Some literal segment is *required*: the admin album editor sits at
  `{year}/album/:slug/edit` under the same root, and httprouter panics at registration on a wildcard sibling of a
  literal segment, so `/album/{slug}/{ref}` cannot exist. `f` satisfies that as well as `foto` did.
- **Spent.** `foto` matched the vocabulary of the `?foto={ordinal}` parameter the route redirects onto, so the
  address and its destination read as the same idea. `/f/ → ?foto=` is a slightly weaker connection, and the
  payment is made in a comment rather than in the URL.

Three characters for a small loss of legibility in the code. Worth it only because the URL is the thing a family
sees and the comment is not.

## Requirements

- [x] R1 — The page mints `/{year}/album/{slug}/f/{ref[:12]}`.
- [x] R2 — **`/foto/{ref}` keeps resolving, forever.** The third time this rule has applied on this one address —
      after `?foto={ordinal}` (447) and the 64-character ref (486) — and it is the same rule each time: an address we
      have handed out keeps working, and the page stops minting it. A `/foto/` link is in mail threads and chat
      histories right now.
- [x] R3 — One handler serves both segments, not two. A second copy would be a second place for the ref matching to
      drift, and the point of the alias is that the two addresses mean exactly the same thing.
- [x] R4 — **No redirect from the old segment to the new one.** That would put a second hop on every link already
      shared, and the two are equals rather than one being the canonical form of the other.
- [x] R5 — Both paths documented on the handler's OpenAPI block, which already supports several `@Router` lines.

## Acceptance Criteria

- [x] A minted permalink uses `/f/`
- [x] `/foto/` with 12 characters resolves
- [x] `/foto/` with the full 64-character ref resolves — the oldest shape, from task 447
- [x] `/f/` with the full ref resolves, which nothing ever minted but a hand-edited URL can produce
- [x] A guard fails if a segment this route has ever minted stops being registered
- [x] Full gate clean: `gofmt`, `go vet`, `GOWORK=off go test ./...`

## Progress Log

- 2026-10-01 — Task created and done in one sitting; it is three characters and an alias.
- 2026-10-01 — The segment is now a constant, `albumPermalinkSegment`, because it is minted in `albumpage.go` and
  registered in `routes.go`. Those two disagreeing produces links that 404 — the better failure of the two available,
  but still one nobody notices until somebody clicks a share.
- 2026-10-01 — **Both segments register the same handler**, and `/foto/` is not redirected to `/f/`. A redirect would
  add a hop to every link already out there and would imply one address is the real one; they are equals.
- 2026-10-01 — Rewrote `TestAlbumPhotoPermalinkLandsOnThePhotograph` to cover **every address this route has ever
  minted** rather than adding a case per shortening: `/f/` + 12, `/foto/` + 12, `/foto/` + 64, plus `/f/` + 64 which
  nothing minted but a hand-edited URL produces. A test per shortening would let the oldest shape quietly stop being
  checked, which is exactly the shape that is hardest to get back.
- 2026-10-01 — Added `TestEverySegmentThePermalinkHasMintedStillResolves`, aimed at a specific future edit: somebody
  tidying the alias away once `/f/` has been live a while and `/foto/` feels historical. It is not historical to the
  family whose mail thread holds one. Verified by deleting the `/foto/` registration — two tests failed, one of them
  naming the reason.
- 2026-10-01 — ✅ `gofmt`, `go vet`, full `go test ./...` green.
