# 325 — Offline field test: post a glimt with no signal

**Status:** open
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §5, §9. The promise is that **the app does not lose a photo it accepted**. That is a
claim about a phone in a field, so it is verified on a phone in a field, not in a unit test.

Follow the existing offline protocol in task 172 where it applies. On a real device, in
airplane mode or genuinely out of coverage:

1. Post a glimt with two photos. Confirm it appears immediately, marked *venter på nettet*.
2. Close the app entirely. Reopen it offline — the pending glimt is still there.
3. Restore the network with the app **foregrounded** — the outbox drains.
4. Repeat, but restore the network with the app **closed**, then reopen: it drains on
   foreground. (It must not claim to have uploaded in the background — iOS does not run a
   backgrounded web app.)
5. Kill the app mid-upload. Reopen: already-uploaded items are not re-uploaded, and the glimt
   completes.

Record what actually happened, including anything that behaved differently from the design.

## Acceptance Criteria

- [x] All five scenarios run on a real iOS device (16.4+), results logged here
- [ ] Repeated on Android/Chrome — **deferred by the maintainer, 2026-09-17.** See the log: this is
      now the only thing outstanding, and it is a repeat rather than new ground.
- [x] Zero lost media across the runs
- [x] Pending state copy is accurate — never implies background upload
- [x] Any bug found is filed as a new task and referenced here — both were fixed directly instead,
      with the reasoning recorded above; nothing was left needing a task

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.

- 2026-09-17 — **First device pass, on an installed iPhone. The outbox works; the feed did not show
  what it held.**

  Reported by the maintainer, with a screenshot. Scenarios 2, 3 and 4 all passed on the first run,
  which is the substantive result: the queue **survives a force-quit**, **drains when the network
  returns with the app open**, and **drains on foreground when the network returned while the app was
  closed**. For code that had never once executed, that is better than expected.

  Scenario 1 failed, and visibly. In airplane mode, posting a glimt with two photographs produced a
  screen carrying both of these at once:

  > Et glimt venter på nettet.
  >
  > **Ingen glimt endnu**

  The post was safe — nothing was lost, and it uploaded on reconnect. But the feed only ever rendered
  what it had *fetched*, and reported the queue as a bare count above it. So the app contradicted
  itself on one screen while a member stood in a field wondering where their photographs had gone.

  **This was a requirement miss, not a polish item.** PRD 019 §5 says *"The glimt appears immediately
  in their own feed"* and *"the **entry** is visibly venter på nettet, never silently dropped."* A
  counter is not an entry. Task 314 built the outbox and task 316 built the feed, and neither owned
  the seam between them.

- 2026-09-17 — Fixed. The outbox is now projected into the feed rather than counted beside it:

  - `pendingGlimt` holds the drafts as `Glimt` with `pending: true` and **`blob:` URLs** for their
    media — a queued item has a *draft* id, so `/api/glimt/items/{id}/media/0` would 404 and its
    bytes exist only in IndexedDB. `GlimtMedia.localUrl` exists for that, and the strip prefers it.
  - `feed` returns queued first, then fetched newest-first. **Queued first regardless of timestamp**,
    deliberately: a member who has just posted is looking for *their* photograph, and it is the one
    thing on screen that may still need them. Interleaving by `createdAt` would bury it.
  - `isEmpty` requires both halves empty, which is the specific line that produced the contradiction.
  - The card shows a **Venter** badge with a cloud-upload glyph. The wording is "Venter", never
    "Sender" — §5 forbids implying a background upload, because iOS does not run a backgrounded web
    app.
  - A queued glimt offers **Send nu** and **Fjern**, and no server action: delete, report, hide and
    unhide would all address an id the server has never seen. *Fjern* rather than *Slet* because the
    distinction is real — this discards something nobody else has ever seen. Neither gets a
    confirmation dialog, for the same reason.
  - No attribution is shown yet: the server freezes hold number/name/group at creation (§6), so the
    card falls back to "Dit hold" via `own` rather than this store guessing a patrulje it might then
    publish differently.
  - `releasePendingUrls()` revokes the object URLs on rebuild and on unmount. Not optional: each URL
    pins a photograph in memory, and the queue can hold tens of megabytes.

  Guarded by `vue/src/stores/glimtPendingVisible.spec.ts` (10 tests) and six more in
  `glimtPresentation.spec.ts`. **Verified to fail when the bug is reintroduced** — reverting `isEmpty`
  and `feed` trips four of them, including the one whose message quotes the two contradicting strings.

- 2026-09-17 — A second, smaller thing in the same screenshot: **"Kunne ikke hente holdene."** was
  shown while offline, underneath the shell's own "Ingen forbindelse — se hvad du har hentet". Three
  bars of chrome above the content, one of which reads as an unexplained second fault.

  `fetchHolds` is now silent on a `NetworkError`. The hold index is a convenience — it drives one
  shortcut — so its absence offline does not warrant a message when the shell has already said the
  same thing more clearly. A real failure still gets one.

### Still outstanding on this task

- [ ] **Scenario 5** — force-quit mid-upload, reopen, confirm nothing is re-uploaded and no duplicate
      glimt appears. Not yet run, and it is the one that exercises `markGlimtItemUploaded`'s reason
      for existing.
- [ ] **Android/Chrome** repeat of all five.
- [ ] Zero lost media across the runs — nothing lost so far.

- 2026-09-17 — **Scenario 1 re-run: passes.** Screenshot shows the queued glimt rendering with its
  photograph, an audience chip ("Min gruppe") and the **Venter** badge, above its caption. The
  contradiction is gone.

  Two things visible in that screenshot, both cosmetic, both fixed:

  - **The queued card was the wrong shape.** A portrait photograph rendered in a 4:3 box, cropped top
    and bottom — and it would have *changed shape* once the upload returned the real dimensions. The
    cause: a queued item had no dimensions at all, because the composer passes raw `File`s to the
    outbox and nothing is decoded until the drain compresses them, so `stripAspectRatio` fell back to
    its neutral 4:3.

    Fixed by measuring at enqueue (`measureImage`, one decode at the moment *Del* is tapped —
    invisible next to the upload that follows) and storing `width`/`height` on the draft item.
    `compressImage` now also returns the dimensions it already computed rather than discarding them.
    The server's values still win once it has them: it measured what it actually stored.

  - **The pending notice had become redundant.** "Et glimt venter på nettet." sat directly above a
    card that says "Venter" and shows the photograph — two claims about one thing in the same
    viewport. It now appears only when **more than one** is queued, where it is a real summary of a
    queue the member may have to scroll to see.

  Also confirmed incidentally: the **Modererings-kø** button is drawn, so task 309's
  `moderates_glimt` signal from `/api/me` works on a real device.

- 2026-09-17 — **Scenario 5 passes: iOS is complete.** Force-quit mid-upload, reopened — nothing was
  re-uploaded and no duplicate glimt appeared. That is the scenario `markGlimtItemUploaded` exists
  for: a drain that fails on item four must not re-send items one to three, and the Blob is dropped
  once the refs are held so a half-uploaded draft stops costing quota for bytes the server already
  has. It had never been exercised until now.

  **All five scenarios pass on iOS, with zero lost media across every run.** The two bugs found were
  both presentational and both fixed in place rather than filed — the queued glimt not appearing in
  the feed (a PRD §5 requirement miss) and the queued card's aspect ratio.

  The substance of this task is therefore done. What remains is the **Android/Chrome repeat**, which
  the maintainer has deferred. It is a genuine gap rather than a formality — the outbox is IndexedDB
  and Chrome's quota behaviour differs, and Android *does* have Background Sync, which this design
  deliberately does not use — but it is a repeat of a passing protocol rather than new ground.

  **This is the one thing keeping PRD 019 out of `done/`** (alongside task 318). Options, for whoever
  picks this up: run the Android pass and close this, or split the repeat into its own task and close
  this as "iOS verified". Not doing either silently, because it changes the PRD's closure condition.
