# 394 — The admin tool's markup, CSS and JavaScript become real files

**Status:** done
**Priority:** high
**Created:** 2026-09-23
**Picked up by:** agent
**Started:** 2026-09-23
**Completed:** 2026-09-23

## Description

Maintainer, weighing a rewrite:

> *"the admin page is very heavy in custom javascript and css — i'm weighting other options: 1. HTMX +
> tailwind (+ maybe Alpine.js) 2. other?"*

After measuring, the decision was **B then D**: first get the three languages out of Go and into real files
(this task), then migrate to **HTMX + Alpine + Pico**, all vendored, no build step (task 395).

This is B. It is deliberately a *source-layout* change and nothing else: the served bytes are the same, the
delivery is the same, no dependency was added, and it is reversible in one commit.

## What was actually wrong

Not the line count. Two specific things:

**The backtick trap.** The page was one Go raw-string literal, so a backtick anywhere in the CSS, JS or HTML
terminated it. **Four incidents in one working session**, each presenting as a Go syntax error pointing at a
line of CSS — `syntax error: unexpected literal ` rule rather than four near-identical…``. Quoting an
identifier in a comment, the most natural thing to write when explaining something, broke the build.

**No editor support for any of the three languages.** No highlighting, no formatting, no linting, no
go-to-definition inside 1,400 lines of JavaScript.

And a third, which this task improves but does not solve: **38 tests assert on source text**, because there is
no way to execute JavaScript that lives in a Go string.

### The measurement that shaped the plan

| | adminpage.go | adminalbumpage.go |
|---|---|---|
| Total | 2063 lines | 418 |
| Hand-written JS | 1280 | 114 |
| Hand-written CSS | 237 | 39 |
| Go around the template | 126 | 138 |

And, by whether a framework would actually replace it: ~400 lines of upload queue (**no** — HTMX cannot express
a bounded concurrent queue with per-file progress), ~250 lines of selection model (**partly**), ~150 lines of
Leaflet (**no**), ~600 lines of sheets, forms and fetch-then-render (**yes** — that is D's target).

## What landed

| | Before | After |
|---|---|---|
| `adminpage.go` | 2063 | **207** |
| `adminalbumpage.go` | 418 | **162** |

```
cmd/api/adminui/
  page.html   240    album.html    96
  page.css    252    album.css     39
  page.js    1436    album.js     129
```

Embedded with `go:embed`. The `@inject` markers in each HTML file are replaced with the other two files'
**source** before parsing.

### The detail that makes it safe

Splicing *source*, not data. `html/template` still parses one document, so it still contextually escapes the
fourteen actions in the markup. Passing the CSS and JS in as template values would have required
`template.CSS`/`template.JS` and thrown that escaping away — on a page where the escaping is a deliberate
safety property, documented in the handler since task 373.

### The finding that made it clean

**The CSS and JS contain zero template actions.** Checked before touching anything, and now asserted. Every
value the script needs is read from a `data-` attribute on an element — which was already the rule this page
followed, stated in its own doc comment.

That is what makes `page.css` and `page.js` *real* files rather than fragments in a new location: they are
exactly what the browser receives, so a formatter or a linter can read them. Had one `{{.Year}}` been in there,
the extraction would have produced two files that look like CSS and JavaScript and are neither.

### The tests got better, not just relocated

The 22 guards that read `adminpage.go` now call **`adminPageSource(t)`**, which assembles the page through the
*same* `mustInjectAdminAssets` the production template is built from. So a rule about the page is tested
against the page.

That is a real improvement, not a lateral move: those guards used to grep **Go source**, which meant a needle
could match a doc comment — passing against the prose explaining a rule rather than the code implementing it.
That failure mode bit three times in this session, including once while writing this task's own guard.

New guards, each break-tested:

| Sabotage | Caught by |
|---|---|
| `{{.Year}}` appended to `page.js` | *"page.js contains a template action"* |
| the CSS marker renamed in `page.html` | panic at init |
| `<div>` put back in a Go file | *"adminpage.go contains `<div>`"* |

Plus `TestAMissingAdminAssetPanics`: a missing asset or an unmatched marker panics at **init**, so an
unassemblable page stops the binary instead of reaching a curator with no stylesheet on the Tuesday after the
event. The marker-mismatch case is the subtle one — the file exists, the splice silently does nothing, and the
page loses its styling with no error anywhere.

## Verified live

Both pages, after a rebuild:

| | |
|---|---|
| `/admin` | 200, 58,466 bytes, 0 surviving markers, `<title>Billedarkiv 2026 — Nathejk` |
| `/admin/album/ved-maalet` | 200, 10,384 bytes, 0 surviving markers, `<title>Ved Målet 2026 — Billedarkiv 2026` |

Spot-checked in the served output: `id="scrim"`, five `class="sheet"`, `function openSheet(`,
`.sheet button:disabled`, `data-act="credit"`, `<h2>Album</h2>`, `attachTileRetry(` — i.e. markup, CSS and JS
all arriving from their new homes, and the year still interpolating.

## Acceptance Criteria

- [x] The markup, CSS and JavaScript live in real files, embedded
- [x] No Go file contains markup, CSS or JavaScript — asserted, not merely done
- [x] `html/template`'s contextual escaping of the markup's actions is preserved
- [x] `page.css` and `page.js` are valid standalone files, and a template action in either fails the suite
- [x] A missing asset or an unmatched marker fails at init rather than at the first request
- [x] The served output is unchanged on both pages, verified live
- [x] The suite is green with no assertion weakened to accommodate the move

## Deviation: the JavaScript is not split per feature

The plan said B would "split into per-feature files". It did not, deliberately, and this is the one place the
task departs from what was agreed.

`page.js` is a **single IIFE whose twelve sections share one closure**. Measured: `selected`, `load()`,
`openSheet()`, `closeSheet()` and the `sheet` element are each referenced from five to seven sections. Splitting
that leaves two options:

1. **Concatenate the files in a declared order in Go.** Each file then references identifiers it does not
   declare, so none of them is valid standalone JavaScript — which defeats the main purpose of this task. It
   also reintroduces a build step in disguise, and an order-dependent one.
2. **A full ES-module refactor**, with an explicit dependency graph and `<script type="module">`. Genuinely
   better, and no build step needed at this browser baseline.

Option 2 is the right end state. It was not done *now* because **task 395 deletes roughly 600 of these 1,400
lines** — the five action sections become HTMX fragments — so converting them to modules first is work thrown
away on code that is about to go. D produces the per-feature split as a by-product, with each file genuinely
independent rather than independent-looking.

So: one valid `page.js` today, per-feature split falls out of 395. Flagged with the maintainer rather than
quietly decided.

## Notes

- **A guard matched its own explanation, for the third time this session.**
  `TestTheAdminHandlersCarryNoMarkup` failed against the doc comment that explains *why* the rule exists — the
  paragraph about `html/template` escaping actions inside `<script>`. Fixed by stripping comment lines, the same
  repair task 386's `foldBody` needed. This is now a reliable pattern: **any guard that searches for forbidden
  text will find it in the prose forbidding it.** Worth reaching for the comment-stripping helper by default.
- While rewriting the doc comment I left the **old paragraph in place alongside the new one**, producing a
  duplicated "one Go-template hazard" section. Caught on reading the file back, not by any test. Comment
  duplication is invisible to the suite.
- `publicsite.go` still carries its template inline (878 lines) and has the same backtick exposure. Out of
  scope here — it is the public website, not the admin tool, and it is much smaller in JS. Worth the same
  treatment if it grows.
