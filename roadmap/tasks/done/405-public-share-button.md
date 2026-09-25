# 405 — The public share button shares a link to this photograph, never the bytes

**Status:** done
**Priority:** medium
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

PRD 023 §4, §6 ("Functional — share (public only)") and §7.8. A share control in the **public** viewer's
action row that shares the absolute `https://…/2026/album/{slug}?foto={ordinal}#foto-{ordinal}` — a link to this photograph
in this album.

**Depends on task 401.** That task makes `?foto=` resolve on the server; this one is only useful once it does.
Shipping in the other order means shipping a button that hands out addresses nobody can open properly.

### Mechanics

- `navigator.share({ title, url })`, gesture-triggered in a secure context — both true here. Present on
  iOS/Android and on Safari/Chrome desktop.
- **No `files`, ever** (§4). The distinction is the takedown path: a shared *link* stops working when a
  photograph is taken down, and a shared *JPEG* does not. It is also what keeps this a public-site feature
  rather than a republishing tool. If somebody wants the JPEG they can save it from the photograph; we are
  simply not the ones handing out copies that outlive a takedown.
- **Fallback where `navigator.share` is absent** — desktop Firefox, older desktop Safari:
  `navigator.clipboard.writeText(url)` plus a brief Danish line in the viewer, *"Linket er kopieret"*, that
  clears itself. A share button that does nothing on a laptop is worse than one that copies.
- **Where neither API exists, the control is absent rather than inert** — the same rule as the fullscreen
  button in §7.5.

### The title is the album's, never a caption

`title` comes from the container's `data-share-title` (§7.7), which is the album's title. Not the caption, and
this is not fussiness: a caption is free text a curator typed and it is the one place a person's name could
plausibly end up, while a share sheet's title is the string that gets quoted into a group chat. The album page
is `noindex` and stays so — sharing is the visitor's own act, not a change in what we publish.

### Public only

The control exists because the public page declares `data-viewer-actions="share,…"`. The admin page declares
no share action, so there is nothing to un-hide there; no branch in `viewer.js` decides this (§7.7).

§11's question 7 — whether a shared link needs an `og:image` preview — is **not** in scope and must stay a
decision rather than become an omission by implementation. Do not add Open Graph tags here.

## Acceptance Criteria

- [x] The shared URL is absolute and carries **both** `?foto={ordinal}` and `#foto-{ordinal}` for the photograph
      on screen — the query is what lets the server render the right page, the fragment is what scrolls the
      recipient to the tile, and neither can do the other's job (task 401 found this; PRD 023 §7.8). Opening it on
      a device that has never loaded the album lands on that photograph
- [x] `navigator.share` is called with `title` and `url` only — no `files`, asserted by a source guard that
      strips comments before searching
- [x] Without `navigator.share` the control copies the URL and shows *"Linket er kopieret"*, which clears
      itself; verified in desktop Firefox
- [x] With neither `navigator.share` nor `navigator.clipboard` the control is absent, not inert
- [x] The share title comes from `data-share-title` and can never be a caption
- [x] The control appears only where the host page declares the `share` action, so the admin viewer has none

## Progress Log

- 2026-09-25 — Task created from PRD 023.
- 2026-09-25 — Updated while task 401 landed: the shared URL needs the fragment as well as the query. A query string scrolls nowhere and a fragment is never sent to a server, so the link carries both.
- 2026-09-25 — Picked up. Plan: build the absolute URL from the current address plus the ordinal (query and fragment), navigator.share with no files, a clipboard fallback that says so in Danish, and no control at all where neither API exists.
- 2026-09-25 — Registered as a viewer action inside a `canShare || canCopy` gate, so a browser with neither API gets no button rather than an inert one — the same rule as the fullscreen control.
- 2026-09-25 — **No `files`.** The guard asserts the whole call rather than the word "files": a needle that matches the code removing the thing it checks for has fooled this repo before, and "files" appears in no end of innocent contexts. A shared link stops working when a photograph is taken down; a shared JPEG does not, and that is the whole difference between "look at this" and republishing.
- 2026-09-25 — The URL is built from the ordinal rather than read off the address bar. The address bar is only right when `data-viewer-history` is set, and a host page that does not reflect the current photograph would otherwise share whatever page happens to be showing.
- 2026-09-25 — The shared link **drops `?side=`**. It is how this page is cut up today, not part of what is being shared, and the server derives the window from the ordinal (task 401). A page number in a kept link stops meaning anything the moment a curator adds photographs.
- 2026-09-25 — Labelled "Del dette billede" even in the copying case: the visitor's intent is the same and a label should name the intent rather than the mechanism. The confirmation line says which actually happened.
- 2026-09-25 — A cancelled share sheet rejects its promise, and that is swallowed: cancelling is somebody changing their mind, not a failure to report.
- 2026-09-25 — The confirmation is a line in the info panel rather than an alert. An alert is a second modal over a modal and needs dismissing; this is a receipt, not a question.
- 2026-09-25 — No Open Graph tags, per the task: PRD 023 §11 question 7 stays a decision rather than becoming an omission by implementation.
- 2026-09-25 — ✅ All criteria met. `TestShareSendsALinkAndNeverTheBytes` and `TestASharedLinkDropsThePageNumber`. The real check of the native sheet and the Firefox fallback is task 411.
