# 323 — /offentligt/glimt — the public page on hej.nathejk.dk

**Status:** open
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §0, §7, §8. The public feed is served **by this service**, on `hej.nathejk.dk` — not a
separate site and not an export into PRD 013.

A server-rendered page at `/offentligt/glimt`: a plain thumbnail grid with the hold attribution
(number, name, group) under each entry. No login, no bundle, no client-side state, working
without JavaScript. Plus:

- `GET /api/public/glimt` and `GET /api/public/glimt/:glimtId/media/:ordinal`
- `POST /api/public/glimt/:glimtId/report` — **unauthenticated**, rate-limited by IP. A parent
  who spots a problem is precisely the person we want to hear from, and they have no session.

Two same-origin traps, both of which must be handled rather than discovered:

1. **The service worker's navigation fallback would swallow the URL** and serve the app shell to
   an installed member following a public link. Needs the `navigateFallbackDenylist` entry
   (task 315).
2. **A logged-in member's browser will send `hej_session`** to these handlers. They must
   **ignore it entirely**. If they ever read it, the public page silently becomes a different
   page for members than for parents, and "is this public-safe?" becomes untestable.

Serves `audience = public AND hiddenAt IS NULL` and nothing else, regardless of who asks. No
personal name, no portrait, no arm number, no phone number — ever.

## Acceptance Criteria

- [ ] `/offentligt/glimt` server-rendered, works with JavaScript disabled
- [ ] **Multiple media are reachable without a swipe and without JavaScript** — see the note below
- [ ] Public API endpoints serve only public, non-hidden glimt
- [ ] **Test: an authenticated member's cookie changes nothing about the response**
- [ ] Test: a hidden glimt disappears from the public page
- [ ] Test: the payload contains no personal name, `personId` or phone number
- [ ] Unauthenticated report endpoint, IP rate-limited
- [ ] `navigateFallbackDenylist` entry present and verified in the built `sw.js`
- [ ] Passes `app.glimtPublicCutoff()` into `PublicFeed` — a zero time there serves the whole archive to the open web
- [ ] `go test ./...` passes

## ⚠️ Multiple media on a page with no JavaScript

Raised by the maintainer on 2026-09-18 while reviewing the feed: *"since the public version needs to be
accessible on a desktop computer, we need some way to move the carousel without swiping."*

The app's own strip was fixed for desktop by showing prev/next buttons where the pointer is fine
(task 316). **That fix does not carry here.** `GlimtMediaStrip.vue` is a Vue component driven by Embla,
and this page is server-rendered with no bundle — so there is no carousel to add arrows to.

The options, and the recommendation:

| approach | verdict |
|---|---|
| **Show every item, stacked or in a grid** | **Recommended.** No JavaScript, no gestures, no controls to discover, works in every browser including the very old ones this page exists for (PRD 013). A glimt has at most ten items and the page is already thumbnail-first. |
| CSS scroll-snap + `#anchor` links | Works without JS and looks like the app. But anchor navigation scrolls the *page*, and getting it to move only the strip needs care per browser — a lot of subtlety for a public page whose whole virtue is that it is dumb. |
| A small inline script | Rejected: the page must work with JavaScript disabled, so this would be a second code path that only some visitors get. |

**Do not port the carousel.** The reason the public page is server-rendered is that it must work on a
browser that cannot parse the app's bundle; a carousel is exactly the sort of thing that pulls a bundle
back in.

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-18 — Added the no-JavaScript media criterion and the analysis above, from the maintainer's
  desktop-access point. Also added the `glimtPublicCutoff` criterion — task 310 built the read-time
  public retention window and this is the only caller that must pass it; a zero time would silently
  serve the whole archive to the open web.
