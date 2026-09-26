# 426 — The public website shows no favicon

**Status:** done
**Priority:** low
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

> *"Make sure public website uses same favicon as pwa site."*

The app's `index.html` declares `<link rel="icon" href="/favicon.svg" type="image/svg+xml">`. The public site's
layout declared nothing, so a browser fell back to asking for `/favicon.ico`, which does not exist — a blank tab
on every public page.

## A reference, not a copy

The file is `vue/public/favicon.svg`, served from the origin's root by Vite in dev and by the Go binary's SPA file
server in production.

`.rules` says a website asset must not live under `vue/`, and the obvious reading of that is to embed a second
copy in this binary. **That would satisfy the letter of the rule and break the actual requirement**, which is
that the two surfaces show the *same* icon: two copies of an image drift, and nothing would ever notice — nobody
looks at a favicon on purpose.

The rule exists to keep the PWA's *build machinery* out of the website, which is why the vendored Leaflet and
`publicmap.js` are recorded as a wrinkle. Pointing at a static file by URL is not that. So this is a reference,
and the sameness is enforced by test rather than asserted in a comment.

## No `apple-touch-icon`

The one tag the app has that this deliberately does not want. It is what iOS uses when a page is added to the
home screen, and a home-screen icon that looks exactly like the app but opens a read-only public page is worse
than no icon: the app is the installable thing (PRD 005), and this site is where a desktop visitor is *sent
instead* of installing it.

## Acceptance Criteria

- [x] Every public page declares the app's favicon, at the app's own URL
- [x] The public site declares no `apple-touch-icon`
- [x] The two declarations cannot drift apart without a test failing

## Progress Log

- 2026-09-25 — One line in `layout-head`, which every public page shares — including the glimt page since task
  424.
- 2026-09-25 — `TestThePublicSiteUsesTheAppsFavicon` reads the href out of `vue/index.html` and requires the
  public template to match it. A test naming `/favicon.svg` twice would have passed on the day somebody renamed
  the app's icon, and the website would have gone quietly back to a blank tab. Verified by moving the app's icon
  to `/brand/favicon.svg` and watching the test fail, naming the new URL.
- 2026-09-25 — The admin tool also declares no favicon. Not changed: it is behind a credential and answers
  `no-store` to everything, so its tab is a curator's own business rather than the event's face. Worth a line if
  anybody minds.
- 2026-09-25 — ✅ All criteria met. `gofmt`, `go vet`, full `go test ./cmd/api/` clean.
