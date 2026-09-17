# 323 — /offentligt/glimt — the public page on hej.nathejk.dk

**Status:** done
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:** agent
**Started:** 2026-09-17
**Completed:** 2026-09-17

## Description

PRD 019 §0, §7, §8. The public feed is served **by this service**, on `hej.nathejk.dk` — not a
separate site and not an export into PRD 013.

A server-rendered page at `/offentligt/glimt`: a plain thumbnail grid with the hold attribution
(number, name, group) under each entry. No login, no bundle, no client-side state, working
without JavaScript. Plus:

- `GET /api/public/glimt` and `GET /api/public/glimt/:glimtId/media/:ordinal`
- `POST /api/public/glimt/:glimtId/report` — **unauthenticated**, rate-limited by IP. A parent
  who spots a problem is precisely the person we want to hear from, and they have no session.

Two same-origin traps, both of which must be handled rather than discovered:

1. **The service worker's navigation fallback would swallow the URL** and serve the app shell to
   an installed member following a public link. Needs the `navigateFallbackDenylist` entry
   (task 315).
2. **A logged-in member's browser will send `hej_session`** to these handlers. They must
   **ignore it entirely**. If they ever read it, the public page silently becomes a different
   page for members than for parents, and "is this public-safe?" becomes untestable.

Serves `audience = public AND hiddenAt IS NULL` and nothing else, regardless of who asks. No
personal name, no portrait, no arm number, no phone number — ever.

## Acceptance Criteria

- [x] `/offentligt/glimt` server-rendered, works with JavaScript disabled
- [x] **Multiple media are reachable without a swipe and without JavaScript** — one `<img>` per item
- [x] Public API endpoints serve only public, non-hidden glimt
- [x] **Test: an authenticated member's cookie changes nothing about the response**
- [x] Test: a hidden glimt disappears from the public page
- [x] Test: the payload contains no personal name, `personId` or phone number
- [x] Unauthenticated report endpoint, IP rate-limited
- [x] `navigateFallbackDenylist` entry present and verified in the built `sw.js`
- [x] Passes `app.glimtPublicCutoff()` into `PublicFeed` — a zero time there serves the whole archive to the open web
- [x] `go test ./...` passes

## ⚠️ Multiple media on a page with no JavaScript

Raised by the maintainer on 2026-09-18 while reviewing the feed: *"since the public version needs to be
accessible on a desktop computer, we need some way to move the carousel without swiping."*

The app's own strip was fixed for desktop by showing prev/next buttons where the pointer is fine
(task 316). **That fix does not carry here.** `GlimtMediaStrip.vue` is a Vue component driven by Embla,
and this page is server-rendered with no bundle — so there is no carousel to add arrows to.

The options, and the recommendation:

| approach | verdict |
|---|---|
| **Show every item, stacked or in a grid** | **Recommended.** No JavaScript, no gestures, no controls to discover, works in every browser including the very old ones this page exists for (PRD 013). A glimt has at most ten items and the page is already thumbnail-first. |
| CSS scroll-snap + `#anchor` links | Works without JS and looks like the app. But anchor navigation scrolls the *page*, and getting it to move only the strip needs care per browser — a lot of subtlety for a public page whose whole virtue is that it is dumb. |
| A small inline script | Rejected: the page must work with JavaScript disabled, so this would be a second code path that only some visitors get. |

**Do not port the carousel.** The reason the public page is server-rendered is that it must work on a
browser that cannot parse the app's bundle; a carousel is exactly the sort of thing that pulls a bundle
back in.

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-18 — Added the no-JavaScript media criterion and the analysis above, from the maintainer's
  desktop-access point. Also added the `glimtPublicCutoff` criterion — task 310 built the read-time
  public retention window and this is the only caller that must pass it; a zero time would silently
  serve the whole archive to the open web.

- 2026-09-17 — Implemented in `go/cmd/api/glimtpublic.go` with `glimtpublic_test.go` (19 tests) and
  `vue/src/publicPageNotSwallowed.spec.ts`.

  **Took the recommended option for media: every item gets its own `<img>`.** No carousel, no
  scroll-snap, no script. A glimt has at most ten items and the page is thumbnail-first, so a
  four-photograph glimt is four tags in a `grid-template-columns: repeat(auto-fit, ...)` — side by
  side where there is room, stacked where there is not. The rejected alternatives are already
  recorded above and the reasoning held up: the whole virtue of this page is being dumb.

- 2026-09-17 — **Trap 2 turned out to be answerable structurally, which is much better than
  "the handlers ignore the cookie".**

  `requireAuth` is the **only** place a session enters the request context (`middleware.go`). So a
  handler registered without it does not "choose not to read" the session — `contextGetSession`
  returns false unconditionally, because nothing put one there. The public routes are therefore
  registered bare, and that registration *is* the guarantee.

  Guarded from three directions, because the guarantee rests on an assumption a future change could
  quietly break:

  1. `TestPublicGlimtRoutesAreNotBehindAuth` (in the task 312 guard) reads the AST and fails if any
     public route is wrapped in `requireAuth`, **and** if any handler in the chain calls
     `contextGetSession` or touches `app.sessions`. The second half is what still fires if somebody
     ever adds a global session middleware and half 1 stops protecting anything.
  2. `TestPublicGlimt_AnAuthenticatedCookieChangesNothing` issues a real session and asserts the
     JSON *and* the HTML are byte-identical with and without it.
  3. The 401 check in the annotation guard is now conditional on the route actually being
     authenticated — and fails in the other direction too: a 401 documented on a public route
     implies it looks at a session.

- 2026-09-17 — Decisions worth keeping:

  - **404, not 403, for a non-public glimt.** The authenticated media route answers 403 deliberately
    — there the caller is a known member and an honest refusal tells them the photo was not shared
    with them. Here the caller is anonymous, so a 403 would confirm that a glimt with this id exists
    and is not public: worth nothing to a parent and something to somebody probing.
  - **`publiclyVisible` re-checks the audience on the media route**, which does not go through
    `PublicFeed` because it fetches one glimt by id. Without it, an id alone would be an
    unauthenticated read of a group-scoped photograph of a child. Tested with a **valid member
    session attached**, because that is the case a reader would assume is fine.
  - **`public` caching here, `private` there.** `streamGlimtMedia` now takes the header value as a
    parameter rather than the public route wrapping the ResponseWriter to overwrite it — the
    authenticated route's `private` is a privacy property and belongs at its own call site, not
    somewhere a reader has to prove nobody downstream changed. There is a test that the
    authenticated route *stayed* private, since the two now share a function.
  - **The anonymous reporter is a sentinel, not an IP.** `glimt_report` is keyed by reporter, so an
    anonymous report needs *a* value — and recording the IP would put a personal identifier of the
    one participant in this feature who never agreed to anything into an append-only audit table, to
    solve a duplicate-counting problem that does not matter. The honest cost: several anonymous
    reports of the same glimt collapse into one row, so the count under-reports. The glimt is hidden
    on the first one anyway. `"public"` also reads usefully in the moderation queue — a report from
    the open web has different weight to one from a participant.
  - **One query for both surfaces** (`app.publicGlimt`). Two would be two chances for one of them to
    forget the cutoff or the hidden filter, and the HTML one is the copy nobody would think to test
    for a leak.
  - **`robots: noindex`.** The photographs were shared publicly by their authors, which is not the
    same as asking to be findable by name in a search engine in five years.
  - **No pagination**, and the page says so implicitly by showing the most recent 60. A "load more"
    is either script (forbidden here) or an offset link, which is an unbounded surface for a crawler.
    Somebody after a specific patrulje belongs in the app, where the hold collection exists.
  - **`Cache-Control: public, max-age=60`** on both the page and the JSON: shareable, because the
    response is identical for every caller by construction — but short, because a takedown has to
    land in minutes, which is the entire purpose of the report endpoint.

- 2026-09-17 — On the service worker: **the denylist entry was already there** (`/^\/offentligt\//`,
  added with task 315) but undocumented, so it read as arbitrary. Documented, and now tested.

  It deserves the test because **it fails asymmetrically**: a parent following a shared link has no
  service worker and gets the real page, so the bug is invisible to everyone who would naturally
  check it. The only people who see it are installed members, who tap a public link and get the app
  shell — and they are the population least likely to be testing whether the public page works.
  Nothing else in the build would notice; removing the entry produces a service worker that works
  perfectly for every other route.

  The spec asserts it in the config *and* in the built `sw.js`, which are different claims — a
  Workbox option can be renamed or ignored and the config would still read correctly. The built check
  is `skipIf` when `dist/` is absent, so a build-less run does not fail; the build runs in the same
  pre-commit command as the tests.

- 2026-09-17 — Two new by-IP limiters (`GLIMT_PUBLIC_READS_PER_MINUTE`, default 3000;
  `GLIMT_PUBLIC_REPORTS_PER_HOUR`, default 30), separate from the member-keyed ones. An IP is a much
  worse key — a school or a parents' group behind one NAT shares a single budget — so the read
  ceiling has to be far more generous than the member one, and mixing them would drag the
  member-keyed limit down to match.

  `publicHoldLabel` **duplicates** the frontend's `attributionLine`, and that is named in the code
  rather than hidden: there is nothing to share it through, because the TypeScript version lives in a
  bundle this page deliberately does not load. If the wording changes in one place it must change in
  both.

  Go suite, `gofmt`, `go vet`, 893 Vue tests, `type-check` and `build` all clean.

  **Not opened in a browser**, and two things want a human eye: whether the page reads well on a
  phone and on a desktop, and the Danish. There is also **no link to this page from anywhere** — not
  from the app, not from nathejk.dk. That is a deliberate stopping point rather than an omission,
  since where a public link belongs is a decision for whoever owns the site.
