# PRD 020 — Video in Glimt

**Status:** draft
**Author:** agent session
**Created:** 2026-09-17
**Last updated:** 2026-09-17
**Approved:**
**Shipped:**
**Target users:** participant (spejder, bandit, crew) — the same population as PRD 019

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 1. Summary

Let a participant include a **short video** — up to 30 seconds — in a glimt, alongside photos.
Videos are accepted in a small set of browser-recorded formats, validated and stripped of metadata
at the container level, stored as-is with no transcoding, and played muted with no autoplay.

Split out of **PRD 019**, which built Glimt for images and phased video last. The split exists
because video is gated on a privacy claim that has to be **verified on real devices** before it can
ship, and PRD 019 should not stay open waiting for it.

## 2. Problem & Motivation

**What problem does this solve?** PRD 019 §0 decided video belongs in Glimt: some moments are not a
photograph. A bandit's chase, a patrulje singing at 02:00, the start-gun — these are the things
participants currently post to Snapchat, which is the behaviour PRD 019 exists to offer an
alternative to. An image-only Glimt competes with Snapchat with one hand tied.

**Why now?** Glimt's image half is built and working. Video was always phase 2 (PRD 019 §10) and is
the last capability in the original design that has not landed. It is also the only one that needs a
new technical capability rather than a new screen.

**Why its own PRD rather than finishing 019?** Because of the gate. PRD 019 §8 is explicit:

> If verification fails, the fallback is option (b) [`ffmpeg` out-of-band] or a shorter cap — **not**
> shipping video with an unverified privacy claim.

That verification needs an iOS device and an Android device and a person holding them. Until it
happens the outcome is genuinely unknown — it could send us to `ffmpeg`, which is a materially
different piece of work. Keeping PRD 019 open around an unknown of that size makes its status
meaningless, and PRD 019 is otherwise complete but for two device checks.

**Evidence.** PRD 019 §0 Q3 (maintainer direction: video after images, 30 s rather than 15 s);
PRD 019 §5's bandit user story; task 322, which accumulated the constraints while the image half was
built.

## 3. Goals

- A participant can record or pick a short video and post it in a glimt, on the same path as a photo.
- **The metadata claim PRD 019 makes about photographs holds for video too**, and is *verified*
  rather than assumed — no location survives an upload.
- Video does not degrade the image experience: not the feed's scroll, not the post-race browse's
  load, not a member's storage quota in a way that costs them photographs.
- An upload from rural mobile data has a realistic chance of completing.
- The 30-second bound is a fact about what is stored, not a hope about what the UI asked for.

## 4. Non-Goals

- **Transcoding.** No `ffmpeg` in the API image, no external service, no re-encoding in the BFF
  process. This is PRD 019 §8's decision (a), inherited unchanged. If verification forces
  transcoding, that is a **reopening of this PRD**, not a quiet expansion of it.
- **Editing.** No trimming, no filters, no rotation UI. The recorder gives us what it gives us.
- **Audio-only.** A voice note is a different feature with different consent questions.
- **Live or streaming.** Nothing real-time.
- **Long video.** 30 seconds is the cap, and raising it is a new decision with a new storage plan.
- **Video in the public scope, initially.** See §10 — public video is the widest reach of the widest
  format and should follow internal video, not accompany it.
- **A poster frame extracted server-side.** Without a decoder the server cannot produce one; §8 puts
  that on the client.

## 5. User Stories & Scenarios

- As a **bandit**, I want to post a ten-second clip of a chase so that the event has a record of the
  thing that actually happened, not a blurred still of it.
- As a **spejder**, I want to post a video of my patrulje singing so my hold's collection has the
  moment in it afterwards.
- As a **parent**, I want the same assurance about a video as about a photograph: it does not say
  where my child was.

### Primary path

1. A member opens the composer and taps to add media. The picker offers video where the device
   supports it.
2. They record or choose a clip. The client shows it in the strip with a duration badge.
3. **If the clip is longer than 30 seconds, the client says so and refuses it** before any upload —
   a member on mobile data must not discover the limit after a two-minute transfer.
4. The client re-encodes/compresses where `MediaRecorder` allows, then uploads.
5. The server validates the container, strips metadata, enforces the duration and byte caps, and
   stores.
6. The glimt posts. The strip shows a poster frame with a play glyph; the viewer plays it muted.

### Edge cases

| case | behaviour |
|---|---|
| Clip longer than 30 s | Refused client-side with a plain message; refused **again** server-side on the parsed container, because a recorder UI's limit is not a limit. |
| Over 50 MB | 413, distinct from 400, so the client can say "vælg en kortere video" rather than reporting a generic failure. |
| Not a video the server can parse | 400, item dropped from the draft, **the rest of the glimt survives** (PRD 019 §5). |
| A format we accept but the device cannot play back | Should not happen if we accept only what browsers record; if it does, the viewer shows the poster and a plain "kan ikke afspilles" rather than a broken control. |
| Poster frame could not be generated | Stored with no `thumbRef`, exactly as a photograph whose thumbnail failed (PRD 019, task 303). The grid falls back rather than showing a gap. |
| Upload interrupted | The outbox retries the whole item; a partially uploaded blob is never referenced, because the ref is only named when the glimt is created. |
| Member at their storage ceiling | 429 with the "delete something" message, as for images (task 311) — but see §8 on the arithmetic. |

## 6. Requirements

### Functional

- [ ] A member may include video items in a glimt, mixed freely with photographs.
- [ ] **Accepted formats are an explicit allow-list**, documented, and validated by parsing the
      container rather than by trusting the declared content type or the file extension.
- [ ] **Duration ≤ 30 s, enforced server-side on the parsed container.** The client also enforces it,
      earlier and more kindly, but the server's check is the one that counts.
- [ ] **≤ 50 MB per video item**, enforced server-side, answering 413.
- [ ] **No location metadata survives an upload** — verified against real iOS and Android recordings,
      with the result recorded.
- [ ] Client-side compression/re-encode where `MediaRecorder` supports it.
- [ ] A **poster frame** is generated client-side and stored as the item's thumbnail, so grids stay
      thumbnail-first.
- [ ] Video plays **muted, with no autoplay**, and never plays sound without an explicit action.
- [ ] Media serving supports **HTTP Range requests** — see §8; without this Safari will not play.
- [ ] The stored `contentType` is served, not a hardcoded `image/jpeg`.
- [ ] Duration is shown in the strip and the viewer.
- [ ] Video obeys every rule images already obey: audience immutability, the visibility check on the
      media route, moderation hide/unhide, author-only delete, retention.

### Non-Functional

- **Privacy.** The metadata claim is the load-bearing requirement of this PRD, not a detail of it.
  Nothing ships until it is verified on hardware.
- **Bandwidth.** A 30 s clip on rural mobile data is the hardest case in the whole feature. Client
  compression, the outbox's retry, and honest progress copy are the mitigations.
- **Storage.** The blob store is the one non-rebuildable thing in the service (PRD 008 §8). Video
  changes its growth rate materially; see §8.
- **Accessibility.** A video's alt text and controls follow the same rules as PRD 019's images:
  captions are real text, controls are ≥44 px, nothing conveys meaning by colour alone.
- **Degradation.** A device that cannot record video sees no video affordance and loses nothing else.

## 7. UX / UI Notes

Everything here is an extension of PRD 019's components, not a new surface.

- **`GlimtComposer.vue`** — the picker's `accept` already includes `video/*` (PRD 019 §7). Add the
  duration check, a duration badge on the strip item, and the refusal message. The refusal must name
  the limit ("Videoer må være højst 30 sekunder") rather than saying the file was invalid.
- **`GlimtMediaStrip.vue`** — already draws a `Play` glyph over video items and already reads
  `hasThumb`. With poster frames it needs no change beyond showing the duration.
- **`GlimtViewer.vue`** — already renders `<video muted playsinline controls>`. Needs the Range work
  behind it to actually play on Safari, and a duration/position that does not fight the dialog's own
  controls.
- **Upload progress.** A photograph uploads in a moment; a 30 s video does not. The composer needs
  honest per-item progress, and it must not claim background upload — iOS does not run a
  backgrounded web app (PRD 019 §5).
- **The audience copy is unchanged**, and that is deliberate: a video shared publicly is a video on
  the open web, and the existing consequence line already says exactly that.

## 8. Technical Considerations

### What already exists, and what does not

Useful groundwork is already in place from PRD 019, which is part of why this is tractable:

| already there | where |
|---|---|
| `kind`, `contentType`, `durationMs` columns | `nathejk/table/glimt/table.sql` |
| `MediaKindVideo` and the consumer's kind normalisation | `nathejk/table/glimt/events.go`, `consumer.go` |
| `<video muted playsinline controls>` in the viewer, `Play` glyph in the strip | `GlimtViewer.vue`, `GlimtMediaStrip.vue` |
| A 5-minute upload read deadline | `glimtUploadTimeout` in `glimtmedia.go` |
| Storage ceilings and byte budgets | task 311 |

**No schema migration is needed for the basics.** What is missing is the four things below.

### 1. Container parsing, without a decoder

`internal/imaging` cannot touch video and there is no transcoder. So the server needs to read an
**ISO-BMFF / MP4** box tree far enough to (a) confirm the file really is what it claims, and (b) read
the duration from `moov/mvhd`. That is a bounded, read-only parse of a well-specified format and is
reasonable to write by hand — it is not a decoder, and it must never be mistaken for validation of
the *video data*, only of the container.

The parser must be defensive: a hostile or truncated file is expected input. Bounded box depth,
bounded box count, no allocation driven by a length field without checking it against the remaining
bytes.

### 2. Metadata stripping is a container *rewrite*, and this is the crux

PRD 019 §8 says "metadata stripping is container-level only". That is accurate but understates the
work: on iOS, location lives in `moov/udta/©xyz`, and both platforms may write `moov/meta` and
maker-specific boxes. **Removing those means rewriting the box tree, not merely reading it** — and a
rewrite that gets an offset wrong produces a file that no longer plays, possibly only on one player.

This is the single riskiest piece of the PRD and the reason for the verification gate. Two honest
possibilities:

- The rewrite is straightforward: drop `udta` and `meta` under `moov`, fix the parent sizes, leave
  `mdat` untouched and untouched-in-place so no sample offsets move.
- It is not, because a recorder interleaves in a way that makes the offsets fragile — in which case
  option (b), `ffmpeg` out-of-band, becomes the answer and **this PRD reopens**.

**Neither can be decided from a desk.** The verification must use real recordings from real devices,
and it must check the *output*, not the input — i.e. download what the server stored and confirm the
boxes are gone, with a tool that reads the container independently of ours.

### 3. The caps interact with limits set for an image-only feature

Numbers currently in the code, and what video does to them:

| setting | now | with 50 MB video |
|---|---|---|
| `maxGlimtUpload` | 12 MiB, one constant | Must become **kind-aware**: 12 MiB for images, 50 MB for video. A single 50 MB limit would also let somebody upload a 50 MB "photograph". |
| `GLIMT_BYTES_PER_HOUR` | 200 MiB | **Four videos an hour**, and nothing else. Probably needs raising, and the raise needs a number rather than a shrug. |
| `GLIMT_MEMBER_STORAGE_BYTES` | 500 MiB | **Ten videos and a member is full** — and then cannot post photographs either. This is the arithmetic that worries me most, because the failure lands on a member who did nothing wrong. |
| `GLIMT_TOTAL_STORAGE_BYTES` | off | Should probably be **on** before video ships. 1,000 members × a few videos is tens of gigabytes on one volume. |

Task 311 set those figures for an image-only feature and said so; task 324 has since shown the
read-limit arithmetic was wrong in the same way. **These want deciding with video's numbers in front
of us, not inherited.**

### 4. Range requests — the requirement most likely to be missed

`streamGlimtMedia` writes the whole object with `io.Copy` and sets no `Accept-Ranges`. That is fine
for a JPEG and **not fine for video**: Safari in particular issues a byte-range request and will
refuse to play a response that ignores it. Seeking needs it too.

So the media route needs `http.ServeContent` (or equivalent) with a `ReadSeeker`, which means the
blob store needs a seekable read — `blob.Store.Get` currently returns an `io.ReadCloser`. That is a
small interface change with two implementations, and it must not disturb the `immutable` + ETag
caching that task 324 measured as worth 21.5×.

### API endpoints

**No new endpoints.** Video rides the existing ones, which is most of the argument for approach (a):

- `POST /api/glimt/media` — accepts video, validates, strips, enforces caps. Annotation must gain the
  video content types, the 30 s condition, and the 50 MB 413.
- `GET /api/glimt/items/{id}/media/{ordinal}` — serves the stored `contentType` and honours Range;
  annotation must gain `206` and the range parameter.
- `GET /api/public/glimt/{id}/media/{ordinal}` — the same, when public video is enabled.

**Every changed endpoint's OpenAPI annotation must be updated** (`.rules`), and
`glimtopenapi_test.go`'s agreement test will fail until the new failure codes are documented — which
is the intended behaviour, not an obstacle.

### Data / storage

No schema change. `durationMs` and `contentType` are already there and already projected;
`glimtmedia.go` simply does not populate them for video yet.

### Dependencies & risks

| risk | mitigation |
|---|---|
| **Metadata stripping does not work** | The gate: verify on real iOS and Android recordings before shipping. Fallback is `ffmpeg` out-of-band, which reopens this PRD. |
| A rewritten container plays on one platform and not another | Verification covers playback as well as metadata, on both platforms, in the installed PWA and in Safari/Chrome. |
| Storage growth | Decide the four numbers in §8.3 before enabling; turn the total ceiling on. |
| A hostile file crashing or exhausting the parser | Defensive bounded parsing, fuzz the box reader, and the parser never decodes sample data. |
| Range support breaking image caching | The 304/ETag path is measured (task 324); re-measure after the change. |
| Upload failure on mobile data | Client compression, the existing outbox, honest progress; measure a real upload on event mobile data (PRD 019's open risk). |

## 9. Success Metrics

- **Zero videos carrying location metadata**, asserted by an independent container reader on real
  recordings from both platforms. A hard pass/fail, not a target — this is the gate.
- **Playback works**: a posted video plays in the installed PWA on iOS and on Android Chrome. Also
  pass/fail.
- **Video is used but does not dominate:** share of glimt containing video. If it is near zero the
  affordance is not discoverable; if it is most of them, the storage plan needs revisiting.
- **Uploads complete:** proportion of video items accepted into the outbox that eventually publish,
  and median time to publish on event mobile data. PRD 019's reliability target is ≥ 99% and zero
  silently lost media; video must not degrade it.
- **Storage per member stays within budget**, and no member is refused a photograph because of
  videos.
- **No regression in the post-race browse**: p95 on the hold collection stays within task 324's
  baseline once videos are in the corpus.

## 10. Rollout / Task Breakdown

**Verification comes first.** This is the ordering that matters and it is the opposite of the usual:
the metadata spike happens **before** the client work, because its outcome decides whether the rest
of this PRD is the right plan at all. Building the composer's duration UI first would be building on
an unverified foundation.

- **Phase 0 — the gate.** Container parse + metadata rewrite behind no UI, verified on real iOS and
  Android recordings. If it fails, stop and reopen.
- **Phase 1 — internal video.** Kind-aware caps, Range serving, poster frames, composer and viewer
  work. `group` and `nathejk` scopes only, and the same role phasing PRD 019 used.
- **Phase 2 — public video.** Only after internal video has been watched working, for the reason
  PRD 019 §10 gives about the public scope: it is the only one whose mistakes are visible outside
  Nathejk.

Proposed tasks for `roadmap/tasks/open/` (created on approval):

- [ ] Task: ISO-BMFF box reader — bounded, defensive, duration from `moov/mvhd`, fuzzed
- [ ] Task: Container metadata rewrite — drop `udta`/`meta`, preserve `mdat` offsets, round-trip tests
- [ ] Task: **Verify on real iOS and Android recordings** that no location metadata survives, with an
      independent reader — the gate; record the result
- [ ] Task: Kind-aware upload caps (12 MiB image / 50 MB video) and the 30 s server-side duration cap
- [ ] Task: Decide and apply the storage numbers — per-hour budget, per-member ceiling, total ceiling
- [ ] Task: `blob.Store` seekable read + Range support on both media routes, re-measuring the 304 path
- [ ] Task: Serve the stored `contentType` instead of a hardcoded `image/jpeg`
- [ ] Task: Client-side poster frame generation, stored as `thumbRef`
- [ ] Task: Client-side duration check and re-encode where `MediaRecorder` allows
- [ ] Task: Composer duration badge, refusal copy, and per-item upload progress
- [ ] Task: Viewer playback verified on iOS installed PWA and Android Chrome
- [ ] Task: OpenAPI annotations for the changed endpoints (206, 413, video content types)
- [ ] Task: Public-scope video, after internal video is in use

## 11. Open Questions

1. **Is 30 s still right?** It came from PRD 019 §0 as a target "if the size and metadata story holds
   up". 30 s at phone bitrates is 30–60 MB before compression; 15 s would halve every storage and
   bandwidth number in §8.3. Worth re-confirming now that the storage arithmetic is visible.
2. **What are the four storage numbers?** §8.3 lists them. Ten videos filling a member's quota is the
   one that needs an answer before anything ships.
3. **Should the total storage ceiling be switched on** before video, and at what value? That needs
   the deployment's volume size, which this PRD does not know.
4. **Is audio kept at all?** Muted playback is decided, but that is a *playback* choice — the
   recording still carries sound, and a clip of children talking is a different privacy object to a
   silent one. Stripping the audio track would be a container edit we are already making. Not
   answered here, and it should be.
5. **Which formats, exactly?** "What browsers record" is MP4/H.264 on iOS and often WebM/VP8-9 on
   Android Chrome. WebM is a different container (Matroska), so supporting both means **two** parsers
   and two rewrites. Restricting to MP4 may mean refusing Android recordings, or forcing a client-side
   re-encode that `MediaRecorder` may not offer. This is the biggest unknown after the metadata gate
   and may deserve its own spike.
6. **What happens to video already in the wild if we later switch to `ffmpeg`?** Nothing, if the
   stored files are valid — but if the rewrite turns out to have damaged some, there is no
   re-derivation. Worth a decision about whether phase 0 keeps the original bytes until the claim is
   verified in production.
7. **Does the 50 MB cap interact badly with the outbox?** A 50 MB item in IndexedDB, retried across
   foregrounds, on a device with little free space. PRD 019's outbox has never been exercised at all
   (task 325), let alone with a 50 MB payload.
