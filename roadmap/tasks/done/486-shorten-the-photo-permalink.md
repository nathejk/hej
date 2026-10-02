# 486 — Shorten the photograph permalink to a 12-character ref

**Status:** done
**Priority:** medium
**Created:** 2026-10-01
**Picked up by:** agent session
**Started:** 2026-10-01
**Completed:** 2026-10-01

**Follows:** task 447 (the permalink and its shape decision), task 456 (refs in public HTML)

## Description

Raised by the maintainer: *"the permalink to a single photo in a specific album is very long — is it possible to
encode the ref in a different encoding (still url safe) or shorten it while still being unique?"*

Today a permalink carries the whole content hash, 64 hex characters:

```
https://nathejk.dk/2026/album/natten/foto/3fa7b2c491d0e8a7b2c491d0e8a7b2c491d0e8a7b2c491d0e83f2a9b41
```

**It only needs to be unique within the album**, and that is the whole reason this is cheap.
`albumPhotoPermalinkHandler` does not hand the ref to the blob store — it scans the album's own items:

```go
for _, it := range items {
    if it.Ref != ref { continue }
```

So the collision domain is one album's items — tens to a few hundred — not the archive. The full 256 bits are doing
almost nothing in that URL.

## The decision (maintainer, 2026-10-01)

A **12-character lowercase hex prefix** of the ref.

```
/2026/album/natten/foto/3fa7b2c491d0
```

Base62 at 8 characters was costed and declined. It carries 47.6 bits against hex-12's 48 — the same safety — but it
buys only 4 characters out of a ~50-character URL, because `https://nathejk.dk/2026/album/natten/foto/` is 42 fixed
characters and the ref has stopped being what makes the link long. Against that it loses three things:

- the short form is no longer a **prefix** of the ref, so back-compat needs a decoder rather than `HasPrefix`, and a
  curator can no longer check a shared link against the full ref in the admin tool by eye;
- `0-9a-zA-Z` is **case-sensitive**, so "capital B or lowercase b?" becomes a real failure for a link read aloud or
  retyped — and these are sent between parents;
- it keeps the confusable characters `0/O` and `1/l/I`.

Crockford base32 (10 chars, case-insensitive, confusables folded) was the better big-alphabet option and makes the
point: 10 against 12 is not worth a new encoding. If more characters are wanted later, `/foto/` → `/f/` is five of
them at no risk.

**Why 12 and not 8.** Collision odds within one album, birthday bound:

| prefix | bits | 200 photos | 4000 (a year's library) |
|---|---|---|---|
| hex[:8] | 32 | 4.7e-06 | **1.9e-03** |
| hex[:10] | 40 | 1.8e-08 | 7.3e-06 |
| **hex[:12]** | **48** | 7.1e-11 | **2.8e-08** |

8 characters is roughly one event in 500 producing an ambiguous link: rare enough never to appear in testing, common
enough to happen to somebody. That is the worst place on the curve to sit.

## Requirements

- [x] R1 — The page mints `/{year}/album/{slug}/foto/{ref[:12]}`.
- [x] R2 — **The 64-character form keeps resolving, forever.** Task 447 set this precedent for `?foto={ordinal}`:
      *"keeps resolving — those links are already in people's chat histories — but the page stops minting them."*
      Same rule, same reason. Any length between the two resolves as a prefix as well, so a hand-trimmed or
      line-wrapped link does something sensible.
- [x] R3 — **Ambiguity is detected, never guessed.** If more than one live item in the album shares the prefix, the
      handler redirects to the **album**, which is the behaviour already designed for a half-rotted link (PRD 023 §8,
      "not found is not an error"). Silently choosing one of two matches is the single unacceptable outcome: a wrong
      photograph that looks right, with nothing anywhere saying so.
- [x] R4 — A short ref **never reaches the blob store**. `blob.Ref.Valid()` demands 64 hex so it would be refused
      rather than mishandled, but that is a backstop and not a boundary — PRD 022 §8.4's narrowed invariant gets an
      explicit guard.
- [x] R5 — The media route (`/api/public/albums/{albumId}/media/{selector}`) is **unchanged**. It is
      `immutable`-cached and never read by a human, so shortening it is a different question about HTML bytes. Its
      `albumItemIs` stays an exact compare, so a prefix cannot match there by accident.
- [x] R6 — Nothing about confidentiality changes, because there was none to lose: task 447 records that a ref is the
      SHA of bytes any visitor can download. The publication check still does the real work.

## Acceptance Criteria

- [x] A minted permalink carries 12 hex characters
- [x] A 64-character permalink still resolves to the same photograph
- [x] An intermediate-length prefix resolves
- [x] An ambiguous prefix lands on the album, and a test constructs a real collision rather than assuming one cannot
      be built
- [x] A ref that is not in the album still lands on the album
- [x] A guard asserts the permalink path cannot pass a short ref to the blob store
- [x] The media route's selector matching is unchanged and rejects a prefix
- [x] Full gate clean: `gofmt`, `go vet`, `GOWORK=off go test ./...`

## Progress Log

- 2026-10-01 — Task created with the decision recorded.
- 2026-10-01 — Done. A shared link goes from **114 to 62 characters**, −46%:
  `https://nathejk.dk/2026/album/loerdag-morgen/foto/766ec78fe2c3`
- 2026-10-01 — `albumPhotoPermalink` mints `ref[:12]` via `shortPhotoRef`; the handler matches a prefix of **any**
  length, which covers the 12-character form, the 64-character form already in chat histories, and anything between —
  one matcher rather than a short-form parser beside a long-form one.
- 2026-10-01 — The match is **case-insensitive** (`strings.EqualFold`). Not in the original plan: a ref is lowercase
  hex, but URLs travel through mail clients and link previewers that change case, and silently not matching is
  indistinguishable from a taken-down photograph. Lowercase hex has no case-collision risk, so this costs nothing.
- 2026-10-01 — Ambiguity: the loop does **not** break on the first hit. A second match discards the first and falls
  through to the album, which is where a half-rotted link already lands (PRD 023 §8). Logged at warn, because an
  ambiguous prefix means 12 characters has stopped being comfortable for this library's size — an operational fact
  nobody would otherwise learn, since the visitor just sees the album.
- 2026-10-01 — `photoRefHasPrefix` is a function rather than an inline `strings.HasPrefix`, and the reason worth
  keeping is the empty case: `HasPrefix(x, "")` is **true**, so a naive match would make `/foto/` resolve to the
  album's first photograph. httprouter will not route an empty parameter today, so it guards a future router change —
  and the failure it prevents is silent.
- 2026-10-01 — **The case that would actually have bitten, and it was not in the plan.** `albumMediaHandler` shares
  `albumItemIs` over the same items, reaches the blob store, and is cached `immutable` for a year. Loosening *that* to
  a prefix for consistency would mean an ambiguous prefix pinning whichever photograph came first, for a year. R5
  leaves it an exact compare and `TestTheMediaRouteRefusesAPermalinkPrefix` now holds the asymmetry — verified by
  making `albumItemIs` prefix-match and watching it fail.
- 2026-10-01 — R4's boundary: the permalink handler is read back and must not name `app.blobs`, `blob.Ref`,
  `readBlob` or `streamGlimtMedia`. `blob.Ref.Valid()` would refuse a 12-character ref anyway — and *that is why the
  guard exists*: the backstop working is what would make a lost invariant quiet. PRD 022 §8.4.
- 2026-10-01 — A test pins that lowering `albumPermalinkRefLen` is unsafe while raising it is fine: a link in
  somebody's chat history cannot grow characters it was never given, so a shorter setting would make
  previously-unambiguous links ambiguous. Verified by setting it to 8 and watching two assertions fail.
- 2026-10-01 — ✅ All three new guards checked against their own failure before being trusted. `gofmt`, `go vet`, full
  `go test ./...` green.
