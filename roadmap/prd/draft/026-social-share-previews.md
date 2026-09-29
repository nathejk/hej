# PRD 026 — Social share previews for the public site

**Status:** draft
**Author:** agent session (2026-09-28)
**Created:** 2026-09-28
**Last updated:** 2026-09-28
**Approved:**
**Shipped:**
**Target users:** families and the open web (the public site's visitors); curators, indirectly, through the cover they choose

---

## 1. Summary

When somebody pastes a Nathejk link into Facebook, Messenger, iMessage or a group chat, decide what the preview
card says and shows — per surface, in Open Graph markup the server controls — instead of letting the scraper guess.
The frontpage gets a neutral branded card; an album and a photograph in one get **the album's cover**; a patrol page
gets **its diploma**.

## 2. Problem & Motivation

**What problem does this solve?** The public site carries no Open Graph markup at all. Facebook therefore guesses,
and its guesses are poor in a way nobody can see from our side:

| | What Facebook does today |
|---|---|
| Title | the `<title>` element — "Test · Nathejk 2026" |
| Description | nothing, or whatever text it scrapes first |
| Image | **its own pick from the page** — on an album page, one of the photographs |

**Why now?** Three things make this live rather than theoretical:

- The viewer has had a **share button** since task 405, and task 447 gave a photograph a durable permalink. We
  built the paths that put these links into chats.
- The frontpage is the one page deliberately *in* the search index (task 427) — it is meant to be found and
  therefore meant to be passed around.
- A parent sharing their patrol's page is the single most likely share on the whole site, and the diploma is the
  thing they are proud of. Right now that card shows whatever Facebook scraped.

**Evidence.** Maintainer, 2026-09-28: *"if a user want to share an album or a photo on facebook, is there anyway
that we can control what that would look like on facebook and maybe add some additional descriptions?"* — and then,
having weighed the alternative, the decisions in §6.

Note the status quo is **not** neutral. A scraper choosing its own image from an album page already copies a
photograph of children onto Facebook's CDN; it just does it unpredictably. Controlling the tag is what makes the
choice ours.

## 3. Goals

- Every public page states its own preview: a deliberate title, a description, and an image we chose.
- A share of an album leads somebody to **that album**, looking like the album.
- A share of a patrol page shows **its diploma** — the artefact the share is about.
- A share of the frontpage is branded and carries no photograph of anybody.
- Curators can see, in the tool, that the cover they pick is the picture the world sees, and what that means.

## 4. Non-Goals

- **A preview per photograph.** `/{year}/album/{slug}/foto/{ref}` is a 302 to the album (task 447), and scrapers
  follow it and read the album's tags. That is the intended outcome, not a limitation to work around: a shared
  photograph shows its album's card and lands on the photograph. Giving a single photograph its own card would mean
  the permalink rendering HTML instead of redirecting — a worse permalink for a better thumbnail.
- **Twitter/X-specific artwork.** `twitter:card` is emitted so the layout is right; nothing beyond that.
- **Dynamic, per-album generated cards** (title composited onto artwork). Possible later; the album's own cover is
  what the maintainer asked for and needs no renderer.
- **Changing the robots policy of any page.** Sharing and indexing are separate decisions and stay separate.
- **oEmbed, `schema.org`, or structured data for search.** Different job, different PRD.

## 5. User Stories & Scenarios

- As a **parent**, I want to post our patrol's page in the family group so that relatives see the diploma and the
  time they finished.
- As a **visitor**, I want to send an album to somebody so that they see which album it is before they tap.
- As a **curator**, I want to know which photograph becomes the public face of an album so that I choose it
  deliberately.

**Primary happy path.** A parent opens `/2026/patrulje/138`, taps share in the browser, pastes into Facebook.
Facebook fetches the page as `facebookexternalhit`, reads `og:title` ("Patrulje 138-5 — Ulvene, 5. Herlev" or
whatever the page already prints), `og:description` (the participation or finish sentence), and `og:image` (the
diploma thumbnail). The card shows the diploma. Clicking it goes to the patrol page.

**Edge cases**

- **An album with no cover** (no live items) — no `og:image`; the card falls back to title and description. It must
  not emit a URL that 404s, which is worse than no image.
- **A patrol page opened by the backstop** (did not finish) — still has a diploma (task 360), with the
  participation wording. So the card is the same shape; only the sentence differs.
- **A closed patrol page** — already 404s before any of this; nothing to preview.
- **`PUBLIC_ALBUMS=false`** — album pages 404, so no album previews exist. The frontpage card is unaffected.
- **A takedown after a share.** Facebook has cached the image on its own CDN by then and our removal does not reach
  it. This is the accepted cost; see §6 Non-Functional.

## 6. Requirements

### The decisions (maintainer, 2026-09-28)

> If people share frontpage we will serve a neutral branded card. If people share an album or a photo in an album,
> we serve the album cover image - we can add some descriptions in the admin pages for how to choose cover photo and
> what to be aware of. We also have the patrulje pages, here we will serve the diploma.

| Surface | `og:image` | `og:title` | `og:description` |
|---|---|---|---|
| Frontpage `/{year}` | neutral branded card | event + year | what the site is |
| Album `/{year}/album/{slug}` | **the album's cover** | the album's title | the album's description + "N billeder" |
| Photo `/{year}/album/{slug}/foto/{ref}` | (302 → the album's card) | — | — |
| Patrol `/{year}/patrulje/{number}` | **the diploma thumbnail** | the patrol, as the page names it | the finish or participation sentence |

This **reverses** the recommendation put to the maintainer, which was a branded card everywhere on the grounds that
`og:image` hands out a copy that outlives a takedown (the "a link, never the bytes" principle in `viewer.js`, task
405). Recorded because the reasoning on both sides should survive:

- **What the principle protects** is the takedown path. A shared *link* stops working when a photograph comes down;
  a copy on Facebook's CDN does not.
- **Why the maintainer decided otherwise** for albums: the album cover is already a **curated** choice — the one
  photograph an organizer picked to represent the album — and PRD 011 §0b.2's consent gate is upstream of it. A card
  with no picture is also a card nobody clicks, which defeats the point of a public gallery. The bound is that it is
  *the cover*, never an arbitrary photograph, and never a photograph the visitor's scraper picked.
- **The patrol page's diploma** carries no photograph of anybody at all in the shared thumbnail — it is the
  artwork, and after task 463 an empty stand-in in the photo box. The patrol's *name* is on the page already.

### Functional

- [ ] A shared `layout-head` block emits `og:type`, `og:site_name`, `og:locale` (`da_DK`), `og:url`, `og:title`,
      `og:description`, `og:image` plus `twitter:card` and `twitter:image`, from data every page provides.
- [ ] Absolute URLs. `og:url` and `og:image` must be absolute; a relative value is silently ignored by Facebook.
- [ ] The frontpage serves a **branded card** asset, embedded in the Go binary, at its own route.
- [ ] An album page's `og:image` is its cover, addressed by **ref** with `?variant=medium` — 800px, which is the
      rendition closest to the 1200×630 a card is rendered at, and already the one the cover grid uses (task 461).
- [ ] An album with no cover emits **no** `og:image`.
- [ ] A patrol page's `og:image` is `/api/public/patrol/{number}/diploma/thumb`.
- [ ] `og:image:alt` on every one of them, and it names no person.
- [ ] `og:description` is **never** built from a caption or a credit. Those are the two free-text fields on this
      surface and a credit is the one field that names somebody (task 393).
- [ ] The curator's tool explains what the cover is for, next to where it is chosen.

### Non-Functional

- **Privacy.** No `og:description`, `og:title` or `og:image:alt` may contain a person's name; the album's cover is
  the only photograph any card carries. `TestNoStructInTheLibraryOrTheAdminToolNamesAPerson` and the public privacy
  walk must be extended to the new view-model fields rather than sidestepped.
- **Accepted, and written down:** a cover photograph shared to Facebook is copied to Facebook's infrastructure and
  our takedown has no reach there. This is the cost of the decision above, not an oversight. The curator-facing text
  must say so plainly — it is the one place somebody can act on it, by choosing a different cover.
- **The robots policy does not change.** Album and patrol pages stay `noindex, nofollow`; the frontpage stays
  `index, follow, noimageindex`. A scraper fetching `og:image` ignores all of it, which is precisely why the
  decision above had to be made deliberately.
- **Caching.** Facebook caches scraped metadata for days. Wording changes need their Sharing Debugger to re-scrape;
  the image URLs are content-addressed, so a changed cover is a changed URL and needs nothing.
- **No new front-end dependency, no build step.** These are meta tags in the Go template (`go-server-rendered-pages`).

## 7. UX / UI Notes

Nothing renders for a visitor — the whole feature is in `<head>`. Two visible surfaces all the same:

- **The curator's album editor card** (`adminui/fragments.html`, the `albumeditor` template) gains a short
  explanation beside "Gør til forsidebillede": that the cover is what the frontpage shows, what a shared link shows
  on Facebook, and that a photograph shared onward is copied to Facebook and cannot be recalled from there — so pick
  one the patrol would be happy to see passed around, and prefer a wide shot over a close-up of one child's face.
  Danish, as everything in that tool.
- **The photographers' guide** (`docs/billedarkiv-for-fotografer.md` §4) gains the same point in one paragraph,
  where the cover is already explained.

## 8. Technical Considerations

- **Frontend (Vue 3 / TS):** none. This is the server-rendered public website, a different surface from the PWA
  (`.rules`, "Three surfaces, two frontends").
- **BFF (Go):**
  - `publicsite.go` — a `shareCard` struct on `publicPageData` (title, description, image URL, alt), rendered by
    `layout-head`. Defaulting matters: a page that says nothing must get the **frontpage's branded card**, not a
    broken URL — the same fail-safe shape as `RobotsPolicy()`.
  - `albumpage.go` — the album page and the frontpage fill it; `publicAlbumSummary` already carries `CoverRef`
    after task 461, so the album page needs the cover's ref for its own album (it currently has the items, so the
    cover rule must be applied consistently — `pickCover` already exists for exactly this).
  - `patrolpage.go` — fills it from the patrol and the verdict.
  - An absolute-URL helper. Scheme from `r.TLS`/`X-Forwarded-Proto` as `adminTransportOK` already does, host from
    `r.Host`, both with the comment that header is only trustworthy because our proxy is the only route in.
    **Preferred over a configured base URL**, which can silently be wrong in production and break every preview at
    once, while a wrong `Host` can only come from a request that was already sent somewhere odd.
- **API endpoints:** one new route, `GET /{year}/share-card.png` (or `/share-card.png` — see §11), serving the
  embedded branded card. **OpenAPI annotations required**, as for every endpoint in this service.
- **Data / storage:** none. No new columns, no events.
- **Dependencies & risks:**
  - The branded card artwork does not exist yet and must be supplied or generated (§11).
  - Facebook's scraper must be able to reach the page unauthenticated — it can; `robots.txt` has no `Disallow`.
  - `og:image` should be ≥ 200×200 and is rendered at ~1.91:1. The album cover is a **square** crop in our grid but
    the stored rendition is the photograph's own aspect; Facebook will crop it to the card. Nothing to do, worth
    knowing before somebody reports "the cover is cropped oddly on Facebook".

## 9. Success Metrics

- A share of each of the three surfaces produces a card with our title, our description and our image — checked
  in Facebook's Sharing Debugger once, per surface, after deploy.
- No card contains a person's name, verified by the privacy walk rather than by looking.
- Zero pages emit an `og:image` that 404s (the empty-album case).

## 10. Rollout / Task Breakdown

No feature flag. The tags are inert for anybody who is not a scraper, and the failure mode of getting one wrong is a
worse preview rather than a broken page. Sequence matters only in that the diploma stand-in should land before the
patrol card is announced, so the first shared diploma does not show an empty box.

- [x] Task 463: a stand-in start photograph in the diploma thumbnail, so it looks like a diploma
- [x] Task 464: the branded share card, rendered from the poster, with a route and OpenAPI annotations
- [ ] Task: `shareCard` on `publicPageData`, rendered in `layout-head`, defaulting to the branded card
- [ ] Task: the absolute-URL helper, with the proxy-header reasoning recorded
- [ ] Task: the album page's and frontpage's cards, including the no-cover case
- [ ] Task: the patrol page's card, from the diploma thumbnail
- [ ] Task: extend the public privacy walk over the new fields
- [ ] Task: curator-facing explanation of the cover, in the album editor card and the photographers' guide

## 11a. Settled during the work

- **The diploma PDF does not change.** Confirmed by the maintainer, 2026-09-28: *"a patrulje with no photo should not
  have an empty photo in their pdf, the pdf's stays as they are today."* The stand-in photograph is the thumbnail's
  only, which is what the patrol page's `og:image` will point at.

## 11. Open Questions

1. ~~**What is the branded card?**~~ **Answered (maintainer, 2026-09-28):** *"create a black image, with logo moon
   and title (NATHEJK) taking up most of the space. Then i might come with a better solution later on, but this is it
   for now"* — built in task 464 by lifting the crescent and the wordmark off the event poster, since those marks
   already exist as pixels the binary ships and typesetting them would need either a font rasteriser or an argument
   about Impact's licence. Explicitly a placeholder; replacing it is a decoded asset and no route change.
2. ~~**Where does it live?**~~ **Decided while building it:** `/{year}/share-card.png`, year-scoped, because the
   artwork it is lifted from is the year's poster. Say so if you would rather it sat at the origin root.

   Was: `/share-card.png` at the origin root, or `/{year}/share-card.png`? The year matters if
   the artwork changes yearly, which the poster does — so the year-scoped path is probably right, matching
   `background-2026.jpg`.
3. **Should the album card's description include the photograph count?** "42 billeder" is factual and makes the card
   more informative; it also goes stale in Facebook's cache when the album grows. My suggestion: include it — a
   stale count on an old share is harmless, and the alternative is a card that says less.
4. **Anything for the privacy page and the "not yet" patrol page?** They currently get the default. The frontpage
   card seems right for both, but say so if you would rather they carried nothing.
