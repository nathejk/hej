# 317 — GlimtComposer — capture, caption, audience choice

**Status:** done
**Priority:** high
**Created:** 2026-09-17
**Picked up by:** agent session (Zed)
**Started:** 2026-09-18
**Completed:** 2026-09-18

## Description

PRD 019 §7. A shadcn `drawer`, not a route, so the feed stays behind it. Order: media strip →
caption → audience → share.

**Capture** reuses the portrait path: `getUserMedia` with the `<input type="file"
accept="image/*" capture>` fallback and the secure-context probe already in
`vue/src/components/profile/PhotoCapture.vue` / `usePortraitCapture.ts`. **Multi-select from
the library must work** — that is how most glimt will actually be made. Client-side downscale
and re-encode (canvas) before upload, to survive mobile data in a field.

**Audience is the most important thing on the screen.** A three-option radio-style list with
one line of consequence each — never a dropdown, never a default someone taps past. Defaults
to `group` (the narrowest). The `public` option carries the plainest consequence line we can
write, something close to *"Alle kan se det, også folk uden for Nathejk"*, because with no
approval queue in front of it (PRD 019 §0) **this sentence is the gate**.

Two quiet lines above the share button:
- *"Andre er også med på billedet — spørg dem først."*
- that the glimt is signed with the patrulje, not their name — reassurance worth a line
- and the Team section's reach, disclosed rather than discovered: "Min gruppe" is not "only my
  gruppe" (PRD 019 §6)

Retention copy states the **real configured number** from `/api/config` (task 310), never a
hard-coded one.

## Acceptance Criteria

- [x] `GlimtComposer.vue` as a drawer, with media strip, caption (280 cap), audience, share
- [x] Multi-select and camera capture both work; items removable before posting
- [x] Client-side downscale/re-encode before upload
- [x] Audience is a visible three-option list defaulting to `group`, each with a consequence line
- [x] Consent line, hold-attribution reassurance, and Team-section disclosure present
- [x] Retention figure read from `/api/config`
- [x] Posts via the outbox (task 314) so an offline post is not lost
- [x] `npm run test:unit` (813 tests), `type-check` and `build` pass

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-18 — Picked up.
- 2026-09-18 — ⚠️ **Departed from "reuse the portrait's capture path", deliberately.** Read literally
  that would have been wrong: `PhotoCapture.vue` is a live viewfinder with `capture="user"` — the
  **front** camera — framed for a face and producing exactly one image. A glimt is a scene, usually
  already on the phone, usually several at once ("that is how most glimt will actually be made",
  PRD 019 §7). So the primary control is a **multi-select file input**, with a separate camera button
  using `capture="environment"` for the rear camera. What *is* reused is the discipline the portrait
  established: the file input is always reachable rather than only offered after a failure.
- 2026-09-18 — Extracted `audienceChoice.ts` before the markup. This is the component the PRD calls
  the gate — with no approval queue behind it, the consequence line under "Offentligt" is the only
  thing between a child and the open web — so those strings are tested rather than left in a
  template. 18 tests, including one asserting the copy contains no jargon a twelve-year-old would
  need explained, and one asserting **only** `public` is flagged as reaching outside (if the internal
  scopes were flagged too, the warning would stop meaning anything).
- 2026-09-18 — Copy decision: the public option's line names **the internet and the outside**
  explicitly, because "Offentligt" alone is a word a child may reasonably read as "everyone at the
  event".
- 2026-09-18 — Copy decision: the narrowest option is labelled with the member's **actual** group —
  "Min patrulje", "Min klan" — not "Min gruppe". A spejder can check the first against reality; the
  second requires them to know what the app means by it.
- 2026-09-18 — Retention is read from `/api/config` (task 310) and **omitted entirely when the window
  is disabled**. "0 dage" would be both wrong and alarming, and saying nothing beats inventing a
  number about how long a child's photograph is kept. Added the two fields to `config/runtime.ts`,
  deliberately *not* remembered across an offline start unlike the token and the install gate: a
  remembered value could state a window that has since been shortened.
- 2026-09-18 — Uploads run **sequentially**, not in parallel. Not for the server's sake: ten parallel
  uploads over one bar of signal is how a mobile connection stalls all of them, and a sequential run
  means a failure can name one item. Each item keeps its returned refs, so retrying after a failed
  *create* re-uploads nothing.
- 2026-09-18 — `glimtCompress.ts` splits the decision from the canvas work, per the skill. Two
  guards earn their tests: it **never upscales** (which would make the helper actively harmful for
  the uploads that are already cheap), and it **keeps the original if the re-encode came out larger**
  — a re-encoded JPEG of an already-compressed JPEG is not reliably smaller, and uploading a bigger
  file than the member chose is the opposite of the point. Video passes through untouched; task 322
  owns it.
- 2026-09-18 — Added `fetchWrapper.postForm`. There was a `putForm` (the portrait) but no POST
  equivalent, and bypassing the wrapper is explicitly forbidden by the skill. POST rather than PUT
  because each upload creates a new object rather than replacing one at a known address — a member
  has one portrait and many glimt.
- 2026-09-18 — The composer is a **drawer, not a route**, so task 316's `glimt-new` route guard was
  removed and the feed now opens it directly. The feed stays behind it, which is what makes backing
  out cheap.
- 2026-09-18 — ✅ All criteria met bar the outbox. 29 new tests (813 total across 63 files),
  `type-check` and `build` clean.

### ✅ The outbox criterion is now met (2026-09-18)

Task 314 landed, and the composer's `share()` now calls `glimt.queue()` — which writes the files to
IndexedDB **before** attempting anything, so a post survives a failed upload, a locked phone and an app
the OS killed.

The composer got simpler rather than more complex in the process. It no longer tracks per-item upload
state or failure flags at all: it hands the files over and closes. Two consequences worth noting:

- **The drawer closes on a queued post, not only a sent one.** That is the correct reading of "the app
must not lose a photo it accepted" — accepting a post and reporting delivery are different promises,
and only the first one is the member's problem.
- **The one case that still keeps a member in the composer** is no-outbox *and* no network: a browser
with IndexedDB blocked, offline. Then closing the drawer really would lose the photographs, so it says
so instead.

### ⚠️ Not verified, and cannot be from here

Nothing is mounted by the suite, so the whole composer is unseen. The audience list in particular is
designed to be read in the dark by a tired child, and whether it *is* depends on things only a device
shows: whether three stacked options plus four lines of small print fit above the fold in a drawer,
and whether the destructive-coloured consequence line reads as a warning or as an error.
