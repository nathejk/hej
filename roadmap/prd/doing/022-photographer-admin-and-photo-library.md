# PRD 022 — Photographer admin: a photo library, albums, locations and patrol tags

**Status:** doing
**Author:** agent session (Zed), with maintainer
**Created:** 2026-09-22
**Last updated:** 2026-09-23
**Approved:** 2026-09-22
**Shipped:**
**Target users:** organizer (photographer, curator)

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 0. What this PRD is

**This is the answer to PRD 011 §11 Q5** — "Who curates the albums, and with what?" — and it
takes the second of the two options that question listed: **a small admin page on the public
service**, not a Team-section surface in the app. §8.1 records why, because the reason is not
preference; the app is structurally unable to host this.

It also makes a **modelling change to PRD 011 section 1** that the maintainer's brief requires and
that PRD 011 did not anticipate: photographs become a **year-scoped library** that albums
*reference*, rather than rows that exist only inside one album. §8.3 is the argument. PRD 011 §6
section 1 stays true in every user-visible respect; what changes is where a photograph's
coordinate, caption and patrol tag live.

## 1. Summary

A password-protected page at `/admin` where our photographers upload the weekend's photographs in
bulk from a laptop, and a curator then sorts them: into one or more albums, with a location on one
or many at a time, and optionally tagged with the patrulje they show. The albums that result are the
ones PRD 011 already puts on the public frontpage. Everything is bound to the configured event year.

## 2. Problem & Motivation

**What problem does this solve?** Today nothing creates an album. The projection, the five events,
the fold and the public pages all ship and are tested; the write side is two removal endpoints and a
development fixture. `devalbum.go` says so in as many words — it exists because "the only way to see
an album before the curation tool exists (PRD 011 §11 Q5) is to hand-write events onto a broker".
So the feature on the frontpage is, right now, a feature with no way to put anything in it.

The specific pain is a real workflow that this service currently cannot accept at all:

- Our photographers come home from the event with **hundreds of photographs on an SD card**. They
  are not going to hand them over one at a time, and they are not going to do it on a phone.
- Deciding *which* photographs become an album is editorial work done **after** the upload, over
  several sittings, looking at contact sheets — not a decision made at the moment a file is picked.
  Upload and curation are two jobs, and the current model conflates them by making "add to album"
  the only way a photograph can exist.
- A photograph often belongs in **more than one** album ("Natten", "Målet", "Postmandskabet"). In
  today's model that is two independent rows with two independent coordinates that can silently
  disagree.

**Why now?** PRD 011 is in `doing/` and section 1 is otherwise complete: the frontpage strip
(task 336), the album page (task 334), the map plotting, the removal path (task 335) and the
"hide the strip until there are photographs" state (task 359) are all in `done/`. Curation is the
last missing piece of that section, and it is the piece without which none of the others do
anything. It is also the piece PRD 011 §0b makes a **safety control** rather than a convenience:
"a photograph is in an album because somebody decided it may be shown". There is currently nowhere
for that somebody to stand.

**Evidence.**
- PRD 011 §11 Q5, unresolved, which names the two options and notes "a directory on disk and a
  deploy is a weaker answer than it looks".
- PRD 011 §6 section 1: "Albums are curated, not participant-submitted: an organizer chooses what
  is in them", and "A photograph's location is a deliberate, reviewable field" — reviewable by
  whom, in what?
- `go/cmd/api/routes.go` (the comment above the dev fixture) and `go/cmd/api/devalbum.go`, both of
  which describe themselves as a stand-in for this tool.
- The maintainer's brief, 2026-09-22: `/admin`, basic auth, several photos at once, upload into a
  bulk, then assign to one or more albums, locations on one or more photographs, patrulje tags,
  albums on the frontpage, everything bound to a year.

## 3. Goals

- A photographer can get **a card's worth of photographs into the service in one sitting**, from a
  laptop, without help and without a deploy.
- A curator can **decide afterwards** what becomes an album, and change their mind, without that
  decision being entangled with the upload.
- **One photograph, one set of facts.** A photograph's location, caption and patrol tag are
  properties of the photograph and are the same in every album it appears in.
- Placing a location is a **bulk** operation, because "these forty are from Post 3" is the true
  shape of the work and doing it forty times is how it does not get done.
- The **safety property PRD 011 §0b relies on keeps working**: nothing is on the open web until a
  curator published it, and anything can be taken back down promptly.
- Everything the tool writes is **bound to the configured event year** and cannot leak into another.

## 4. Non-Goals

- **A general CMS.** This edits photographs and albums. The rules, the programme and the practical
  information stay in PRD 013's site.
- **A participant-facing upload path.** PRD 011 §4 already excludes uploading to albums from the
  app, and that stands. Participants share photographs through glimt (PRD 019); this is a different
  permission with a different keeper (PRD 011 §11 Q7).
- **A role, an account system, or per-curator identity.** §8.2 takes a shared credential, and §8.7
  records what that costs. If per-person attribution is ever wanted it is a new decision, not a
  field added quietly.
- **Editing the photographs themselves.** No crop, no rotate beyond the upright correction
  `imaging.Prepare` already applies, no filters, no retouching. That happened before the file
  reached us.
- **A roster of patrols in the admin UI.** The curator types the number from the patrol's sign and
  we confirm the name back. §8.6 is the reasoning; a picker is deferred, deliberately.
- **Face recognition, auto-tagging, or any other inference about who is in a photograph.** Not a
  performance concern — a consent one. Nobody in these photographs agreed to be identified by us.
- **Naming a person anywhere.** A photograph is attributed to a *patrulje*, never to a member.
  PRD 011 §4 and `.rules` both already require this; it is repeated here because a tagging UI is
  exactly where somebody would reach for a name.
- **Mobile use.** The page must not be *broken* on a phone, but it is designed for a laptop with a
  filesystem and a keyboard. No install prompt, no service worker, no offline support.
- **Video.** PRD 020 is a separate draft.
- **Reordering as a first-class gesture in v1.** Ordinals are editable numbers; drag-to-reorder is
  listed in §10 as a follow-up, not a launch requirement.

## 5. User Stories & Scenarios

- As a **photographer**, I want to drop 300 files onto a page and walk away, so that handing in my
  weekend's work is not an evening's clicking.
- As a **curator**, I want to look at everything that was uploaded and *then* choose what goes in
  an album, so that the album is edited rather than accumulated.
- As a **curator**, I want to put one photograph in two albums without copying it, so that its
  location and caption cannot drift apart between them.
- As a **curator**, I want to select forty photographs and say "these are from Post 3", so that the
  map gets its pins without me typing a coordinate forty times.
- As a **curator**, I want to tag a photograph with the patrulje it shows, so that a family can find
  their own patrol's photographs.
- As a **curator**, I want an album to be invisible until I say so, so that a half-finished album is
  not on the open web while I think about it.
- As an **organizer taking a complaint**, I want to remove a photograph and have it gone from the
  public page quickly, so that "we will take it down" is true.

### Primary happy path, end to end

1. The photographer opens `https://hej.nathejk.dk/admin` on a laptop. The browser asks for a
   username and password; they paste the credential the maintainer gave them. The page says which
   **year** they are working in, prominently, because that is the one thing that cannot be undone by
   editing.
2. They drag 300 JPEGs onto the drop zone. The page uploads them **one request per file, three at a
   time**, with a row per file showing a thumbnail once it lands and a red row with a reason if it
   does not. Total and remaining are shown. They can close the tab and reopen it; what landed,
   landed.
3. Each upload becomes a **library photograph** for the year. If the file carried a GPS fix, it is
   read *before* the bytes are re-encoded, bounds-checked, and shown on the row as a small "har
   position" marker — inside, outside or none, distinguishably (PRD 011 §6, and §8.4 below).
4. Later, the curator opens `/admin`. The library is a contact sheet of thumbnails, newest first,
   with filters for *unplaced*, *not in any album*, *has position* and *tagged*.
5. They select photographs by clicking (shift-click for a range, a "select all in filter" link).
   With a selection they can, in one action:
   - **add to an album** — choosing one or more existing albums, or creating a new one inline;
   - **set a location** — by clicking a point on a small map, or by picking a checkpoint from the
     event's positioned checkpoints, which is what "Post 3" actually means;
   - **clear the location**;
   - **tag a patrulje** — typing the number, which resolves to a name shown back for confirmation;
   - **delete** — a soft removal, with an optional reason.
6. They open an album, adjust the order, set captions, and press **Udgiv**. It appears on the
   frontpage, in the position its sort order gives it.

### Edge cases and error scenarios

- **A file that is not an image** (a `.mov` from the same card, a `.CR2` raw, a `Thumbs.db`): the
  row fails with a plain reason in Danish and the rest of the batch continues. The decode is the
  validation, as everywhere else in this service; the declared content type is not trusted.
- **A file over the size limit**: `413`, a plain reason, batch continues.
- **The same file uploaded twice** — routine, because a photographer re-drags a folder. Content
  addressing means the bytes are one object; we must also not create a **second library row**. §8.5:
  the photo id is derived from the content hash, so a re-upload is idempotent and lands on the row
  that already exists, *without* resurrecting one that was deliberately deleted.
- **The upload dies half way** (a closed laptop, a train tunnel). Every file is its own request, so
  the batch is resumable by re-dragging the same folder; already-uploaded files are recognised and
  skipped (previous point).
- **A photograph in two albums, removed from one**: it stays in the other and stays in the library.
  Removing it from the *library* removes it from every album, and that difference must be obvious in
  the copy, because one of the two is the thing an organizer means when they say "take it down".
- **A coordinate outside the race area**: stored, not plotted, and shown to the curator *as
  rejected* — PRD 011 §6 requires this, and the point is that a curator must not have to wonder why
  a pin is missing.
- **No positioned checkpoints yet** (early season): the bounds check has nothing to judge against
  and the verdict is `unknown`, not `outside`. The UI must say "kunne ikke vurderes", because
  `unknown` is a statement about us.
- **The event stream is unavailable**: writes fail with `503` and say so. The page must not appear to
  have saved something it did not — the projection is downstream of the log, so a silent failure
  would look like success until a reload.
- **Two curators at once**: last write wins, per field. Acceptable, and stated so nobody later
  mistakes it for a bug; there will realistically be one curator.
- **A wrong credential**: `401` and the browser's own dialog. No custom login page, no account
  lockout, and deliberately no "forgot password" — §8.2.

## 6. Requirements

### Functional — upload

- [ ] `GET /admin` serves a password-protected page. A wrong or missing credential is `401`.
- [ ] The page states the **event year** it is writing to, unmistakably.
- [ ] **Several photographs at once**: the file input accepts `multiple`, and a drag-and-drop zone
      accepts a folder's worth. There is no per-batch count limit.
- [ ] Each file is uploaded as **its own request**, with bounded concurrency, so one bad file cannot
      fail a batch and a dead connection loses one file rather than three hundred.
- [ ] Per-file progress and a per-file outcome, with a **plain-language Danish reason** on failure.
      Failures do not stop the batch.
- [ ] An upload over the size limit is refused with `413`; a file that does not decode as an image is
      refused with `400`. Neither is retried automatically.
- [ ] Re-uploading identical bytes is **idempotent**: one library photograph, not two, and a
      previously deleted photograph is **not** resurrected by a re-upload.
- [ ] Every stored rendition is re-encoded, which **strips all EXIF including GPS**. This does not
      change (PRD 011 §6, PRD 019, `internal/imaging`).
- [ ] A GPS fix, if present, is read **before** re-encoding, bounds-checked against the race area,
      and stored as a reviewable field with its verdict.

### Functional — the library (the bulk)

- [ ] Uploads land in a **year-scoped library**, not in an album. A photograph can exist having never
      been in one.
- [ ] The library is browsable as a thumbnail contact sheet, newest first, paged.
- [ ] Filters, at least: *not in any album*, *without location*, *with location*, *out of bounds*,
      *tagged with a patrulje*, *untagged*.
- [ ] Multi-select, including shift-click ranges and "select everything matching this filter".
- [ ] A photograph can be **soft-deleted** from the library with an optional reason. It then
      disappears from every album and every public read, and its blobs are purged unless another
      row — including a glimt — still references them.
- [ ] Deleting is recoverable in the log (the event is a soft delete, as PRD 011's removal path
      already is), even if v1 ships no undelete button.

### Functional — albums

- [ ] A curator can **create** an album: title, optional description, sort order. It is
      **unpublished** on creation — an album is assembled over several sittings, and a create that
      could publish would put the first photograph on the open web before the second was chosen.
- [ ] The **slug is frozen at creation**. Retitling must never break a link already shared.
- [ ] A curator can edit title, description and sort order, and **publish / unpublish**.
- [ ] A curator can see **unpublished and deleted** albums and their contents. No public read may
      gain this ability (§8.8).
- [ ] A photograph can be added to **one or more albums** in a single action, and the same photograph
      may be in any number of albums.
- [ ] Adding a photograph already in an album is a no-op, not a duplicate row.
- [ ] Items have an **explicit curator order**, editable. The **cover** is the first live item.
- [ ] Removing a photograph from an album leaves it in the library and in other albums.
- [ ] Deleting an album does not delete its photographs from the library.

### Functional — location

- [ ] A location can be set on **one or many selected photographs in a single action**.
- [ ] Two ways to give it: clicking a point on a map, and choosing one of the event's **positioned
      checkpoints** (which is how an organizer actually thinks about where a photograph was taken).
- [ ] Setting a location **re-runs the bounds check** and stores the new verdict. A curator-placed
      point is not exempt from the check — nothing reaches the public map unverified.
- [ ] A location can be **cleared**, and clearing is distinct from "never had one" in the UI but not
      in the public result: both are simply a photograph with no pin.
- [ ] A photograph's location is **one value shared by every album it is in**.
- [ ] Only `inside` is plotted publicly. `outside` and `unknown` are stored, visible to the curator,
      and never on the public map.

### Functional — patrol tags

- [ ] A photograph can be tagged with **a patrulje**, identified by the number printed on its sign.
- [ ] The number is **validated** against the year's patrols and the patrol's name is shown back
      before the tag is saved. An unknown number is refused with a plain reason.
- [ ] Tagging works on a **multi-selection**.
- [ ] A tag can be removed.
- [ ] A photograph may carry more than one tag (two patrols in one frame is ordinary).
- [ ] A tag names **a patrol, never a person**. There is no field for a member, and none may be
      added.
- [ ] Publicly, a tag renders at most as the patrol's number, name, group and korps label — exactly
      the attribution the public patrol page already carries, and nothing more.

### Functional — the frontpage

- [ ] A published album appears on the public frontpage, in curator sort order, with its cover,
      title, optional description and photograph count. **This already works** (task 336); the
      requirement here is that the admin tool is what fills it.
- [ ] Unpublishing or deleting removes it from the frontpage within the page's cache window
      (60 seconds) and no longer.
- [ ] The existing `PUBLIC_ALBUMS` flag still governs whether the section exists at all (task 359),
      and the admin tool is **not** gated by it — a curator must be able to assemble albums before
      the section is switched on.

### Functional — the year

- [ ] Every photograph, album, location and tag is written under the **configured event year**, and
      every read the tool performs is scoped to it.
- [ ] The tool can only write to the configured year. Changing years is a deploy-time configuration
      change, as everywhere else in this service.
- [ ] A previous year's photographs and albums remain readable on their own year-prefixed public
      pages and are not visible in this year's library.

### Non-Functional

- **Transport.** The credential is sent on every request, so the page must be **HTTPS-only**. In
  production Traefik already terminates TLS; the handler additionally refuses to serve over plain
  HTTP unless running in development.
- **Not indexed.** `X-Robots-Tag: noindex, nofollow` and `Cache-Control: no-store` on every admin
  response. An admin page in a search index is an invitation, and a cached one on a shared laptop is
  a leak.
- **Rate limiting** on the credential check, by client IP, so a shared password is not brute-forceable
  at speed. The comparison itself is constant-time.
- **No personal data in the new projection.** The library row has no uploader, no curator, no person
  id, no name, no phone number, and emphatically no `phoneParent`. The structural privacy test
  (task 337) must walk the new types too.
- **Upload limits.** A per-file ceiling (larger than glimt's 12 MB, since these are camera JPEGs —
  §8.9 proposes 32 MB) enforced with `http.MaxBytesReader`, plus a read deadline generous enough for
  a large file on a hotel connection.
- **Storage.** A card's worth of photographs at ~3 MB of stored rendition each is order 1 GB per
  event. The blob store is the only non-rebuildable data in the service and is already the whole
  backup scope (PRD 008 §8); this materially increases what that backup must hold, and the rollout
  must confirm there is room before a photographer is invited to fill it.
- **Browser baseline.** The admin page may use modern JavaScript freely — it is behind auth, on a
  laptop, and needs `File`, `fetch` and `FormData`. It does **not** inherit the public site's
  deliberately lower floor, because unlike those pages it is not for the public.
- **The public pages stay dumb.** No requirement here may add JavaScript to the album page or the
  frontpage. Their no-JS rendering is a property of PRD 011 §6 and is not negotiable from this side.
- **Accessibility.** Keyboard-operable selection, focus-visible controls, and real `<label>`s. This
  is a tool two or three people use for hours; it does not get a pass.
- **Observability.** Every write logs the action, the affected ids and the client IP. With a shared
  credential the log is the only audit trail there is, which is an argument for it being good.

## 7. UX / UI Notes

**Not in `vue/`.** This is a server-rendered page in the Go service, built the way the rest of the
public site is built: an `html/template` in `cmd/api`, inline CSS, and — new for this page — a small
amount of hand-written vanilla JavaScript for the uploader and the selection model. No bundler, no
framework, no `npm` dependency, no shadcn-vue. §8.1 explains why it cannot be in the PWA, and the
absence of a build step is the reason not to start a second frontend for it.

The one exception: the location picker needs a map, and the public patrol page already ships a
**map island** (task 342, ≈60 KB gzipped, MapLibre) using the app's own layers (task 353). The admin
picker reuses that island rather than introducing a second mapping approach.

Layout, one page with three regions:

- **A header** with the event year in large type, the upload button, and counts ("312 billeder ·
  47 uden album · 12 med position").
- **A drop zone** that collapses to a thin bar once a batch finishes, expanding to a per-file list
  while one is running.
- **A contact sheet** of thumbnails with a filter bar above it, and a **selection action bar** that
  appears pinned to the bottom when anything is selected: *Tilføj til album · Sæt position · Tag
  patrulje · Slet*. Each opens a small inline panel rather than navigating away, because navigating
  away loses the selection.

An album's own view is a second page, `/admin/album/<slug>`: its fields, its publish toggle, and its
items in order with caption fields and an ordinal.

Copy is **Danish**, matching the public site. Two sentences carry real weight and should be written
carefully rather than generated:

- The difference between removing from an album and deleting from the library.
- What `uden for området` and `kunne ikke vurderes` mean on a photograph's position.

Headlines use the Nathejk face, as everywhere else. Icons, where the page needs them, are Lucide —
but note this page has no build step, so they are inline SVG copied from Lucide rather than the Vue
package.

## 8. Technical Considerations

### 8.1 Why this is not in the app, which is where organizer tools live

The precedent argues for the app. Glimt moderation is a Team-section view in the PWA
(`GlimtModerationView.vue`), and its comment makes the case well: "a separate admin console would
mean the person best placed to act on a report is the person least likely to be at a laptop". The
album removal endpoints follow it, gating on `isGlimtModerator` rather than inventing a curator role.

Two things make this feature the opposite case.

1. **The PWA ejects desktops before it authenticates anybody.** `vue/src/router/index.ts` runs a
   device-class gate ahead of the auth gate and `window.location.replace`s a desktop visitor to the
   website. Its comment: "If organizer desktop access is ever wanted, it is a new PRD revisiting
   this gate — not a flag added here." Bulk upload from an SD card *is* desktop access. Revisiting
   that gate is a bigger, riskier change to the participants' app than adding a page to the service,
   and it would be done for the benefit of two people.
2. **The job is a filesystem job.** Three hundred files, a folder, a keyboard, a big screen to judge
   photographs on. A phone-shaped surface is the wrong instrument regardless of what the router says.

So: a page on the service. This answers PRD 011 §11 Q5 with its option (b).

### 8.2 The credential: basic auth, and what it costs

The maintainer asked for basic auth, and it is the right call here, but it must be recorded as a
**deliberate exception** because it is genuinely new to this service.

> **Decision, 2026-09-23 (maintainer): this is explicitly interim.** "We will have a major rework of the
> authentication workflow before next year's race adding role based access, so for now we just need
> something simple, a shared account with username/password supplied as env vars in docker-compose file
> will do."
>
> That changes the status of everything below from *the design* to *the stopgap*, and it is worth being
> precise about what it does and does not license.
>
> It **does** license the shared credential, the env-var configuration, and the absence of per-person
> identity — accepted costs of an interim tool rather than problems to solve now. It also settles §11 Q4's
> harder half.
>
> It does **not** license building anything a role model would later have to unpick. Two things follow, and
> both are already true of the implementation: the credential stays confined to this one tool (it is not a
> session and must never become a second way to authenticate as a person, below), and no projection may
> grow a curator or uploader column that a real account system would have to reconcile with (§8.3). An
> interim credential is cheap to replace; an interim *identity* written into a year's event data is not.
>
> The migration this leaves is deliberately dull: when roles arrive, `requireAdmin` is replaced by whatever
> the new model provides and the two env vars are deleted. Nothing else in this PRD's surface changes,
> because nothing else knows the credential exists.

Today there is no inbound
credential of any kind in `go/`: every authenticated route is `requireAuth` → an HMAC-signed session
cookie from an SMS PIN → a **per-request lookup** in the `person` projection for authorization. There
is no bearer token, no API key, and no admin role. (The single `Authorization: Basic` in the tree is
outbound, to the SMS provider.)

Why the existing mechanism does not fit: it requires the curator to be a **person in this year's
`person` projection with a phone number we hold**, reachable by SMS, and — because the section lookup
is how authorization works — assigned to the right Team section upstream. Our photographers are
frequently none of those things. They are volunteers with a camera, sometimes not registered as
personnel at all, and the tool has to work for them on the Tuesday after the event when the SMS
pipeline is the last thing anybody wants in the loop.

What basic auth costs, stated plainly:

- **No attribution.** Events written by this tool cannot honestly name who wrote them, so they
  should not pretend to. The library row deliberately has no curator field (which also keeps it
  aligned with the album projection's existing "no uploader, no curator" rule).
- **A shared secret, shared.** It will be pasted into a chat message. Rotating it is a config change
  and a redeploy, and the PRD should say so rather than imagine otherwise.
- **No revocation of one person's access** without rotating for everybody.

Mitigations, all required by §6: HTTPS-only, `no-store`, `noindex`, IP rate limiting on the check,
constant-time comparison, a long generated password, and a log line for every write. And one
structural mitigation worth naming: the credential grants **exactly this tool**. It is not a session,
it does not populate the request context, and it must not become a second way to reach any existing
authenticated endpoint.

Implementation: a new wrapper beside `requireAuth` in `middleware.go`, e.g.
`app.requireAdmin(next)`, reading `ADMIN_USER` / `ADMIN_PASSWORD` from config. **If the password is
unset the routes are not registered at all** — the tool is absent rather than open, which is the same
shape as `devRoutesEnabled`. This is important: a misconfigured deploy must not publish an
unprotected upload endpoint.

### 8.3 The data model change: a library, and albums that reference it

This is the substantive technical decision in this PRD.

**Today** `album_item` *is* the photograph: keyed `(albumId, ordinal)`, carrying `blobRef`,
`thumbRef`, `caption`, dimensions, `latitude`, `longitude` and `boundsVerdict` inline. That was a
sound design for the feature PRD 011 described, where a photograph came into existence by being put
in an album.

The brief breaks it in three places:

- "upload into a bulk" — a photograph must be able to exist in **no** album.
- "add photos to one or more albums" — the same photograph in two albums would today be two rows
  with two coordinates, two captions and two verdicts, free to disagree. A curator who fixes a
  location in one album and not the other has created a bug they cannot see.
- "assign a location to one or more photos" — the location is plainly a fact about the *photograph*.

So the model becomes:

```
photo        (the year's library — one row per photograph)
album        (unchanged)
album_item   (membership: which photo sits at which position in which album)
photo_patrol (tags: which patrols a photograph shows)
```

`photo` owns `blobRef`, `thumbRef`, dimensions, `bytes`, `caption`, `latitude`, `longitude`,
`boundsVerdict`, `uploadedAt`, `deleted`. `album_item` keeps `(albumId, ordinal)` as its key — which
preserves the public media URL `/api/public/albums/{albumId}/media/{ordinal}` unchanged — and
replaces every media column with a `photoId`.

Consequences to handle deliberately:

- **The public map read** (`Plottable`) now joins `photo` → `album_item` → `album` to answer "a
  published album's plottable photographs". The index that made that read cheap on `album_item`
  moves to `photo`.
- **The shared-blob check** (`RefsInUse`, which `glimtdelete.go` also depends on) now asks `photo`
  rather than `album_item`. It must keep answering "does anything still reference these bytes", and
  the existing rule stands: **any error means nothing is deleted**.
- **Caption lives on the photo**, one place to edit, shared by every album. A per-album caption
  override is imaginable and is deliberately deferred to §11 Q3 rather than built speculatively.
- **The cover** stays "the first live item", as it is today. No cover column.

### 8.4 What does *not* change, and must not

- **`imaging.Prepare` destroys EXIF, including GPS, by re-encoding.** A coordinate is read from the
  original bytes *before* that and stored in a column. Both `album/table.sql` and `albummedia.go`
  carry an explicit warning not to "fix" the pipeline because a feature wants a coordinate, and this
  PRD adds a reason to want one. The warning holds: a coordinate in a column is a decision somebody
  made, which a curator can see, correct and delete; a coordinate inside a stored file is a leak
  waiting to happen.
- **The four bounds verdicts and `Plottable`.** Only `inside` is plotted. `unknown` is not folded
  into `outside`.
- **Publication filtering in `album.Queries`.** That interface is read by unauthenticated handlers
  and its doc comment calls the shape the privacy boundary. §8.8.
- **The public album page and frontpage render without JavaScript.**
- **Blobs are never addressed by ref in a URL.** Always `(entity, ordinal[, variant])` → a
  projection read that applies the visibility filter → `blob.Ref` → `streamGlimtMedia`.
- **`storeAlbumImage`'s existing pipeline** — `ReadGPS`, then `Prepare`, then `blobs.Put`, with a
  failed thumbnail logged rather than fatal. The new upload handler wraps it; it does not replace it.

### 8.5 Idempotent upload

The photo id is derived from the content hash (`blob.ComputeRef` of the stored full rendition — a
deterministic function of the uploaded bytes, since `Prepare` is deterministic). Re-uploading the
same file therefore publishes an `Uploaded` event with the same id, and the fold upserts. Two
properties this must have, and they are in tension:

- A re-upload must **not** create a second row, because re-dragging a folder is routine.
- A re-upload must **not** resurrect a row a curator deleted, because the delete may have honoured
  somebody's objection. This is exactly the rule `album`'s `created` fold already follows: the upsert
  **does not touch `deleted`**.

The upload handler should additionally tell the browser which it was, so the UI can say "already
uploaded" rather than claiming a fresh success — and, for a previously deleted photograph, say so
plainly instead of silently doing nothing.

### 8.6 Patrol tags without a roster read

`publicpatrol.Queries` has exactly one method, `ByNumber(year, number)`, and its doc comment says
the absence of a list read is deliberate: "a list read is what a scraper would ask for". There is no
patrol list anywhere in the service at any layer.

An autocomplete picker would need one. **v1 does not build one.** The curator types the number from
the patrol's sign — which is how they know it — and the tool resolves it through the existing
`ByNumber` and shows the name back for confirmation. This gets the feature with no new read, no new
enumerable surface, and no widening of a type that unauthenticated handlers read through. If a picker
is later wanted, it must be a **separate authenticated interface**, not a method added to
`publicpatrol.Queries`.

Note `teamNumber` is **not unique per year** (the index is deliberately non-unique and `ByNumber`
does `ORDER BY teamId LIMIT 1`). A tag therefore stores the resolved `teamId` *and* the number, so
the tag survives a renumbering and does not silently point at a different patrol.

### 8.7 Events

All new events follow the established shape: one event per fact, references never bytes, pointers
for optional-update fields so "not mentioned" and "set to empty" differ. Subjects follow
`album.Subject`'s pattern and validate their tokens.

New, on `NATHEJK.<year>.photo.<photoId>.*`:

| Verb | Fact |
|---|---|
| `uploaded` | a photograph entered the library: refs, dimensions, bytes, EXIF coordinate + verdict |
| `updated` | caption, and/or location + verdict, changed (pointer fields) |
| `locationcleared` | the coordinate was deliberately removed — distinct from `updated` so the log records the intent |
| `patroltagged` / `patroluntagged` | a patrol attribution added or removed |
| `deleted` | soft delete, with a reason |

Changed, on the existing album subject: **`itemadded` carries a `photoId` instead of media fields.**
This is a breaking change to a published event shape, and the honest reason it is acceptable is that
**no album has ever been created outside a development fixture** — there is no curation tool, which
is why this PRD exists. Still, the fold must **tolerate** a legacy `itemadded` (one with a `ref` and
no `photoId`) rather than error the replay: an error is logged and dropped by the stream library, so
a strict fold would turn old dev fixtures into a wall of warnings on every boot. Recommended
handling: ignore legacy item events, and note it in the fold with the reason.

A new `itemremoved` is not needed — the existing one is keyed `(albumId, ordinal)` and still fits.

### 8.8 Reads: the draft-visible interface is a new one

`album.Queries` is publication-filtered in SQL, and `BySlug` returns the same "not found" for
unknown, unpublished and deleted so that drafts cannot be enumerated. The curator needs the
opposite: every album including unpublished ones, every item including removed ones.

**Do not widen `album.Queries`.** Add a separate interface — `album.CuratorQueries` /
`photo.CuratorQueries` — wired onto its own field on `app.models`, so that a public handler
*structurally cannot* reach a draft read. This mirrors the reasoning `publicpatrol.table.go` already
records about not reading the organizers' table from a public handler: "It is not here" is a
property; "we do not select it" is a habit.

### 8.9 API endpoints

All under `/api/admin/`, all wrapped in `app.requireAdmin`, all returning JSON.
**Every one needs OpenAPI annotations** — `@Summary`, `@Description`, `@Tags`, `@Produce`,
`@Success`, `@Router`, and a documented `@Failure 401`. Two notes on the guard:

- `glimtopenapi_test.go`'s `isInScope` does **not** currently cover these paths (they are not
  `/api/glimt*`, not `/api/public/`, not year-prefixed), so the annotations would not actually be
  enforced. Widening it is `roadmap/tasks/open/328-openapi-guard-whole-api.md`; this PRD should at
  minimum extend `isInScope` to `/api/admin/`, and a new tag `admin` must be added to
  `allowedGlimtTags`.
- Handler names must end in `Handler`, and route paths must be plain string literals, or the
  AST-based guards cannot parse them.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/admin` | the tool (HTML) |
| `GET` | `/admin/album/:slug` | one album's editor (HTML) |
| `POST` | `/api/admin/photos` | upload **one** file; `413` over limit, `400` if it does not decode |
| `GET` | `/api/admin/photos` | the library, paged and filtered |
| `GET` | `/api/admin/photos/:photoId/media` | thumbnail/full bytes for the contact sheet |
| `PATCH` | `/api/admin/photos` | bulk: set/clear location, set caption on a selection |
| `DELETE` | `/api/admin/photos/:photoId` | soft delete, optional reason |
| `POST` | `/api/admin/photos/tags` | bulk: tag a selection with a patrol number |
| `DELETE` | `/api/admin/photos/:photoId/tags/:teamId` | untag |
| `GET` | `/api/admin/patrols/:number` | resolve a number to a patrol, for confirmation |
| `GET` | `/api/admin/albums` | every album, including unpublished |
| `POST` | `/api/admin/albums` | create (always unpublished) |
| `PATCH` | `/api/admin/albums/:albumId` | title, description, sort order, published |
| `POST` | `/api/admin/albums/items` | bulk: add a selection of photos to one or more albums |
| `PATCH` | `/api/admin/albums/:albumId/items` | reorder, set per-item fields |

The two existing removal endpoints (`DELETE /api/albums/:albumId` and
`.../items/:ordinal`, gated on the Team section) **stay as they are**. They are the
organizer-in-the-app takedown path and they answer a different question — "somebody complained and
I am at a barbecue with my phone" — than the curator's tool does. Two doors to the same removal is
correct here; what must not happen is the two disagreeing about blob purging, so both must go through
the same purge helper.

The upload limit is proposed at **32 MB** per file (glimt's is 12 MB), with a generous read deadline.
Camera JPEGs from a full-frame body routinely exceed 12 MB and a photographer hitting `413` on a
normal file would reasonably conclude the tool is broken.

Rate limiting: the per-user glimt limiters do **not** apply (there is no user), and
`storeAlbumImage`'s comment already records that album ingest deliberately skips them as "an
organizer action on a known set of photographs". That stands for throughput, but the **storage
ceiling** deserves a second look at rollout — an organizer with a stuck upload script can fill a disk
just as effectively as a participant.

### 8.10 Frontend (Vue 3 / TS)

**No change.** This is worth stating explicitly rather than leaving as an absence: the PWA is not
touched, no route is added to `vue/src/router/index.ts`, the device gate is not modified, and no
shadcn-vue component is generated. If a future PRD moves curation into the app, that is where the
device gate gets revisited.

### 8.11 Dependencies & risks

- **No new Go module and no new npm package.** The map island already exists; the uploader is
  hand-written.
- **Schema.** One new table (`photo`), one new join (`photo_patrol`), and a **rewrite of
  `album_item`**. These are projections replayed from the log on boot, so there is no data migration
  in the usual sense — but the replay must survive the changed `itemadded` shape (§8.7), and
  `CREATE TABLE IF NOT EXISTS` will **not** alter an existing `album_item`. Dropping and rebuilding
  that table on deploy is the intended path and must be handled deliberately, not discovered.
- **Blob backup scope grows materially** (§6 Non-Functional). This is the largest operational risk in
  the PRD, and it is not a code risk: it is somebody confirming there is disk and backup capacity
  before three hundred photographs arrive.
- **A shared password will leak eventually.** Accepted, mitigated, and the reason the credential
  grants nothing but this tool.
- **The tool is unattended between events.** It will be used hard for a week and then not at all for
  a year, which is exactly the situation in which an unprotected or stale deployment goes unnoticed.
  Hence "no password set means the routes do not exist".

## 9. Success Metrics

- **A photographer uploads a full card unaided.** The binary success condition: one photographer,
  one laptop, no maintainer on the phone. If it needs a shell, it failed.
- **Batch completion rate ≥ 99%** of decodable files in a batch land on the first attempt, measured
  from the upload log for the first real hand-in.
- **Time to publish an album**: a curator gets from "files are up" to "album is on the frontpage" in
  under 20 minutes for a 30-photograph album.
- **3–5 albums on the frontpage** after the 2026 event — PRD 011 §6's own number, and the thing that
  proves this feature worked at all.
- **No photograph on a public page that a curator did not publish.** Verified by test, not observed;
  zero is the only acceptable number.
- **No coordinate on the public map with a verdict other than `inside`.** Same: a test, and zero.
- **Every takedown request satisfied within the cache window** once the curator acts.
- **Zero person-shaped fields** in the new projections, enforced by extending the task 337
  structural test rather than by review.

## 10. Rollout / Task Breakdown

Sequenced so that nothing public changes until the last step, and so the model change lands before
anything is built on top of it. `PUBLIC_ALBUMS` stays off until a curator has actually assembled
something, which makes the whole of phases 1–3 invisible to the public by configuration as well as
by design.

**Phase 1 — the model.** The library, and albums referencing it. No UI.
- [ ] Task: the `photo` projection — the year's photo library, its events and its fold
- [ ] Task: repoint `album_item` at a photo, and tolerate the legacy item event on replay
- [ ] Task: move the coordinate and its verdict onto the photograph, where a photograph's facts belong
- [ ] Task: the curator's read interface, separate from the public one so a draft cannot leak
- [ ] Task: `photo_patrol` — a photograph is attributed to a patrol, never to a person
- [ ] Task: the shared-blob check asks the library, and still refuses to guess

**Phase 2 — the door.** Basic auth, absent rather than open when unconfigured.
- [ ] Task: `requireAdmin` — a shared credential that grants exactly one tool
- [ ] Task: no password, no routes: the admin surface is absent when unconfigured
- [ ] Task: HTTPS-only, `no-store`, `noindex`, and a rate limit on the guess

**Phase 3 — the tool.**
- [ ] Task: upload one photograph, idempotently, without resurrecting a deletion
- [ ] Task: the drop zone — three hundred files, three at a time, one bad file fails alone
- [ ] Task: the contact sheet, its filters and its selection model
- [ ] Task: put a selection in one or more albums
- [ ] Task: a position for many photographs at once, from the map or from a checkpoint
- [ ] Task: tag a patrulje by the number on its sign, confirmed by name
- [ ] Task: the album editor — fields, order, captions, and the publish switch
- [ ] Task: delete from the library, and say plainly how that differs from removing from an album

**Phase 4 — guards and going live.**
- [ ] Task: extend the OpenAPI guard to `/api/admin`, and annotate every endpoint
- [ ] Task: extend the structural privacy walk to the library and the tag
- [ ] Task: assert no unpublished photograph is reachable on any public surface
- [ ] Task: retire the album dev fixture in favour of the real tool
- [ ] Task: confirm blob storage and backup capacity before the first real hand-in
- [ ] Task: write the curator's half-page — the credential, the year, and what delete means

## 11. Open Questions

1. ~~**Does a patrol tag surface a photograph on that patrol's public page?**~~ **Resolved
   (2026-09-22, maintainer): yes, but not in v1.** The destination is agreed — over time, a
   photograph tagged with a patrulje should appear on that patrol's public page, which is the payoff
   the tagging exists for and follows the precedent task 361 already set with the diploma
   photograph. What is deferred is the *publishing* half, and the deferral is the useful part: in v1
   a tag is **curator metadata with no public effect**, so the tagging can be used in anger, checked
   for accuracy, and corrected before a mistag can put a photograph on the wrong family's page.

   Two consequences to hold on to while building v1, because they are cheap now and expensive later:
   the tag stores the resolved `teamId` as well as the number (§8.6), so it will still point at the
   right patrol when it does become public; and no public read may be written against
   `photo_patrol` until that second decision is taken. Surfacing it is its own task and its own
   review, not a flag flipped at the end of this PRD.
2. ~~**Should the library be purged with the event, or kept?**~~ **Resolved (2026-09-22,
   maintainer): no — photographs are not purged.** The library is kept, including the photographs no
   album was built from. These are our photographers' work and the event's raw record, and unlike
   glimt — which is participant-shared material under a retention promise (PRD 019) — nobody was
   told these would disappear.

   So this PRD adds **no retention setting and no purge job**, and the two things that follow from
   that are requirements rather than notes. First, the blob store grows monotonically, one event's
   worth per year, which makes §6's capacity and backup point the real operational risk in this PRD
   rather than a formality (§8.11). Second, **deletion stays entirely a curator's act**: the soft
   delete in §6 is now the *only* way a photograph ever leaves, which strengthens the case for Q6's
   undelete and for the copy that distinguishes removing from an album from deleting from the
   library.
3. **Does a per-album caption override get built?** §8.3 defers it. A photograph in "Natten" and in
   "Postmandskabet" might want different words. My recommendation is to wait for a curator to ask.
4. ~~**Who holds the credential, and how is it rotated?**~~ **Partly resolved (2026-09-23, maintainer):**
   the shared env-var credential is accepted as an **interim** arrangement, to be replaced by role-based
   access in the authentication rework planned before next year's race (§8.2). That settles the shape of the
   answer and removes any pressure to design something better now.

   What remains is the operational half, still a process question rather than a code one: who generates the
   password for this event, where it is kept, who is told, and whether it is retired afterwards. The prod
   compose points at `openssl rand -base64 24`, and the tool is absent until somebody sets one — so the
   default outcome of deciding nothing is a tool that does not exist. That is the safe direction, but it is
   not a plan.
5. **Is 32 MB the right per-file ceiling** (§8.9), and does a storage ceiling apply to an organizer
   upload at all? I recommend a ceiling generous enough never to be hit by a real photograph but
   present enough to stop a stuck script.
6. **Does the curator need an undelete?** The events are soft deletes, so the capability exists in
   the log; §6 ships no button. If a curator deletes 40 photographs with a mis-aimed "select all",
   the only recovery is a maintainer. That may be acceptable for v1; it should be a choice.
