# PRD 023 — A shared photo viewer, and album pages that stay bounded and sharp

**Status:** doing
**Author:** agent session (Zed), with maintainer
**Created:** 2026-09-25
**Last updated:** 2026-09-25 (§4's two reversals — a caption editor in admin, a share button on the public
viewer; then §11's 1, 3, 5 and 6, the paging pushback in §2a, an editable credit line, and a medium
rendition for small screens)
**Approved:** 2026-09-25
**Shipped:**
**Target users:** family / open web (the public album page), organizer (the curator's contact sheet)

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 0. What this PRD is

Two changes that arrive together because the second one is what makes the first one safe to shrink:

1. **The public album page is paged.** It renders every item it has today, in tiles wide enough that the
   320 px thumbnail is upscaled. An album of 100–400 photographs is the expected size after PRD 022's bulk
   hand-in, and that page is read by families on the event's connection.
2. **A photograph opens in a viewer** — one full-viewport overlay, dark, with a filmstrip, arrows, an info
   panel and a fullscreen button — and **the same viewer is used by the public album page and by the
   curator's admin tool.**

It builds on PRD 011 (the public site) and PRD 022 (the photo library and the admin tool). It does not
change what is published, who may see it, or the privacy posture of either surface.

---

## 1. Summary

The public album page becomes a dense, uniform grid of sharp thumbnails that loads a page at a time, and a
click on any photograph opens it in a shared full-viewport viewer — dark background, previous/next, a
scrollable filmstrip centred on the current photograph, a translucent panel with the caption and the
photographer's credit, and a button for real fullscreen where the browser has it. Publicly it can **share a
link to this photograph in this album**; in the curator's tool the caption is **editable in place**, which is
the one edit worth making while looking at a photograph large. The curator's contact sheet opens the same
viewer, from the same file, so "look at this photograph properly" means the same thing on both surfaces.

---

## 2. Problem & Motivation

**What problem does this solve?**

- **An album page's response is its whole album, however large that is.** `albumPageHandler` renders every
  item `BySlug` returns, on every request. `loading="lazy"` already keeps the *bytes* off the wire — see §2a,
  which is honest about how much of this problem that solves — but the database read, the Go allocation, the
  markup and the DOM are all proportional to the album, and an album is now as large as an SD card.
- **The tiles are bigger than the image behind them.** The grid is `minmax(18rem, 1fr)` and the stored
  thumbnail is 320 px on its longest edge, so a tile is 288 px CSS — soft at 1× and visibly soft at 2×,
  which is every phone anybody reads this page on. Paying for upscaled pixels is the worst of both: slow
  *and* blurry. **This is the largest and cheapest win in the PRD** and nothing else depends on it.
- **There is no way to actually look at a photograph.** The page has one size, the 320 px thumbnail. The
  stored display image is 1600 px (`imaging.Prepare(raw, 1600, [320], 85, true)`) and nothing on the public
  site ever asks for it. A family that wants to see their patrol's faces has to open the media URL by hand.
- **The curator has the same gap, for a different reason.** The contact sheet exists to *sort* — a click
  selects, which is right — so there is no gesture that means "show me this one big enough to judge". A
  curator deciding whether a photograph is worth publishing is doing it from a 150 px tile.

**When this is used, which changes the engineering.** Maintainer, 2026-09-25: *"The photo viewer is mostly
an after-event thing, with photos from photographers — the glimt will be the during-event with user
photos."* That reframes the load picture and the first draft of this PRD had it wrong:

- **The peak is not the finish line.** That is glimt's peak — PRD 019 §0a.3, a thousand people on a
  congested cell, which is why `publicMediaReadsPerMinute` is 12000 and why the glimt grid was tuned the way
  it was. Albums are read in the days and weeks *after*, from home, on wifi and unloaded 4G, by far fewer
  people at once. Arguments of the form "on the event's connection" do not apply here and have been removed
  from §6 where they had crept in.
- **Desktop is a first-class case, not a fallback.** Looking through a photographer's album on a laptop is a
  normal way to use this, which the frontpage's install-only PWA gate cannot serve at all. That makes the
  fullscreen button (§7.5) and the filmstrip more valuable than a phone-first reading suggests — and it is
  the maintainer's explicit instruction: *"this feature is intended for both desktop + mobile"*.
- **Quality beats frugality here.** The 1600 px display image is the right thing to serve, and §11.6's
  medium variant is answered: no.

**Why now?** PRD 022 is in `doing/` and the bulk upload path is the point of it. The first real album will
be the first page that is too big, and the week after the event is when somebody notices.

**Evidence.** Maintainer, 2026-09-25: *"An album can easily contain +100 photos, so we need to be aware of
initial load and maybe add infinite scrolling"* — note the *maybe*, which §2a takes seriously — and a
reference screenshot of a Google-Photos-style viewer: one row of smaller scrollable thumbnails at the bottom
centred on the current photo, above it a semi-transparent grey box with caption and photo credit, dark
background, left/right arrows, an action row top right. Plus the question this PRD answers in §7.5: *"Is it
possible to have a button for entering, not just full viewport but also a fullscreen view?"*

---

## 2a. Paging versus lazy loading — the honest accounting

Asked directly by the maintainer, 2026-09-25: *"why is paging better than lazy load?"* It is a fair
challenge and the first draft over-claimed, so here is the arithmetic rather than the instinct.

**They are not alternatives.** `loading="lazy"` is already on every tile and stays. It is what keeps the
image *bytes* off the wire, and image bytes are ~95% of what an album page costs. So the question is only
what remains unbounded once lazy loading has done its work.

For a 400-item album, per request:

| What | Cost, unbounded | Does lazy loading help? |
|---|---|---|
| Thumbnail bytes | ~8 MB if all fetched | **Yes — this is the whole point of it.** Only what is scrolled to is fetched. |
| HTML | ~160 KB raw, but **~20 KB gzipped** — the markup is near-identical per tile, which is exactly what gzip eats | No, and it matters much less than it looks |
| DOM / layout | ~1200 nodes, 400 lazy-load observers; fine on a laptop, sluggish on a cheap Android | No |
| Database + Go | `BySlug` joins and materialises **all 400 rows on every request**, and there is no server-side cache — `max-age=60` is a header, not a cache | No |
| The viewer's list | 400 items built at open | No |

So paging buys three real things: **bounded server work per request**, bounded DOM on a weak device, and a
bounded viewer list. It buys approximately **nothing** on transferred bytes, which is where the intuition
says the win is.

And it costs: a fragment endpoint, an IntersectionObserver, a no-script "Vis flere" path, a second rendering
of the item list to keep in step, and — the one that is easy to miss — **`?foto=` has to derive which page
an item is on** before the share link can resolve. Without paging, the share link is one anchor and no
derivation.

**Conclusion, and it shrinks this PRD:**

1. **Keep lazy loading.** It is doing the heavy lifting and the first draft under-credited it.
2. **Fix the tile size.** Unrelated to paging, biggest single win, no new machinery: ~150 px tiles means a
   320 px thumbnail is sharp at 2× *and* a quarter of the decode work per tile.
3. **Add a cap with a plain link, no JavaScript.** Not 60 — a **generous** cap, proposed **200**, rendered
   with a real `<a href="?side=2">Vis flere</a>`. This is insurance against the 2000-photograph SD-card
   dump, and it is about twenty lines in one handler.
4. **Defer infinite scroll until it is measured.** Render the 2025 import (task 348) at the fixed tile size
   and look at the numbers on a real phone. If a 200-item page is fine — and on the after-event usage in
   §2 it may well be — the observer, the fragment endpoint and their two guards are never written. If it is
   not fine, they are a known, specified, self-contained task.

That is the shape this PRD now specifies. The deferred work is kept in §10 rather than deleted, because
"decided against for now, with a number to revisit it at" is worth more than silence.

---

## 3. Goals

- An album page's first response is a bounded amount of work, whatever the album's size.
- Thumbnails are displayed at or below their stored resolution, so the grid is sharp.
- Any photograph on either surface can be looked at large, with its caption and its credit, without
  leaving the page.
- A visitor can send one photograph to one person, as a link that lands on that photograph.
- A curator can correct a caption while looking at the photograph it describes, and move to the next one
  without leaving the viewer.
- The viewer is **one implementation**, used by the public website and the admin tool.
- The album page keeps working with JavaScript disabled, including reaching every photograph and reaching
  the large version of one. This is PRD 011 §8 and it is not negotiable for a public page.
- The curator's sorting gestures are unchanged. Nothing about this may make a click ambiguous on the
  contact sheet.

---

## 4. Non-Goals

- **No editing in the viewer — except the caption and the credit, and only in admin.** Decided by the
  maintainer, 2026-09-25, against the first draft of this section (caption) and then §11.5 (credit). The
  reasoning it overrides is worth keeping, because it still holds for everything else: PRD 022 §5 spent its
  copy budget making "remove from album" and "delete from library" impossible to confuse, and a destructive
  action in a dark overlay one icon from a share button is how that work gets undone. The two text fields are
  different in kind — **non-destructive, reversible, and the two things you can only judge while looking at
  the photograph large**, which is exactly the state the viewer puts you in. So: caption and credit yes;
  delete, album membership, position and patrol tag stay on the sheet's action bar.
- **No download button, and the share button shares a link, not bytes.** `navigator.share({url})` with no
  `files`. The distinction is the takedown path: a shared *link* stops working when a photograph is taken
  down, and a shared *JPEG* does not. It is also what keeps this a public-site feature rather than a
  republishing tool — see §7.8.
- **No pinch-zoom or pan inside the viewer.** The stored display image is 1600 px; there is nothing to zoom
  into. The reference screenshot's magnifier is a Google Photos affordance for originals we do not keep.
- **One new rendition, and no backfill required.** §11.6 was answered "no" and then reversed by the
  maintainer, 2026-09-25: *"if a small screen then don't fetch original image — a medium size would do"*. An
  **800 px** rendition joins the 1600 px display image and the 320 px thumbnail. What stays out of scope is a
  migration: a photograph without one falls back to the display image, exactly as a missing thumbnail already
  does. See §7.9 and §8.
- **No infinite scroll in v1.** Decided in §2a after the maintainer asked why paging beats lazy loading: the
  answer is that lazy loading already wins on bytes, so v1 is lazy loading + a fixed tile size + a generous
  cap with a plain link. The observer and its fragment endpoint are specified and **deferred behind a
  measurement**, not designed away.
- **Not the PWA.** The app's glimt strip is a Vue component driven by Embla and stays that way. This viewer
  is for the two server-rendered surfaces only — the hard rule in `.rules`. Two implementations of a
  lightbox is the honest cost of two stacks, and the cheaper-looking alternative (share one) is the thing
  that rule forbids. The division of labour is now explicit (§2): **glimt is during the event, from
  participants, in the app; albums are after it, from photographers, on the website.**

---

## 5. User Stories & Scenarios

- As a **parent, a week after the event**, I want to look through the photographer's album on the laptop
  with my child sitting next to me, so that we can find the ones with their patrol in them.
- As a **parent on a phone**, I want to tap a photograph and see it fill the screen with its credit line,
  and swipe to the next one, so that looking through the album feels like looking through photographs.
- As a **parent**, I want to send my sister the one photograph of her son, not the whole album, so that she
  does not have to scroll 300 tiles looking for him.
- As a **curator**, I want to fix a caption while I am looking at the photograph large enough to read what
  is actually in it, so that I do not have to hold it in my head on the way back to the sheet.
- As a **family member on a laptop**, I want to press a button and have the photograph fill the actual
  screen, so that eight children around a campfire are more than 400 px of it.
- As a **curator**, I want to open a photograph from the contact sheet at a size where I can see whether it
  is sharp and whether anybody is identifiable, and move through the sheet with the arrow keys, so that
  judging 300 photographs is a sitting rather than an afternoon.
- As a **visitor with JavaScript off** (or a blocked asset, or a browser too old), I want the album page to
  still show me every photograph and still let me open one at full size, so that the page is not a shell.

**Happy path, public.** `/2026/album/loerdag-morgen` renders its items as square ~150 px tiles, lazily
loaded, up to a cap of 200 with a "Vis flere" link if the album is larger. Tapping a tile opens the viewer on
that item: the photograph on near-black, caption and credit in a translucent panel, arrows either side, and
— on a laptop — a filmstrip below with the current tile ringed. Right arrow, a swipe, or a click on a
filmstrip thumbnail moves on. `Esc`, the back button, or the close control returns to the grid at the same
scroll position.

**Happy path, admin.** On the contact sheet a cell has a small expand control in its corner. Clicking the
cell still selects it; clicking the control opens the viewer on that photograph, with the loaded sheet as
its list. Arrow keys walk the sheet. `Esc` closes and the selection is exactly as it was.

**Edge cases.**

- The last tile on a capped page, with more on the server: the viewer's "next" is **disabled at the end of
  what is loaded**, and the grid's "Vis flere" link is how you get the rest. With the deferred observer this
  becomes "next loads the following page"; without it, pretending otherwise would be a control that hangs.
- An album with one photograph: no arrows, no filmstrip.
- A photograph with no caption and no credit: no panel at all, rather than an empty grey box.
- The display image 404s (bytes purged after a takedown between page load and click): the viewer shows the
  Danish "billedet er ikke tilgængeligt" line in place of the image and still moves on.
- A deleted photograph on the admin sheet is already marked `gone`; the viewer must keep showing it as such
  rather than presenting it as live.

---

## 6. Requirements

### Functional — the album page

- [ ] Tiles are square, uniform, and sized so a 320 px thumbnail is **not upscaled at 2×** — a tile no wider
      than ~160 px CSS. Two to three per row on a narrow phone, six to eight on a laptop. This is the item
      with the best ratio of benefit to risk in the PRD; it ships first and alone if need be.
- [ ] `loading="lazy"` and `decoding="async"` stay on every tile, and intrinsic `width`/`height` stay so the
      page does not reflow as it loads. §2a: this is what actually bounds the bytes, and it already works.
- [ ] The page renders at most `albumPageCap` items per response. Proposed **200** — deliberately generous,
      because it is a guard against the 2000-photograph dump rather than a pagination feature.
- [ ] When an album exceeds the cap, a **"Vis flere"** control appears: a real `<a href="?side=2">`, no
      script involved, rendering the next 200 with the same layout. That page is a normal, shareable address.
- [ ] Every tile is wrapped in an `<a href>` pointing at the **display variant's media URL**, so a click
      with no script opens the large photograph in the browser. The viewer, when present, intercepts it.
- [ ] Each tile carries `id="foto-{ordinal}"`, so a deep link lands on it with no script at all.
- [ ] **Deferred behind a measurement (§2a.4), not in v1:** an IntersectionObserver that appends the next
      page from an items fragment. If it is built, the "Vis flere" link stays in the markup — an observer
      that never fires must not be the only way to reach photograph 201.

### Functional — the viewer

- [ ] One overlay, full viewport, dark background, on top of everything.
- [ ] The large image is **the rendition that fits the screen**, not always the 1600 px one: an 800 px
      variant on a phone, the display image on a laptop, chosen by the browser from `srcset`/`sizes` rather
      than by JavaScript measuring the window (§7.9).
- [ ] A photograph with no 800 px rendition (anything uploaded before this ships) falls back to the display
      image. Same rule, same reason as the existing missing-thumbnail fallback — a rendition is an
      optimisation, and losing one costs bandwidth rather than the photograph.
- [ ] Prefetching follows the same choice: two ahead and one back, **of the variant this viewport uses**.
      Prefetching 1600 px images to a phone that will display 800 px ones would be the bug this requirement
      exists to remove, arriving by the back door.
- [ ] Previous/next controls; keyboard `←`, `→`, `Esc`, `Home`/`End`; swipe left/right on touch.
- [ ] A horizontally scrollable filmstrip of thumbnails along the bottom, the current one marked and
      scrolled to centre. Clicking one jumps to it.
- [ ] **The filmstrip is hidden on a small screen** (maintainer, 2026-09-25, answering §11.3). Below roughly
      `40rem` wide — or on any short viewport, which is a landscape phone — it costs ~15% of the height to
      show about five thumbnails, and swipe plus the arrows already cover moving through the album. Hidden by
      a media query in `viewer.css`, not by JavaScript measuring the window: a media query re-evaluates on
      rotation for free, and the feature is explicitly **for both desktop and mobile**, so the phone layout
      is a first-class variant rather than a degradation.
- [ ] A translucent panel above the filmstrip carrying exactly two things: the photograph's **caption** and
      the photographer's **credit** (task 393's credit line, verbatim, never derived from `person`).
      **No album description** — §11.1 answered: it repeats on every photograph and says nothing about the
      one you are looking at. Rendered only when there is something to say.
- [ ] An action row top right. **Which controls appear is decided by the host page, not by the viewer**:
      the page declares them and the viewer renders what it is given. Public: share, fullscreen, close.
      Admin: caption, fullscreen, close. This is what keeps one file serving two surfaces without an
      `if (isAdmin)` in it — see §7.7.
- [ ] The current photograph is reflected in the URL as **`?foto={ordinal}`**, replacing the browser's
      history entry as you move and pushing one when the viewer opens, so back closes the viewer rather than
      leaving the album. Not a hash: the same parameter has to work as a **server-side** deep link, because
      the share button sends it to somebody who does not have the album's page loaded — see §7.8.
- [ ] `?foto={ordinal}` on a cold load renders **the page that contains that item** (the server derives which
      `side` that is from the ordinal and the cap), scrolls to it, and opens the viewer on it. Without script
      it is still the right page scrolled to the right tile, because each tile carries `id="foto-{ordinal}"`.
      With a cap of 200 this derivation is usually the identity — which is the point: it is a few lines, and
      it is what stops a shared link rotting the day an album grows past the cap.
- [ ] Opening the viewer locks the page behind it from scrolling; closing restores the scroll position.
- [ ] Focus moves into the viewer on open, is trapped while it is open, and returns to the tile that opened
      it on close.
- [ ] The viewer's item list is read from the DOM via `data-` attributes on the tiles — never from a
      surface-specific endpoint — which is what lets one file serve both surfaces.
- [ ] Neighbouring display images are prefetched **two ahead and one back** — see the variant rule above.
      §2 revised this upward from one: albums are read after the event on home connections, not on a
      congested cell at the finish line, so the cost of being wrong is small and the benefit — the next
      photograph already there when you press `→` — is the whole feel of the thing. Still bounded, and still
      not the whole album.

### Functional — share (public only)

- [ ] A share control in the public viewer's action row shares **a link to this photograph in this album**:
      the absolute `https://…/2026/album/{slug}?foto={ordinal}`.
- [ ] `navigator.share({ title, url })` where available. **No `files`** — a link, never the bytes (§4).
- [ ] Where `navigator.share` is absent (desktop Firefox, older desktop Safari), the control copies the URL
      via `navigator.clipboard.writeText` and says so in Danish, in the viewer, briefly. A share button that
      does nothing on a laptop is worse than one that copies.
- [ ] Where neither exists, the control is absent rather than inert.
- [ ] The shared link must survive being opened by somebody who has never seen the album: that is the whole
      reason `?foto=` is a server-side parameter and not a hash.
- [ ] The title passed to the share sheet is the **album's** title, not a person's, and never a caption that
      might name one. The album page is `noindex` and stays so; sharing is the visitor's own act, not a
      change in what we publish.

### Functional — admin

- [ ] Contact-sheet cells gain an expand control that opens the viewer. **A click on the cell still
      selects, and only selects.**
- [ ] The album editor's sheet gets the same control, with the album's order as the viewer's order.
- [ ] Deleted photographs stay visibly deleted in the viewer.
- [ ] **A caption editor in the admin viewer**: the caption is editable in place in the info panel, saved
      with the existing `PATCH /api/admin/photos` (`{photoIds:[id], caption}`) — one photograph, the bulk
      endpoint, no new route.
- [ ] **And the credit line**, the same way, through the same endpoint (`{photoIds:[id], credit}`) — §11.5
      answered yes. Two separate fields with two separate saves, never one form that writes both: a curator
      fixing a typo in a caption must not be able to blank a credit by not touching it.
- [ ] The credit field carries the sheet's own warning about what it is — **the one field in this tool that
      records a person's name**, published — and shows how it will read publicly ("Foto: …"). `creditaction.js`
      already makes this point in the sheet; the viewer must not be the quiet way to do the same thing.
- [ ] Clearing a credit is **its own act**, not "save an empty field" — the rule `creditaction.js` already
      applies with a separate "Fjern fotokredit" button, for the reason it records: removing an attribution
      should not be something a stray select-all-and-delete does on its way past.
- [ ] The credit field is prefilled from the photograph's current credit. Not from `localStorage` — the
      sheet's "last credit typed on this laptop" exists because a *batch* has no single existing value to
      show, and here there is exactly one photograph in front of you.
- [ ] It says, in the same words the caption sheet already uses, that **the caption belongs to the
      photograph and not to the album**, so a curator knows they are changing it everywhere it appears. The
      same is true of the credit.
- [ ] Saving updates the item's `alt` and `data-caption` in the host page so the sheet behind the overlay
      is not stale, and reuses whatever refresh the sheet's existing actions use rather than inventing a
      second path.
- [ ] A failed save says so and **keeps the typed text**. A caption is a sentence somebody composed; losing
      it to a dropped hotel connection is the failure mode that makes a curator stop trusting the tool.
- [ ] The editor is not reachable on the public surface. Not hidden — **absent**: the control is rendered by
      the admin page's action declaration, and the public page declares no such action, so there is nothing
      for a visitor to un-hide. The endpoint is behind `requireAdmin` regardless.

### Non-Functional

- **No build step, no npm, no CDN, nothing new under `vue/`.** The viewer is our own vanilla JS and CSS,
  real files under `go/cmd/api/`, embedded in the binary and served from it (`go-server-rendered-pages`).
- **No framework.** It cannot use Alpine or htmx: the public page loads neither, and it must be the same
  file on both surfaces. It also must not *require* them to be absent.
- **Progressive enhancement.** Every requirement above degrades to the plain page: grid, "Vis flere" link,
  and a link per photograph to its large version.
- **Accessibility.** Keyboard-operable throughout; the overlay is a modal dialog with a label; the
  filmstrip is reachable by keyboard; `prefers-reduced-motion` suppresses the transitions.
- **Baseline.** iOS/iPadOS Safari 16.4+, Chrome 111+ (`.rules`). `<dialog>.showModal()`,
  `IntersectionObserver` and `scrollIntoView({block:'nearest'})` are all inside it.
- **Copy is Danish**, on both surfaces.
- **Privacy is unchanged.** The viewer displays two text fields, caption and credit; the credit is PRD
  011's single documented exception and `publicprivacy_test.go`'s `isPersonShaped` must still pass
  unmodified. No coordinate reaches the viewer — `publicAlbumItem` carries none, and that is deliberate
  (see its doc comment); the viewer must not become the reason one is added.
- **Caching.** The viewer's JS and CSS are served `public, max-age` long with a version in the path, from a
  route **outside `/admin`**, so the admin tool's `no-store` rule (task 371) stays true without an
  exception. `?side=` and `?foto=` pages get the public site's ordinary 60-second window, like every other
  public response — including the one a share link opens, so a takedown lands on a shared link too.

---

## 7. UX / UI Notes

### 7.1 The grid

Square crops, `object-fit: cover`, the same treatment the frontpage's album cards just got (task 414), at
roughly `minmax(8rem, 1fr)` — about 150 px per tile, which is the number the 320 px stored thumbnail is
sharp at on a 2× display. Captions do **not** appear under the tiles at that size — there is no room, and the
caption is in the viewer where it is legible. This is a deliberate loss on the plain page and the reason every
tile is a link: without script the caption is reachable by opening the photograph.

**The order of those two changes matters, and it is not the obvious one.** The tile size ships first, on its
own (task 398), and it ships with the caption and the credit still under the tile — slightly ragged rows and
all. The text only moves into the viewer in task 403, *with the viewer*. The reason is the credit line: task
393's photographer credit is a **published attribution**, it is PRD 011's one documented exception to naming
no person, and `TestAlbumPageShowsThePhotographersCredit` guards it on this page. Taking it off the page in
the tile-size task would remove a shipped, deliberately-argued feature and leave nothing in its place for
however long the viewer takes. A dense grid with a caption under it is merely less pretty; a public page that
silently stops crediting its photographers is a broken promise.

The caption stays **in the markup** regardless, as the tile's `alt` and its `data-caption`, so a screen
reader still reads it, the viewer has it without a request, and
`TestAlbumPageRendersItsPhotographs` — which asserts a fixture caption appears on the page — keeps
meaning what it was written to mean. When the visible line goes, only the *visible* line goes.

### 7.2 The viewer, laid out

```
┌──────────────────────────────────────────────┐
│ ←(back)              (share) ⤴  ⛶  ✕         │  public
│ ←(back)            (caption) ✎  ⛶  ✕         │  admin
│                                              │
│  ‹            [ the photograph ]           › │
│                                              │
│      ┌────────────────────────────────┐      │
│      │ caption                        │      │  translucent, only if it has content
│      │ Foto: <credit>                 │      │  (admin: the caption line is editable)
│      └────────────────────────────────┘      │
│  ▭ ▭ ▭ [▣] ▭ ▭ ▭   (filmstrip — wide viewports only)   │
└──────────────────────────────────────────────┘
```

Dark, not black: `#111` behind a photograph reads as a room rather than a hole, and pure black makes a
dark photograph look like a loading failure.

On a phone the filmstrip is gone (§6) and the photograph takes the height back. Everything else is
unchanged — same controls, same panel, same gestures — because this is one feature for two form factors,
not a desktop feature with a mobile cut-down.

### 7.3 Where the viewer's code lives

```
go/cmd/api/viewer/
  viewer.js     ← no template actions, ever (the rule that already governs adminui/*.js)
  viewer.css
```

Embedded and served at `/viewer/viewer.js` and `/viewer/viewer.css`. Both surfaces load the same two URLs,
which is the whole point: one cache entry, one file to fix a bug in.

This is a **third** delivery shape in the repo and worth naming so it is chosen rather than drifted into:
`adminui/*` is injected into its template's source (one document, one parse, contextual escaping); the
public site's CSS is inline in its template (a stylesheet is a second request that can fail). The viewer is
neither — it is a separate cached request, because it is shared. It is allowed to be a second request
precisely because it is an *enhancement*: if it fails to load, the page it is on is the plain page, which is
a state we require to work anyway.

In dev this needs a `'/viewer'` key in `vite.config.ts`'s proxy, for the same reason `/admin` and
`^/\d{4}` have one: without it Vite answers with the SPA shell and the asset looks broken rather than
unrouted. That is dev routing, not an asset under `vue/`.

### 7.4 The data contract

Each tile carries what the viewer needs, and nothing else:

```html
<a class="tile" href="/api/public/albums/al-1/media/12"
   data-viewer-item data-full="/api/public/albums/al-1/media/12"
   data-thumb="/api/public/albums/al-1/media/12?variant=thumb"
   data-caption="Morgenmad i regnen" data-credit="Foto: Jens Hansen">
```

`data-` attributes rather than a JSON blob in a `<script>` tag, for the reason the admin tool already
gives: a value in an attribute is escaped as an attribute by `html/template` and is inspectable in dev
tools, while a value interpolated into JavaScript is escaped as JavaScript and mangles in ways nobody
notices until a browser does something strange. Admin cells carry the same four attributes pointing at
`/api/admin/photos/{id}/media?year=…`. The viewer knows nothing about albums, ordinals or photo ids.

### 7.5 Fullscreen — yes, with one platform caveat worth knowing in advance

Yes. `Element.requestFullscreen()` on the overlay, `document.exitFullscreen()` to leave, driven by a user
gesture (which a button click is), and it escapes the browser chrome — address bar, tabs, the lot — which
the full-viewport overlay cannot do on its own.

Three things to build in rather than discover:

- **Safari needs a prefixed fallback.** `el.requestFullscreen || el.webkitRequestFullscreen` and the
  matching exit/`fullscreenchange` pair. Two lines, and without them the button silently does nothing on a
  Mac.
- **iPhone has no element fullscreen at all.** iOS Safari implements fullscreen only for `<video>`;
  iPadOS Safari does support it for elements. So the button is **feature-gated on
  `document.fullscreenEnabled || document.webkitFullscreenEnabled` and absent when false** — a visible
  button that does nothing is worse than no button. This costs an iPhone nothing real: the overlay already
  covers the viewport, and an installed PWA is standalone anyway.
- **The button must reflect state**, icon and label both (Lucide `maximize` / `minimize`), because the user
  can leave fullscreen by pressing `Esc` or swiping without touching our button — and `Esc` in fullscreen
  must leave fullscreen *without* also closing the viewer, which means listening to `fullscreenchange`
  rather than tracking it ourselves.

### 7.6 Icons

Inline SVG copied from Lucide, as `.rules` requires on server-rendered surfaces: `chevron-left`,
`chevron-right`, `x`, `maximize`, `minimize`, `share-2` (public), `pencil` and `check` (admin). Same icon
set as the app, different delivery.

### 7.7 The action row is declared by the page, and the caption editor

The viewer has no idea which surface it is on, and must not acquire one. The host page declares its actions
on the container the viewer reads its items from:

```html
<!-- the public album page -->
<div id="album" data-viewer data-viewer-actions="share,fullscreen"
     data-share-title="Lørdag morgen — Nathejk 2026">

<!-- the admin contact sheet -->
<div id="sheet" data-viewer data-viewer-actions="caption,fullscreen"
     data-caption-endpoint="/api/admin/photos">
```

So "the public viewer has no caption editor" is not a branch that can be inverted by a visitor — the code
for it is never asked to render, and the endpoint behind it is behind `requireAdmin` anyway. An
`if (isAdmin)` inside `viewer.js` would be the same behaviour with a worse failure mode: one boolean away
from a public edit button, and nothing in the test suite able to execute the branch to prove otherwise.

**The caption editor.** The caption line in the info panel becomes a plain `<textarea>` (one line, growing to
three) with a save control, opened by the pencil in the action row. It carries the caption sheet's own
sentence — the caption belongs to the *photograph*, not to this album — in whatever exact Danish
`captionaction.js` already uses, copied rather than re-worded, because two phrasings of one fact is how a
curator learns to distrust both.

**The credit editor**, added by §11.5's answer, sits under it and is deliberately **not** the same control.
Three differences, each with a reason already established elsewhere in the codebase:

- **Its own save.** One form writing both fields means a curator fixing a caption typo can blank a credit by
  leaving it alone in a form that submits everything.
- **Its own "remove" action**, because `creditaction.js` decided that clearing an attribution should be a
  deliberate act rather than what a stray select-all-and-delete does on the way past. That reasoning does not
  weaken in a viewer; if anything the arrow keys make it stronger.
- **Its own warning.** This is the one field in the whole tool that records a person's name, and the only one
  on the public site that does (task 393, PRD 011's single documented exception). The field shows the line as
  it will publicly read — *"Foto: Jens Hansen"* — so a curator typing a colleague's name knows that is what
  they are doing. The sheet says this; the viewer must not be the quiet way to do the same thing.

Prefilled from the photograph, not from the sheet's `hej.admin.lastCredit`. That key exists because a *batch*
has no single current value to show and retyping one line per card is real tedium; with one photograph in
front of you, the honest prefill is its own credit.

Both call `PATCH /api/admin/photos` with a single-element `photoIds`. That endpoint exists, is annotated, is
behind `requireAdmin`, and is the same call the two sheets make — which means the viewer inherits their
behaviour for free, including the fact that both fields belong to the photograph and therefore change in
every album at once.

On save: update the host item's `alt` and `data-caption` / `data-credit`, then reuse the sheet's existing
post-action refresh. Do **not** invent a second way for either field to reach the sheet; the one that exists
is the one that is already tested.

The arrow keys are where this gets nice: caption a photograph, press `→`, caption the next. Or credit a whole
card one photograph at a time while actually looking at them. That is the real curator loop after a hand-in,
and it is the reason this was worth overriding §4 for.

### 7.8 Share — a link to this photograph in this album

The shared URL is `https://…/2026/album/{slug}?foto={ordinal}#foto-{ordinal}` — absolute, because it is going
into somebody else's messaging app, and **carrying both a query and a fragment**, because they do two different
jobs. Task 401 found this while implementing: the first draft of this section said the query alone would scroll
the recipient to the photograph, and it will not.

- The **query** is for the server: it is what lets the handler render the page that holds item 137 rather than
  the album's first page.
- The **fragment** is for the browser: a query string scrolls nowhere, and only `#foto-137` matching a tile's
  `id` gets a recipient to the photograph without script.

Neither half is redundant and neither can do the other's work — a fragment is never sent to a server, so a
hash-only link cannot reach past the first page at all. Each still degrades sensibly alone: query-only lands on
the right page unscrolled, fragment-only scrolls correctly within whatever page it got.

This is why the viewer's address is a **query parameter and not a fragment**, and the requirement is worth
reading in that order: a hash is client-only, so a recipient who opens `#foto-137` lands on the album's
first page and, if the album is past the cap, the photograph they were sent is a "Vis flere" press away.
`?foto=137` lets the **server** do the work it is holding the data for: it renders the page containing item
137, the tile carries `id="foto-137"` so the browser scrolls there by itself once the fragment is on the URL
too, and the viewer opens on it if it loaded. A recipient with JavaScript off still gets the right photograph
on the right page, which a hash cannot do at all. With a 200-item cap most albums are one page and the
derivation is trivial — it is there so that a shared link does not quietly stop working the first time an album
is bigger.

**And the derivation is a lookup, not a division.** An ordinal identifies a slot in the album, not a position
in it: `album_item` rows are soft-deleted and a deleted photograph stops satisfying `BySlug`'s join, so a
long-lived album hands back sparse ordinals and `ordinal / cap` sends a visitor to a page the photograph is not
on (task 401).

Also worth noting given §2's timing: a shared album link is read **days later**, often on a different device
from the one that sent it. That is the opposite of glimt's "look at this now" and it is the reason the address
has to be a real server-side one rather than app state.

Mechanics:

- `navigator.share({ title, url })`, gesture-triggered, secure context — both true. Present on iOS/Android
  and on Safari/Chrome desktop; **absent on desktop Firefox**, which is the case the fallback exists for.
- Fallback: `navigator.clipboard.writeText(url)` plus a Danish line in the viewer — *"Linket er kopieret"* —
  that clears itself. Copying is a real answer; a button that opens nothing is not.
- Neither available: no control. Same rule as the fullscreen button in §7.5.
- `title` is the album's title, from `data-share-title`. Never the caption: a caption is free text a curator
  typed and the one place a person's name could plausibly end up in it, and a share sheet's title is the
  string that gets quoted into a group chat.
- No `files`, ever (§4). If somebody wants the JPEG they can save it from the photograph; the difference is
  that we are not the ones handing out copies that outlive a takedown.

### 7.9 An 800 px rendition, and letting the browser choose it

Maintainer, 2026-09-25: *"if a small screen then don't fetch original image — a medium size would do"*. Right,
and the numbers back it: the display image is 1600 px and ~200–400 KB, while a 390 pt phone can show about
800 px of it at 2×. Four times the pixels for no visible gain, times however many photographs somebody swipes
through.

**A third rendition, not a resize on request.** `imaging.Prepare` already takes a *list* of thumbnail edges
(`glimtThumbEdges` is `[]int{320}` today and the loop over `thumbEdges` is already there), so producing an
800 px rendition at upload is a one-element change in that slice. There is no image-resizing endpoint in this
service and this is not the feature that should introduce one.

**The browser chooses, not our JavaScript.** The viewer's `<img>` gets `srcset` with both URLs and their
widths, plus a `sizes` hint:

```html
<img srcset="…/media/12?variant=medium 800w, …/media/12 1600w"
     sizes="(max-width: 40rem) 400px, 100vw" …>
```

Two reasons this is better than `matchMedia` in `viewer.js`:

- **Device pixel ratio is part of the decision and we do not have to think about it.** A 390 pt phone at 3×
  and an iPad at 2× want different answers to "is this a small screen", and `srcset` already knows both
  numbers.
- **Rotation and window resizing are free.** A JS branch taken once at open is wrong the moment a phone turns
  sideways.

The `sizes` value is worth reading carefully, because it is deliberately not the truth: a phone's slot is
~100vw, and declaring that on a 3× display would ask for 1170 px and therefore pick the 1600 w candidate —
re-introducing exactly what this section removes. Declaring `400px` caps a narrow viewport at the 800 px
rendition, which is still a genuine 2× image on a 390 pt screen. Only a 3× flagship gives up anything, and at
800 px over 390 pt it is not perceptible. **This is a known technique and it is a lie about layout, so it gets
a comment in `viewer.css`/`viewer.js` saying so** — otherwise the next person "fixes" it.

**Missing renditions fall back, so there is no migration.** Every photograph uploaded before this ships has
no 800 px variant, and `album_item`/`photo` already establish the rule for that: `thumbRef` may be `""` and
readers fall back to the full image rather than rendering a gap. `variant=medium` behaves identically. A
backfill is therefore optional, and if it is ever run, the cheapest moment is **now** — PRD 022 is still in
`doing/` and the library holds test data rather than an event's photographs.

---

## 8. Technical Considerations

### Frontend (Vue 3 / TS)

**None.** This PRD touches neither `vue/src/` nor the PWA, beyond one dev-proxy key in `vite.config.ts`.

### BFF (Go)

- `albumpage.go` — `albumPageHandler` gains a **cap** (`albumPageCap`, proposed 200) and `?side=N` for
  albums past it, plus `HasMore`/`NextSide` for the "Vis flere" link. It also accepts **`?foto={ordinal}`**
  and derives which page to render from it (§7.8) — the one piece of server work the share button adds. An
  out-of-range or non-numeric `foto` is **ignored**, not a 404: the address still names a real album, and a
  link that half-rots should land on the album rather than on an error. The item view type gains nothing (it
  already carries caption, credit, dimensions — and deliberately no coordinate).
- `publicsite.go` — the `album` template's item list moves into `{{define "album-items"}}`, which is cheap
  now and is what the deferred fragment endpoint would reuse. The page's `<head>` picks up the viewer's
  `<link>` and `<script defer>`, and its container carries `data-viewer-actions="share,fullscreen"` plus
  `data-share-title`.
- `viewer.go` (new) — `//go:embed viewer/viewer.js viewer/viewer.css`, plus a handler serving exactly those
  two by a fixed map lookup. **Not** by joining the URL parameter to a directory: that is one encoded `../`
  from serving this service's templates out of the embedded filesystem, which is the reasoning
  `serveAdminVendorHandler` already records.
- `adminui/fragments.html`, `page.css`, `contactsheet.js` — the expand control on a cell, and the two lines
  that open the viewer. `page.html` gains the viewer's two tags and
  `data-viewer-actions="caption,fullscreen"`.
- **No Go change for the caption editor.** It is the existing `PATCH /api/admin/photos`, called with one id.
  That is the point of choosing it: a second write path for one field would be a second place to get
  "belongs to the photograph, not the album" wrong.
- **Deferred (§2a):** the items-fragment handler. Not written until a measurement says the cap is not enough.

### API endpoints

| Method | Path | Purpose |
|---|---|---|
| GET | `/{year}/album/{slug}` | now accepts `?side=N` and `?foto=N`; **existing annotation needs updating** |
| GET | `/viewer/{asset}` | the viewer's JS and CSS, from a fixed allowlist |
| GET | `/api/public/albums/{albumId}/media/{ordinal}` | gains `variant=medium`; **annotation needs updating** |
| GET | `/api/admin/photos/{photoId}/media` | gains `variant=medium`; **annotation needs updating** |
| PATCH | `/api/admin/photos` | **unchanged, reused** by the caption and credit editors — already annotated |
| GET | `/{year}/album/{slug}/side/{n}` | **deferred** — the items fragment, only if infinite scroll is built |

All three carry **OpenAPI annotations** (`.rules`: every endpoint, including the HTML ones — the public
site's handlers are already annotated this way, `@Produce html`). The fragment route documents that it
answers the same 404/503/429 set as the page it belongs to, and that it is **not** a JSON API.

`?side=` rather than `?page=`: the surrounding copy is Danish and the query string is part of what a visitor
sees and sends.

### Data / storage

**One new rendition, and it is the only schema change in this PRD.** §7.9 adds an 800 px variant, which means:

- `photo.mediumRef VARCHAR(64) NOT NULL DEFAULT ""` — the same shape as `thumbRef`, including the documented
  "may be empty, readers fall back to the full image" rule, which is what removes the need for a migration.
- The upload event gains the ref, and the fold writes it. `imaging.Prepare`'s `thumbEdges` becomes
  `[]int{800, 320}` for photographs (`glimtThumbEdges` is shared with glimt — **decide explicitly whether
  glimt gets the second rendition too, or whether the constant splits**; the album path reads it today, and a
  shared constant quietly changing glimt's storage is the kind of change nobody reviews).
- `album.Item` and `photo.LibraryPhoto` carry it; the two media routes accept `variant=medium`.
- **`KEY medium_lookup (mediumRef)`, and the shared-blob delete check must ask about it.** This is the part
  that would bite: `glimtRefsOf`, `RefsUsedElsewhere` and the album/photo delete paths enumerate *every*
  column that can name a blob, because identical bytes are one object and a delete has to ask every table
  (task 368). A new ref column that those walks do not know about is either an orphaned object forever or —
  worse — a live object deleted because nothing claimed it. Task 368's reasoning applies verbatim.
- Nothing about the 1600 px display image or the 320 px thumbnail changes.

**No backfill in scope.** Old photographs serve the display image, which is correct rather than degraded. If
we want one, now is the cheapest it will ever be: PRD 022 is in `doing/` and there is no event's worth of
photographs in the library yet.

### Dependencies & risks

- **No new dependency**, third-party or otherwise. Nothing vendored, so `vendor.txt` and its version guard
  are untouched.
- **`TestTheAdminToolAddsNothingToTheFrontend` will fail**, by design: it allowlists exactly four asset tags
  on the admin page. The fix is to add `/viewer/viewer.js` and `/viewer/viewer.css` **by name, with the
  reason**, exactly as the guard's existing four entries do — never by loosening the needle. That guard
  failing is the process working.
- **The `PICO:` list gains a fourth entry.** Pico puts native `<dialog>` at `z-index: 999` and styles it
  (flex, backdrop-filter, its own background); the admin tool's local overlays deliberately sit at
  1040/1050 above it. The viewer is a real `<dialog>` and must be reset and placed above both, scoped so the
  public site — which has no Pico — is unaffected.
- **Risk: a deferred decision becomes an invisible one.** §2a defers infinite scroll behind a measurement. If
  that measurement is never taken, the cap is the whole answer and nobody revisits it. Hence the first task
  in §10 is the measurement, with a number attached, rather than a note to be careful.
- **Risk: a JS bug takes the album page with it.** Mitigated by the enhancement shape — the grid, the links
  and the "Vis flere" control are server-rendered and functional before `viewer.js` parses. Worth an
  explicit test that the page contains a working `href` per photograph.
- **Testing reality (from the skill):** there is no way to execute the viewer's JavaScript in this test
  suite. The cap, the deep link, the links and the asset route are all behavioural Go tests. The viewer's own
  behaviour is guarded by source-reading tests, which must strip comment lines before searching — a guard
  matching the comment that explains it has happened four times in this repo — and by manual QA on the
  devices in §6's baseline. This asymmetry is a reason to keep the viewer small, and a second reason to be
  glad §2a removed the observer from v1.

---

## 9. Success Metrics

- **A phone fetches the 800 px rendition, not the 1600 px one** — checkable in a network panel, and the
  clearest single signal that §7.9 works.
- No thumbnail or large image is displayed above its stored resolution, on any viewport in the baseline.
- An album page's response is bounded: at most 200 items, whatever the album holds.
- **A measured answer to §2a exists** — render the 2025 import (task 348) at the fixed tile size and record
  HTML bytes, node count and time-to-interactive on a real mid-range Android. Either infinite scroll is
  justified by that number or it is closed as unnecessary. "We never checked" is the failure.
- A curator can judge and sort a 300-photograph hand-in without leaving the sheet — the qualitative check
  is a single sitting with the maintainer after the first real upload.
- Zero regressions in the no-JavaScript path: the existing public-site tests stay green, plus a new one per
  new degradation.
- The viewer is one file used by two surfaces. If a second copy appears, this PRD failed.
- A shared `?foto=` link opens on the right photograph on a device that has never loaded the album — tested
  with script and without.
- Captioning a run of photographs in the admin viewer is faster than the sheet's caption panel for the
  one-at-a-time case. If it is not, the editor is in the wrong place.
- The viewer is used on **both** desktop and mobile. If the filmstrip decision or the layout makes one of
  them second-class, §2's instruction was not met.

---

## 10. Rollout / Task Breakdown

Sequenced so each step ships on its own. The first two are worth doing even if the rest slips, and the
viewer task is the one that must not be split, because a half-wired viewer is a broken click.

- [ ] Task 398: the album grid's tile size — square ~150 px tiles, sharp thumbnails. Caption and credit stay
      under the tile for now; they move into the viewer in task 403, **with** the viewer (§7.1 explains why
      that order is not negotiable). **Ships alone, no dependencies, biggest single win** (§2a.2)
- [ ] Task: cap the album page at 200 items with a plain `<a href="?side=2">Vis flere</a>` — no JavaScript,
      about twenty lines in one handler (§2a.3)
- [ ] Task: **measure** the 2025 import at the new tile size — HTML bytes, node count, time-to-interactive on
      a mid-range Android — and record the answer to §2a in this PRD. This is what decides whether the two
      deferred tasks at the bottom are ever written
- [ ] Task: `?foto={ordinal}` as a server-side deep link — the page containing the item, `id="foto-n"` per
      tile, an out-of-range value ignored rather than 404'd. **Before** the share button, which is only
      useful once the link it produces resolves
- [ ] Task: the viewer itself — `go/cmd/api/viewer/{viewer.js,viewer.css}`, the embedded asset route, the
      `/viewer` dev-proxy key, the host-declared action row (§7.7), and the filmstrip's wide-viewport-only
      media query
- [ ] Task: wire the viewer into the public album page, including the per-photograph `href` fallback and
      the `?foto=` history handling
- [ ] Task: the fullscreen button, prefixed fallback, feature gate, `fullscreenchange` state
- [ ] Task: the public share button — `navigator.share`, clipboard fallback with a Danish confirmation,
      absent when neither exists, album title never a caption
- [ ] Task: wire the viewer into the admin contact sheet and album editor, with the expand control, and
      extend the frontend-allowlist guard by name
- [ ] Task: the admin caption editor in the viewer — reusing `PATCH /api/admin/photos`, the caption sheet's
      own "belongs to the photograph" sentence, typed text kept on failure, and the existing post-action
      refresh
- [ ] Task: the admin credit editor in the viewer — its own field, its own save, its own "remove" action, the
      public-consequence line ("Foto: …"), prefilled from the photograph and **not** from
      `hej.admin.lastCredit`
- [ ] Task: the 800 px rendition — `mediumRef` column, event field, fold, querier, `variant=medium` on both
      media routes, **the shared-blob delete walks taught about the new column**, and the glimt-or-not
      decision on `glimtThumbEdges`
- [ ] Task: the viewer picks its rendition with `srcset`/`sizes`, capped to 800 px on narrow viewports, with
      the comment explaining why `sizes` under-declares; prefetch follows the same choice
- [ ] Task: QA pass on the baseline devices — iPhone Safari (no fullscreen button, native share sheet, no
      filmstrip, **800 px fetched not 1600**), iPad Safari (fullscreen, filmstrip), Chrome desktop, desktop
      Firefox (clipboard fallback), and once with JavaScript disabled including opening a shared `?foto=` link

**Deferred, written only if the measurement says so (§2a.4):**

- [ ] Task: the items fragment endpoint, rendered from the shared `{{define}}`, with the guard that it and
      the page render identical items
- [ ] Task: the IntersectionObserver that appends it, keeping "Vis flere" in the markup as the floor

No feature flag. The album page is already behind `PUBLIC_ALBUMS` (task 359) and the admin tool behind its
credential, which is flag enough; a second switch inside a page would be a state nobody tests.

---

## 11. Open Questions

Five of the seven are answered. They are kept rather than deleted, because the answer is the interesting
part and a question that vanishes looks like one nobody asked.

- ~~1. Caption, description or credit in the panel?~~ **Answered 2026-09-25: caption and credit only, no
  description.** The album's description repeats on every photograph and says nothing about the one in front
  of you.
- ~~3. Filmstrip on a phone?~~ **Answered: dropped on small screens**, by media query, with swipe and arrows
  carrying the phone. The feature is for desktop *and* mobile, so this is two layouts of one thing rather
  than a cut-down.
- ~~4. Should the public viewer offer the large image as a link?~~ **Answered: it gets a share button
  instead** (§7.8) — the same capability aimed at what people actually do, which is send one photograph to
  one person. Still no download button.
- ~~5. Does the admin viewer edit anything?~~ **Answered: the caption, and only the caption** (§7.7).
- ~~5. Does the credit follow the caption into the editor?~~ **Answered 2026-09-25: yes**, with its own field,
  its own save, its own "remove" action and its own warning about being the one field that publishes a
  person's name (§7.7).
- ~~6. A third, medium (≈800 px) variant?~~ **Answered "no", then reversed: yes, for small screens.** Both
  halves were about different things, which is why the reversal is a refinement rather than a contradiction:
  the "no" was about *not degrading the desktop image* on the grounds that after-event home connections are
  not short of bandwidth, and it stands — a laptop still gets 1600 px. The "yes" is about not sending a phone
  four times the pixels it can display. `srcset` serves both answers at once (§7.9).

Still open:

2. **Is the cap 200, and does infinite scroll get built at all?** Open by design rather than undecided: §2a
   says the honest answer needs a measurement, and §10's third task takes it. What is needed now is only
   agreement that 200-with-a-link is the right v1 and that the observer waits for a number.
4. **Does the admin viewer need the sheet's marks** (position / album count / patrol / deleted) in the info
   panel? They are part of why a curator opens a photograph. Reading them is not editing, so it no longer
   conflicts with §4 — but it is more surface, and the marks are already on the cell behind the overlay.
8. **Does glimt get the 800 px rendition too?** `glimtThumbEdges` is shared, so the medium variant either
   lands in both features or the constant splits. Glimt's own viewer is the PWA's, which has the same phone
   problem — so "both" may be right, but it changes a second feature's storage and belongs to whoever owns
   PRD 019, not to a silent constant edit here.
9. **Is an 800 px backfill worth running?** Not needed for correctness (§7.9's fallback), but it is cheapest
   today and grows more expensive with every hand-in.
5. **Does the credit follow the caption into the editor?** Same shape of field, same endpoint family
   (`creditaction.js`), arguably the same moment — you learn who took a photograph by looking at it. Left out
   because it was not asked for, and because a credit is the one public field that names a person, so putting
   it one keystroke from the arrow keys deserves its own yes.
7. **Does a shared `?foto=` link need an Open Graph preview?** A link pasted into a chat currently unfurls as
   nothing, and the page is `noindex`. A thumbnail preview would make a shared link *look* like a shared
   photograph — which is either the point or exactly the republishing §4 rules out. My inclination is no
   `og:image`, and it should be a decision rather than an omission. Sharpened by §2's timing: these links are
   opened days later in a chat thread, which is precisely where an unfurled preview would live longest.
