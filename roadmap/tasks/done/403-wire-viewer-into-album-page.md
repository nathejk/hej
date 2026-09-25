# 403 — Wire the viewer into the public album page, and only now move the caption off the tile

**Status:** done
**Priority:** high
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

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

- [x] Every tile carries `data-viewer-item`, `data-full`, `data-medium`, `data-thumb`, `data-caption` and
      `data-credit`, and the container carries `data-viewer-actions="share,fullscreen"` and `data-share-title`
- [x] Every tile is still wrapped in an `<a href>` to the display variant, asserted by test, and a click with
      JavaScript disabled opens the large photograph
- [x] Moving through the album replaces the history entry and opening the viewer pushes one, so back closes
      the viewer rather than leaving the album
- [x] The caption and credit are rendered in the viewer's info panel before — or in the same change as — their
      removal from under the tiles, and no release ships with neither
- [x] `TestAlbumPageShowsThePhotographersCredit` and `TestAlbumPageRendersItsPhotographs` pass unmodified
- [x] The item list is rendered from `{{define "album-items"}}`, used by the page

## Progress Log

- 2026-09-25 — Task created from PRD 023.
- 2026-09-25 — Picked up. Plan: `{{define "album-items"}}`, an anchor per tile with the data contract, the container declaration, the viewer tags in the body next to the patrol page's Leaflet precedent, and the two "no script at all" assertions translated into what they actually meant.
- 2026-09-25 — `{{define "album-items"}}` holds the tile; the container declares `data-viewer`, `data-viewer-actions="share,fullscreen"`, `data-viewer-history="foto"`, `data-viewer-label` and `data-share-title`; the viewer's two tags sit in the body beside the patrol page's Leaflet precedent rather than in the shared layout head, since only this page wants them.
- 2026-09-25 — **The credit's visible line stays; only the caption's goes.** A deliberate departure from a literal reading of §7.1, and the reason is the one the task itself gives: the credit is a *published attribution* (task 393), the single documented exception to PRD 011's "names no person". If it lived only in `data-credit`, every visitor whose script did not run would get a photograph with no attribution — we would have stopped crediting the photographer for them. A layout change is not entitled to decide that, so the figcaption survives carrying the credit alone. `TestAlbumPageShowsThePhotographersCredit` therefore passes unmodified, as the criteria require.
- 2026-09-25 — **No `data-medium` yet.** Task 409 adds the 800px rendition and this attribute together: a URL in the DOM that the server does not serve is a broken image waiting for somebody to write the `srcset` that uses it. Noted in 409's criteria.
- 2026-09-25 — Two tests had to change, and it is worth being explicit that the requirement did not. `TestAlbumPageRendersEveryItemWithoutACarousel` and `TestABigAlbumIsCappedWithAPlainLink` both forbade the string `<script`, which was a **proxy** for "this page works without JavaScript". The page now loads the viewer, so the proxy had to go — but deleting the assertion would have quietly dropped the guarantee. It is written out instead, in the three parts that were always the point: a link per photograph (asserted by count), no inline script, and every script deferred and same-origin. `assertScriptsAreDeferredAndOurs` holds the last two, because a CDN tag or an inline block arrives in a change that is about something else.
- 2026-09-25 — The deep-link tests' needles moved from `"Billede nr N<"` to `data-caption="Billede nr N"`, since the caption is no longer a visible line. Worth noting the old needle was matching a figcaption that no longer exists — the tests failed loudly rather than silently passing, which is the good outcome of having asserted a specific piece of markup.
- 2026-09-25 — ✅ All criteria met. New guards: `TestTheAlbumPageWiresTheViewer` reads the action declaration back out and checks **every** name against an allowlist — a substring hunt can be defeated by a spelling nobody thought of, and this markup is the only place a public caption editor could be introduced, since no Go test can execute the viewer. `TestTheCreditStaysOnThePageWhileTheCaptionMovesToTheViewer` holds the attribution decision. Full `go test ./cmd/api/` clean.
