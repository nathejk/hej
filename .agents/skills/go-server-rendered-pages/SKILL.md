---
name: go-server-rendered-pages
description: >
  Conventions for the HTML surfaces the Go service renders itself in the `hej` repo:
  the public website (`/{year}/…`) and the photographer admin tool (`/admin`).
  These are **not** the Vue PWA and do not share its stack — no Tailwind, no
  shadcn-vue, no bundler, no npm, no build step. Apply this skill when adding or
  editing a server-rendered page, its CSS or its JavaScript; when reaching for a
  third-party front-end library on those surfaces; or when a change touches the
  boundary between the PWA and the website. Trigger phrases: "the admin page",
  "the public site", "html/template", "server-rendered", "Pico", "htmx", "Alpine",
  "the album page", "the frontpage", "vendored", "no build step", "adminui".
---

# Server-rendered pages in the Go service

## Which surface am I on?

Decided by **where the file lives**, never by what you are building.

| Surface | Files | Stack |
|---|---|---|
| **The public website** | `go/cmd/api/publicsite.go` (+ `vue/public/publicmap.js`, see the wrinkle below) | Go `html/template`, hand-written CSS inline in the template, vendored Leaflet + MarkerCluster |
| **The admin tool** | `go/cmd/api/adminui/*`, `adminpage.go`, `adminalbumpage.go` | Go `html/template`, **Pico + htmx + Alpine** (vendored), real `.html`/`.css`/`.js` files |
| **The PWA** | `vue/` | **Not this skill.** Use `vue3-pwa-layout`. |

If you are in `vue/`, stop and use the other skill. If you are editing HTML that a Go handler renders, you are
here.

### One pre-existing wrinkle, so you do not copy it

The public website's map island (`publicmap.js`) and its Leaflet copy are served out of **`vue/public/`** — the
PWA project's folder. So today a website asset does live under `vue/`, which is exactly the coupling the hard
rule below is against.

That is history, not a pattern. **New website assets go in the Go service and are embedded** — see
`cmd/api/adminui/` for the shape. Untangling the existing two is worth doing and has not been done.

## The hard rule

> *"It's essential that we do not mix the pwa and the website, it's two different things and should be kept as
> such."* — maintainer, 2026-09-23

So, on these surfaces:

- **No Tailwind.** It needs a build step; the only Tailwind in the repo is inside the PWA's Vite build. Using it
  here means either a second pipeline or coupling the two things that must stay apart.
- **No shadcn-vue, no Vue, no components.** There is no bundler to build them with.
- **No `npm` dependency and no build step.** A page must render from the Go binary alone.
- **Nothing new added under `vue/`** for a website feature. (Two assets already are — see the wrinkle above.)

## Adding a third-party library

Vendored, pinned, committed, served from the binary. **Never a CDN** — a CDN would let a third party log who
looked at the event's photographs.

The pattern is `go/cmd/api/adminui/vendor/`: the file, plus `vendor.txt` (machine-readable pins) and
`README.md` (the same table, plus why each library is there). `TestTheVendoredAssetsMatchTheirPinnedVersions`
checks each file still announces the version claimed, so a re-download at a different release fails the suite.

**The committed file is the lockfile.** There is no `package.json` and no integrity hash, by design.

Currently permitted, and it is an allowlist rather than a licence —
`TestTheAdminToolAddsNothingToTheFrontend` fails on a fourth script:

| Library | Version | Served from | Why |
|---|---|---|---|
| Leaflet (+ MarkerCluster) | pinned in `vue/public/vendor/` | `/vendor/…` | the map island, shared by the public patrol page and the admin position picker |
| Pico CSS | 2.0.6 | `/admin/vendor/…` | classless base; chosen over Tailwind precisely because it needs no pipeline |
| htmx | 2.0.4 | `/admin/vendor/…` | server-rendered fragments instead of fetch-then-render JS |
| Alpine.js | 3.14.9 | `/admin/vendor/…` | local state and overlays without a build step |

The admin three are embedded in the binary and served behind `requireAdmin`, which costs them caching — every
`/admin/*` response carries `Cache-Control: no-store` (task 371), and keeping that rule true without exceptions
was judged worth more than saving ~180 KB for three curators.

## How a page is assembled (the admin tool)

Markup, CSS and JavaScript are **separate real files**, embedded and spliced:

```
cmd/api/adminui/
  page.html    ← markup, with /* @inject page.css */ and // @inject page.js markers
  page.css
  page.js
  album.{html,css,js}
  vendor/
```

`mustInjectAdminAssets` replaces each marker with the other file's **source** before `template.Parse`. Two things
follow, and both matter:

1. **`html/template` still parses one document**, so it still contextually escapes the markup's actions. Passing
   CSS or JS in as template *data* would need `template.CSS`/`template.JS` and throw that escaping away.
2. **A missing asset or an unmatched marker panics at init**, so an unassemblable page stops the binary rather
   than reaching a curator with no stylesheet.

### Never put a template action in a `.css` or `.js` file

`page.css` and `page.js` contain **zero** `{{…}}`, and `TestTheAdminAssetsCarryNoTemplateActions` keeps it that
way. Two reasons:

- an action inside `<script>` is escaped as *JavaScript* by `html/template`, which mangles values in ways nobody
  notices until a browser does something strange;
- it stops the file being something a formatter or linter can read, which is the whole reason these are real
  files rather than fragments.

**Pass values through `data-` attributes** on an element instead — `data-year`, `data-max-mb`, `data-album` —
and read them in JS. That was already the rule before the extraction and is what made it safe.

### The backtick trap (historical, do not reopen)

`publicsite.go` still holds its template in a Go **raw string**. A backtick anywhere in its HTML, CSS or JS
terminates the literal, and the symptom is a Go syntax error pointing at a line of CSS. This cost four
incidents in one session before the admin tool was extracted (task 394). If you must edit that template, do not
write a backtick in it — and if it grows, extract it the same way.

## Working with Pico

Pico styles **bare elements**; the pages' own CSS is almost entirely id- and class-scoped. So:

- **Pico is loaded first, the page's CSS second.** Pico supplies the base, local rules win where they exist.
  That is also the migration strategy: the hand-written CSS can shrink one rule at a time.
- Reconciliations are marked `PICO:` in the CSS, with the reason. Three exist so far and each is the kind of
  thing that is invisible until you look:
  - **Pico scales the root font size to 131.25%** on wide viewports. Every `rem` on a page designed at a 16px
    root inflates by a third on a laptop. Pinned with `:root { font-size: 100% }` — Pico uses `:where(:root)`,
    which has zero specificity, so no `!important` is needed.
  - **Pico's bare `a` rule underlines Leaflet's zoom controls**, which are anchors, not buttons. Reset scoped to
    `.leaflet-container`.
  - **Pico puts native `<dialog>` at `z-index: 999`.** Local overlays sit at 1040/1050 so a future real
    `<dialog>` cannot land above the scrim.

## Testing these surfaces

This is the weak spot, and knowing why saves time.

**There is no way to execute JavaScript that a Go template renders.** So many guards read the assembled page and
grep it. Use `adminPageSource(t)` — it assembles through the *same* function production uses, so a rule about
the page is tested against the page. Use `adminAsset(t, "page.js")` when a rule is about one language.

Two standing hazards with source-reading guards:

1. **A guard will match the comment explaining it.** This has happened four times in this repo. *Strip comment
   lines before searching* — see `foldBody` in `nathejk/table/album/membershipsafety_test.go`.
2. **A needle can match the code that removes the thing you are checking for.** A test for a cache-buster
   matched the regex that strips it, and passed over a broken retry. Assert the whole statement, not a fragment.

**Prefer a behavioural test when one is possible.** An htmx fragment endpoint returns HTML, which an ordinary Go
HTTP test can assert on — converting source-text guards into real ones is the main reason htmx is being adopted
here (task 395).

## Danish, and copy as substance

All user-facing copy is Danish. On these surfaces copy is sometimes *the feature*: PRD 022 §5 requires that
"remove from an album" and "delete from the library" cannot be confused, and the delete sheet's wording is the
mechanism, not decoration. Write those sentences deliberately; do not generate them.

## Privacy

The public website **names no human being**, with exactly one documented exception: a photographer's credit line
(task 393), which is text a curator typed and is never derived from the `person` projection. `isPersonShaped` in
`cmd/api/publicprivacy_test.go` enforces the rule and records the exception — if a new field trips it, except it
**by name with reasoning**, never by loosening the needle.

Guardian phone numbers appear on no surface here at all. See `.rules`.

## Related

- `.rules` → "Three surfaces, two frontends — do not mix them"
- `go-bff-layout` → handlers, routes, projections, the `cmd/api` entrypoint
- `vue3-pwa-layout` → anything in `vue/`
- PRD 011 (public frontpage), PRD 022 §7 and §8.1 (why the admin tool is not in the PWA)
