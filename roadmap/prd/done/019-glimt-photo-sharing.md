# PRD 019 — Glimt: sharing a moment inside the event

**Status:** done
**Author:** agent session (Zed)
**Created:** 2026-09-16
**Last updated:** 2026-09-17 (post-ship: `group` scope relabelled after it was found to understate reach, and the composer's Team-reach line moved to PrivacyView only — both in §11; video split out into PRD 020; §8 endpoint table corrected: no `/glimt/version`, freshness is a `/api/sync` key)
**Approved:** 2026-09-17
**Shipped:** 2026-09-17
<!-- 2026-09-16: maintainer decided §11 Q1 (publish-and-hide-on-report), Q2 (no consent gate,
close monitoring instead), Q3 (video after images, 30 s target) and authorship (a glimt belongs
to a person, but is attributed to their hold). 2026-09-16 (later): public feed is served by this
service on hej.nathejk.dk, moderation belongs to the Team section (slug `team`), public
attribution is hold number + name + group, and the post-race browse is a primary use.
2026-09-17: retention is configurable via env var, 0 disables (dev/testing). Recorded in §0.
Approved 2026-09-17; tasks 299–325 created.
2026-09-17 (later): video split out into PRD 020 (§11); shipped after the iOS field pass — all five
offline scenarios and the save-to-Photos path verified on an installed iPhone (tasks 318, 325). The
Android/Chrome repeat is task 329. -->
**Target users:** participants — **spejdere (patruljer) first**, then bandit, then crew roles; parents last, as read-only viewers

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 0. Decisions taken before approval (2026-09-16)

Four questions were open when this was first drafted. The maintainer has answered them, and
the rest of the document is written on top of these answers rather than around them:

| Question | Decision |
|---|---|
| Moderation of the `public` scope | **Publish immediately, hide on report**, backed by *close monitoring* by the Team section while the feature is young. No pre-publication approval queue. |
| Guardian consent for `public` | **Not required for now.** We rely on the same Team-section monitoring rather than a recorded consent. |
| Video | **Split out into PRD 020.** Decided here as "yes, after images, 30 s cap", and the reasoning is preserved in §8 — but it is gated on a metadata claim that can only be verified on real devices, and that gate is too large an unknown to hold this PRD open around. See PRD 020. |
| Authorship | **A glimt belongs to a person** — they own it and they alone can delete it — **but the person is never disclosed. It is attributed to their hold.** |
| Where the public feed lives | **In this service, on `hej.nathejk.dk`.** Not a separate site and not a build-time export to PRD 013 — the BFF serves it directly. |
| Who moderates | **Whoever has the Team section assigned** (slug `team`) — the organizers and the people responsible. The Team section can **view every glimt at every scope** and hide anything unwanted. |
| Public attribution | **Hold number + hold name + group** — e.g. "Patrulje 42 · Ørnene · Spejder". Blunter anonymity was considered and rejected: a parent needs to recognise their child's patrulje for the public feed to be worth anything. |
| Post-race browsing | **A primary use, not an afterthought.** Spejdere will spend hours going through the race's photos afterwards. See §0a. |

### 0b. A naming collision worth settling now: *hold* vs *Team*

The moderating section is called **Team** (slug `team`). The word "team" was already taken in
this feature — it is what `person.teamId` / `teamNumber` / `teamName` call a participant's
patrulje or klan — so leaving both in play would produce sentences like "the team can see every
team's glimt", and eventually a `mayModerate` that checks the wrong column. This PRD therefore
fixes the vocabulary:

| Term | Means | Where it appears |
|---|---|---|
| **hold** | the participant unit — patrulje, klan, or crew section | prose, UI copy, routes (`/glimt/hold/:number`), API paths |
| **Team** (capitalised, "the Team section") | the organizers and the people responsible; slug `team` | moderation only |
| `teamNumber`, `teamName` | the *hold's* number and name | Go and SQL identifiers only, because they mirror `person` |

So the DB columns keep their names — renaming them to diverge from `person` would be worse than
the ambiguity — but everything a human reads says *hold* for a patrulje and *Team section* for
the organizers. **Bare "team" in new code or copy is a bug**; it should be one or the other.
The UI never shows the English word at all: it shows "Patrulje 42" or "Klan 7".

The first two are choices to accept risk in exchange for shipping something, and they are
only safe while the monitoring is real. §9 therefore treats time-to-hidden as a headline
metric, and §11 keeps one question open about who is actually watching.

### 0a. The post-race browse changes the shape of this feature

"After the race all the spejdere will spend hours going through photos from the race"
(maintainer, 2026-09-16). That single sentence moves Glimt from *a way to post* to *a way to
look*, and four things follow that a posting-first design would have got wrong:

1. **The chronological feed is the wrong primary surface afterwards.** During the race, newest
   first is exactly right. Afterwards, a spejder wants *their patrulje's* photos, then their
   friends' patrulje, then everything. Browsing by hold is a first-class view, not a filter
   hidden in a menu (§7).
2. **Retention must comfortably outlive the race**, because the peak use happens after it.
   A purge window measured against the event's end has to leave room for the hours-long
   browse and for showing parents later (§11 Q4).
3. **The load is spiky and it arrives at the finish line**, where a thousand people are on the
   same congested mobile network at the same time, all pulling full-size media. This is the
   performance case to design for — not the trickle of uploads during the race (§8).
4. **They will want to keep the photos they find.** "Going through photos" ends in "I want
   that one." Saving to the camera roll is a requirement (§6), and yes, it lets someone put a
   photo on Instagram afterwards — that is fine. The goal was never to trap the photos; it was
   to stop those platforms being the *only* place to put them.

---

## 1. Summary

**Glimt** lets a participant post a small collection of photos or videos with a short
caption, and choose who sees it: their own group (spejder / bandit / crew), everyone at
Nathejk, or publicly so parents can follow along. Everybody can also see the glimt shared
with them, in one feed.

A glimt is **posted by a person but signed by their hold** — "Patrulje 42", not a name. The
person is recorded so they can delete it and so it can be moderated; they are simply never
shown. That single choice removes most of the individual exposure this feature would
otherwise create, and it matches how participants experience the event anyway: they are out
there as a patrulje.

A single post is **et glimt** (plural: *glimt*) — that is the word this PRD uses, and it
replaces "story" and "reel" everywhere. It is not a story: it does not expire after 24
hours, it is not a vertical auto-advancing tape, and there is no streak to keep alive. It
is one moment, deliberately shared, that stays for the event — and, crucially, well past it:
the hours spejdere spend going through the race's photos afterwards are as much the point as
the posting itself (§0a).

## 2. Problem & Motivation

- **What problem does this solve?** Participants photograph the event constantly and the
  pictures go to Snapchat and Instagram, because that is the only place to put them. Those
  platforms are the wrong destination for three reasons that all belong to us rather than to
  them: they are outside the event, so a photo taken with a patrol lands in front of an
  audience that has nothing to do with Nathejk; they are age-gated at 13 and many spejdere
  are younger, so we are effectively routing children to an account they should not have;
  and the sharing is one-way — a parent who is not on Snapchat sees nothing, and neither do
  the other patrols. We currently make no alternative available, so the behaviour is not a
  user choice, it is an absence on our side.
- **Why now?** The pieces this needs already exist and were built for the portrait feature
  (PRD 003): a content-addressed blob store, an upload endpoint with size limits and rate
  limits, an imaging pipeline that decodes, re-orients and **strips EXIF/GPS**, and a camera
  capture component with a file-input fallback. Glimt is mostly composition of things that
  already work, plus a feed and an audience model.
- **Why spejdere first?** They are the largest group, the youngest, the ones with the
  strongest pull toward Snapchat, and — unlike every other role — they have **no contacts
  pane** and almost nothing in the app that is theirs. Glimt would be the first feature
  built for them rather than around them.
- **Evidence.** Maintainer direction, 2026-09-16: *"participants want to share photos, and
  often these photos end up on snapchat or instagram, we don't want to push users towards
  these platforms."* Follow-up direction the same day: *"after race all the spejdere will
  spend hours going through photos from the race"* — the basis for §0a. Related: PRD 013,
  which establishes the login-free surface for people outside the app; PRD 011, which wants a
  post-race artefact for a hold.

## 3. Goals

- A spejder can share a moment from the event **without leaving the app** and without an
  account on a commercial platform.
- Sharing has an **explicit, understood audience**. The person posting knows who will see
  it before they post, not after.
- Everybody sees what has been shared with them, in one place, and it is worth opening more
  than once.
- **After the race, a spejder can find their own patrulje's photos in seconds** and browse
  the rest for as long as they like — and keep the ones they want (§0a).
- Parents can follow the event without installing anything or logging in, and can tell which
  patrulje they are looking at.
- The photos are **less exposed than the status quo**, not more: no location metadata, no
  indefinite retention by default, no third-party analytics, and a way to take something down.
- Posting works from a phone in a field with bad signal — the app must not lose a photo it
  accepted.

## 4. Non-Goals

- **Not a social network.** No follows, no friend graph, no direct messages, no profiles
  beyond the member directory we already have. Adding one would recreate the problem we are
  solving.
- **No engagement mechanics.** No likes, no view counts, no streaks, no ranking, no
  "trending". The default order is chronological. (Reactions: see §11.)
- **No comments** in the first release. Free-text replies between minors is a moderation
  product, not a feature, and it is not what was asked for.
- **No editing after posting.** A glimt can be deleted, not rewritten.
- **No expiry-after-24-hours mechanic.** Retention is a configured window, stated up front
  (§6), not a countdown used as pressure to post.
- **Not a photo backup or archive.** Media is retained for a stated window and then purged;
  it is not the participant's copy of their pictures. Saving individual photos to the camera
  roll (§6) is how someone keeps what matters to them.
- **No cross-year feed.** A glimt belongs to one event year.
- **No push notification for every glimt.** (Digest: see §11.)

## 5. User Stories & Scenarios

- As a **spejder**, I want to post two photos of my patrol at a post so that the other
  patrols and my parents can see what we are doing, without opening Snapchat.
- As a **spejder**, I want to share something only with other spejdere, so that I can post
  something for participants without it being on a public page.
- As a **bandit**, I want to post a short video from a chase so that the event has a record
  of it.
- As a **crew member**, I want to share a moment inside crew only, because it is an in-joke
  and not for participants.
- As a **parent**, I want to open a public page on my laptop and see what is happening
  tonight, without installing an app or making an account — **and know which patrulje I am
  looking at**, so I can find my own child's.
- As a **spejder after the race**, I want to sit with my patrulje and go through everything
  that was posted — ours first, then everyone else's — and save the good ones to my phone.
- As a member of the **Team section**, I want to see everything that has been posted, at any scope, and hide anything
  that should not be up.
- As a **participant**, I want to remove a glimt I regret, and have it actually gone.
- As **anyone in a picture**, I want a way to say "take that down" that does not require
  knowing who posted it.

### Primary happy path

1. Spejder taps **Glimt** in the nav and lands on the feed of glimt shared with them.
2. Taps the compose button. The camera opens (same capture path as the portrait, PRD 003),
   or they pick from the library.
3. They add 1–N items (photos, and video where enabled), see them as a strip, and can remove
   any before posting.
4. They type a short caption (optional, hard-capped — see §6).
5. They pick an audience from three clearly labelled options, **defaulting to their own
   group** — the narrowest, never the widest.
6. Tap **Del** (share). The glimt appears immediately in their own feed marked *sender…*
   while media uploads; it becomes a normal entry when the last item is stored.
7. Others see it on their next freshness check (foreground, interval, or reconnect).

### Edge cases and error scenarios

| Situation | Behaviour |
|---|---|
| Offline when posting | Accepted into a local outbox and uploaded on reconnect. The entry is visibly *venter på nettet*, never silently dropped. |
| App closed mid-upload | Upload resumes on next foreground. iOS does not run a backgrounded web app (PRD 002 measurement: 2% background coverage), so there is **no** background upload — the UI must never imply otherwise. |
| File too large | Rejected client-side before upload with a specific message, and by the server at the byte limit (413). |
| Not a decodable image/video | Rejected at the server after decode-validation; the item is dropped from the draft, the rest of the glimt survives. |
| Rate limit hit | 429 with a plain-language message and the time it clears. |
| Event stream unavailable | 503 and the glimt stays in the outbox. Never a fake success (matches `internal/commands`). |
| Role changes / profile switch | The feed is profile-scoped storage (PRD 012) and is dropped on switch — a bandit must not see the crew feed cached on the same device. |
| Author deletes a glimt | Metadata tombstoned, media purged, and it disappears from every cached feed on the next refresh. |
| Someone reports a glimt | Immediately hidden from the public scope, then reviewed. Hiding is cheap and reversible; leaving something up is not. |
| A glimt is reported after parents have already seen it | Accepted consequence of publish-and-hide-on-report (§0). It is why time-to-hidden is a headline metric and why the report control is on every card rather than buried. |
| The whole race arrives at the finish line at once | The post-race browse is the load peak (§0a.3) on the worst network of the weekend. Thumbnails first, full media on demand, aggressive caching — and the feed must stay usable, not just not crash. |
| A Team-section member loses the assignment | Moderation power must be gone on their next request. This is why it is looked up rather than carried in the session cookie (§8). |
| Author leaves / profile is switched | Ownership is the `personId`, not the session, so deletion still works after a profile switch and the glimt does not become unowned. |

## 6. Requirements

### Functional

- [x] A participant can create a glimt with **1–10 media items** and an optional caption of
      at most **280 characters**.
- [x] Media items may be **images** (JPEG/PNG/HEIC-decoded). **Video moved to PRD 020** — the
      constraints and the reasoning that produced them are still in §8, because they were decided
      here.
- [x] A glimt is **owned by the author's `personId`** — only they can delete it — and
      **attributed in every UI to their hold**, never to them. Attribution carries **number,
      name and group**:
      | role | attribution shown |
      |---|---|
      | `spejder` | "Patrulje 42 · Ørnene · Spejder" |
      | `bandit` | "Klan 7 · <klan name> · Bandit" |
      | crew roles | section name + "Crew" (crew has no hold number) |
- [x] The **same attribution is shown at every scope**, including `public`. A parent must be
      able to recognise their own child's patrulje, which is the only thing that makes the
      public feed worth publishing.
- [x] **The author's name, portrait and person id are projected out of every response**,
      including the author's own feed — the card says "Din patrulje" plus a *Slet* action, so
      ownership is visible without a name ever being on screen. The client cannot render what
      it never receives.
- [x] Hold attribution is **captured at creation time**, not resolved at read time, so a
      glimt keeps saying what it said even if a hold is renamed or a member moves. Number,
      name and group are all frozen together.
- [x] **Browse by hold.** In addition to the chronological feed, a member can see all glimt
      from one hold, and reach **their own hold's collection in one tap**. This is the primary
      post-race surface (§0a.1).
- [x] **Save to device.** A member can save an individual photo or video they are viewing to
      their camera roll / downloads.
- [x] Each glimt has exactly one **audience** chosen at creation from:
      | value | who sees it | label (da) |
      |---|---|---|
      | `group` | members whose role maps to the author's group (spejder / bandit / crew) | "Min gruppe" |
      | `nathejk` | every authenticated member, any role | "Alle på Nathejk" |
      | `public` | anyone, no login, via `/offentligt/glimt` on `hej.nathejk.dk` | "Offentligt — også forældre" |
- [x] The audience selector **defaults to `group`**. Widening is an explicit act.
- [x] Audience is **immutable after posting**. Changing it would retroactively expose an
      already-shared photo; deleting and reposting is the honest path.
- [x] Role → group mapping: `spejder` → `spejder`; `bandit` → `bandit`; all of
      `postmandskab`, `guide`, `samarit`, `gøgler`, `crew` → `crew`. This reuses
      `isCrewRole` semantics already in `vue/src/config/roles.ts` and
      `go/internal/users/`.
- [x] Every authenticated member gets a **single chronological feed** of glimt visible to
      them: their group's, all-Nathejk, and public — plus their own, always.
- [x] A member can **delete their own glimt**, which purges the media.
- [x] Any authenticated member can **report** a glimt. Reporting hides it from `public`
      immediately, before any human looks at it (§0).
- [x] **Members with the Team section assigned can see every glimt at every scope**, including
      `group`-scoped glimt they are not in the audience for, and can **hide or unhide** any of
      them. This is the moderation capability §0 depends on.
- [x] The Team section's reach must be **disclosed, not discovered**: the composer and `PrivacyView` state
      that the Team section can see everything posted, whatever the audience. A participant choosing "Min
      gruppe" is entitled to know that is not the same as "only my gruppe".
- [x] Hiding is **reversible and recorded**, not a delete: hidden glimt stay visible to the Team section and
      to their author, and only the author can actually destroy media.
- [x] Moderation authority is derived from the **current** Team section assignment on every
      request, so revoking the assignment revokes the power (§8).
- [x] Reports are surfaced to the **Team section** with the owning `personId` — the one place the person
      behind a glimt is disclosed, because moderation cannot work against an anonymous author.
- [x] **All uploaded media has EXIF/GPS stripped** before storage, reusing
      `go/internal/imaging` and the `normalizePortrait` approach.
- [x] Media is stored in **variants**: a feed-sized version and a thumbnail; the original is
      not served.
- [x] Feed freshness follows the existing shared loop (`useFreshnessLoop`) with a cheap
      version probe, not a bespoke poller.
- [x] Retention: media and metadata are **purged after a configurable window that comfortably
      outlives the race** (§0a.2), on the same mechanism as the portrait purge
      (`portraitpurge.go`).
- [x] The window is set by **environment variable**, and **`0` disables the purge entirely** —
      required for dev and testing, where a fixture posted last month must still be there
      tomorrow. Two knobs, because the public feed should be able to close before the internal
      one:
      | var | controls | default |
      |---|---|---|
      | `GLIMT_RETENTION` | how long a glimt is kept after creation | see §11 Q1 |
      | `GLIMT_PUBLIC_RETENTION` | how long a glimt stays on the public feed | see §11 Q1 |
- [x] Whatever the configured window is, the **UI must state the real number**, not a
      hard-coded sentence. A composer that says "90 dage" while the deployment is set to 30 is
      worse than saying nothing, so the value is served to the client (`/api/config`) rather
      than written into copy.
- [x] Retention is measured **from creation**, not from a configured event end date — the same
      simplification `portraitRetention` already makes, and it fails in the safe direction.
- [x] A disabled purge must be **visible, not silent**: log it at startup, exactly as
      `portraitpurge.go` already does ("portrait retention disabled").
- [x] The privacy page (`/privatliv`) documents Glimt: what is stored, who can see it —
      **including the Team section** — how long it lives, and how to get something removed.
- [x] **No guardian phone number, and no phone number at all, appears anywhere in Glimt.**
      Nor does any personal name, portrait or arm number — attribution is the hold
      (`.rules`, and §0).
- [x] The public surface shows **the hold attribution (number, name, group) and nothing else**
      about who posted — no personal name, no portrait, no arm number.

### Non-Functional

- **Privacy is the load-bearing requirement.** This feature invites minors to publish
  pictures of other minors. Three rules follow and are not negotiable per-feature: no
  location data survives upload; no personal name or portrait is attached to a glimt; and
  anything reported is hidden from `public` before it is judged.
- **The public scope is unmoderated on the way in** (§0), so the safety of this feature rests
  on how fast a report acts and how closely the feed is watched while it is young. Both are
  requirements, not aspirations: the report control must be reachable in one tap from every
  card, and hiding must not wait on a human.
- **Consent by design.** A photo contains people who did not choose to be in it. The
  composer must say so — one line, before posting — and the report path must work without
  knowing who posted (which, by §0, nobody outside moderation does).
- **Accessibility:** the feed is a list of articles, not a gesture-only tape. Captions are
  real text. Video is muted by default and never autoplays with sound.
- **i18n:** Danish UI copy, consistent with the rest of the app.
- **Performance:** the design target is **the post-race browse, not the in-race trickle**
  (§0a.3): a thousand people on a congested network at the finish line, each paging through
  thousands of items. That means thumbnails in the list and full media only on demand,
  pagination, lazy loading, long-lived immutable cache headers on content-addressed media, and
  a feed that stays usable rather than merely not crashing. Respect the origin storage budget in `vue/src/config/offline.ts` — Glimt gets
  its own dataset entry and its own eviction priority, below `tiles` in value and above
  nothing.
- **Browser baseline** unchanged: iOS Safari 16.4+ / Chrome 111+.
- **Bandwidth honesty:** participants are on mobile data in a field. Uploads are compressed
  client-side before leaving the device; video has a hard duration cap.

## 7. UX / UI Notes

**New route and destination.** `{ name: 'glimt', path: '/glimt', label: 'Glimt', icon: Camera }`
in `vue/src/config/navigation.ts`, available to all roles, with the view loader in
`vue/src/router/index.ts`. `BottomNav` has `MAX_SLOTS = 5` and already overflows into
`MoreMenu` for most roles — Glimt is a primary destination for spejdere and must land in the
bottom bar for them, which means the ordering (and possibly what gets demoted to "Mere")
needs a decision. Flagged in §11.

**Feed (`GlimtView.vue`).** Chronological, newest first. Each card: **the hold attribution**
("Patrulje 42 · Ørnene · Spejder", "Din patrulje" on your own) — no name, no portrait, so there
is no avatar slot in this design and no reason to fetch one — relative time, an audience chip
("Min gruppe" / "Alle" / "Offentligt"), the media (thumbnail-first; single item fills the card,
multiple items are a swipeable strip with dots), the caption, and an overflow menu with *Slet*
for your own and *Anmeld* for everyone else's. The attribution is **tappable and goes to that
hold's collection** — that is the main way anyone gets into the post-race browse. `Anmeld`
sits in the same one-tap overflow on every card, not behind a long-press — with an unmoderated
public scope, reporting is the safety mechanism (§0) and must be as easy as posting. Built on
the existing `card` and `dropdown-menu` shadcn primitives; the swipe strip is the one component
with no shadcn equivalent (`carousel` is not in `vue/src/components/ui/` — add it from the
catalogue rather than hand-rolling, per `.rules`).

**Hold collection (`GlimtHoldView.vue`, `/glimt/hold/:number`).** A dense thumbnail grid,
oldest-first (a race reads forward in time), with the hold attribution as the heading. The
feed view carries a persistent shortcut to **your own** hold's collection. This is the surface
that matters after the race (§0a.1), and it is the one to build for a spejder sitting on a bus
with a patchy connection: grid of thumbnails, tap to open full media, nothing loaded that is
not on screen.

**Saving.** In the full-screen viewer, a *Gem* action — `showSaveFilePicker` is not on the iOS
baseline, so this is a plain anchor `download` on the media URL, which on iOS Safari 16.4 opens
the share sheet and lets "Save to Photos" through. Worth verifying on device rather than
assuming; a long-press on the image is the fallback iOS users already know.

**Composer (`GlimtComposer.vue`).** A `drawer` (existing primitive), not a route, so the
feed stays behind it. Order: media strip → caption `input` → audience choice → share
`button`. Audience is a **three-option radio-style list with one line of consequence each**,
not a dropdown — the choice is the most important thing on the screen and must not be a
default someone taps past. The `public` option carries the plainest consequence line we can
write — something close to *"Alle kan se det, også folk uden for Nathejk"* — because with no
approval queue in front of it (§0) this sentence *is* the gate. Above the share button, one
quiet line: *"Andre er også med på billedet — spørg dem først."* The composer also states
that the glimt will be signed with the patrulje and not their name — that is reassurance
worth spending a line on.

**Capture.** Reuse the portrait path: `getUserMedia` with the `<input type="file"
accept="image/*,video/*" capture>` fallback and the secure-context probe already in
`vue/src/components/profile/PhotoCapture.vue` / `usePortraitCapture.ts`. Multi-select from
the library must work — that is how most glimt will actually be made.

**Viewer.** Tapping media opens a full-screen `dialog` with swipe between items. No
auto-advance.

**Empty state.** For a spejder opening Glimt before anything exists, the empty state is the
invitation to post — this is the feature's front door for its primary audience.

**Public surface — served by this service on `hej.nathejk.dk`** (§0). A server-rendered page at
`/offentligt/glimt`: a plain thumbnail grid with the hold attribution under each entry, no
login, no bundle, no client-side state, working without JavaScript. It shares the host with the
app, which has two consequences worth designing for rather than discovering (§8): the service
worker's navigation fallback would otherwise swallow the URL, and the session cookie will be
sent by any logged-in member's browser — the handler must ignore it, so that what a parent sees
and what a member sees on that URL are provably the same page. It needs its own **report
link**: a parent who spots a problem is precisely the person we want to hear from, and they have
no session to report with.

**Team-section moderation view (`/glimt/moderation`).** Not a separate tool — Team-section
members are in the app, on their phones, at the event. The same feed component with three
differences: every scope is included,
each card shows its audience and report count, and the overflow carries *Skjul* / *Vis igen*.
Default sort puts reported-and-not-yet-reviewed first. Visible only when the Team section is
assigned.

**PRD 013 relationship.** 013 owns the anonymous website for people who cannot use the app;
Glimt's public page is a sibling page on the same host, not a section inside 013's static
HTML. They should link to each other, but this PRD no longer *depends* on 013 shipping first —
the coupling that made phase 3 blocked is gone (§10).

**Headlines** use `font-nathejk`; **icons** are Lucide (`Camera`, `Users`, `Globe`,
`Trash2`, `Flag`, `Plus`, `Download`, `EyeOff`). No font families in components (`.rules`).

## 8. Technical Considerations

### Frontend (Vue 3 / TS)

- `src/views/GlimtView.vue` — feed; `src/views/GlimtHoldView.vue` — hold collection;
  `src/views/GlimtModerationView.vue` — Team section; `src/components/glimt/GlimtCard.vue`,
  `GlimtComposer.vue`, `GlimtMediaStrip.vue`, `GlimtViewer.vue`, `GlimtGrid.vue`,
  `AudienceChoice.vue`.
- `src/stores/glimt.store.ts` — feed state, following the **contacts store pattern**
  verbatim: schema-versioned profile-scoped key, runtime type guards on read, server-issued
  `expiresAt`, a storage seam for tests, `refreshIfStale()` on a cheap `/version` probe, and
  **replace-never-merge** on fetch so a purge or a takedown is not decorative.
- Feed **metadata** in `local-storage` (small, and must be dropped on profile switch);
  feed **media** in the Cache API via a new Workbox `runtimeCaching` route, matching how
  portraits are cached. Note the `generateSW` stringification trap: any identifier used
  inside a Workbox config function must be a literal.
- **Outbox** in IndexedDB (`src/helpers/glimtOutbox.ts`), modelled on `trackDb.ts` — raw
  IDB, no wrapper library, holding draft metadata plus the media blobs until each is
  uploaded. Drained on foreground and on `online`, never by Background Sync (unavailable on
  iOS, and the app does not run backgrounded).
- Client-side downscale/re-encode before upload (canvas for images) to keep uploads within
  the byte limit on mobile data.
- New entry in `src/config/offline.ts`: `{ id: 'glimt', kind: 'cache-api', sensitive: true }`
  with a budget, placed in the priority array so it is evicted before `track` (unrecoverable)
  and `directory`, and the outbox is **never** evicted while it holds unsent media.
- **Thumbnails and full media are two Workbox routes with different budgets.** The post-race
  browse (§0a.3) will pull thousands of thumbnails and a handful of full images; caching them
  under one budget means the thumbnails that make the grid usable get evicted by the full-size
  media that does not. Thumbnails are small, numerous and worth keeping; full media is not.
- The public page is **not** part of the SPA and must be added to `navigateFallbackDenylist` in
  `vue/vite.config.ts`, exactly as `/desktop\.html$` already is. Without it, Workbox's
  `generateSW` navigation fallback serves the app shell to an installed member who follows a
  public link — a silent, confusing failure.

### BFF (Go)

- `go/cmd/api/glimt.go` — handlers; `glimtmedia.go` — upload/serve; `glimtmoderation.go` —
  Team-section queue and hide/unhide; `glimtpublic.go` — the server-rendered public page;
  `glimtpurge.go` — retention, mirroring `portraitpurge.go`.
- `go/nathejk/table/glimt/` — projection package in the house shape (`table.go` with embedded
  `table.sql` + `cqrs.EnsureColumn`, `consumer.go`, `querier.go`). Two tables: `glimt`
  (`glimtId`, `year`, `authorPersonId`, `authorGroup`, `attribution`, `audience`, `caption`,
  `createdAt`, `mediaCount`, `hiddenAt`, `hiddenBy`, `reportCount`, `deleted`) and
  `glimt_media` (`glimtId`, `ordinal`, `blobRef`, `thumbRef`, `kind`, `width`, `height`,
  `durationMs`), plus `glimt_report` (`glimtId`, `reporterPersonId`, `reason`, `createdAt`).
- `attribution` is stored as its parts — `teamNumber`, `teamName`, `authorGroup` — frozen at
  creation, not as a pre-formatted string, so the public page and the app can present them
  differently without re-deriving anything. `teamNumber` also carries the index the hold
  collection query needs (`year_teamnumber` already exists on `person`).
- **`authorPersonId` is an ownership and moderation column, not a display column.** It is
  read to authorize `DELETE` and to answer a report; `attribution` (the frozen hold fields)
  is the only author-ish field any response may carry. Project the person id out in the
  response type rather than trusting a handler to omit it — same discipline as `phoneParent`
  (`.rules`).
- **Media reuses `go/internal/blob`** — content-addressed, `FileStore`, `sha256` `Ref` with
  the existing path-traversal guard. Images reuse `go/internal/imaging` for
  decode-validate → orient → strip metadata → re-encode + variants.
- **Team-section moderation authority cannot come from the session, and this is the one genuinely new
  authorization problem in the PRD.** `session.Session` carries only `UserID`, `Role` and
  `ExpiresAt`, and it is a stateless signed cookie with no server-side store — so a
  `section: "team"` claim baked into it would keep working after the assignment was revoked, for
  as long as the cookie lived, with no way to invalidate it. Every moderation handler must instead
  **look up the caller's current `sectionSlug` from the `person` table** (the column already
  exists) and check it against a `SectionTeam = "team"` constant. That is one indexed read per
  moderation request, which is the right price. Two supporting notes: the slug must be that
  named constant, declared next to `crewFunctionBySlug` in
  `nathejk/table/person/classify.go` rather than written as a string literal in a handler,
  since section slugs are organizer-authored and validated by nothing — and `"team"` is a
  particularly easy literal to typo or to confuse with the `team*` columns three lines away
  (§0b); and the predicate belongs in `go/internal/users/` beside the visibility rules so
  `mayModerate` has exactly one definition.
- **The Team section can read every scope, so the visibility predicate has an override, not a bypass.** The
  override lives inside the single predicate — if a handler can reach media by skipping the
  check when the caller is in the Team section, then the check is no longer the only thing standing between a
  `group`-scoped photo and the world. Same predicate, one extra branch, tested both ways.
- **The public page and media handlers must ignore the session cookie entirely.** They are on
  the same host as the app, so a logged-in member's browser will send `hej_session` to them.
  If those handlers ever read it, the public page silently becomes a different page for members
  than for parents, and the "is this public-safe?" question becomes untestable. They serve
  `audience = public AND hiddenAt IS NULL` and nothing else, regardless of who asks.
- Hiding is a column (`hiddenAt`, `hiddenBy`) and an event, not a deletion — reversible, and
  the media survives so an unhide is possible. Only the author's `DELETE` purges blobs.
- **The media handler must apply the same visibility check as the feed.** A blob URL is a
  capability; an unauthenticated `GET` on a `sha256` path would make every "group only"
  glimt public to anyone holding the hash. Public-scope media is the only media served
  without a session, and from a separate path.
- **Video is the one genuinely new capability** and does not fit the existing pipeline:
  `internal/imaging` cannot touch it, there is no transcoder, and re-encoding video in the
  BFF process is not something to add casually. Options, cheapest first: (a) accept a small
  number of formats, validate the container, store as-is with a hard cap and rely on the
  browser to record H.264/MP4; (b) add `ffmpeg` to the image and transcode out-of-band; (c) an
  external transcoding service. **Decision: (a), with a 30 s cap** (§0). 30 s of phone video
  is roughly 30–60 MB unrecompressed, i.e. several times the 8 MiB portrait limit, so (a)
  needs its own byte ceiling — call it **50 MB per item** — and client-side re-encode before
  upload where `MediaRecorder` allows it. Two consequences to accept explicitly: metadata
  stripping is **container-level only** and must be verified against real iOS and Android
  recordings rather than assumed, and the 30 s cap must be enforced **server-side** on the
  parsed container, not just by the recorder UI. If verification fails, the fallback is
  option (b) or a shorter cap — not shipping video with an unverified privacy claim.

  **Moved to PRD 020 (2026-09-17).** Kept here rather than deleted because this is where the
  decision was made and the alternatives were weighed — PRD 020 inherits all of it unchanged. The
  split happened because that verification gate can only be closed with an iOS device and an
  Android device in hand, and its outcome could force option (b), which is a materially different
  piece of work. An unknown that large should not hold an otherwise-finished PRD open. PRD 020 also
  found three things this paragraph missed: the metadata "stripping" is a container **rewrite**
  rather than a read, video needs **HTTP Range** support that the media route does not have, and the
  storage numbers task 311 set for an image-only feature give a member **ten videos** before they
  are full.
- **Storage growth is unbounded and non-rebuildable.** Every projection in this service is
  replayed from the stream on boot; the blob store is the only thing that is not. Glimt
  multiplies its size by a large factor, and `FileStore` is a Docker volume on one host. A
  size ceiling per member and a real answer for capacity are prerequisites, not follow-ups
  (PRD 008 §11 Q4 names an S3-compatible `Store` as the drop-in).
- **Media is content-addressed, so it is immutable and should be cached like it.** Serve media
  with a long `Cache-Control: immutable` and support conditional requests — this is most of the
  answer to the post-race load spike (§0a.3), and it costs one header. The blob store already
  makes it correct.
- **Retention config follows the `portraitRetention` precedent exactly**, which already solves
  this problem in this codebase: a `time.Duration` field on `config` in `go/cmd/api/env.go`,
  registered with `flag.DurationVar(&cfg.glimtRetention, "glimt-retention",
  envDuration("GLIMT_RETENTION", …), "… (0 disables the purge)")`, plus
  `GLIMT_PUBLIC_RETENTION` alongside it. The sweep guards with `if app.config.glimtRetention
  <= 0 { log “disabled”; return }` — the same shape as `portraitpurge.go:44`, so `0` and any
  negative value both mean off and a misconfigured deployment cannot silently delete
  everything. Reusing the pattern rather than inventing a second one also means the dev stack
  can turn both purges off the same way.
- The effective retention days are exposed on `/api/config` (which already carries
  `contacts_poll_seconds`) so the composer and `PrivacyView` can state the real number instead
  of a hard-coded one.
- Rate limits via `go/internal/ratelimit`: per-member caps on glimt created per hour and
  media bytes per hour. **Read** paths need a separate, looser limit: the post-race browse is a
  legitimate flood, and a limiter tuned for uploads would throttle exactly the use we most want
  to work.

### API endpoints

**Every endpoint below gets swaggo OpenAPI annotations** (`@Summary`, `@Tags`, `@Accept`,
`@Produce`, `@Success`, `@Failure`, `@Router`) in the style of
`updatePhotoHandler` — this is a `.rules` requirement, not a nicety.

| Method | Path | Notes |
|---|---|---|
| `POST` | `/api/glimt/media` | `multipart/form-data`, one item, returns a blob ref. 413 / 429 / 503 as on `/me/photo`. |
| `POST` | `/api/glimt` | Create with caption, audience and ordered media refs. Publishes the created event. |
| `GET` | `/api/glimt/feed` | Paginated, visibility-filtered for the caller. |
| `GET` | `/api/glimt/hold` | Teams that have posted anything visible to the caller — the index for the post-race browse. |
| `GET` | `/api/glimt/hold/:number` | One hold's collection, oldest first, visibility-filtered. |
| `GET` | `/api/glimt/version` | ❌ **Not built — superseded.** `routes.go` says outright not to add another per-dataset version endpoint (task 292 retired the last one), so freshness is a **`glimt` key on `/api/sync`** instead. Seven endpoints would be seven requests per foreground from a few hundred devices. Corrected in task 304. |
| `GET` | `/api/glimt/items/:glimtId` | Single glimt, visibility-checked. |
| `GET` | `/api/glimt/items/:glimtId/media/:ordinal` | Serves a variant (`?variant=thumb`), visibility-checked, `immutable` cache headers. |
| `DELETE` | `/api/glimt/items/:glimtId` | **Owning `personId` only** — not "same hold", not the Team section. Tombstone + purge. |
| `POST` | `/api/glimt/items/:glimtId/report` | Any member. Hides from `public` synchronously. |
| `GET` | `/api/glimt/moderation` | **Team section only.** Every scope, reported-first; the one response carrying `authorPersonId`. |
| `POST` | `/api/glimt/items/:glimtId/hide` · `/unhide` | **Team section only.** Reversible, records `hiddenBy`. |
| `GET` | `/offentligt/glimt` | **No auth, ignores the session cookie.** Server-rendered HTML page on `hej.nathejk.dk`. Must be in `navigateFallbackDenylist`. |
| `GET` | `/api/public/glimt` | **No auth.** Public-scope, not hidden, hold attribution only. |
| `GET` | `/api/public/glimt/items/:glimtId/media/:ordinal` | **No auth.** Public-scope media only. |
| `POST` | `/api/public/glimt/items/:glimtId/report` | **No auth**, rate-limited by IP. Lets a parent report. |

**Note on the `items/` segment** (added 2026-09-17, task 305): every per-glimt path carries
`/items/` because httprouter **panics at construction** if a wildcard segment sits alongside static
siblings — and `/api/glimt/` already has `feed`, `media`, `moderation` and `hold`. The contacts pane
hit the same wall and answered it the same way, with `/api/contacts/people/:personId`. This is a
router constraint, not a design preference.

### Data / events

Subjects follow the house convention `NATHEJK.<year>.glimt.<glimtId>.<verb>` (dot form, as
this service already publishes for `portrait`):

- `…created` — author person id, group, frozen hold number/name/group, audience, caption,
  ordered media refs
- `…deleted` — tombstone
- `…reported` — reporter, reason
- `…hidden` / `…unhidden` — moderation outcome, with the Team-section member who did it
- `…purged` — retention

### Dependencies & risks

| Risk | Note |
|---|---|
| **An unmoderated public feed of children's photos** | Still the largest risk in this PRD, and now an *accepted* one: §0 chose publish-and-hide-on-report over an approval queue, with the Team section watching. The mitigations are structural — hold-only attribution, one-tap reporting including from the public page, synchronous hiding, GPS stripped — plus the Team section actually looking. If nobody is on the Team-section rota at 03:00, the honest response is to turn the `public` scope off, not to ship it unwatched. |
| **The Team section can see every `group`-scoped glimt** | The price of moderation, and a real expansion of who sees a "private" post. Mitigated by disclosure rather than by limiting it: the composer and `PrivacyView` say so, so "Min gruppe" is not read as "only my gruppe". |
| **Team-section authority in a stateless session** | A `section` claim in the signed cookie would outlive the assignment. Mitigated by looking the section up per request (§8) — must not be "optimised" into a claim later without solving revocation. |
| **Post-race load spike** | The peak arrives at the finish line on the worst network of the weekend (§0a.3). Mitigated by thumbnail-first grids, `immutable` media caching, separate read rate limits, and pagination — and it should be load-tested against a realistic item count before the first event, because it cannot be fixed on the night. |
| **Public page on the app's host** | Same origin means the service worker's navigation fallback and the session cookie both reach it. Mitigated by `navigateFallbackDenylist` and by the public handlers ignoring the cookie outright; both need a test. |
| **No recorded guardian consent for `public`** | Accepted (§0). Hold-only attribution is what makes this defensible: what reaches the open web is a patrulje's photo, not a named child's. Revisit if the mix of public glimt turns out to be dominated by identifiable faces. |
| Storage capacity | Non-rebuildable, unbounded, single-volume — and 30 s video makes it materially worse than an image-only feature. Needs a ceiling and a capacity plan before video ships. |
| Video metadata | Container-level stripping only under option (a); must be verified against real iOS and Android recordings before release, and the duration cap enforced server-side. |
| Blob URL as capability | Mitigated by visibility-checking the media handler; must be covered by a test that asserts a group-scoped ref 403s for an outsider. |
| Author id leaking into a response | Mitigated by projecting it out of the response type, with a test asserting the feed payload contains no `personId` and no name. |
| Bandwidth in the field | Client-side compression; measure a real upload on event mobile data. Video on rural mobile data is the hardest case. |
| Nav slot pressure | `MAX_SLOTS = 5` already overflows; adding a primary destination forces a re-ordering decision. |
| PRD 013 coupling | **Resolved:** the public feed is served by this service (§0), so phase 3 is no longer blocked on 013. They are sibling pages on one host and should link to each other. |

## 9. Success Metrics

- **Adoption among spejdere:** ≥ 30% of active spejdere post at least one glimt during the
  event; ≥ 70% open the feed at least once.
- **Sharing stays inside the app:** glimt created per participant per event, tracked across
  years — the number going up is the whole point. (No proxy for "posts avoided on Snapchat"
  exists; a post-event question is the honest instrument.)
- **Audience choice is real, not a default:** the distribution across `group` / `nathejk` /
  `public` is not ~100% on the default. If it is, the selector is being tapped past.
- **Safety — the headline metric, because §0 traded an approval queue for reaction speed:**
  reports per 100 glimt, and **median and worst-case time-to-hidden** for a reported glimt.
  Hiding on report is automatic, so this measures the system, and **time-to-*reviewed***
  measures whether the monitoring is actually happening. Target: hidden in seconds, reviewed
  in minutes while the event runs.
- **Attribution holds:** zero glimt responses containing a personal name, portrait, phone
  number or `personId`. A hard pass/fail asserted by a test, not a target.
- **The post-race browse actually happens** (§0a): median time spent in Glimt in the week after
  the race, glimt viewed per spejder, and photos saved to device. If the hours-long browse is
  real, this is the largest engagement number the app has — and if it is not, the hold
  collection view was the wrong bet.
- **Reliability:** ≥ 99% of glimt accepted into the outbox eventually publish; zero
  silently lost media.
- **The feed survives the finish line:** p95 response time and error rate on
  `/api/glimt/hold/:number` during the hour after the race. This is the moment the
  feature is judged.
- **Parents reached:** unique visitors to `/offentligt/glimt` during and after the event.
- **Moderation is used but not overwhelmed:** glimt hidden by the Team section per event, and how many were
  hidden proactively versus after a report. A lot of proactive hiding means the composer's
  audience copy is not landing.

## 10. Rollout / Task Breakdown

Phased by audience, as directed: **spejder first, bandit second, everyone else third.** The
`public` scope is last — not because it needs a moderation gate built first (§0 chose not to
have one) but because it is the only scope whose mistakes are visible outside Nathejk, so it
should land on top of a feature that has already been watched working.

**Team-section moderation is not a phase.** It ships in phase 1, before there is anything public to
moderate, because §0 makes it the safety mechanism for *every* scope and because it is easier
to build against a small feed than a full one.

**Phase 1 — images, spejder, `group` and `nathejk` scopes, moderation.** Behind a role gate:
only `spejder` sees the destination. No video, no public scope.
**Phase 2 — bandit; the post-race browse.** Extend the role gate. The hold
collection view and saving to device belong here at the latest — they are what makes the first
event's photos worth having (§0a). ~~then video~~ — **video is now PRD 020**, which carries the
30 s / 50 MB caps and the metadata-verification gate.
**Phase 3 — remaining roles, then the `public` scope** and `/offentligt/glimt`. Prerequisites
are concrete: the report path works end to end including unauthenticated, the moderation view is in use
rather than merely built, someone is on the rota, and the load test has been run.

Proposed tasks for `roadmap/tasks/open/`:

- [x] Task: Glimt visibility predicate in `internal/users` (role → group, may-see rules, Team-section override) + tests
- [x] Task: `mayModerate` — per-request `sectionSlug` lookup against the `team` slug constant, revocation test
- [x] Task: `nathejk/table/glimt` projection — schema, consumer, querier
- [x] Task: Freeze hold number/name/group at creation and project `authorPersonId` out of every response, with a test
- [x] Task: `POST /api/glimt/media` — upload, decode-validate, EXIF strip, variants, limits
- [x] Task: `POST /api/glimt` + `GET /api/glimt/feed` + `GET /api/glimt/version`
- [x] Task: `GET /api/glimt/:id/media/:ordinal` — visibility check, `immutable` cache headers, test that a group ref 403s for an outsider
- [x] Task: `DELETE /api/glimt/:id` — owner-only (not the Team section), tombstone and blob purge
- [x] Task: `POST /api/glimt/:id/report` — synchronous hide from public
- [x] Task: Moderation API (Team section) — `GET /api/glimt/moderation`, `hide`/`unhide`, reversible + recorded
- [x] Task: `GlimtModerationView` — all scopes, reported-first, hide/unhide
- [x] Task: Glimt retention purge (mirror `portraitpurge.go`) — `GLIMT_RETENTION` and `GLIMT_PUBLIC_RETENTION`, `0` disables, logged at startup, effective value on `/api/config`
- [x] Task: Per-member rate limits, looser read limits, and a storage ceiling for Glimt media
- [x] Task: OpenAPI annotations for all Glimt endpoints
- [x] Task: `glimt.store.ts` — feed fetch/cache on the contacts-store pattern
- [x] Task: `glimtOutbox.ts` — IndexedDB outbox, drain on foreground and `online`
- [x] Task: Add `glimt` to `config/offline.ts` datasets, split thumbnail/full Workbox routes and budgets
- [x] Task: `GlimtView` feed + `GlimtCard` (hold attribution, tappable, one-tap report) + `GlimtMediaStrip` (add shadcn `carousel`)
- [x] Task: `GlimtComposer` — multi-capture, client-side compression, caption, audience choice, Team-section visibility disclosure
- [x] Task: `GlimtViewer` full-screen dialog + save-to-device (verify on iOS 16.4)
- [x] Task: `GET /api/glimt/hold` + `/hold/:number` and `GlimtHoldView` thumbnail grid
- [x] Task: Nav slot ordering decision — where Glimt sits per role, what moves to `MoreMenu`
- [x] Task: Document Glimt in `PrivacyView` — storage, audience, Team-section reach, retention, takedown
- [x] ~~Task: Video support — container validation, server-side 30 s / 50 MB enforcement, metadata-strip verification on real iOS/Android recordings~~ — **moved to PRD 020** (was task 322)
- [x] Task: `/offentligt/glimt` — server-rendered public page, ignores session cookie, `navigateFallbackDenylist` entry, unauthenticated report link
- [x] Task: Load-test the post-race browse against a realistic item count
- [x] Task: Offline field test — post a glimt with no signal, verify outbox drain on reconnect

## 11. Open Questions

**Correction, 2026-09-17 (after shipping) — the `group` scope was mislabelled, and the label
understated reach.** Recorded here rather than only in a commit, because it is the one class of
mistake this PRD's §6 exists to prevent.

The composer's narrowest option read **"Min patrulje"**, with "Kun dem der er med i din gruppe kan se
det." But `users.MaySeeGlimt` matches this scope on **group** — `GlimtGroupFor(viewer.Role) ==
g.AuthorGroup` — and never on the patrulje number. So for a spejder it reaches *every spejder at the
event*, some 750 people, not the six in their patrulje.

The code was correct and the label was a lie. No test caught it because nothing compares a label to a
predicate; the maintainer caught it by reading the screen. The options now name the population they
actually reach ("Alle spejderpatruljer" / "Alle klaner" / "Alt crew"), and
`audienceChoice.spec.ts` asserts no role's group label ever says patrulje or klan again.

**Narrowing of §6, same date.** §6 requires the Team-section reach to be stated by "the composer and
`PrivacyView`". It is now stated **only on `PrivacyView`**.

The rule the maintainer gave for this, which is worth keeping because it decides what belongs in the
composer at all: **the composer carries what a member must be aware of *each time they post*; standing
facts about the system belong on the privacy page.**

By that test only one of the three notes qualified. "Spørg dem først" is a per-post obligation to
somebody who is not the author and who did not choose to be in the photograph — it is a different
question every time, because it is a different photograph. That the Team section can see everything,
and that media is deleted after 90 days, are facts about the service: true before the member opened
the composer, unchanged by anything they do in it, and read once rather than each time. Three grey
lines also read as boilerplate, which is how the one that matters gets skipped along with the ones
that do not.

`TEAM_DISCLOSURE` is kept exported and tested so the wording does not rot while it is out of the
composer, and restoring it is one line. §6's requirement that the reach be *disclosed rather than
discovered* is still met — by `/privatliv`, in the longer form that has room to say who Team are and
why they can see everything.

**Scope change, 2026-09-17 — video moved to PRD 020.** Recorded here because it changes what this
PRD promises. Everything about video was decided here (§0, §8) and the reasoning stays in §8; what
moved is the *building* of it. The trigger: video's privacy claim can only be verified with an iOS
device and an Android device in hand, and if that verification fails the answer is `ffmpeg`
out-of-band — a materially different piece of work. An unknown of that size should not keep an
otherwise-finished PRD in `doing/`.

This is deliberately **not** a `reopen`: nothing already built is invalidated and nothing needs
re-agreeing. It is a narrowing, and the closure condition narrows with it — PRD 019 is done when its
remaining tasks (318, 325) are, both of which are device checks.

Seven questions from earlier drafts — public-scope moderation, guardian consent, video,
person-vs-patrol authorship, who moderates, where the public feed lives, and how much the
public page reveals — have been answered and moved to §0. What remains:

1. **Retention defaults — now just the numbers, since the mechanism is settled.** The window is
   an env var with `0` meaning off (§6), so what remains is what the *production* defaults are.
   §0a.2 rules out a tight event-scoped purge, but "until next year" turns Glimt into the photo
   archive §4 says it is not. A concrete proposal to react to: **`GLIMT_RETENTION=90d`** and
   **`GLIMT_PUBLIC_RETENTION=30d`** — long enough for the post-race browse and for showing
   family, short enough to be a real limit, with the public feed closing first. Dev and CI run
   with both at `0`.
2. **Is `team` the only moderating slug, and what if it is spelled differently one year?** Section slugs are
   organizer-authored and validated by nothing (`crewFunctionBySlug`), so this needs the actual
   string, and a decision about what happens if it is spelled differently one year — fail
   closed, presumably, which means nobody can moderate until it is fixed.
3. **Can the Team section hide a whole hold's glimt at once?** If something goes wrong it will likely be one
   hold, and hiding twenty items one at a time at 03:00 is the kind of gap that gets discovered
   at the worst moment.
4. **Reactions?** A single tap-to-react with no counter shown to others would let a feed feel
   alive without becoming a popularity contest. Deliberately excluded from §4 pending a
   decision. Comments stay out either way.
5. **A digest notification?** "12 nye glimt i nat" once a day would drive return visits, and a
   single "alle billeder fra løbet er klar" after the race would land on exactly the moment
   §0a describes. Per glimt is out of the question.
6. **`nathejk` scope for spejdere:** should a spejder be able to post to all 1000+ members,
   or only to their own group and public? With no approval queue anywhere, this is the widest
   *internal* unmoderated reach in the system.
7. ~~**Nav placement per role.** With `MAX_SLOTS = 5`, which existing destination moves into
   `MoreMenu` for a spejder so Glimt can be in the bar?~~ **Resolved (task 320): none.** Placing
   `glimt` above `updates` in the single ordered `destinations` array puts it in the bar for all
   seven roles at once, because role-gating already frees the slot — a spejder has no `contacts`
   entry, so Glimt lands third for them and fourth for everyone else. `schedule`, `faq`, `privacy`
   and `sos` were all already behind "Mere". No per-role ordering was introduced; the outcome is
   pinned by `vue/src/config/navSlots.spec.ts`.
8. ~~**Storage ceiling.** What is the per-member and total budget, and what happens when it is
   reached mid-event — reject, or evict oldest? Rejecting a photo in a field is a bad
   experience; deleting someone's memory is worse. 30 s video makes this urgent rather than
   theoretical.~~ **Resolved (task 311): reject, never evict.** Eviction would mean this
   feature's one irreversible operation firing with no human involved and nobody told; retention
   is what frees space, on a schedule everybody was told about in advance. Two ceilings, both
   configurable and both `0` for unlimited: `GLIMT_MEMBER_STORAGE_BYTES` (500 MiB default) answers
   **429** — the member's own quota, which they can act on by deleting something — and
   `GLIMT_TOTAL_STORAGE_BYTES` (off by default) answers **507**, because a full volume is the
   server's problem and there is nothing the member can do. The check **fails open** if it cannot
   be measured: a ceiling is a safety margin, not an authorization. **The byte figures are still
   unconfirmed** — same status as Q1's retention numbers.
9. **Is this PRD 011's team artefact?** §0a makes this much more likely than it was: a
   patrulje's collection, browsable after the race, is most of a post-race memento already.
   Worth deciding before 011 designs its own.
10. **Does the public feed show everything, or a curated selection?** Everything is simpler and
    matches "parents can follow along". But if the public feed is where the event's public face
    lives, the Team section might want to feature rather than merely permit. Not for the first release.
