# 423 — A header and a footer for the public site

**Status:** done
**Priority:** medium
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

> *"We need a header and a footer, the header should be inspired by https://tilmelding.nathejk.dk/ — the moon
> icon exists in this repo as an svg, content of header should follow same width as content area. Header stays
> fixed at top even when scrolling. Footer stay at bottom of viewport when little content, but scrolls out of
> viewport when lots of content. Same color for header and footer (some kind of dark grey)."*

And four pieces of boilerplate to move into the footer: the patrol page's takedown form, "Tilbage til forsiden",
the general takedown line, and "Data og privatliv".

### The layout

```html
<header class="sitehead">   fixed, dark, full width; inner div carries the page's measure
<main>                      max-width 60rem, the page's own content
<footer class="sitefoot">   same dark bar, same measure
```

The body is a flex column with `min-height: 100vh` and a growing `main`. That is the whole sticky-footer
mechanism: bottom of the viewport on a short page, scrolled away on a long one. **Not** `position: fixed`, which
would take a strip of every screen for text nobody is reading yet.

The measure moved off the `body` and onto `main`, because the bars are full width and their contents are not.
The header's height and the body's `padding-top` are necessarily the same number: a fixed header is out of the
flow, so nothing else reserves its space.

### The moon is inlined, not linked

The master is `vue/src/assets/brand/nathejk-moon.svg`, and `.rules` forbids a website asset living under `vue/`.
So the path is transcribed into the template: one path, no request, cannot 404. The coordinates are the original
Bézier control points from the 2017 logo artwork and the colour is the one that directory's README settles on —
if either changes, that README is the source of truth and this is a copy to update.

### Moving the boilerplate needed two fields, not one

The footer is shared, and a Go template errors on a field the data does not have. So `ReportPath` and `Reported`
moved onto `publicPageData`, and the patrol handler sets them. `AtRoot` is set by the frontpage alone, so the
footer can omit the link back to itself — a positive flag set by the one handler that knows, rather than inferred
from `Title == ""`, which is true of the frontpage today and is a fact about its heading.

The footer renders the takedown invitation in exactly **one** of its two forms: the form where a page names where
it posts, the plain line otherwise. Both would read as though the first had not worked.

## Acceptance Criteria

- [x] Header fixed at the top, dark grey, moon and wordmark, contents on the content area's measure
- [x] Footer at the bottom of the viewport on a short page, scrolling away on a long one, same colour
- [x] All four pieces of boilerplate are in the footer, and nowhere else
- [x] The frontpage does not link to itself
- [x] Deep links still scroll their photograph clear of the fixed header
- [x] Every page wears both, asserted per page

## Progress Log

- 2026-09-25 — Built as described. Footer links get their own colour: the page's `#1d4ed8` on dark grey is not a
  link, it is a smudge.
- 2026-09-25 — `scroll-margin-top` on the album's figures, because a fixed header overlaps whatever a fragment
  scrolls to — task 401's `?foto=9#foto-9` would otherwise land its photograph under the bar.
- 2026-09-25 — **Four existing guards failed, and they were right to.** The moon is forty Bézier coordinates, so
  `10.4336`, `57.4219` and `45.5854` put "43", "42" and "5.5" on every page of the site — and three guards ask
  "does this page contain this string anywhere at all" about exactly those, to catch a patrol number leaking onto
  a closed page. `TestAClosedPageLeaksNothingAboutARealPatrol` had already been failed once by a hex colour and
  its comment argued for keeping the bluntness. Forty numbers changes that arithmetic: it stops being an
  occasional false positive and becomes a guard that fails for most two-digit patrol numbers whatever the page
  does.
- 2026-09-25 — Fixed with `withoutSVG`, which strips artwork before searching, and **licensed** by a new guard:
  `TestNoInlineSvgOnThePublicSiteCarriesATemplateAction`. The strip is only safe while marks are constants, so
  that property is now asserted rather than assumed — otherwise this would be a hole nobody remembers opening
  rather than an exception somebody argued for.
- 2026-09-25 — `TestEveryPublicPageWearsTheHeaderAndFooter` walks three pages and asks each the same questions,
  including that it carries exactly one takedown invitation. The failure mode of the old scattered arrangement was
  never a wrong page; it was a missing line on the fifth page somebody adds, which nobody notices because the
  other four are right.
- 2026-09-25 — Walked into the raw-string backtick trap twice more in one sitting, both times in a new CSS
  comment. That is eight for the repo. The compiler catches it immediately and points at a line of CSS, which is
  the only reason it stays cheap.
- 2026-09-25 — **Not done here, and worth stating:** `glimtpublic.go` renders its own self-contained page and
  does not use this layout, so `/2026/glimt` now looks different from every other public page — no header, no
  footer. That page was deliberately left out of the shared layout when it was built (see the comment on
  `publicSiteTemplates`), and migrating a shipped, tested public page inside a layout task is how a refactor
  becomes an outage. It is now the only page without the chrome, which makes it the obvious next task.
- 2026-09-25 — ✅ All criteria met. `gofmt`, `go vet`, full `go test ./cmd/api/` clean.
