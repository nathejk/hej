# PRD 007 — Portrait identification (offline face lookup, scoped by race role)

**Status:** draft
**Author:** agent session (Zed)
**Created:** 2026-08-25
**Last updated:** 2026-08-25
**Approved:**
**Shipped:**
**Target users:** everyone accepted into the race — crew (samarit, guide, postmandskab), spejder, bandit, gøgler

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 1. Summary

Let people in the race put a face to a name **without a network connection**. The
portraits captured during onboarding (PRD 005) are synced to the device ahead of
time and looked up offline, at night, in a forest. Who you may see is scoped by
your race role: crew see everyone, teams see their own, and **spejdere and
banditter can never see each other** — that would spoil the game. Portraits never
leave the race.

## 2. Problem & Motivation

- **What problem does this solve?** PRD 005 captures a self-portrait whose stated
  purpose is identifying people during the race — much of which runs at night,
  when faces are hard to see. Nothing consumes that photo today. Without a viewing
  surface we are asking several hundred members, many of them minors, for a
  photograph that no one ever looks at.
- **Why offline is the whole problem.** The moment the photo matters most is the
  moment the network is least likely to be there: 03:00, in woodland, on a phone
  that has been navigating and reporting position for eight hours. A viewer that
  needs connectivity fails precisely when it is needed. Offline is therefore not a
  nice-to-have or a later phase — it is the feature. A design that fetches on
  demand should be understood as *not solving the problem*.
- **Why now?** PRD 005 is in draft and its portrait step is blocked partly on this:
  capture-first-viewer-later means collecting sensitive data with no realised
  benefit, which is both a poor trade for members and a weak position to defend if
  someone asks why we hold photographs of their child.
- **Evidence.**
  - PRD 005 §11 (2026-08-25) established the portrait's purpose as operational
    night-time identification, and flagged this viewing surface as unowned.
  - PRD 003 non-goals cross-person photo viewing explicitly, deferring it to "a
    separate feature with its own consent and access story". This is that PRD.
  - A survey of `shared-go`, `hq` and `tilmelding` (2026-08-25) found **no photo
    storage of any kind** in any of the three repos — the only hits are unprojected
    legacy `PhotoID` wire fields on `MonolithNathejkTeam`/`Member`. Everything here
    is new.

## 3. Goals

- A member of the race can identify a person in front of them, in the dark, with
  no connectivity.
- A member can recognise a person they are looking for, before they find them.
- Portraits are visible **only** to those whose race role permits it, enforced on
  the server, not in the interface.
- Spejdere and banditter remain unable to see each other's faces, so the game
  dynamic is preserved.
- Portraits stay inside the race: no export, no sharing surface, and a hard purge
  when the event ends.
- The privacy cost stays proportionate — the smallest audience that makes the
  feature work, not the largest one that is technically easy.

## 4. Non-Goals

- **Capturing portraits.** PRD 005 (onboarding) and PRD 003 (profile page) own
  capture and replacement. This PRD only reads.
- **Face recognition or automated matching.** Humans look at faces here. No
  biometric processing, which would also change the legal basis materially.
- **A public or parent-facing surface.** Guardians do not get a viewer.
- **Organizer bulk browsing or export.** No "all portraits" grid, no download, no
  print. If HQ needs an identification tool it is a separate decision with a
  separate justification.
- **Using portraits for anything but identification** — not scoring, not
  social features, not the finish-line diploma.
- **Replacing existing identification mechanisms.** Arm numbers
  (`senior.ArmNumber`) and patrol numbers already identify people without a face
  and keep working when this feature is unavailable.
- **Guaranteeing portraits cannot be exfiltrated from a device.** Honestly out of
  reach; see §8. We reduce exposure and deter, we do not prevent.

## 5. User Stories & Scenarios

- As a **samarit** called to a member at 03:00 with no signal, I want to see that
  member's face before I arrive, so that I find the right person quickly.
- As a **postmandskab member** at a checkpoint, I want to confirm the patrol in
  front of me is who they say they are, without a network.
- As a **spejder**, I want to see the faces of my own patrol, so that I can find my
  team-mates in a crowd of identically dressed teenagers at night.
- As a **bandit**, I want to see my own klan, and I want to be confident that no
  spejder can study my face in advance.
- As a **participant**, I want to know who can see my photograph before I take it,
  so that my consent means something.

### Primary path — samarit, offline

1. Before the event (on wifi, at check-in), the app syncs the portraits the user's
   role permits, as small thumbnails, into a service-worker-managed cache.
2. During the race HQ tells the samarit over the radio: patrol 138, member Freja.
3. The samarit opens the identification view offline, searches patrol 138, sees
   Freja's face and name.
4. They locate her, confirm visually, and continue.

### Primary path — spejder finding their own patrol

1. A spejder opens the app at a busy checkpoint.
2. Their own patrol's members are listed with faces, from cache, with no request.

### Edge cases

- **Portrait missing.** Skippable at onboarding, so many will be absent early on.
  Show a clear "no photo" placeholder with the name and patrol; never an error, and
  never a broken image.
- **Portrait added after the last sync.** The device shows what it has. Staleness
  is expected and acceptable; the view must say when it last synced so a user can
  tell "no photo" from "not synced yet".
- **Cache evicted by the OS.** Real and likely on iOS (§8). The view must degrade
  to names-only rather than appear empty, and re-sync opportunistically when a
  network appears.
- **Never synced at all** (installed on the morning of the race, on mobile data).
  Offer an explicit "download portraits" action with a size estimate, rather than
  silently pulling tens of megabytes over a metered connection.
- **A spejder tries to reach a bandit portrait** by URL, id guessing, or a stale
  cached response. Must fail server-side, and must not be inferable — a spejder
  should not be able to learn *that* a bandit exists at a given id.
- **Role changes mid-event** (crew reassignment). Permission is re-evaluated on
  the next sync; already-cached portraits outside the new scope must be purged.
- **Member leaves the race** (`released`, `reunited`). Their portrait stops being
  synced. Whether it disappears from devices immediately is a policy question
  (§11) — an identification need can outlive a withdrawal by hours.
- **Event ends.** Portraits are purged from the server and from devices. A device
  that never reopens the app cannot be purged remotely; see §8.
- **Shared device, or a phone handed to a friend.** The viewer is behind the
  normal session, so this is the session's problem, not a new one — but it is why
  there is no bulk grid.

## 6. Requirements

### Functional

- [ ] An offline-first identification view, reachable from the app shell, that
      works with the radio off.
- [ ] Portraits are stored as **small thumbnails** sized for on-screen
      identification, not as originals.
- [ ] Portraits **register as a PRD 009 dataset**: cache-first, binary, high
      priority, server-issued expiry, purged after the event. The sync engine,
      progress reporting, resumability, storage budget and eviction policy are PRD
      009's — this PRD declares properties, it does not implement sync.
      *(Corrected 2026-08-25: these three bullets previously specified a sync
      mechanism and a user-triggerable sync with a size estimate, contradicting this
      PRD's own §8 deferral to 009.)*
- [ ] A **metadata index** (person id, name, team, portrait version) registered as
      structured data, so search and lists keep working when images have been
      evicted.
- [ ] The BFF's portrait endpoints follow **PRD 009's manifest/delta convention**
      (`If-None-Match`/version), so the generic engine can drive them.
- [ ] The view supports both use cases: **look up a named person** (search by
      name, patrol/team number, arm number) and **browse a team**.
- [ ] The server returns **only** the portraits the caller's role permits. No
      client-side filtering of a broader payload — a device must never hold a
      portrait its user may not see.
- [ ] **Spejder and bandit are mutually invisible**, enforced server-side and
      symmetric.
- [ ] Access is evaluated per request and per sync against the caller's current
      role, from the directory (PRD 006).
- [ ] No export affordance: no download, no share sheet, no long-press save, no
      open-in-new-tab.
- [ ] Displayed portraits are watermarked with the viewer's identity and the
      timestamp, as a deterrent against onward sharing.
- [ ] Cached portraits carry a **server-issued** expiry and are purged after the
      event; the server stops serving them at the same point. (Server-issued so a
      wrong device clock cannot defeat it.)
- [ ] Every portrait *view* is logged, batched and uploaded after the fact so the
      log survives offline use — fire-and-forget, **not** part of PRD 009's sync
      (which explicitly excludes offline writes). Subject to §11.7.
- [ ] Names-only degradation when portraits are unavailable for any reason.

### Access matrix (proposed — confirm, see §11)

The rule is least disclosure that still solves the problem, not "everyone who is
in the race sees everyone in the race".

| Viewer | spejder | bandit | gøgler | crew |
|---|---|---|---|---|
| **crew** — identified function (samarit, guide, postmandskab) | ✅ all | ✅ all | ✅ all | ✅ all |
| **crew** — unclassified (generic `crew`, see PRD 006) | ❌ | ❌ | ❌ | ✅ crew on duty |
| **spejder** | ✅ own patrol only | ❌ **never** | ❌ | ✅ crew on duty |
| **bandit** | ❌ **never** | ✅ own klan only | ❌ | ✅ crew on duty |
| **gøgler** *(provisional — see §11.3)* | ❌ | ❌ | ✅ own team | ✅ crew on duty |

Three deliberate choices in there, all of which need your confirmation:

- **Cross-team is closed for participants.** A spejder sees their own patrol, not
  every other patrol. Nothing in the stated need requires cross-patrol browsing,
  and it multiplies the audience for a minor's photograph by the size of the whole
  event. If you want it open, say so — but it is the difference between ~6 people
  and ~1000 seeing a given face.
- **Participants can see crew.** Useful and low-risk: knowing what the samarit
  coming to help you looks like is reassuring, and crew are adults acting in an
  official capacity. Worth confirming it does not leak anything about post
  locations.
- **Unclassified crew get almost nothing.** Added 2026-08-25 after a consistency
  pass found a hole: PRD 006 falls back to a generic `crew` role when a section slug
  cannot be mapped, and the original matrix had a single "crew" row granting ✅ all.
  An account that exists only because classification *failed* would therefore have
  received every portrait in the event, straight across the spejder/bandit divide
  this PRD calls a correctness requirement. The fallback role must be
  least-privileged. It also means **PRD 006's role question (§11.3 there) is blocking
  for this matrix**, not a parked concern.

### Non-Functional

- **Offline-first.** Identification must be fully functional with no network. This
  is a hard requirement, not a degradation mode.
- **Night-legible.** Dark UI, high contrast, large image, minimal chrome; avoid a
  full-brightness white screen, which destroys the user's dark adaptation.
- **One-handed and glanceable** — used standing up, in the cold, possibly wearing
  gloves.
- **Storage-frugal.** Thumbnails must be as small as identification allows. This
  PRD's obligation is to **supply sizing numbers** to PRD 009's global budget; the
  budget itself, and the priority order against map tiles, are 009's to set.
- **Battery-frugal.** Sync never runs on mobile data unprompted, and never during
  the race by default (enforced by PRD 009's engine).
- **Privacy by construction.** Least disclosure, server-side enforcement, no
  export, watermarking, expiry, audit logging.
- **Baseline** stays iOS/iPadOS Safari 16.4+ / Chrome 111+ per `.rules`.

## 7. UX / UI Notes

A new destination in `vue/src/config/navigation.ts`: `name: 'identify'`,
`path: '/identify'`, label **`Genkend`**, view `IdentifyView.vue`, role-gated. The
destination `name` and the `viewLoaders` key in `router/index.ts` must match, so
these identifiers are fixed rather than TBD.

**Not `fullBleed`.** The list scrolls, and `fullBleed` currently removes the
`overflow-y-auto` wrapper in `App.vue`. Use the standard shell for the list, and a
`Dialog`/`Drawer` for the full-screen portrait.

- **Landing:** a search field plus "my team" as the default list, so the common
  case needs no typing.
- **Result list:** large thumbnail, name, team; tap for a full-screen portrait.
- **Full-screen portrait:** the photo at maximum legible size, name and team, the
  viewer watermark, and nothing else. No action bar that could imply sharing.
- **Sync state** is shown as a read-only status line sourced from `offline.store`
  ("Synkroniseret kl. 21:40 · 412 billeder"). The sync **button**, size estimate and
  metered-connection warning live in PRD 009's readiness surface, not here — one
  place, one answer.
- **Missing portrait:** a neutral placeholder with initials — visibly "no photo",
  not a failed load.
- **Night surface, scoped to this view.** A dark, high-contrast surface built on
  the existing `.dark` token block that PRD 004 retained — **not** a global theme,
  and `color-scheme` stays `light` (PRD 004 corrected it from `light dark` precisely
  because app-wide dark mode is not supported). *(Clarified 2026-08-25; this
  previously read "regardless of the app theme", implying a theming capability the
  app does not have.)*
- Use shadcn-vue primitives (`Command` or `Input` for search, `Avatar` for
  thumbnails, `Dialog`/`Drawer` for the full-screen view) per `.rules`, with
  `font-nathejk` reserved for the page headline. **`command` and `avatar` are not yet
  generated** in `vue/src/components/ui/` — generating them is a task below. Icons
  from Lucide (`ScanFace`, `Search`, `RefreshCw`, `WifiOff`).

## 8. Technical Considerations

### The central tension

Offline availability and least privilege pull against each other, and this is the
design problem of the PRD:

> To work offline, portraits must be **on the device in advance**. Anything on the
> device is, to a determined user, extractable — cache inspection, a desktop
> browser's devtools against the same origin, or simply a screenshot.

So "portraits must not be shared outside the race" cannot be *enforced* at the
device. It can only be made unlikely and unrewarding. What follows from that:

1. **Scope the payload, not the view.** The single most effective control is that a
   bandit's device never receives a spejder portrait *at all*. If enforcement lived
   in the UI, the race dynamic would be one devtools panel away from broken. This is
   why §6 forbids client-side filtering.
2. **Ship the smallest useful image.** A thumbnail sufficient to recognise a face
   is a much smaller loss if it leaks than a high-resolution portrait, and it also
   solves the storage budget. Sizing needs a decision (§11), but the principle is
   "as small as identification allows".
3. **Deter, log, expire.** Watermarking with the viewer's identity, an audit log of
   views, and a hard post-event purge. None of these prevent a screenshot; together
   they make casual sharing traceable and time-limited.
4. **Say so in the consent.** Since exfiltration cannot be excluded, the consent
   given at capture must be honest about who can see the photo. This is why the
   access matrix and the consent text are one decision, not two.

### Storage and the iOS eviction risk

- Budget scales with the permitted set. Crew get the largest bundle: roughly
  *participants × thumbnail size* (order of tens of megabytes for a four-figure
  event at ~25 KB per face — needs real numbers, §11). Participants get a handful
  of faces, which is trivial.
- Use the **Cache API** via the existing Workbox setup (`vite.config.ts` already
  configures `VitePWA` with a custom `push-sw.js`) rather than IndexedDB blobs —
  it is what service workers are built to serve from, and it survives reloads.
  PRD 009 owns the route and expiry policy.
- **iOS evicts service-worker caches for web apps that go unused**, on a timescale
  of days. This interacts badly with PRD 005, which pushes users to install
  *early*: a member who installs three weeks out and does not reopen the app may
  arrive with an empty cache. Mitigations: re-sync on every launch when a network
  is present; prompt a sync at check-in over venue wifi; treat "synced" as a state
  the app reports rather than assumes.
- Coexist with PRD 002's map tiles, which compete for the same budget. Neither
  feature should be able to starve the other — **PRD 009 owns that budget** and the
  priority order; this PRD supplies the sizing numbers (§11).

### Frontend (Vue 3 / TS)

**The sync mechanism is not built here.** Per the 2026-08-25 direction that
everything cacheable should have a local copy, the generic sync engine, storage
budget, eviction policy and readiness UI belong to **PRD 009** (offline-first
client data layer). Portraits are its most demanding consumer, not its owner.

What this PRD contributes to that layer:

- A **dataset registration** for portraits: cache-first, binary, high priority,
  server-issued expiry, purge after the event.
- A **metadata index** (person id, name, team, portrait version) registered as
  structured data so search and lists keep working when images have been evicted.
  This separation is a PRD 009 requirement precisely because this feature needs it.
- `views/IdentifyView.vue` + `components/identify/*`. Thumbnails compose the
  shadcn-vue `avatar` primitive directly — the same primitive PRD 003's
  `ProfilePhoto.vue` composes. Do not reuse or fork PRD 003's capture component.

If PRD 009 is not approved, this PRD has to build a private sync engine — which is
the outcome 009 exists to prevent, and a reason to sequence 009 first.

### BFF (Go)

- New `go/cmd/api/portraits.go`, handlers behind `app.requireAuth`.
- Authorization is a **single server-side function** — `mayView(viewer, subject)`
  — used by both the manifest and the image handler, with table-driven unit tests
  covering every role pair, including the symmetry of the spejder/bandit rule. If
  that logic exists in two places it will diverge, and the failure mode is a
  broken game or a privacy breach.
- Depends on **PRD 006** for the caller's role and team, and on PRD 003/005 for
  the stored portraits. It cannot be built before the directory is real.
- Serve images through an authenticated handler, never a static path — PRD 003
  already establishes that rule so the URL is not a bearer-less capability.

### API endpoints (OpenAPI annotations mandatory, per `.rules`)

- `GET /api/portraits/manifest` — the list the caller may see: person id, name,
  team, portrait version/etag, and whether a portrait exists. Supports
  `If-None-Match` for delta sync. `200` / `304` / `401`.
- `GET /api/portraits/{personId}` — one thumbnail, authorized per request. `200` /
  `304` / `401` / `403` / `404`. **`403` and `404` must be indistinguishable** to
  the caller, or the endpoint becomes an oracle telling a spejder which ids are
  banditter.
- `POST /api/portraits/views` — queued view-audit events, batched, accepted after
  the fact so the log survives offline use. `202` / `401`.

### Data / storage

- The portrait **bytes** live in a blob store, not on the event stream; their
  existence and metadata travel as events (architecture rule, 2026-08-25 — see PRD
  008 §8). This PRD reads the resulting projection plus the blob.
- Generate and store the thumbnail server-side at upload time so every client gets
  an identical, small, EXIF-corrected image, content-addressed like the original.
- The view audit log is a projection of published audit events, not a direct write.

### Dependencies & risks

- **Blocked by PRD 006** (directory: roles *and* the role-taxonomy decision the
  matrix depends on), by PRD 003/005 (portrait capture and storage), by PRD 008
  (persistence, blob store, event publishing). **Needs PRD 009's mechanism**, but is
  not blocked in both directions: 009's storage budget needs only this PRD's *sizing
  answers* (§11.4/§11.5), not this PRD shipping. Coordinates with PRD 002 on the
  shared budget — coordination, not blocking. Realistically the last of the set to
  ship.
- **Risk: consent is the gating item, not the code.** ~~Photographs of minors…~~
  **Narrowed 2026-08-28 (task 102):** the legal basis for *holding* the portrait is
  settled (safety, consent from sign-up), so what remains a decision is only the
  **audience**: photographs of minors shown to other minors and cached on personal
  devices. If the answer is "we cannot justify participants seeing participants",
  the access matrix shrinks to crew-only and the feature is still worth building —
  design it so that narrowing is a configuration change, not a rewrite.
- **Risk: the race dynamic is a correctness requirement.** A leak of spejder faces
  to banditter damages the event itself, not just privacy. It deserves the same
  seriousness as an auth bug: explicit tests, and no client-side shortcuts.
- **Risk: silent sync failure.** A user who believes they have portraits and does
  not is worse off than one who knows they do not. Sync state must be prominent.
- **Risk: storage competition with map tiles**, and OS eviction of both.
- **Risk: scope creep into surveillance.** A searchable index of participants'
  faces and locations is a different product from an in-event companion app.
  The non-goals in §4 are load-bearing.

## 9. Success Metrics

- Identification works with the network disabled, on a real device, for every
  permitted role — verified by test, not by assumption.
- ≥ 90% of crew devices report a completed sync before the race starts.
- Zero incidents of a spejder seeing a bandit portrait, or the reverse.
- Zero portraits found outside the race (no leak reports, no sharing incidents).
- Personnel report faster identification of members at night than the previous
  event — worth a baseline, as with PRD 005's check-in timing.
- Complete purge confirmed after the event: server storage empty, and cache
  expiry verified on a sample of devices.

## 10. Rollout / Task Breakdown

Sequence authorization first — it is the part that must not be wrong — then the
manifest, then offline sync, then the view. Ship behind a runtime flag in
`config/runtime.ts`, and start with **crew-only access** even if a wider matrix is
approved: it delivers the stated night-identification value with the smallest
audience, and widening later is easy while narrowing after the fact is not.

Proposed tasks for `roadmap/tasks/open/`:

- [ ] Task: server-side thumbnail generation — **owned by PRD 003** (it owns
      upload); listed here as a dependency, not a duplicate task
- [ ] Task: `mayView(viewer, subject)` authorization + exhaustive role-pair tests
      (every role including `gøgler` and unclassified `crew`, both directions)
- [ ] Task: generate the `avatar` and `command` shadcn-vue primitives
- [ ] Task: `GET /api/portraits/manifest` with etag/delta support
- [ ] Task: `GET /api/portraits/{personId}` with indistinguishable 403/404
- [ ] Task: `POST /api/portraits/views` audit ingestion (batched, offline-tolerant)
- [ ] Task: register portraits as a PRD 009 dataset (cache-first, TTL, priority)
- [ ] Task: metadata index for offline search independent of the image cache
- [ ] Task: IdentifyView — search, team browse, night-legible full-screen portrait
- [ ] Task: viewer watermark overlay
- [ ] Task: sync-state UI, manual sync, metered-connection warning (via PRD 009)
- [ ] Task: names-only degradation + missing-portrait placeholder
- [ ] Task: supply sizing numbers to PRD 009's storage budget
- [ ] Task: offline test protocol on real iOS + Android devices, radio off
- [ ] Task: post-event purge procedure (server + client) and verification

## 11. Open Questions

1. **Confirm the access matrix in §6.** Two points specifically: (a) should a
   spejder see *only* their own patrol, or all patrols? (b) should participants see
   crew? "Everyone accepted into the race has a right to see the portrait" could
   mean "everyone gets the feature" (which is how this PRD reads it) or "everyone
   may see everyone" (a much larger disclosure). Which?
2. **Consent text and legal basis** — **the legal basis is ANSWERED 2026-08-28**
   (task 102, PRD 003 §6): consent is held from sign-up and the portrait is an
   in-race safety feature, purged after the event. That unblocks capture in PRDs 003
   and 005.

   What is **still this PRD's to settle** is the audience: "it is a security
   feature" justifies *staff* viewing a member's face and says nothing about
   participants viewing each other, which is a race-integrity question as much as a
   privacy one. Design for that narrowing being a configuration change, as §8
   already requires.
3. **Where do gøglere fit?** They are participants at posts; do they need to
   identify members, and should anyone be able to identify them?
4. **Thumbnail size** — what dimensions are sufficient to recognise a face on a
   phone at night? This decides the storage budget. Needs a real judgement call,
   ideally tested on a device outdoors.
5. **How many participants**, per role, for the sizing in §8? Answerable from
   `tilmelding`.
6. **When a member leaves the race** (`released`/`reunited`), does their portrait
   vanish from devices immediately, or persist to the end of the event? An
   identification need can outlive a withdrawal.
7. **Is a view audit log proportionate**, or does logging who looked at which
   child's face create a worse privacy problem than it solves? Argument both ways;
   needs a decision rather than a default.
8. **Does identification need a scan path?** The app has a read-only scan
   **history** (`go/cmd/api/scans.go`, `scans.store`); there is **no scanner** — PRD
   002 §4 explicitly excludes scanning as post-personnel tooling. So a scan path
   would be new work. It would also only help with a cooperative, conscious person,
   which is not the samarit case. Complement or distraction?
9. **Should the map (PRD 002) show faces?** Tempting and probably wrong — it turns
   an identification tool into a tracking overlay. Explicitly out of scope here.
10. **Post-event purge on a dormant device** — a phone that never reopens the app
    keeps its cache until the OS evicts it. Is a cache expiry timestamp baked into
    the payload sufficient, given the service worker may never run again?
