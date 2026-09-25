# 402 — The shared photo viewer: one file, two surfaces, no build step

**Status:** open
**Priority:** high
**Created:** 2026-09-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 023 §6 ("Functional — the viewer"), §7.2, §7.3 and §7.7. The viewer itself, as its own vanilla JS and
CSS, used unchanged by the public album page and by the curator's admin tool.

**This is the task that must not be split** (§10). A half-wired viewer is a broken click: a tile whose
`href` has been intercepted by a script that cannot yet render anything is worse than the plain page it
replaced. Everything below ships together.

### What to build

```
go/cmd/api/viewer/
  viewer.js     ← no template actions, ever (the rule that already governs adminui/*.js)
  viewer.css
```

Plus `go/cmd/api/viewer.go`: `//go:embed viewer/viewer.js viewer/viewer.css` and a handler at
`/viewer/{asset}` that resolves the name through a **fixed map**. Never by joining the URL parameter to a
directory — that is one encoded `../` from serving this service's templates out of the embedded filesystem,
which is the reasoning `serveAdminVendorHandler` already records in `adminpage.go`. Copy its shape, not just
its outcome: unknown name → `NotFoundResponse`, content type from the map.

The route sits **outside `/admin`** on purpose (§6, Non-Functional): it is served `public, max-age` long with
a version in the path, so the admin tool's `no-store` rule (task 371) stays true without an exception.

Dev needs a `'/viewer'` key in `vue/vite.config.ts`'s proxy, beside `/api`, `^/\d{4}(/|$)` and `/admin`, for
the reason that file's comment already gives: without it Vite answers with the SPA shell and the asset looks
broken rather than unrouted. **That is dev routing, not an asset under `vue/`** — nothing here goes into
`vue/` otherwise.

### The shape it has to have

- **Native `<dialog>` + `showModal()`.** Inside the baseline (iOS/iPadOS Safari 16.4+, Chrome 111+), and it
  gives focus trapping, the backdrop and `Esc` without writing them.
- **No framework.** It cannot use Alpine or htmx: the public page loads neither, and this must be the same
  file on both surfaces. It also must not *require* them to be absent.
- **Items read from the DOM** via `data-viewer-item` and the per-tile `data-` attributes, never from a
  surface-specific endpoint. That is what lets one file serve two surfaces.
- **The action row is declared by the host page** (§7.7), via `data-viewer-actions` on the container — e.g.
  `data-viewer-actions="share,fullscreen"`. The viewer renders what it is given and has no idea which surface
  it is on. An `if (isAdmin)` inside `viewer.js` would be the same behaviour with a much worse failure mode:
  one boolean away from a public edit button, and nothing in the test suite able to execute the branch to
  prove otherwise.
- **Layout per §7.2**: dark but not black (`#111` — pure black makes a dark photograph look like a loading
  failure), previous/next either side, a translucent info panel rendered **only when it has content**, and a
  filmstrip along the bottom with the current thumbnail marked and scrolled to centre.
- **The filmstrip is hidden on narrow or short viewports by a media query in `viewer.css`** (§6, answering
  §11.3), not by JavaScript measuring the window: a media query re-evaluates on rotation for free, and a
  landscape phone is a short viewport rather than a narrow one. Below roughly `40rem`, or short, it costs ~15%
  of the height to show five thumbnails, and swipe plus the arrows already cover moving through the album.
- Keyboard `←`, `→`, `Esc`, `Home`/`End`; swipe left/right on touch; scroll lock while open and the scroll
  position restored on close; focus into the viewer on open and back to the tile that opened it on close.
- Prefetch two ahead and one back (§6) — bounded, and not the whole album.
- Missing image: the Danish line *"billedet er ikke tilgængeligt"* in place of the photograph, and the viewer
  still moves on.
- `prefers-reduced-motion` suppresses the transitions.

**No template actions in the `.js` or `.css` files, ever.** They are static assets served from a fixed map,
not templates — the same rule that already governs `adminui/*.js`. Anything page-specific arrives through a
`data-` attribute.

The fullscreen button (task 404), the share button (task 405), the `srcset` rendition choice (task 410) and
the caption/credit editors (tasks 407, 408) are separate tasks that plug into the action row this one
defines. The wiring into each host page is tasks 403 and 406.

**Testing reality** (§8): there is no way to execute this JavaScript in the Go suite. The asset route is a
behavioural Go test; the viewer's own behaviour is guarded by source-reading tests, which **must strip comment
lines before searching** — a needle matching the comment that explains it has cost this repo four false
positives — and by the manual QA in task 411. That asymmetry is the reason to keep this file small.

## Acceptance Criteria

- [ ] `viewer.js` and `viewer.css` exist as real files under `go/cmd/api/viewer/`, embedded, with no build
      step, no npm dependency, no CDN and nothing added under `vue/` except the dev-proxy key
- [ ] `/viewer/{asset}` serves exactly those two assets through a **fixed map**; an encoded `../` or any
      other name 404s, asserted by test
- [ ] The assets are cacheable long with a version in the path, from a route outside `/admin`, so task 371's
      `no-store` rule needs no exception
- [ ] The overlay is a native `<dialog>` opened with `showModal()`, labelled, keyboard-operable throughout,
      with focus returning to the tile that opened it
- [ ] Which controls appear comes only from the host page's `data-viewer-actions`; `viewer.js` contains no
      surface check and no reference to either surface
- [ ] The filmstrip is hidden on narrow **and** short viewports by media query alone, and the source guards
      strip comments before searching

## Progress Log

- 2026-09-25 — Task created from PRD 023.
