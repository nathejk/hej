# 403 — Wire the viewer into the public album page, and only now move the caption off the tile

**Status:** open
**Priority:** high
**Created:** 2026-09-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 023 §7.1 and §7.4. The album page starts using the viewer from task 402: the two asset tags in `<head>`,
the container's action declaration, the per-tile data contract, and the `?foto=` history handling.

### The data contract (§7.4)

Each tile carries what the viewer needs and nothing else:

```html
<a class="tile" href="/api/public/albums/al-1/media/12"
   data-viewer-item data-full="/api/public/albums/al-1/media/12"
   data-medium="/api/public/albums/al-1/media/12?variant=medium"
   data-thumb="/api/public/albums/al-1/media/12?variant=thumb"
   data-caption="Morgenmad i regnen" data-credit="Foto: Jens Hansen">
```

`data-` attributes rather than a JSON blob in a `<script>`, for the reason the admin tool already gives: a
value in an attribute is escaped as an attribute by `html/template` and is inspectable in dev tools, while a
value interpolated into JavaScript is escaped as JavaScript and mangles in ways nobody notices until a browser
does something strange.

The container gets `data-viewer data-viewer-actions="share,fullscreen"` and `data-share-title` (§7.7). The
`<a href>` around every tile points at the **display variant's media URL**, so a click with no script opens
the large photograph in the browser; the viewer intercepts it when it has loaded. That fallback is the whole
mitigation for "a JS bug takes the album page with it" (§8), so it is worth an explicit test that the page
contains a working `href` per photograph.

`?foto=` history: `replaceState` as you move through the album, one `pushState` when the viewer opens, so back
closes the viewer rather than leaving the album (§6). A cold load carrying `?foto=` opens the viewer on that
item — the server side of that is task 401.

### The caption and credit lines move here, and nowhere earlier

**This is the only task that may remove the visible caption and credit lines from under the tiles, and it is
deliberately not task 398.** §7.1 calls the loss on the plain page deliberate and pays for it with the
viewer's info panel — but the payment has to exist first. The photographer's credit (task 393) is a
**published attribution**: a person asked to be credited, and PRD 011's privacy claim was formally narrowed to
allow it. Taking that line off a public page during the window between the grid change and the viewer landing
would be un-publishing an attribution for a release or two, which is not a styling decision anybody is
entitled to make quietly.

`TestAlbumPageShowsThePhotographersCredit` in `albumpage_test.go` guards exactly this, and it must keep
passing throughout. So must `TestAlbumPageRendersItsPhotographs`: the caption stays **in the markup**
regardless, as the tile's `alt` and its `data-caption`, so a screen reader still reads it and the viewer has
it without a request. Only the *visible* line under the tile goes, and only once there is somewhere for it to
be read.

`publicsite.go` also moves the item list into `{{define "album-items"}}` here (§8) — cheap now, and it is what
the deferred fragment endpoint (task 412) would reuse rather than a second rendering of the same list.

## Acceptance Criteria

- [ ] Every tile carries `data-viewer-item`, `data-full`, `data-medium`, `data-thumb`, `data-caption` and
      `data-credit`, and the container carries `data-viewer-actions="share,fullscreen"` and `data-share-title`
- [ ] Every tile is still wrapped in an `<a href>` to the display variant, asserted by test, and a click with
      JavaScript disabled opens the large photograph
- [ ] Moving through the album replaces the history entry and opening the viewer pushes one, so back closes
      the viewer rather than leaving the album
- [ ] The caption and credit are rendered in the viewer's info panel before — or in the same change as — their
      removal from under the tiles, and no release ships with neither
- [ ] `TestAlbumPageShowsThePhotographersCredit` and `TestAlbumPageRendersItsPhotographs` pass unmodified
- [ ] The item list is rendered from `{{define "album-items"}}`, used by the page

## Progress Log

- 2026-09-25 — Task created from PRD 023.
