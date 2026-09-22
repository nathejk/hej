# 373 — The drop zone: three hundred files, three at a time, one bad file fails alone

**Status:** open
**Priority:** high
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

## Description

The uploader half of `GET /admin`: a `multiple` file input and a drag-and-drop zone that accepts a
folder's worth, uploading **one request per file with a concurrency of three**, a row per file with a
thumbnail once it lands and a red row with a reason if it does not, and total/remaining counts.

Built as an `html/template` in `go/cmd/api` with inline CSS and a small amount of hand-written vanilla
JavaScript — **not in `vue/`**. PRD 022 §7 and §8.10: the PWA is not touched, no route is added to
`vue/src/router/index.ts`, and the device gate is not modified. The reason curation cannot live in the
app is §8.1: `vue/src/router/index.ts` runs a device-class gate *ahead* of the auth gate and
`window.location.replace`s a desktop visitor to the website, and its own comment says revisiting that
gate is a new PRD. Bulk upload from an SD card is desktop access by definition. The absence of a build
step here is the reason not to start a second frontend for it either.

**One request per file** is the whole design, not an implementation detail. One bad file cannot fail a
batch; a dead connection — a closed laptop, a train tunnel — loses one file rather than three hundred;
and because upload is idempotent (task 372) the batch is resumable by re-dragging the same folder, with
already-uploaded files recognised and skipped. Failures never stop the batch. The rejected alternative,
one multipart request with 300 parts, fails all of them on the last byte and gives the photographer
nothing to retry but the whole card.

The page states the **event year in large type**, because it is the one thing that cannot be undone by
editing (PRD 022 §7). Copy is Danish. Headlines use the Nathejk face. Icons, where needed, are inline
SVG copied from Lucide — this page has no build step, so not the Vue package. The page may use modern
JavaScript freely: it is behind auth, on a laptop, and needs `File`, `fetch` and `FormData`; it does
not inherit the public site's deliberately lower floor. Keyboard operability and real `<label>`s are
required (PRD 022 §6) — two or three people use this for hours.

## Acceptance Criteria

- [ ] A folder of 300 files uploads with at most three requests in flight, each its own request
- [ ] Per-file progress, per-file outcome, and a plain-language Danish reason on failure; failures do
      not stop or slow the batch
- [ ] Closing the tab and re-dragging the same folder skips what landed and uploads only the rest
- [ ] A row shows "har position" distinguishably for inside / outside / none / kunne ikke vurderes
- [ ] The event year is stated unmistakably on the page
- [ ] No `npm` dependency, no bundler, no framework, and no file added under `vue/`
- [ ] The drop zone collapses to a thin bar when idle and expands during a batch
- [ ] Keyboard-operable: the file input is reachable and labelled without a pointer
