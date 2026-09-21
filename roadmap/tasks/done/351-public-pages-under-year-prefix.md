# 351 — The public pages under /2026, and desktop visitors sent there

**Status:** done
**Priority:** high
**Created:** 2026-09-21
**Picked up by:** agent
**Started:** 2026-09-21
**Completed:** 2026-09-21

## Description

PRD 021 §0 (Phase −0), plus a bug the maintainer hit:

> *"when i visit / i'm redirected to /desktop.html saying more to come. This is more change redirect to
> /2026"*

Two things, and they are the same thing from two directions. The public site built by PRD 011 was at
`/offentligt`, and the app's desktop/unsupported gate sent visitors to `/desktop.html` — a static placeholder
reading *"more to come…"*. So the most memorable URL we own answered a parent with a dead end while the site
they wanted sat one directory away.

**Move the pages to the event-year prefix** (`/2026`, the interim step PRD 021 chose over the full root move)
**and point the app's gate at them.** The full restructure — public site at `/`, app at `/app`, two binaries —
stays deferred to a quiet month; this carries none of its service-worker risk.

## Acceptance Criteria

- [x] The public pages are served under the configured event year: `/2026`, `/2026/patrulje/{number}`,
      `/2026/album/{slug}`, `/2026/glimt`, `/2026/patrulje/{number}/anmeld`.
- [x] Every `/offentligt*` path answers **301** to the same page under the new prefix, query string included.
- [x] Links inside the pages are built from the prefix rather than written out, so the next move is one change.
- [x] An unknown path under a year prefix — including a year this deployment does not serve — gets the public
      site's own 404, never the app shell.
- [x] The app's desktop/unsupported gate leaves for the public site instead of the placeholder.
- [x] The service worker's navigation denylist covers the new prefix, by shape rather than by this year's
      number.
- [x] The two guards that parse `routes.go` still cover every public route, and say so loudly if they cannot
      read one.
- [x] Verified in the dev stack through the Vite path a browser actually takes.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-21 — **Done.** The pages are at `/2026`, `/offentligt*` 301s to them permanently, and the app's
  gate sends a desktop visitor to the public site.

  ### Why the routes are built as strings

  httprouter **panics** if a wildcard segment is a sibling of static ones, so `/:year` alongside `/api`,
  `/offentligt` and `/privatliv` is not expressible. The prefix is therefore interpolated from
  `config.eventYear` at registration time (`app.publicRoot()`), which has a useful consequence: this
  deployment serves exactly one year, and a request for another year's prefix is a page we do not have rather
  than one improvised from this year's data.

  ### `/offentligt` is an alias, not just legacy debt

  It stays as a permanent 301 because those URLs went round family group chats — but it is also **how the app
  names the public site**. The bundle cannot know the event year: that is runtime configuration, and the
  desktop decision is taken during the router's first navigation, before anything is fetched. So the app
  points at `/offentligt` and the server owns the mapping. Recorded at both ends, because a frontend constant
  pointing at a legacy path looks like a mistake otherwise.

  ### Three traps, two of which were live

  **1. The SPA fallback would have eaten the misses.** `router.NotFound` answers anything unmatched with
  `index.html`, so `/2026/patruljer` or a link to `/2025/patrulje/42` would boot the app — which on a desktop
  now redirects straight back to the public site. A **loop**, and the same shape as the bug task 332 shipped
  through the service worker's denylist. Paths whose first segment is year-shaped now get the public 404,
  matched by shape so next year needs no change.

  **2. `/privatliv` in the footer was a link into the app.** I had flagged it in PRD 021 §2 as a dead end; by
  pointing the desktop gate at the public site I would have **promoted it to a loop** (public page → app →
  public page). So the site now has its own privacy page at `/2026/privatliv`, with the wording lifted from
  `PrivacyView.vue` rather than rewritten. The two are not duplication to delete: the app's page explains what
  the app does with a member's *own* data to somebody signed in who can act on it, and this one explains what
  the *public pages* show to somebody who is not a member. PRD 021 §11 Q3 asks which owns the shared parts.

  **3. `/desktop.html` in the footer** pointed at the placeholder the maintainer complained about. Link
  removed rather than redirected: PRD 013's content (rules, programme, practical) has no home on the public
  site yet, and a link to "more to come…" is worse than no link. The file itself stays, because PRD 014's dev
  device simulation boots through it.

  ### The guards caught three real regressions, which is the whole reason they parse the source

  Both `glimtopenapi_test.go` and `publicprivacy_test.go` read `routes.go` to find public routes. The move
  broke all three of their assumptions, and each failure would otherwise have been silent:

  - **`stringLit` only accepted literals.** `publicRoot + "/patrulje/:number"` is a `BinaryExpr`, so every
    public page route would have vanished from both guards — the privacy walk reporting success over an empty
    list. Now resolved, and an unparseable path **fails loudly** rather than being skipped.
  - **`isInScope` / `isPublicSurface` matched `/offentligt`.** `/2026/patrulje/{number}` matched neither, so
    the annotation check would have kept only the glimt page (which matches on the word "glimt"). Both
    predicates now also match a year-shaped prefix.
  - **`handlerName` only recognises names ending in `Handler`** — a convention I broke by calling the redirect
    `legacyPublicRedirect`, which made the route invisible. Renamed, and the convention is now documented as
    enforced rather than observed.

  Two smaller guard improvements fell out: `@Router` is read as *all* its lines, since one handler legitimately
  answers two addresses (the bare alias and the catch-all); and a handler with no error branches passes only if
  it documents no `@Failure`, which keeps the stale-annotation check while allowing a pure redirect to exist.

  ### Verified in the dev stack, through Vite

  Not against the api's internal port but through the path a browser takes, since the Vite proxy needed a
  second prefix:

  | request | result |
  |---|---|
  | `/offentligt` | 301 → 200 |
  | `/offentligt/patrulje/71` | 301 → 200 |
  | `/2026`, `/2026/patrulje/71` | 200 |
  | `/2026/ingenting` | 404, public page |

  And every link the frontpage renders points at `/2026/…` — no `/offentligt`, no `/privatliv`, no
  `/desktop.html`.

  ### Left undone, deliberately

  - The Vite dev proxy hardcodes `/2026` (a proxy key cannot be a pattern). Noted in the config: next year,
    either update it or use `/offentligt` in dev and let the redirect do the work.
  - PRD 011 §6 still describes the addresses as `/offentligt/…`; updated there as part of this change.
  - The full restructure remains PRD 021, unstarted, gated on its Phase 0 device test.

  `gofmt`, `go vet`, `go test ./...` clean; `npm run type-check` and 944 frontend tests pass.
