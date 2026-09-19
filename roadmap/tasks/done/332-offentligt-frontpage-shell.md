# 332 — `/offentligt` frontpage shell

**Status:** done
**Priority:** high
**Created:** 2026-09-19
**Picked up by:** agent session (Zed)
**Started:** 2026-09-19
**Completed:** 2026-09-19

## Description

PRD 011 §7. The public frontpage: three sections in this order — **albums → find din patrulje →
glimt** — at `/offentligt`, with `/offentligt/glimt` (PRD 019) already living under it.

Order is deliberate. Albums first because they are the most inviting thing and need no input;
find-your-patrol second because the visitor who came for it arrived with a number in their hand; the
glimt strip last, as a strip with a link, because the full feed has its own page and does not need
reproducing.

**Server-rendered Go `html/template`, extending `go/cmd/api/glimtpublic.go`'s pattern.** Not a
preference: this page exists so someone on a ten-year-old browser can see what the weekend looked
like, and anything needing the Vue bundle to parse defeats the only thing it is for. Read
`publicGlimtPageHandler`'s comment before starting — the no-carousel decision, the inline CSS, the
thumbnail-always rule and the `noindex` reasoning all apply here and should not be re-litigated.

`html/template` escapes every interpolation, which matters because captions on this surface are
participant- or curator-authored text on an unauthenticated page.

**Relationship to `/desktop.html`** (PRD 013): that page is the event's *manual* — rules, programme,
practical, privacy. This is the event's *memory*. They link to each other and duplicate no content.

**"Find din patrulje"** takes a patrol number and only a number. It must work with no JavaScript — a
plain `<form>` with a `GET` — and it must answer identically for a patrol that does not exist and one
whose gate has not opened (task 330), or it becomes a live finish-order feed.

Scope note: this task is the shell and the frontpage itself. Albums are 333/334, the glimt strip is
336, the patrol page is 341.

## Acceptance Criteria

- [x] `GET /offentligt` renders the three sections in the stated order, with OpenAPI annotations
      (repo rule — see `glimtpublic.go` for the HTML-endpoint precedent).
- [x] Unauthenticated, and **ignores the session cookie entirely**: the route does not use the auth
      middleware, so a signed-in member sees exactly what a parent sees.
- [x] Complete with **JavaScript disabled** and readable with CSS disabled: semantic headings, real
      lists, real links.
- [x] "Find din patrulje" is a no-JS form; unknown number and not-yet-open answer identically.
- [x] `noindex, nofollow`.
- [x] Headlines use the Nathejk font (`.rules`). Note it must be reachable from outside the Vue
      bundle — if it is not, say so in the log rather than silently using a fallback.
- [x] Links to `/offentligt/glimt` and to `/desktop.html`; no content duplicated from either.
- [x] Empty states read as "not yet", never as an error or a blank region.
- [x] Renders on the oldest device available (iPad mini 2, iOS 12.5.8) — hex colours, no `oklch()`, no
      CSS nesting, matching task 204's constraints. *(Constraints held in the source; the device run
      itself is task 348.)*

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §7 / §8 / §10 (Phase 1).
- 2026-09-19 — Picked up. Read `glimtpublic.go` first, as the task asks. Its decisions carry over
  wholesale and are not re-argued: inline hex CSS, thumbnails always, no carousel, `noindex`, and the
  structural no-session property (routes registered bare, because `requireAuth` is the only place a
  session enters the request context).
- 2026-09-19 — **Built a shared layout rather than a fourth self-contained page.** The glimt page is one
  template with everything in it, which was right when there was one. There are now four pages
  (frontpage, album, patrol, not-yet) that must agree on the wordmark, the footer, the privacy link and
  the CSS — and the first thing to diverge would be the footer's takedown line, which is the one piece
  of text on this surface somebody needs when something is wrong. `publicSiteTemplates` holds a
  `layout-head`/`layout-foot` pair.
- 2026-09-19 — Deliberately did **not** migrate the existing glimt page onto the new layout. It works,
  it is tested, and rewriting a shipped public page to prove a point about template sharing is how a
  refactor becomes an outage. Noted in the code as where it should land if it is ever touched anyway.
- 2026-09-19 — The "find din patrulje" form needed a bridge: a no-JS form submits a query string, but
  the patrol page's address is a path (`/offentligt/patrulje/42`), chosen because it is memorable and
  sendable by voice. So `/offentligt/patrulje?nummer=42` does a **303** to the path form. It performs
  **no lookup at all** — an unknown number redirects exactly like a known one, because a validity check
  there would reintroduce the distinction the not-yet page exists to remove, on the one route a visitor
  can hammer with guesses. Asserted.
- 2026-09-19 — `/offentligt/patrulje/:number` is registered now and answers the not-yet page for every
  number. That is **correct rather than a stub**: the page itself is task 341, so there is nothing to
  show for anyone yet, and this is also what the route will answer for most of the year once it exists.
  The gate is deliberately *not* called yet — a call that looks like a check and then renders the same
  page whatever it said would be worse than no call, because the next reader would believe the route
  was gated. 341 adds the gate and the open branch as one change.
- 2026-09-19 — Chose **200 with a friendly page** for the closed answer, not 404 — diverging from the
  crew patrol lookup, which answers 404 so a refusal cannot be told from a nonexistent patrol. The
  reasoning differs because the audience does: there the caller is authenticated crew reading JSON, here
  it is a parent who followed a link, and "404" in a browser reads as *broken* and sends them to ask a
  leader why. Equal friendly answers achieve the same indistinguishability as equal errors. Asserted
  byte-identical across numbers, and asserted to carry no patrol data — **including the requested number
  itself**, since echoing it back is how "is 42 real?" becomes answerable by diffing two responses.
- 2026-09-19 — ✅ **Found and fixed a real bug outside this task's scope.** `vite.config.ts`'s
  `navigateFallbackDenylist` had `/^\/offentligt\//` — which does not match `/offentligt` itself. An
  installed member following a link to the new frontpage would have been served the app shell by the
  service worker instead of the page. Added the bare path.
- 2026-09-19 — ✅ **Extended the OpenAPI guard to the public site and it immediately earned its keep.**
  `isInScope` in `glimtopenapi_test.go` only covered paths containing "glimt", so the new routes were
  unchecked. Widening it to `/offentligt` and `/api/public/` caught two things I had wrong: the
  `public-site` tag was not in the agreed set (added, with a note on why it is separate from
  `glimt-public`), and `patrolSearchLookupHandler` was missing `@Produce` — correctly, since
  `http.Redirect` does write a small HTML body for a GET. The `@Failure`-agreement check passed for all
  three new handlers first time. Repo-wide widening remains task 328.
- 2026-09-19 — Font note, per the criterion asking for honesty rather than silence: the Nathejk font is
  loaded by the app's CSS, which this page deliberately does not load. So the wordmark and headings use
  the same Impact + narrow-bold stack that `--font-nathejk` itself falls back to. Stated in the template
  rather than left to look like an oversight; serving the webfont from here would mean a second request
  on the surface whose whole virtue is not needing one.
- 2026-09-19 — Rate limiting shares `publicGlimtReadLimiter` rather than adding a second budget: same
  anonymous audience, same origin, and the same NAT reasoning that made it IP-keyed and generous.
  Whether the site wants its own ceiling is task 347's question, which has the load numbers.
- 2026-09-19 — Verified by hand against the running dev stack: `/offentligt` renders, sections in order,
  empty states reading as "not yet". `go vet ./...` and `go test ./...` clean; frontend type-check and
  942 tests pass. Moving to done.
