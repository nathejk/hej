# 140 — Desktop placeholder as a plain page outside the app

**Status:** done
**Priority:** medium
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

## Description

Task 123 shipped the desktop placeholder as a Vue view (`DesktopView.vue`) on a route inside
the SPA. Per the maintainer (2026-08-30) that is wrong in kind, not just in content: **the
desktop side is a regular website and must not be the app or part of it.** What eventually
lives there is a separate PRD; what is needed now is a plain page.

So the placeholder moves out of the SPA entirely:

- A static HTML page, served as a file, with no Vue, no bundle, no router and no stores.
- Content, for now: **"Hej Nathejk — more to come…"**.
- A top banner **"Installér app"** linking back to the install instructions, shown **only
  when the device qualifies for the PWA** — a mouse-only desktop must not be invited to
  install something it cannot use, and a tablet that was misclassified as desktop needs a way
  back to the wall.

## Why "not part of the app" is the whole point

PRD 005 §4 already says the desktop *scope* is out of scope, and task 123 kept the view
deliberately isolated (its only imports were a Lucide icon and `APP_NAME`) so the real site
could replace it. That was the right instinct at the wrong layer: as long as the page is a
route in the SPA it still boots the whole application — Pinia, the router, the service worker
registration, the auth guard — to render one sentence. Consequences worth naming:

- The real website will not be a Vue view in this repo, so anything built as one is thrown
  away rather than replaced.
- A page inside the SPA is inside the **service worker's** scope, so the placeholder can be
  served from a stale precache and can be intercepted by the navigation fallback.
- Shipping app JavaScript to visitors who are not app users is a cost with no return.

## The install banner needs its own tiny detection

The page cannot import `helpers/platform.ts` — that is app code, and importing it would drag
the bundle back in. So it carries a **few lines of inline script** doing the minimum:
coarse-pointer/touch capability. This is duplication, and it is accepted deliberately rather
than by accident:

- The alternatives are worse: bundling the app to answer one boolean, or omitting the banner
  and leaving a misclassified iPad with no route to the wall.
- The two copies cannot drift into a *dangerous* state. `platform.ts` decides whether someone
  is gated; this decides whether a link is visible. A false positive here shows an install
  link to a desktop user, which is a shrug — and a false negative loses nothing that the
  address bar cannot fix.

Keep it minimal and say in a comment that `helpers/platform.ts` is the authority and this is
not.

## Scope

- Add the static page; remove `views/DesktopView.vue` and its route.
- The router's device gate does a **full-page navigation** to the static page rather than a
  Vue route change — otherwise the SPA stays mounted, which is the thing being fixed.
- Exclude the page from the service worker's navigation fallback, or an installed client will
  be served `index.html` for it and boot the app after all.
- `RouteMeta.desktop` was added by task 123 purely so the guard could recognise this route. If
  nothing else uses it, remove it rather than leaving a meta flag no route sets.

## Acceptance Criteria

- [x] A static page exists that is **not** part of the Vue app: no Vue, no app bundle, no
      router, no stores
- [x] It reads "Hej Nathejk — more to come…" and nothing more
- [x] A top banner links to the install instructions, shown **only** on devices that qualify
      for the PWA
- [x] `views/DesktopView.vue` and the `/desktop` Vue route are gone
- [x] The device gate leaves the SPA with a full-page navigation rather than routing within it
- [x] No redirect loop is possible: the placeholder must never be served by the SPA fallback
      (which would boot the app, which would redirect again)
- [x] The page is excluded from the service worker's navigation fallback
- [x] `RouteMeta.desktop` is removed if nothing sets it any more
- [x] The inline detection carries a comment naming `helpers/platform.ts` as the authority
- [x] `vue-tsc`, `npm test` and `npm run build` clean

## Depends on

- **Task 123** — the view this replaces.
- **Task 137** — the guard that redirects here.

## Progress Log

- 2026-08-30 — Task created: maintainer clarified that the desktop placeholder is a regular
  website and must not be part of the app (PRD 005 §4).
- 2026-08-30 — **Implemented as `vue/public/desktop.html`.** A plain file: inline styles, one
  headline, one line of text, ~10 lines of script. `views/DesktopView.vue` and the `/desktop` route
  are deleted, and `RouteMeta.desktop` went with them — it existed only so the guard could recognise
  that route, and a meta flag no route sets is a trap for the next reader.

  Four decisions:

  - **The URL is `/desktop.html`, not `/desktop`, and that is load-bearing rather than lazy.** A
    tidier path would be answered by the SPA fallback (`routes.go` serves `index.html` for anything
    that is not a file), which would boot the app, which would redirect to the placeholder again — a
    loop. A real file cannot be caught by the fallback, in dev or in production, so the ugly
    extension is what makes the loop impossible instead of merely unlikely. Recorded in the router
    next to the constant.
  - **`navigateFallbackDenylist: [/^\/desktop\.html$/]`** closes the same hole for an *installed*
    client, where the service worker — not the Go handler — answers navigations. Verified in the
    generated worker rather than assumed: `denylist:[/^\/desktop\.html$/]` is present in `dist/sw.js`.
    Without it the page would have been the one thing that worked for browser visitors and looped for
    installed ones.
  - **The gate does `window.location.replace()` and returns `false`.** Returning false aborts the
    in-app navigation, which is normally the blank-screen failure of task 090 — acceptable here
    precisely because the browser is already leaving for a static file that has nothing to boot. The
    guard's return type widened from `true | RouteLocationRaw` to `boolean | RouteLocationRaw` to say
    so in the types.
  - **The install banner has its own ~10 lines of detection**, duplicating a slice of
    `platform.ts`. Accepted deliberately, with a comment naming that file as the authority: importing
    app code to answer one boolean would drag the bundle back in and undo the point of the task. The
    two cannot drift dangerously — `platform.ts` decides whether somebody is *gated*, this decides
    whether a *link* is visible, so a false positive is a shrug and a false negative costs nothing
    the address bar cannot fix. It does include the iPadOS touch-point check, since a misclassified
    iPad is exactly the visitor who needs the banner.

  The font stack mirrors `--font-nathejk` (Impact + narrow-bold fallbacks) so the headline still
  looks like Nathejk, and the background matches the app's `theme_color` — duplicated in a `<style>`
  block because sharing anything with the app's CSS would mean a build step, which is what "not part
  of the app" rules out. `noindex` is set: a page saying "more to come" should not be what a search
  engine has on file for Nathejk.
- 2026-08-30 — ✅ `vue-tsc`, 27 tests and `npm run build` clean. `dist/desktop.html` is emitted and
  precached, so it also works offline for an installed client that happens to open it.

  Not verified from here (no browser): that the banner actually appears on a real tablet and stays
  hidden on a real desktop. Added to task 139's matrix, where the desktop and iPadOS rows now also
  cover this page.
