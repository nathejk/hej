# 424 — The wordmark drops the year and sits on the moon's baseline; the glimt page joins the layout

**Status:** done
**Priority:** medium
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

> *"In the header it should just say Nathejk, no year — the bottom of the moon and the foot of the font need to be
> aligned on one horizontal line. Header/footer should also apply to public glimt page."*

### The wordmark

The year is gone from the bar; the `<title>` still carries it, which is where a year is useful.

The alignment is a one-word change with a reason worth recording. A flex item with no baseline of its own — an
SVG — has one **synthesised from the bottom of its box**, so `align-items: baseline` puts the bottom of the
crescent on the same line as the feet of the letters. `center`, which is what it had, leaves the moon floating
half a letter high.

The moon is taller than the caps, and that is correct rather than an accident: in the source artwork its height
is the distance from the top of the mark down to the wordmark's baseline, so it rises above the word by
construction (`vue/src/assets/brand/README.md`). Its `viewBox` is the path's tight bounding box, so the box *is*
the artwork and its bottom edge is the lowest point of the crescent — which is what makes the alignment exact
rather than eyeballed.

### The glimt page joins the shared layout

Task 423 left it out and said why: `publicSiteTemplates`' own comment had recorded that this page *"works, it is
tested, and rewriting a shipped public page to prove a point about layout sharing is how a refactor becomes an
outage. If it is ever touched for another reason, this is where it should land."*

Being asked for the header and footer here is that reason. So:

- `publicGlimtPageData` embeds `publicPageData`, which is what carries the year, title and base path the layout
  needs;
- the handler renders through `renderPublicPage`, so the page now also gets the `X-Robots-Tag` header that the
  meta tag carried alone;
- its five page-specific CSS rules moved into the shared stylesheet, and its **duplicate** `.intro` and `.empty`
  went — those two had been defined twice in the service since the page was built;
- the standalone 4.4 KB template and its private FuncMap are gone. `hold` and `date` were already in
  `publicSiteFuncs`, which is how the duplication was found.

**On task 362's removal of the disclaimers from this page.** The maintainer took the takedown line, the retention
period and the privacy link off it because *"it's already stated elsewhere and it seems very overwhelming with
all these disclaimer everywhere"*. The shared footer does not reverse that judgement, it answers it: the
complaint was repetition on a page that is just photographs, and one footer on every page of the site is the
opposite arrangement — which is exactly what task 423 was for.

## Acceptance Criteria

- [x] The header says "Nathejk", with no year
- [x] The bottom of the moon and the baseline of the wordmark are one horizontal line
- [x] `/2026/glimt` wears the same header and footer as every other public page
- [x] The glimt page has no stylesheet, template or funcs of its own
- [x] Its existing tests pass unmodified

## Progress Log

- 2026-09-25 — Both done. `align-items: baseline` and `height` on the mark rather than `width`, since the viewBox
  is the tight bbox.
- 2026-09-25 — The glimt page's migration turned up two duplications that had been invisible while it had its own
  template: `.intro` and `.empty` defined twice in the service, and a private FuncMap repeating `hold` and `date`
  from `publicSiteFuncs`. Neither was wrong, and both are the ordinary cost of a page that renders itself.
- 2026-09-25 — `TestEveryPublicPageWearsTheHeaderAndFooter` and `TestPublicSitePagesAreNotIndexed` gained
  `/2026/glimt`. The second is the interesting one: that page had the robots *meta tag* but not the *header*,
  because the header comes from the shared renderer it was not using. A migration that only moved markup would
  have left that quietly unchanged.
- 2026-09-25 — Every existing glimt test passed without modification, including the one asserting no script of any
  kind on the page: the layout adds a header and a footer, and the viewer is not loaded there.
- 2026-09-25 — Backtick number nine, in a new CSS comment naming two class selectors. The compiler catches it
  instantly; the habit is the problem, not the trap.
- 2026-09-25 — ✅ All criteria met. `gofmt`, `go vet`, and `go test ./...` across every package clean.
