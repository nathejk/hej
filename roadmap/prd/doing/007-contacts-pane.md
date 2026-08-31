# PRD 007 — Contacts pane (person lookup with portraits, scoped by race role)

**Status:** doing
**Author:** agent session (Zed)
**Created:** 2026-08-25
**Last updated:** 2026-08-31
**Approved:** 2026-08-31
**Shipped:**
**Target users:** crew (samarit / guide / postmandskab are crew), bandit, gøgler. **Not spejder.**

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.

Scope was reshaped by maintainer direction on 2026-08-31. This is no longer a
standalone "identification" surface: it fills the existing `Kontakter` pane, it is a
crew/bandit/gøgler directory, and it adds search and favourites. Spejdere are not
viewers and are not browsable — crew reach them only by looking up one patrol at a
time. The file name keeps its original slug because PRDs are referred to by number,
not path (README).
-->

---

## 1. Summary

Fill the app's existing **`Kontakter`** pane with a **crew / bandit / gøgler
directory** — name, portrait, group, phone — **cached on the device** so it works with
no network. Who you see is scoped by race role: banditter see banditter (grouped by
klan), gøglere see gøglere, crew see crew. **Spejdere neither get the pane nor appear
in it.** Crew additionally get a **live patrol lookup** — one patrol at a time, by
number, fetched on demand and **not cached** — returning that patrol's members with
status and phone numbers. A search field spans the cached directory, and anyone in it
can be favourited.

## 2. Problem & Motivation

- **What problem does this solve?** Two gaps close at once. `Kontakter` is a
  `PagePlaceholder` in `vue/src/views/ContactsView.vue` — a destination in the
  bottom nav that does nothing. And PRD 005 captures a self-portrait whose stated
  purpose is identifying people during the race, which nothing consumes today.
- **Why the directory is cached.** The people you work alongside are a fixed,
  known-in-advance set, and you need them at 03:00 in woodland on a phone that has
  been navigating for eight hours. A directory that fetches on demand is unavailable
  exactly when it is used, so the directory is offline-first.
- **Why the patrol lookup is not cached** (decision 2026-08-31). It is the opposite
  case: rare, targeted, and about data that changes during the race — status and phone
  numbers — where a stale answer is worse than no answer. Fetching it live also means
  **no spejder record is ever stored on a crew device**, which keeps "scope the
  payload, not the view" intact rather than relaxed (§8). The cost is honest and worth
  stating: **with no signal, the lookup does not work.** The fallback is the radio and
  HQ, which is exactly how this works today, so it is a gap rather than a regression.
- **Why scoping is the whole design.** Spejdere are the minors. Giving them a
  browsable index of other participants' faces — or making them browsable to
  others — is both the largest privacy cost in the event and a way to spoil the
  game. So the *directory* is adults only, and a spejder's details are reachable
  only one patrol at a time, live, by crew.
- **Why now?** PRD 005's portrait step is partly blocked on this: capture-first,
  viewer-later means collecting sensitive data with no realised benefit.
- **Evidence.**
  - PRD 005 §11 (2026-08-25) established the portrait's purpose as operational
    night-time identification and flagged the viewing surface as unowned.
  - PRD 003 non-goals cross-person photo viewing explicitly, deferring it to "a
    separate feature with its own consent and access story". This is that PRD.
  - Task 102 (2026-08-28) settled the legal basis for *holding* a portrait and
    left the **audience** to this PRD. §6 is that answer.
  - Task 104/110 delivered server-side thumbnails already (`thumb256`, ~4.5 KB),
    so the read side is the only part missing.

## 3. Goals

- A bandit, gøgler or crew member can identify someone in front of them, in the
  dark, with no connectivity.
- Finding a specific person in the directory takes one search, not a scroll.
- The people you work alongside most are one tap away (favourites, and own-klan-
  first ordering for banditter).
- A samarit with a signal can look up patrol 138 and get its members' faces, status
  and numbers — without the event's participants ever being stored on their phone.
- Access is enforced **on the server**, not in the interface.
- Spejdere are not viewers and are not browsable, so the game dynamic and the bulk
  of the minors' privacy exposure are handled by scope, not by rules.
- Portraits stay inside the race: no export surface, and a hard post-event purge.

## 4. Non-Goals

- **A contacts pane for spejdere.** The destination is hidden for them entirely.
  Spejdere keep the identification mechanisms they already have — patrol numbers
  and arm numbers (`senior.ArmNumber`).
- **Browsing spejdere.** There is no patrol list, no "all patrols" index, no prefix
  or partial matching on patrol number, and spejdere never appear in search
  results. The lookup answers "show me patrol 138" and refuses to answer "which
  patrols exist".
- **Capturing portraits.** PRD 005 (onboarding) and PRD 003 (profile) own capture
  and replacement. This PRD only reads.
- **Face recognition or automated matching.** Humans look at faces here. No
  biometric processing, which would change the legal basis materially.
- **Cross-population browsing.** A bandit cannot reach a gøgler, a gøgler cannot
  reach a bandit, and neither can reach a spejder. Each population sees its own,
  plus crew per §6.
- **A public or parent-facing surface.** Guardians do not get a viewer.
- **Organizer bulk browsing or export.** No "all portraits" grid, no download, no
  print.
- **Messaging or calling as a feature of this pane.** Rows show a phone number and a
  `tel:` link is fine, but there is no in-app messaging, no call log and no group
  broadcast. Which numbers appear is settled in §6: directory rows carry the person's
  phone, and nothing carries a guardian's.
- **Guaranteeing portraits cannot be exfiltrated from a device.** Honestly out of
  reach; see §8. We reduce exposure and deter, we do not prevent.

## 5. User Stories & Scenarios

- As a **samarit** sent to patrol 138, I want to look up that patrol and see its
  members' faces, status and phone numbers, so I find or reach the right person.
- As a **bandit**, I want to see the other banditter grouped by klan — mine open
  already — so I know who is out tonight.
- As a **bandit**, I want the crew members who are out as banditter to appear in
  their klan alongside everyone else, because operationally they are banditter.
- As a **gøgler**, I want to see all other gøglere, including section gøglere, so I
  can find the person staffing the next post.
- As a **crew member**, I want to see the rest of crew, so I can put a face to the
  voice on the radio.
- As **anyone with the pane**, I want to search across everyone I may see, so I can
  go straight to "Freja" instead of guessing which group she is in.
- As **anyone with the pane**, I want to favourite the handful of people I work
  with, so they are at the top without searching.
- As a **participant**, I want to know who can see my photograph before I take it,
  so that my consent means something.

### Primary path — bandit, offline

1. Before the event (on wifi, at check-in), the app syncs the people this user's
   role permits, as metadata plus small thumbnails, into local storage managed by
   PRD 009.
2. During the race the bandit opens `Kontakter` with no signal.
3. Favourites sit at the top. Below them, banditter grouped by klan, with the
   user's own klan expanded and the others collapsed.
4. They recognise the face they are looking for and continue.

### Primary path — samarit patrol lookup, live

1. HQ tells the samarit over the radio: patrol 138, member Freja.
2. They open `Kontakter`, tap **Slå patrulje op**, and type `138`.
3. The app fetches that patrol and lists its members with faces, names, current status
   and phone numbers. Nothing is written to the device, and nothing else about the
   event's patrols is reachable from here.
4. They locate or call Freja and continue.
5. **With no signal** the lookup says so plainly and suggests the radio. The cached
   crew/bandit/gøgler directory keeps working regardless.

### Primary path — search

1. A gøgler taps the search field and types "fre".
2. Matches across every person they may see are listed with face, name and group,
   ranked with favourites first.

### Edge cases

- **Portrait missing.** Skippable at onboarding, so many will be absent early on.
  Show a neutral placeholder with initials and the name; never an error, never a
  broken image.
- **Directory updated mid-race** (a crew member added, a number corrected, someone
  withdrawn). Propagates on the next version check — foreground, reconnect, or within
  ~60 s while open (§8). The list must update in place without losing the user's scroll
  position or collapsing their expanded group.
- **Portrait added after the last sync.** The device shows what it has until the next
  version check picks it up. The pane must say when it last synced, so a user can tell
  "no photo" from "not updated yet".
- **Cache evicted by the OS.** Real on iOS (§8). The pane must degrade to
  names-only rather than appear empty, and re-sync when a network appears.
- **Never synced at all** (installed on the morning of the race, on mobile data).
  PRD 009's readiness surface owns the explicit download action and its warning.
- **A spejder reaches `/contacts`** by URL after the nav entry is hidden. The route
  must be role-gated *and* the endpoints must refuse — a hidden nav item is not
  access control.
- **A bandit tries to reach a gøgler or spejder record** by id guessing or a stale
  cached response. Must fail server-side, and must not be inferable: `403` and
  `404` are indistinguishable, so the endpoint cannot be used to enumerate who
  exists.
- **A crew member marked as bandit** (section slug `bandit`). They are listed in the
  bandit population, in their klan, and — since their role is still `crew` — also
  among the crew. Appearing twice is correct: both lists are answering "who is out
  as what", and hiding them from one would make a real colleague unfindable.
- **A person with no klan** (bandit population, missing grouping data). Falls into a
  single clearly-labelled "Uden klan" group rather than disappearing.
- **Patrol number does not exist, or is a bandit klan number.** One neutral "ingen
  patrulje med det nummer" for every miss. The lookup must not distinguish "no such
  patrol" from "that is not a patrol you may look up", or it becomes a way to map
  the klan numbering.
- **A bandit or gøgler tries the patrol lookup.** Refused server-side; the entry
  point is not rendered for them, but the handler is what enforces it.
- **Role changes mid-event** (crew reassignment). Permission is re-evaluated on the
  next sync; anything now out of scope must be purged from the device.
- **Favourited person leaves scope** (role change, withdrawal). The favourite is
  dropped, not left as a dangling row that hints at someone the user may no longer
  see.
- **Member leaves the race** (`released`, `reunited`). Their **phone number is
  purged**, their **name remains visible until the end of the race** with a clear
  status marking, and the row is no longer something you can call. Withdrawn members
  are excluded from favourites' "still reachable" assumption but not hidden — a
  disappearing row invites "did I imagine them?", a marked row answers the question.
- **Event ends.** Portraits are purged server-side (already implemented —
  `go/cmd/api/portraitpurge.go`) and expire on devices. A device that never reopens
  the app cannot be purged remotely; see §8.

## 6. Requirements

### Functional

- [ ] The existing **`contacts` destination** in `vue/src/config/navigation.ts`
      gains a `roles` list excluding `spejder`, and `ContactsView.vue` replaces its
      `PagePlaceholder` with the real pane.
- [ ] The **directory** is **offline-first**: fully usable with the radio off.
- [ ] **Search field at the top**, spanning every person the caller may see.
      Matches on name, group (klan / section) and arm number. Runs locally
      against the synced index — a search that needs the network is not a search.
- [ ] **Favourites**: any visible person can be favourited, favourites are shown
      first, and the toggle is on the row rather than buried in the profile page.
- [ ] **Grouping is per population**, and comes from the server rather than being
      inferred client-side:
      - bandit → **grouped by klan**, the caller's own klan **expanded by default**,
        the rest collapsed;
      - gøgler → all gøglere, **including the crew gøglere** (section slug
        `goeglerledelse`);
      - crew → all crew.
- [ ] **Directory placement is separate from app role.** A crew member whose section
      slug is `bandit` is listed in the bandit population (in their klan); one whose
      slug is `goeglerledelse` is listed among the gøglere. Their *role* stays `crew`,
      so they view as crew and keep the patrol lookup. Placement decides where you
      appear; role decides what you see. See §8.
- [ ] **Row layout** (confirmed 2026-08-31): avatar on the **left**; **member name**,
      with the **team/group name in smaller grey print below it** where applicable;
      **phone number on the right**. Tapping the row opens that person's profile.
- [ ] **Person profile page**: a route of its own showing a **large avatar**, name,
      group/team, phone number, and function/section for crew. **Postal address and
      guardian number are excluded** (2026-08-31). Guardian numbers are excluded
      everywhere — see `.rules`; the BFF projects the field out rather than relying on
      the client not to render it.
- [ ] **Favourites are device-local** (confirmed 2026-08-31): stored on the device,
      no endpoint, no server-side record of who is interested in whom. They do not
      survive a reinstall, which is accepted. Favourites are re-validated against the
      manifest on each sync so one cannot outlive the user's access to that person.
- [ ] **Crew-only patrol lookup**: an explicit action that takes a **full, exact**
      patrol number and returns that one patrol's members with portrait, **current
      status and phone number**. No listing, no prefix matching, no inclusion in the
      main search, and a single indistinguishable "not found" for every miss.
- [ ] The patrol lookup is **live and not cached** (2026-08-31): fetched on demand,
      held in memory for the duration of the lookup, never written to local storage and
      never registered as a PRD 009 dataset. No spejder record is stored on any device.
- [ ] With no connectivity the lookup **fails clearly** — "kræver forbindelse", with a
      pointer to the radio — rather than showing an empty patrol or a stale one.
- [ ] The server returns **only** the people the caller's role permits. No
      client-side filtering of a broader payload — a device must never hold a
      portrait its user may not see.
- [ ] Access is evaluated per request and per sync against the caller's **current**
      role and group, from the directory (PRD 006).
- [ ] Portraits are served as the existing **`thumb256`** rendition (task 104/110),
      not as originals.
- [ ] **Portraits and the person index register as PRD 009 datasets** — the
      crew/bandit/gøgler directory only. Cache-first binary for images, structured data
      for the index, high priority, server-issued expiry, purged after the event. The
      patrol lookup is **not** a dataset and is not registered.
- [ ] The **metadata index is separable from the images**, so search, groups and
      favourites keep working when thumbnails have been evicted.
- [ ] Endpoints follow **PRD 009's manifest/delta convention**
      (`If-None-Match` / version) so the generic engine can drive them.
- [ ] **Directory updates reach devices during the event without much delay**
      (2026-08-31). Concretely: a change made upstream is visible in the pane
      **immediately when the app is brought to the foreground**, and **within ~60
      seconds while the app is open**. Additions, edits, withdrawals (status marking +
      number purge) and portrait changes all propagate.
- [ ] **Metadata deltas may run during the race, on mobile data.** They are small
      enough that the usual "bulk sync only on wifi, not during the race" rule must not
      apply to them — see the split in Non-Functional. Bulk image sync keeps it.
- [ ] Metadata propagates **ahead of images**: a corrected name or number must not wait
      on a portrait download, and a new person appears with a placeholder rather than
      not at all.
- [ ] No export affordance: no download, no share sheet, no long-press save, no
      open-in-new-tab.
- [ ] Cached portraits carry a **server-issued** expiry and are purged after the
      event; the server stops serving them at the same point (already true —
      `portraitpurge.go`). Server-issued so a wrong device clock cannot defeat it.
- [ ] **Withdrawn members** (`released` / `reunited`) keep their **name and portrait**
      until the end of the race, carry a **clear status marking** in the list and on
      the profile, and have their **phone number purged** from the manifest — so a
      device that already synced it drops it on the next delta.
- [ ] Names-only degradation whenever portraits are unavailable, for any reason.
- [ ] *(§11.7)* Every patrol lookup is logged **server-side** on the request path — no
      client-side queue, no batching, no ingestion endpoint, because the lookup is
      online by definition.

### Access matrix

Maintainer direction, 2026-08-31. `samarit`, `guide` and `postmandskab` are **all
crew** — the matrix does not split crew by function, which also removes this PRD's
dependency on PRD 006 Q3 (the three functions have zero members in the real data;
see §11.2).

| Viewer | spejder | bandit | gøgler | crew |
|---|---|---|---|---|
| **spejder** | — no pane — | — no pane — | — no pane — | — no pane — |
| **bandit** | ❌ | ✅ all, grouped by klan (incl. crew banditter) | ❌ | ✅ all |
| **gøgler** | ❌ | ❌ | ✅ all, incl. crew gøglere | ✅ all |
| **crew** | ⚠️ **exact patrol lookup only** — never listed, never searched | ✅ all | ✅ all | ✅ all |

What this settles:

- **It is a crew / bandit / gøgler directory.** Confirmed 2026-08-31. Adults only,
  which is why the audience question task 102 left open is answerable at all.
- **Spejdere are not in the directory in either direction.** The "minors browsing
  minors" problem that dominated earlier drafts is closed by scope, not by policy.
- **The samarit case survives as a lookup, not a list.** Confirmed 2026-08-31:
  "ok to have samarit looking up a patrol one by one". The operational value is
  retained; the browsable index of minors' faces is not built.
- **Spejder/bandit mutual invisibility**, which earlier drafts called a correctness
  requirement, now falls out of the structure for free. Still worth an explicit
  test: it is a game-integrity property, not just a side effect.
- **Crew is one role, not three.** `samarit`, `guide` and `postmandskab` are all
  crew (2026-08-31), so the matrix does not split crew by function. That removes
  this PRD's dependency on PRD 006 Q3. **All crew may use the patrol lookup**
  (confirmed 2026-08-31), including the generic `crew` fallback — which in the real
  data is *every* crew member, since task 078 found nobody classified into the three
  capability sections. This is a deliberate "yes, we mean all crew", not an accident
  of the fallback: ~20 known adults.

### Non-Functional

- **Offline-first for the directory.** The crew/bandit/gøgler directory must be fully
  usable with no network. The patrol lookup is deliberately online-only (§6) and must
  say so when there is no signal.
- **Night-legible.** Dark, high-contrast surface, large image, minimal chrome;
  avoid a full-brightness white screen, which destroys dark adaptation.
- **One-handed and glanceable** — used standing up, in the cold, possibly gloved.
- **Fast search.** Results feel instant on a mid-range Android at ~1,000 records.
- **Storage-frugal.** At `thumb256` (~4.5 KB measured, task 104) every cached
  population is small: ~151 banditter ≈ 0.7 MB, ~99 gøglere ≈ 0.4 MB, ~20 crew ≈
  0.1 MB (counts from task 078). Nothing else is cached — the patrol lookup stores
  nothing — so the largest bundle in the event is under ~1 MB. This PRD supplies those
  numbers to PRD 009's global budget; the budget and the priority order against map
  tiles are 009's to set.
- **Battery- and data-frugal, split by sync class.** These are two different jobs and
  one rule cannot serve both:
  - **Bulk sync** (portrait images, first full index) — never on mobile data
    unprompted, and not during the race by default. Enforced by PRD 009's engine.
  - **Incremental metadata deltas** (a new crew member, a changed number, a
    withdrawal) — **allowed during the race and on mobile data**, because they are a
    few hundred bytes and freshness is the point. A version check while the app is open
    must cost bytes, not kilobytes.

  *(Corrected 2026-08-31: this previously read "sync never runs on mobile data
  unprompted, and not during the race by default", which contradicted the freshness
  requirement added the same day. The distinction is size and purpose, not timing.)*
- **Freshness.** Directory changes visible on foreground immediately, within ~60 s
  while the app is open, and no polling at all when the app is not in use.
- **Privacy by construction.** Least disclosure, server-side enforcement, no
  export, expiry, and spejdere excluded by scope.
- **Baseline** stays iOS/iPadOS Safari 16.4+ / Chrome 111+ per `.rules`.

## 7. UX / UI Notes

No new route. The existing destination
(`{ name: 'contacts', path: '/contacts', label: 'Kontakter', icon: Users }`) gains a
`roles` list, and `views/ContactsView.vue` is built out. **Not `fullBleed`** — the
list scrolls, and `fullBleed` removes the `overflow-y-auto` wrapper in `App.vue`.

- **Top:** the search field, sticky, so it is reachable with a thumb while
  scrolling.
- **Favourites** as the first section, horizontally scrollable avatars or a compact
  list, with a clear way to unfavourite.
- **Groups** below: collapsible sections (bandit → klan, gøgler → one list plus
  section gøglere, crew → one list). The caller's own group is expanded; expansion
  state is remembered per user.
- **Crew only — "Slå patrulje op"**: a distinct, secondary entry, not merged into the
  main search field, so it is obvious that a patrol is a different thing being asked
  for in a different way. Numeric input, exact match, results shown as a transient
  panel rather than appended to the directory listing. No recent-lookups list — that
  would accumulate into the browsable index this design avoids.
- **Row:** avatar left; name, with team/group in smaller grey text beneath it; phone
  number right-aligned. The favourite toggle lives on the row too — keep it clear of
  the phone number so a thumb aiming for "favourite" does not start a call. Tapping
  the row (not the number) opens the profile.
- **Person profile page** (`/contacts/:personId`): large avatar, name, group, phone,
  and function/section for crew — no postal address, no guardian number. Same
  night-legible surface. No action bar implying sharing; a `tel:` link on the number
  is fine, an export is not.
- **Sync state** is a read-only status line sourced from PRD 009's store
  ("Synkroniseret kl. 21:40 · 412 billeder"). During the race it should read as
  *current* rather than as a timestamp the user has to interpret — "Opdateret nu" when
  the last version check succeeded recently, an explicit stale/offline state otherwise.
  The sync **button**, size estimate and metered-connection warning live in PRD 009's
  readiness surface — one place, one answer.
- **Missing portrait:** neutral placeholder with initials, visibly "no photo"
  rather than a failed load.
- **Night surface, scoped to this view**, built on the existing `.dark` token block
  PRD 004 retained — **not** a global theme, and `color-scheme` stays `light`
  (`main.css:62`; PRD 004 corrected it from `light dark` precisely because app-wide
  dark mode is not supported).
- **shadcn-vue primitives** per `.rules`: `input` or `command` for search, `avatar`
  for thumbnails and the profile's large avatar, `accordion` for the groups, `card`
  for the profile detail block, `drawer` for the patrol-lookup panel. `avatar`,
  `accordion`, `input`, `card`, `dialog` and `drawer` **already exist** in
  `vue/src/components/ui/`; only **`command`** would need generating, and only if
  search wants the command-palette treatment.
- **Icons** from Lucide: `Users` (already the nav icon), `Search`, `Star`, `Phone`,
  `RefreshCw`, `WifiOff`.
- `font-nathejk` on the page headline only.

## 8. Technical Considerations

### The central tension

Offline availability and least privilege pull against each other:

> To work offline, portraits must be **on the device in advance**. Anything on the
> device is, to a determined user, extractable — cache inspection, a desktop
> browser's devtools against the same origin, or simply a screenshot.

So "portraits must not leave the race" cannot be *enforced* at the device. What
follows:

1. **Scope the payload, not the view.** The single most effective control is that a
   bandit's device never receives a gøgler or spejder record *at all*. If
   enforcement lived in the UI, the game dynamic would be one devtools panel away
   from broken. This is why §6 forbids client-side filtering.
2. **Keeping spejdere out of the directory is the biggest privacy win available**,
   and it is structural rather than procedural. It costs nothing to enforce and
   cannot regress by accident, unlike a rule inside a filter.
3. **The live patrol lookup keeps rule 1 intact.** An earlier draft of this PRD had the
   lookup working offline, which meant shipping ~557 spejder thumbnails to every crew
   device — a documented, deliberate relaxation of rule 1, and the largest privacy cost
   in the design. The 2026-08-31 decision to leave the lookup uncached removes it
   entirely: **no spejder record is stored on any device, ever.** The trade is
   availability for exposure, and it is a good one — the exposure was permanent and
   event-wide, the unavailability is occasional and has a working fallback (radio, HQ).
   It also removes ~2.5 MB from every crew device and the whole class of "what if a
   crew phone is lost" reasoning.
4. **Ship the smallest useful image.** `thumb256` is a much smaller loss if it leaks
   than a full portrait, and it makes the storage budget a rounding error.
5. **Expire and purge.** Server-side purge exists; the cached directory carries a
   server-issued expiry. The lookup needs neither, since it stores nothing.
6. **Say so in the consent.** Since exfiltration cannot be excluded, the consent at
   capture must be honest about who can see the photo. The access matrix and the
   consent text are one decision, not two.

### Storage and the iOS eviction risk

- Budget is small (§6, Non-Functional). The earlier draft's "tens of megabytes at
  ~25 KB per face" was an estimate made before task 104 measured the real
  thumbnail; the corrected figure is ~4.5 KB and ~3.7 MB event-wide, which largely
  dissolves the "competing with map tiles" risk. PRD 009 still owns the budget.
- Use the **Cache API** via the existing Workbox setup (`vite.config.ts` configures
  `VitePWA` with a custom `push-sw.js`) for the images, and structured storage for
  the index. PRD 009 owns route and expiry policy.
- **iOS evicts service-worker caches for web apps that go unused**, within days.
  This interacts badly with PRD 005, which pushes users to install *early*: someone
  who installs three weeks out and never reopens may arrive with an empty cache.
  Mitigations: re-sync on every launch when a network is present; prompt a sync at
  check-in over venue wifi; treat "synced" as a state the app reports rather than
  assumes.

### Keeping the directory fresh — and why not push

The freshness requirement (§6) is a pull, not a push, and that is a platform
constraint rather than a preference.

**Push cannot be used for silent invalidation on our baseline.** iOS 16.4+ web push for
home-screen web apps requires every push to result in a **user-visible notification**;
there is no silent/data-only push. The repo's own `vue/public/push-sw.js` reflects that
— its `push` handler always calls `showNotification`. Using push to say "the directory
changed" would therefore either buzz every crew member's phone for a corrected phone
number, or risk the permission being revoked. Wrong tool.

So the mechanism is a **cheap version check, pulled at moments that already exist**:

1. **On foreground** (`visibilitychange` → visible, and on app launch): check
   immediately. This is the case that matters most — someone opening the pane to look
   something up wants it current.
2. **While the app is open**: poll the version endpoint on a ~60 s interval, and stop
   polling the moment the app is hidden. No background timers, no periodic background
   sync (which iOS does not offer reliably anyway).
3. **On reconnect** (`online` event): check, since the device may have missed changes
   while offline.

The check is `GET /api/contacts/version`, which returns a monotonic version for the
caller's permitted set and nothing else. It must be cheap enough to poll every minute
by a few hundred devices: a small integer or hash, `ETag`-able, answered from a
projection read rather than a recomputation. Only when the version differs does the
client fetch the manifest delta, and only then does it fetch any newly-referenced
images.

**Metadata before images**, deliberately: the delta carries names, groups, numbers and
status, and image fetches follow. A corrected number appearing a minute before the new
portrait does is fine; the reverse is not.

This mechanism is generic — "tell me if this dataset changed" is not portrait-specific
— so **it belongs in PRD 009's engine**, with this PRD as the consumer that requires
it. If 009 does not provide a freshness/invalidation contract, this requirement forces
this PRD to build one, which is the duplication 009 exists to prevent. Worth raising
against 009 explicitly, since its current draft frames sync as a pre-event readiness
problem rather than a during-event freshness one.

### Member status — decided (task 150)

"Clear status marking" (§6) needs the member lifecycle, and **this repo deliberately
does not have it.** `person.memberStatus` is `racing` or empty, and the comment above
`handleTeamStarted` in `go/nathejk/table/person/consumer.go` says why in as many words:
`hq` already projects the full lifecycle (`spejderstatus` — withdrawals, pickups,
shelter placements, handovers), and mirroring it here would create "a second, lagging
notion of member status in a second repo, disagreeing with `hq` in ways nobody would
notice until an organizer compared two screens". It then names the sanctioned route:
**read `hq`'s projection, or lift it to `shared-go` — not grow a parallel one here.**

The uncached lookup makes this easier, and investigation (task 150) narrowed it further
than this section originally claimed.

**Decided 2026-08-31 (task 150): lift the transitions to `shared-go` and consume them
here.** Two corrections to what this section said before:

- **"Read `hq`'s projection at request time" is not available.** This PRD ranked it
  first. `hej` has no `hq` client, no access to its database, and is event-sourced from
  the stream — its own projection is the only read model it has. The option was ranked
  without checking.
- **Most of the lift already exists.** `shared-go/types/member.go` already defines the
  full lifecycle vocabulary — `registered`, `seated`, `racing`, `finished`, `waiting`,
  `transit`, `sheltered`, `reunited`, `released` — with documentation and helpers
  (`Valid()`, `CanFinish()`, `InOurCare()`). What is missing is only the **transition
  message types**, which still live in `hq` (task 174), and a "left the race" predicate
  to sit beside `InOurCare()` (task 175).

**Why this is not the parallel projection `consumer.go` warns against.** That warning is
about a second *notion* of status — a different vocabulary, or rules re-derived locally,
which can then disagree with `hq`. Storing `shared-go`'s `types.MemberStatus` verbatim,
from the same events, is a cache of one shared notion. The line to hold: this repo may
store and read the value, but must not implement lifecycle rules.

**The split:**

- the **live patrol lookup** returns the full current `types.MemberStatus`, fresh because
  it is online;
- the **cached manifest** carries a single coarse `stillInRace` boolean — one bit, not a
  lifecycle, because that is all the directory needs to mark a withdrawn colleague.

Until task 174 lands no transition events arrive, so the flag reads `true` for everyone.
That is correct for a pre-event state, and is documented at the call site rather than
assumed.

### Frontend (Vue 3 / TS)

**The sync mechanism is not built here.** The generic sync engine, storage budget,
eviction policy and readiness UI belong to **PRD 009**. This pane is its most
demanding consumer, not its owner. What this PRD contributes:

- Dataset registrations: portraits (cache-first, binary, high priority,
  server-issued expiry) and the person index (structured, searchable, survives
  image eviction).
- `views/ContactsView.vue` + `components/contacts/*`: search field, favourites
  section, grouped accordion, portrait dialog.
- Thumbnails compose the existing shadcn-vue `avatar` primitive — the same one PRD
  003's `ProfilePhoto.vue` composes. Do not reuse or fork PRD 003's *capture*
  component.
- `roles` gating on the `contacts` destination, plus a router guard. Note the
  comment already in `navigation.ts`: a destination without `roles` is visible to
  every signed-in role including the `crew` fallback, so this must gate explicitly.

If PRD 009 is not approved, this PRD has to build a private sync engine — the
outcome 009 exists to prevent, and the reason to sequence 009 first.

### BFF (Go)

- New `go/cmd/api/contacts.go`, handlers behind `app.requireAuth`. Portrait bytes
  reuse the existing `portrait.go` / `photo.go` machinery (`normalizePortrait`,
  `PortraitThumb`, `hasPortrait`) rather than a parallel path.
- Authorization is a **single server-side function** — `mayView(viewer, subject)` —
  used by both the manifest and the image handler, with table-driven unit tests
  covering every role pair in both directions, including spejder in every position.
  If that logic exists in two places it will diverge, and the failure mode is a
  broken game or a privacy breach.
- **Grouping is server-side.** The manifest carries a group id and label per person,
  and a flag for "this is the caller's own group", so the client never infers group
  membership from names it happens to have.
- Depends on **PRD 006** for the caller's role and group, and on PRD 003/005 for
  stored portraits.

### API endpoints (OpenAPI annotations mandatory, per `.rules`)

- `GET /api/contacts/manifest` — the people the caller may see: person id, name,
  group id + label, own-group flag, phone, still-in-race flag, portrait version/etag,
  and whether a portrait exists. Supports `If-None-Match` for delta sync, and must be
  able to express **removal** of a field (see Data / storage). `200` / `304` / `401` /
  `403` (spejder).
- `GET /api/contacts/version` — a monotonic version for the caller's permitted set,
  used for the freshness poll (§8). Deliberately tiny and `ETag`-able; must be
  answerable from a projection read, since a few hundred devices may call it every
  minute while the app is open. `200` / `304` / `401` / `403` (spejder).
- `GET /api/contacts/patrols/{number}` — one patrol's members with portrait ref,
  current status and phone number. **Crew only**, exact match on the full number, and
  explicitly **not cacheable**: `Cache-Control: no-store`, excluded from the service
  worker's routes, so "we decided not to cache it" is enforced by headers rather than
  by client discipline. `200` / `401` / `403` / `404`, with `403` and `404`
  indistinguishable so it cannot be used to enumerate patrols or probe the klan
  numbering. Every call is logged server-side (§11.7).
- `GET /api/contacts/patrols/{number}/photo/{personId}?size=thumb` — a spejder
  thumbnail, reachable only in the context of a permitted lookup, `no-store`. Keeps
  spejder images off the general portrait route so "never cached" is a property of the
  endpoint rather than a convention.
- `GET /api/contacts/{personId}/photo?size=thumb` — one thumbnail, authorized per
  request. Follows the `?size=` convention task 104 established on `/me/photo`
  rather than inventing a second one. `200` / `304` / `401` / `403` / `404`, with
  **`403` and `404` indistinguishable** so the endpoint is not an enumeration
  oracle.
- `PUT` / `DELETE /api/contacts/favourites/{personId}` — **not needed.** Favourites
  are device-local (2026-08-31), so there is no favourites endpoint and no
  server-side record of them.
- `POST /api/contacts/views` — **no longer needed.** With the lookup online, every
  spejder view *is* a server request, so the audit log is a server-side log line on the
  lookup handler rather than a client-side queue, a batch upload and an ingestion
  endpoint. One of the larger simplifications the 2026-08-31 caching decision buys.

### Data / storage

- Portrait **bytes** live in the blob store, not on the event stream; their
  existence and metadata travel as events (PRD 008 §8). Thumbnails are already
  generated and stored at upload (task 104), with multiple renditions modelled
  (`person.PortraitThumb`, task 110).
- **Bandit grouping is the klan**, i.e. the existing `users.User.PatrolName` /
  `PatrolID` (a patrulje for a spejder, a klan for a bandit). No new projection is
  needed and nothing here blocks on data work.

  *Lok* — the grouping of klaner that prompted the original wording — is **not
  modelled in this repo** (a grep for `lok` across `go/`, `vue/src` and `roadmap/`
  returns nothing) and is being migrated upstream into **subsections**. Decision
  2026-08-31: **group by klan now, do not model lok.** The implication for the code is
  that grouping must be a *server-supplied group id + label*, not a hardcoded "klan"
  concept in the client — when subsections arrive, the bandit view should gain a tier
  by changing what the manifest emits, not by rewriting the pane. A two-level
  grouping is the likely near-future shape, so the component should not assume one
  level is all there will ever be.
- **Crew banditter and crew gøglere are placed by section slug.** Section slug
  `bandit` → listed in the bandit population; `goeglerledelse` → listed among the
  gøglere (confirmed 2026-08-31). Both slugs already exist in
  `go/nathejk/table/person/classify.go`, where they map to `RoleCrew` — and that must
  stay true. **Do not "fix" the classifier to map slug `bandit` → `RoleBandit`:** it
  would hand a crew member the bandit pane and take away the patrol lookup, and
  `classify.go` already carries a comment saying `goeglerledelse` is deliberately not
  `RoleGoegler`. Placement is therefore a **second, orthogonal mapping** — slug →
  directory population — that the manifest computes, and it belongs next to the role
  map with a comment explaining why there are two.

  This is **magic slug matching for now**, and is expected to migrate to a **section
  flag** upstream (2026-08-31). Same shape as the existing role map, which task 078
  established as mechanism rather than stopgap: keep the placement map in one place,
  log unmapped slugs, and treat the eventual flag as a change of *source* rather than
  of structure.
- The **profile page** reads the same synced directory records as the list, so it works
  offline for directory members. For a member reached through a patrol lookup it is
  live, like the lookup itself, and equally not persisted. It must not become a second,
  wider data path: whatever the profile shows for a directory member is in the manifest
  and therefore on the device. The allow-list is **avatar, name, group, phone, and
  function/section for crew** — an allow-list in the handler, not a `SELECT *` the
  client happens to render narrowly.
- **`PhoneParent` must never appear in any response this PRD adds.** It is a `.rules`
  invariant, not a preference: guardian numbers stay out of the PWA except where a
  user approves their own (PRD 003/005). Project it out server-side, and treat a test
  asserting its absence from the manifest and the profile as part of the definition of
  done — the field is present on `users.User`, so "we just won't select it" is one
  careless change away from being false.
- **Phone numbers are part of the synced payload.** Accepted for crew (2026-08-31):
  before this app the same numbers circulated on printed lists, so a synced,
  expiring, role-scoped directory is a reduction in exposure rather than a new one.
  Worth stating plainly because the earlier drafts' storage reasoning was about images
  only, and because the printed-list argument is specific to crew — it does not
  automatically extend to participants' numbers (§11.4).
- **Favourites are device-local** — no table, no endpoint, nothing held server-side
  about who a user cares about. Store person ids only, and resolve them against the
  synced index at render time so a stale favourite cannot display a name the user may
  no longer see.
- **Purging a phone number is a *removal* in the sync delta**, not an update, which is
  a slightly different demand on PRD 009 than "fetch what changed": a device that
  already holds the number must drop it. Field-level removal (or re-issuing the record
  without the field, and having the client replace rather than merge) needs to be part
  of the manifest contract, or a withdrawn member's number survives on every device
  that synced before they left — which would make the purge decorative.
- The **patrol lookup** reads live and stores nothing: no patrol index on the device, no
  spejder rows, no spejder thumbnails. The only client-side state is whatever the open
  view holds in memory.
- Favourites are device-local (§6): person ids only, resolved against the synced
  index at render time.

### Dependencies & risks

- **Blocked by PRD 006** (roles and groups), **by PRD 003/005** (portrait capture —
  largely landed), **by PRD 008** (persistence, blob store, events — done).
  **Needs PRD 009's mechanism**, which is still in `draft/`; 009 in turn needs only
  this PRD's sizing numbers, which §6 now supplies. Sequence 009 first.
- **No longer blocked by PRD 006 Q3.** Collapsing samarit/guide/postmandskab into
  crew removes the dependency on settling the crew-function taxonomy. Worth noting
  in PRD 006 that this consumer's need has changed.
- **Risk: member status pulls in cross-repo work.** The status marking cannot be built
  from what this repo projects today (§8 "Member status"), though the uncached lookup
  reduces it to a live read plus a coarse still-in-race flag. Left unresolved, the
  likely failure is someone quietly adding a second status field to `person` and the
  two repos disagreeing during the race.
- **Risk: a decorative purge.** If the sync delta cannot remove a field, withdrawn
  members' numbers stay on every device that synced earlier and the purge is a
  server-side gesture only. Worth verifying with a test that syncs, withdraws, and
  re-syncs.
- **Risk: the freshness poll is a new load pattern.** A few hundred devices calling a
  version endpoint every 60 s is not large, but it is the first *continuous* traffic
  this app generates during the race, and it lands on the same BFF as position
  reporting (PRD 002). The endpoint must be trivially cheap and the interval must be
  configurable at runtime, so it can be widened without shipping a release.
- **Risk: PRD 009 does not cover during-event freshness.** Its draft frames sync as
  pre-event readiness. If the invalidation contract is not part of 009, this PRD builds
  a private one and the duplication 009 exists to prevent happens anyway.
- **Risk: the samarit has no signal.** The motivating scenario — 03:00 in woodland — is
  the one where the lookup is least likely to work, and this design accepts that. The
  mitigation is not technical: the radio and HQ already answer "who am I looking for",
  and the app is an improvement when there is coverage rather than a replacement for
  the existing chain. Worth revisiting if crew report coverage is worse than assumed —
  the reversal (cache the lookup) is a known, costed option, documented in §8.
- **Risk: the lookup gets cached by accident.** A service worker route added later, a
  well-meaning "offline support" change, or a generic PRD 009 registration would
  silently undo the central privacy property. `no-store` plus an explicit exclusion
  from the SW routes, plus a test, are what make the decision durable.
- **Risk: the patrol lookup drifts into a browser.** It is one product decision away
  from becoming the index of minors' faces this design exists to avoid — a
  recent-lookups list, a prefix search "for convenience", or a patrol picker would
  each do it, and the data is already on the device. Worth a comment in both the
  handler and the component saying so, because the next person to touch it will not
  have read this PRD.
- **Risk: subsections and the section flag arrive mid-build.** Lok is being migrated
  into subsections, and crew placement is expected to move from magic slugs to a
  section flag. Both are additive *if* grouping is a server-supplied group and
  placement is a single map with one call site; both are rewrites if either is spread
  through the code. Cheap to get right now, expensive later.
- **Risk: the profile page grows by accretion.** The field list is settled now
  (avatar, name, group, phone, crew function), but every future field added to it is a
  field cached on other people's devices. The handler should project an explicit
  allow-list rather than serialising a row, so adding a field is a visible decision
  rather than a side effect of a schema change. A test asserting `phoneParent` is
  absent is the tripwire.
- **Risk: game integrity is a correctness requirement.** A leak of spejder faces to
  banditter damages the event, not just privacy. Same seriousness as an auth bug:
  explicit tests, no client-side shortcuts.
- **Risk: silent sync failure.** A user who believes they have portraits and does
  not is worse off than one who knows they do not. Sync state must be prominent.
- **Risk: favourites leak scope.** A favourite persisted across a role change could
  surface someone the user may no longer see. Favourites must be re-validated
  against the manifest, not trusted.
- **Risk: scope creep into surveillance.** A searchable index of participants' faces
  is a different product from an in-event companion app. §4 is load-bearing.

## 9. Success Metrics

- The **directory** works with the network disabled, on a real device, for every
  permitted role — verified by test, not assumption.
- **No spejder record is ever written to a device** — asserted by test: after a patrol
  lookup, local storage and the caches contain no spejder row and no spejder image.
- Search returns a match in under ~150 ms at full event size on a mid-range
  Android.
- ≥ 90% of devices with the pane report a completed sync before the race starts.
- A spejder cannot reach a contacts endpoint — asserted in tests, in both the route
  guard and the handler.
- A samarit can put a face to a named member of a given patrol in under ~15 seconds
  from opening the app, with a signal.
- Zero cross-population disclosures: bandit ↔ gøgler, bandit/gøgler → spejder, and
  no spejder ever appearing in a directory listing or search result.
- The patrol lookup cannot enumerate: probing it returns nothing distinguishable from
  a miss, asserted in tests.
- Favourites are used: a majority of active users of the pane favourite at least
  one person, or the feature is not earning its complexity.
- A directory change made upstream is visible on a device within ~60 s with the app
  open, and immediately on foreground — measured on a real device, not assumed.
- A withdrawn member's number is gone from a device that had already synced it,
  verified by a sync → withdraw → re-sync test, and their name still resolves with a
  visible status.
- Complete purge confirmed after the event: server storage empty, client expiry
  verified on a sample of devices.

## 10. Rollout / Task Breakdown

Sequence authorization first — it is the part that must not be wrong — then the
manifest, then the offline datasets, then the pane. Ship behind a runtime flag in
`config/runtime.ts`. Start with **crew-only visibility of the pane** even if a wider
matrix is approved: it exercises the whole path with the smallest audience, and
widening later is easy while narrowing after the fact is not.

Proposed tasks for `roadmap/tasks/open/`:

- [ ] Task: `mayView(viewer, subject)` authorization + exhaustive role-pair tests
      (every role both directions; spejder never a viewer, never listed, reachable by
      crew only through the patrol lookup)
- [ ] Task: server-supplied grouping model — group id + label, own-group flag, shaped
      so subsections can add a tier without a client rewrite
- [ ] Task: `GET /api/contacts/manifest` with server-side grouping, own-group flag,
      etag/delta support
- [ ] Task: `GET /api/contacts/{personId}/photo?size=thumb`, indistinguishable
      403/404, reusing `portrait.go`
- [ ] Task: hide the `contacts` destination for `spejder` (nav `roles` + router
      guard + test)
- [ ] Task: register the directory as PRD 009 datasets (index + images, TTL, priority)
      — explicitly excluding the patrol lookup
- [ ] Task: ContactsView — grouped accordion, own group expanded by default
- [ ] Task: contact row component — avatar left, name + grey group line, phone right,
      favourite toggle
- [ ] Task: person profile route (`/contacts/:personId`) — large avatar, allow-listed
      fields (no address, no guardian number), offline, role-guarded on deep link
- [ ] Task: **decide how member status arrives** — lift `hq`'s lifecycle projection to
      `shared-go`, or a narrow in-race flag (§8); blocks the manifest
- [ ] Task: `GET /api/contacts/version` — cheap monotonic version per permitted set,
      `ETag`, projection-read only
- [ ] Task: freshness loop — check on foreground, on reconnect, and every ~60 s while
      visible; runtime-configurable interval; no polling when hidden
- [ ] Task: raise the invalidation/freshness contract against PRD 009 (during-event
      freshness, not just pre-event readiness)
- [ ] Task: withdrawn-member handling — status marking in list and profile, phone
      number purged, removal propagated through the sync delta (sync → withdraw →
      re-sync test)
- [ ] Task: assert `phoneParent` is absent from every contacts response (`.rules`
      invariant) — test-only, but the tripwire for the whole rule
- [ ] Task: crew placement map — section slug `bandit` → bandit population,
      `goeglerledelse` → gøgler population, kept orthogonal to the role map and
      logging unmapped slugs
- [ ] Task: device-local favourites — ids only, ordering, re-validation against the
      manifest
- [ ] Task: local search across the synced index (name, group, arm number) —
      spejdere excluded from the index it searches
- [ ] Task: crew patrol lookup — `GET /api/contacts/patrols/{number}`, exact match,
      indistinguishable miss, no enumeration, `no-store`, server-side audit log
- [ ] Task: patrol-lookup UI — live fetch, status + phone, clear no-connectivity state,
      nothing persisted
- [ ] Task: assert nothing from a patrol lookup is cached (SW routes, storage, headers)
- [ ] Task: names-only degradation + missing-portrait placeholder
- [ ] Task: night-legible profile surface (large avatar) reused by the patrol lookup
- [ ] Task: *(§11.7)* server-side audit log of patrol lookups
- [ ] Task: generate the `command` shadcn-vue primitive *(only if search uses it —
      `avatar`, `accordion`, `input`, `dialog`, `drawer` already exist)*
- [ ] Task: offline test protocol on real iOS + Android devices, radio off — directory
      usable, patrol lookup failing clearly
- [ ] Task: post-event purge verification (server purge exists; verify client
      expiry)

## 11. Open Questions

1. ~~**What do crew see?**~~ *Answered 2026-08-31: **a crew / bandit / gøgler
   directory**, plus a crew-only patrol lookup — "ok to have samarit looking up a
   patrol one by one".* This is option (c) of the three that were on the table: the
   directory is adults only, and a spejder's face is reachable one patrol at a time
   with no browsable index. Encoded in the §6 matrix.
2. ~~**What is `lok`?**~~ *Answered 2026-08-31: **group banditter by klan for now.***
   Lok is a grouping of klaner being migrated upstream into subsections, and is not
   modelled here. The remaining engineering note is in §8: make grouping a
   server-supplied group id + label so subsections add a tier rather than force a
   rewrite.
3. ~~**"Crew marked as bandit"**, and who counts as crew for the lookup?~~ *Answered
   2026-08-31.* **All crew may use the patrol lookup**, generic `crew` fallback
   included. Crew placement comes from the section slug: **`bandit` → bandit
   population, `goeglerledelse` → gøgler population**, while the app role stays
   `crew`. Magic slug matching for now, migrating to a **section flag** later. The
   engineering consequences — placement as a second orthogonal map, and the warning
   not to "fix" `classify.go` — are in §8.
4. ~~**Which fields does the profile page show?**~~ *Answered 2026-08-31.* The profile
   shows **avatar, name, group/team and phone number** (plus function/section for
   crew), and **excludes postal address and guardian number**.

   The guardian half of that answer is broader than this PRD and has been promoted to
   `.rules`: **guardian numbers never enter the PWA at all**, the sole exception being
   a user approving *their own* guardian's number during onboarding or on their
   profile. The BFF must project the field out rather than trusting the client not to
   render it.

   **Crew numbers being cached on crew devices is accepted** — before this app the same
   numbers circulated on printed lists, so a synced directory is a reduction in
   exposure, not an increase.

   The residual question about a spejder's own number is **answered**: the patrol
   lookup returns status and phone numbers (2026-08-31). It is defensible here in a way
   it would not have been in a cached payload — the number is fetched for one patrol, by
   one crew member, on a logged request, and is never stored.
5. ~~**Are favourites device-local or server-side?**~~ *Answered 2026-08-31:
   **device-local.*** No endpoint, no server-side record of who is interested in whom;
   they do not survive a reinstall, which is accepted.
4. **Do rows expose a phone number?** It is a contacts pane, so the expectation is
   there, but a number is a separate disclosure from a face and PRD 006 is
   deliberately careful about showing one person another's details. Face-only,
   face + number, or number only for crew?
5. **Are favourites device-local or server-side?** Local needs no endpoint and no
   new stored personal data; server-side survives reinstall and a second device,
   but means we hold "who is interested in whom", which is more sensitive than it
   first sounds.
6. ~~**When a member leaves the race**, do they vanish from devices?~~ *Answered
   2026-08-31: **no — purge the number, keep the name.*** On withdrawal
   (`released` / `reunited`) the phone number is purged, while the name stays visible
   until the end of the race with a **clear status marking**. Rationale: an
   identification need outlives a withdrawal, but a reason to call does not.

   This has a real dependency, and it is the last piece of engineering work this PRD
   discovers rather than assumes — see §8 "Member status". This repo does **not**
   project the member lifecycle, on purpose.

   One residual, settled at approval (2026-08-31): a withdrawn member **keeps their
   portrait** alongside the name and status marking. A name alone rarely settles "is
   this the person in front of me", which is the whole reason the record is retained.
7. ~~**Is a view audit log proportionate?**~~ *Answered by construction 2026-08-31.*
   With the lookup online, every spejder view is already a server request, so logging is
   a log line on a handler rather than a client queue and an ingestion endpoint. Log the
   patrol lookups; do not log directory views, which are adults looking at adults'
   faces and numbers that used to be printed.
8. **Post-event purge on a dormant device** — a phone that never reopens the app keeps
   its cached directory until the OS evicts it. Is a baked-in expiry timestamp
   sufficient, given the service worker may never run again? (Shared with PRD 009.)
   Smaller than it was: only adults' records are cached now.
9. **Does anything about this change PRD 005's portrait step?** Less pressing now that
   §11.1 is answered — a spejder's portrait does get used, by crew, through the patrol
   lookup. But the consent copy should describe *that* audience honestly ("crew can
   look up your patrol to find you") rather than implying a mutual directory the
   spejder is part of. Worth a line back to PRD 005 and PRD 003.
10. **Do banditter or gøglere need any patrol-adjacent lookup?** Assumed no — the
    lookup exists for a safety task, not a game one, and giving banditter any path to
    spejder faces would undo the game-integrity property. Confirming it stays
    crew-only.
