# 405 — The public share button shares a link to this photograph, never the bytes

**Status:** open
**Priority:** medium
**Created:** 2026-09-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 023 §4, §6 ("Functional — share (public only)") and §7.8. A share control in the **public** viewer's
action row that shares the absolute `https://…/2026/album/{slug}?foto={ordinal}` — a link to this photograph
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

- [ ] The shared URL is absolute and carries `?foto={ordinal}` for the photograph on screen, and opening it on
      a device that has never loaded the album lands on that photograph
- [ ] `navigator.share` is called with `title` and `url` only — no `files`, asserted by a source guard that
      strips comments before searching
- [ ] Without `navigator.share` the control copies the URL and shows *"Linket er kopieret"*, which clears
      itself; verified in desktop Firefox
- [ ] With neither `navigator.share` nor `navigator.clipboard` the control is absent, not inert
- [ ] The share title comes from `data-share-title` and can never be a caption
- [ ] The control appears only where the host page declares the `share` action, so the admin viewer has none

## Progress Log

- 2026-09-25 — Task created from PRD 023.
