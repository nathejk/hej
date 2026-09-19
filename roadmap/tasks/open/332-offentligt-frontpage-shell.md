# 332 — `/offentligt` frontpage shell

**Status:** open
**Priority:** high
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `GET /offentligt` renders the three sections in the stated order, with OpenAPI annotations
      (repo rule — see `glimtpublic.go` for the HTML-endpoint precedent).
- [ ] Unauthenticated, and **ignores the session cookie entirely**: the route does not use the auth
      middleware, so a signed-in member sees exactly what a parent sees.
- [ ] Complete with **JavaScript disabled** and readable with CSS disabled: semantic headings, real
      lists, real links.
- [ ] "Find din patrulje" is a no-JS form; unknown number and not-yet-open answer identically.
- [ ] `noindex, nofollow`.
- [ ] Headlines use the Nathejk font (`.rules`). Note it must be reachable from outside the Vue
      bundle — if it is not, say so in the log rather than silently using a fallback.
- [ ] Links to `/offentligt/glimt` and to `/desktop.html`; no content duplicated from either.
- [ ] Empty states read as "not yet", never as an error or a blank region.
- [ ] Renders on the oldest device available (iPad mini 2, iOS 12.5.8) — hex colours, no `oklch()`, no
      CSS nesting, matching task 204's constraints.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §7 / §8 / §10 (Phase 1).
