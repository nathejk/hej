# PRD 011 — The public frontpage: albums, patruljens egen side, and public glimt

**Status:** doing
**Author:** agent session (Zed)
**Created:** 2026-08-28
**Last updated:** 2026-09-19
**Approved:** 2026-09-19
**Shipped:**
**Target users:** anyone outside the app — parents, grandparents, siblings, friends, a leader on a
laptop — and, through them, the participants who want something to show

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 0. Scope change (2026-09-19) — read this first

This PRD was written on 2026-08-28 as **"post-race experience: diploma, team tracks and scan
history"**: three sections *inside the installed app*, for a signed-in member, showing their own
team's night.

On 2026-09-19 the maintainer redirected it. The same material — a patrol's facts, its diploma, its
route, its scans — now lands on **the public frontpage of hej**, next to curated photo albums and
the public glimt feed. Three sections, no login:

1. a few **curated albums** (3–5) of photographs,
2. **patruljens egen side** — one page per patrol: name, gruppe, korps, an estimated distance, a
   diploma thumbnail, and a map of the race area carrying all its scans and its merged track,
3. a list of **public glimt**.

**What that changes, and it is not a detail.** The old version's hardest question was whether a
member may see a *teammate's* track. The new version's hardest question was whether **anybody at all**
may see a patrol's track, because the audience is now the open web. Everything in §6 that used to be
"resolved from the session, never from a parameter" inverts: there is no session, and the patrol is
addressed by number.

**That question is answered (§0b). It is not reopened by implementation detail.**

**What survives unchanged:** §0a's measurements, the honesty requirements they force, the diploma
options in §8, and the scan-classification problem. Those were about the data, not the audience.

**Boundaries with neighbouring PRDs.** This PRD does not own the public surface, it adds to it:

| | owns | this PRD |
|---|---|---|
| **PRD 013** (draft) | the anonymous website — rules, programme, practical, privacy, on `/desktop.html` | must not become a second front door; §7 states how the two relate |
| **PRD 019** (done) | the public glimt page at `/offentligt/glimt`, its retention, its reporting, its no-JS rendering | section 3 is that page's feed, surfaced — not a reimplementation. Answers 019 §11 Q10 ("everything, or a curated selection?") with **both** |
| **PRD 002** (done) | the event map, its base layers, the position track | section 2's map reuses the layers; see §8 for why it may not reuse the component |

## 0a. Inherited measurements (from PRD 002, added 2026-09-02)

PRD 002 shipped the position track and then measured it on real devices. Whatever this PRD draws,
it inherits these facts — and the first two constrain what can honestly be drawn:

| | |
|---|---|
| accuracy | **10.5 m median** on an iPhone (GPS) · **35 m** on a Wi-Fi-only iPad |
| coverage | good while the app is foregrounded; **2% of a 22-hour day**, because a web app does not run backgrounded |
| gaps | one per backgrounding, matching it almost to the second |
| iOS kills | 8 of 28 backgroundings, always resumed |

**So a track is a dotted record of where the app was open, not a route.** Drawing it as a continuous
line implies a precision and a continuity it does not have: it would invent path between two points
recorded twenty minutes and half a kilometre apart, and on an iPad it would invent it at 35 m
accuracy. The gaps are information — they say "the phone was in a pocket here" — and smoothing them
away is the one presentation choice this PRD should not make by default.

**2% coverage is also why "estimated distance" cannot be a sum of track segments.** See §6, Distance.

Task 086 (the original post-race team track) was closed unbuilt in favour of this PRD; its analysis
is in that task file.

## 0b. Decisions taken (2026-09-19, maintainer)

Two questions this PRD raised as blocking were answered the same day. They are recorded here rather
than in §11 because the rest of the document is built on them.

**1. A patrol's track is public. The unit of identity is the patrulje, not the person.**

> *"we don't show personal names only patrulje names — so yes show track"*

The reasoning is the same one the public glimt page already runs on: nothing on this surface names a
human being. A patrol is a patrol — a number, a name, a gruppe, a korps — and its route is the
patrol's route. There is no person to identify in it, which is exactly why §6 requires the track to
be **merged and unattributed**: that requirement is no longer a mitigation for a privacy problem, it
is what makes the statement true. Per-member tracks would reintroduce the person, and are therefore
not a deferred nicety but something this design forbids.

So: `/offentligt/patrulje/{number}` carries the facts, the diploma, the scans and the merged track,
openly, from the moment the gate in decision 3 opens.

**The right to publish the tracks has been obtained** (maintainer, 2026-09-19). So this is not a
consent gap being papered over — permission exists, on the same footing as the photographs in
decision 2. What remains is a **transparency** obligation rather than a permission one: PRD 002's
consent copy says the recorded route goes to *the organizers* and promised to show it back to *jer*,
and it does not mention the open web. A participant reading today's wording would be surprised by
the patrol page, and being surprised by a true thing is still a failure of the copy. `/privatliv`
and the consent text must describe it before section 2 ships — a task in §10, not an open question,
and not a blocker on the decision.

**2. Permission for photographs is obtained, and absence is the mechanism.**

> *"we have obtained permission from all to show images — if we do not have permission to show a
> picture it will not be there"*

Consent is therefore handled **upstream of this feature**, at the point a photograph enters a
curated album. This PRD does not build a rights model, a per-subject flag or a consent register; it
renders what a curator published, and the curator publishes only what may be shown. The safety
property is *the set*, not the page — which is also why the albums are curated rather than
participant-submitted, and why the public glimt feed keeps its own separate consent story (a member
tapping "Offentligt" is the one giving permission there).

What this leaves for §6 to require is narrow and worth stating: a curator must be able to **remove**
a photograph and have it leave the public page promptly, because permission is withdrawable and a
mistake is possible. That is a takedown path, which this surface needs anyway.

**3. A patrol's page opens when that patrol has a scan at the last checkgroup — which is the finish
line, so the gate means "finished" — and everything opens when the last checkpoint closes.**

> *"we have a list of checkgroup, when patrulje have a scan on the last one, the page should open"*

**Per patrol, not race-wide.** An earlier draft of this section guessed race-wide and guessed wrong;
the rule is per patrol, and it is a better rule than the one I proposed. It is derivable from data
`hej` already projects, it needs no new concept, no flag and no human, and it means a patrol's page
appears at the moment that patrol's race ends rather than when somebody else's does. It is also the
same definition `diplom` already uses for the finish time — the scan at the last checkpoint — so the
page and the diploma cannot disagree about whether a patrol finished.

**The last checkgroup is the finish line.** Stated here because it is the fact the rest of this
section rests on, and it is not recoverable from the schema: `checkgroup` knows route order but not
that the last position in that order is where the race ends. "Has a scan at the last checkgroup" and
"has finished" are therefore the same predicate, which is why one gate can honestly serve as both.

**The mechanism, concretely.** `checkgroup.sortOrder` is route order and the comment in
`checkgroup/table.sql` records that there is **one sequence for every patrol** — nothing upstream
carries a per-team route (PRD 016 §11.9). So "the last one" is the non-deleted checkgroup with the
highest `sortOrder`, and the condition is: the patrol has a scan attributed to a checkpoint in that
group. Attribution runs scan → scanner → `checkpersonnel` shift → checkpoint → checkgroup, which is
the join PRD 016 already built.

**One failure mode to design for, because it is not hypothetical.** A scan carries no checkpoint of
its own (see §8); it is attributed by asking which post its scanner was on shift at, and an
**unattributed scan is a normal outcome**, not an error. So a patrol can genuinely finish and have
its gate stay shut, because the scan that should have opened it could not be placed. That fails in
the safe direction for the gate and the disappointing direction for the patrol — the one patrol most
entitled to a page. This needs a **manual override** for løbsledelsen, and a way to notice it
happened, or the failure is silent and lands on whoever has the worst luck.

**No staged reveal: the page opens all at once, or not at all.**

> *"race dynamics are not affected by this — everything goes public at the same time"*
>
> *"when the patrulje reaches the last checkpoint they are finished — last checkpoint is equal to
> finishline, at this time there is no secrecy on the track for that patrulje — patrulje still active
> in the race will not benefit from seeing the other patruljes track, at this time they already know
> where the finish line is, they just have not reached it themself"* (2026-09-19)

I had raised the opposite — that opening one patrol's page publishes most of the course to patrols
still walking it, and that PRD 016's reveal model (grounded in physical possession: a patrol sees a
checkpoint because it stood there or holds the sheet) would be bypassed rather than extended. I
proposed holding the map and the scan pins back until the field cleared.

**Withdrawn — and on the facts, not as a judgement call.** The missing fact was the one thing I could
not read from the code: **the last checkgroup is the finish line.** That collapses the objection
entirely, and it is worth spelling out why, because it is the reasoning that keeps a future reader
from "fixing" this gate:

- **The gate and the finish are the same event.** A patrol scanning at the last checkgroup has
  finished walking. So its track describes a route it will not walk again, and there is no secrecy
  left in it *for that patrol* — nothing it discloses can affect a race that is, for them, over.
- **It discloses nothing useful to the patrols still out.** The finish line is not a secret — every
  patrol knows where it is; they simply have not got there yet. A page whose newest information is
  "here is the finish you already know about" confers no advantage, so there is no advantage to
  withhold.
- **Therefore the possession model is not bypassed, it is satisfied.** PRD 016 withholds a checkpoint
  from a patrol that has not reached it because knowing early would change how they navigate. Here
  the earliest a position becomes public is after the patrol that generated it has finished, and the
  positions themselves are ones every patrol is already navigating towards. Different mechanism,
  same principle — which is why this is an extension of PRD 016's rule rather than a hole in it.

So the gate is **all-or-nothing per patrol**: when it opens, the facts, the distance, the diploma,
the scans, the pins and the track all appear together. No second gate, no partial page, no
per-element reveal rules to keep consistent — a real simplification, and it removes the only place
this design was going to need two gate states.

**One thing this reasoning does depend on, and it should be checked rather than assumed:** that the
last checkgroup really is the finish for *every* patrol. `checkgroup/table.sql` records that there is
one route sequence for the whole event and nothing upstream carries a per-team route (PRD 016
§11.9), so this holds by construction today. If a future event ever gives patrols different routes,
or puts a post after the finish, the gate stops meaning "finished" and this whole argument needs
re-reading — worth a comment at the gate itself saying so.

**The second trigger: everything is public when the last checkpoint closes.**

> *"everything will be public when last checkpoint closes"* (2026-09-19)

So there are **two ways a patrol's page opens, whichever comes first**:

| | trigger | scope | meaning |
|---|---|---|---|
| 1 | the patrol scans at the last checkgroup | that patrol | *this patrol has finished* |
| 2 | the last checkpoint closes | everything | *the race is over* |

Read as a **backstop**, not a replacement — trigger 1 still opens a finishing patrol's page while the
race runs. *(If the intent was instead that trigger 2 replaces trigger 1 and nothing at all is public
until the post closes, say so and I will simplify: it is a smaller design, and the per-patrol
machinery disappears.)*

**This is a better rule than it first looks, because it fixes two things I had raised as problems:**

- **It retires the manual override.** §0b.3's failure mode — a patrol finishes, but the deciding scan
  could not be attributed to a checkpoint, so its gate never opens — now **self-heals** at closing
  time. A human override may still be wanted to open a page *sooner*, but it is no longer the only
  thing standing between an unlucky patrol and never having a page at all. That is a much better
  place to be: the safety net is a rule, not somebody remembering.
- **It answers §11 Q4.** A patrol that retired, or was driven home, or was never scanned at the
  finish, gets its page when the last checkpoint closes — along with everyone else. Nobody is
  silently excluded from the feature for not finishing, and nothing has to publish the fact that they
  retired in order to give them a page. This is the outcome Q4 was groping for, reached by a simpler
  route than any of the options I listed there.

**The mechanism.** `checkpoint.openUntilUts` is exactly this instant — the schema's `fixed` scheme
stores absolute open/close instants — so "the last checkpoint closes" is the greatest `openUntilUts`
across the event's non-deleted checkpoints. No new concept, no new event, no flag.

**But it is not always computable in principle, and the gate still guards for it.**
`checkpoint/table.sql` records three schemes: `fixed` carries instants, `relative` carries only a
duration anchored on the patrol's own scan elsewhere, and `none` carries zeros — and `0` means "not
set", not an instant in 1970.

**The maintainer has confirmed (2026-09-19) that the last checkpoint will always have absolute
opening hours**, so in practice trigger 2 always has an instant to fire on, and every patrol gets a
page by closing time. Two consequences:

- The guard stays anyway — a missing or zero `openUntilUts` means trigger 2 never fires rather than
  firing immediately — because it costs one comparison and the failure it prevents is publishing every
  patrol's page at the epoch. An invariant that holds by organizer practice is worth asserting in
  code, not assumed in it.
- **The manual override is demoted to a convenience.** With the backstop guaranteed, no patrol can end
  up with no page at all; the worst case is a patrol whose finish scan went unattributed waiting until
  closing time instead of getting its page at the finish line. Worth having so løbsledelsen can fix
  that by hand, not worth blocking the feature on.

Before either trigger fires, sections 1 and 3 — albums and glimt — are unaffected and can be live
throughout the event.

**4. Gruppe and korps come from shared-go's `patrulje` projection.**

> *"gruppe/korps are available on patrulje projection from shared-go"*

Confirmed in `github.com/nathejk/shared-go/tables/patrulje/table.sql`, which carries `teamNumber`,
`name`, `groupName` and `korps` — so the header §6 asks for needs no upstream change and no new
event. Two implementation notes worth recording now:

- **`korps` is a slug, not a label.** `shared-go/types/corps.go` maps `dds` → *Det Danske
  Spejderkorps*, `kfuk` → *De grønne pigespejdere*, and so on via `CorpsSlug.Label()`. The public
  page renders the label, never the slug. `andet` ("Andet") and an empty value are both
  "unspecified" and should be **omitted** rather than printed — a header line reading "Andet" tells
  a visitor nothing and looks like a bug.
- **That table also carries `contactName`.** A personal name, on the row this page's header comes
  from. This is exactly why §8 insists the public surface reads a **purpose-built projection** with
  no name column, rather than querying `patrulje` directly: the invariant "this page names no
  person" should be impossible to break by adding a field to a template, and one careless `SELECT *`
  is otherwise all it takes.

**5. The map stays interactive. Zoom is a requirement, not a nicety.**

> *"people want to be able to zoom in, that points to leaflet"*

This closes §11 Q8, which had been left open deliberately until the island was built and weighed. The
measurement came in at **≈60 KB gzipped** (Leaflet 42.4, markercluster 8.7, CSS 4.2, the island 4.2)
and I put it forward as an argument *against* the island, because a server-rendered static image would
have cost comparable bytes with no JavaScript at all.

**The argument was about the wrong axis.** A static image and an interactive map are not two
implementations of one feature at different weights — they are different features. What a family does
with this page is zoom in on the bit of forest they know, and a PNG cannot do that at any size. So the
bytes are the price of the feature rather than a reason to reconsider it, and there is no second
implementation to maintain as a fallback: where the script does not run the page still carries the scan
list, which is the honest degradation and already shipped (task 342).

**6. There are no transfer sections. An inactive patrol's track is what a car looks like.**

> *"there are no such thing as a transfer section. if a user is no longer active, their track should
> not be taken into account, and then they might be transfered by car"*
>
> *"we are relying on a lot of different gps units, some more accurate than others, if we can identify
> an outlyer or something obvious wrong or just suspecious, then drop that single coordinate from
> calculation"*

This corrects an assumption the distance estimate was built on. Task 339 filtered legs faster than
7 km/h as "vehicle transfer" and recorded a residual overstatement — one 2025 patrol reaching 103 km —
as an accepted cost, on the theory that a slow transfer is indistinguishable from a long walk. **The
real signal is not speed, it is status.** A patrol still in the race walks; a patrol that has left the
race may be sitting in a car, and it is that predicate rather than a velocity threshold that separates
the two cases.

Two consequences for the estimate:

- **Points recorded while a person is no longer active do not count.** Not smoothed, not down-weighted —
  excluded, because they are not the patrol walking.
- **Outliers are dropped per coordinate, not per leg.** The fleet is a mixture of phones of varying
  quality, so a single wild fix is the common failure and it currently inflates two segments at once.
  The unit of rejection is therefore **one point**, and the bar is "obviously wrong or suspicious"
  rather than a speed limit on a whole stretch.

This also settles what the label may claim. *"mindst"* ("at least") was in tension with a figure that
could overstate; once a car is excluded by status and bad fixes are dropped individually, the remaining
error is dominated by **missing** data rather than spurious data — which is the direction *mindst*
honestly describes. The word stays, and the reasoning for it is now true rather than aspirational.

## 1. Summary

A public, login-free frontpage for Nathejk that shows what the event looked like: a handful of
**curated photo albums**, a **page per patrol** carrying its name, gruppe and korps, an estimated
distance, its diploma and a map of everywhere it was registered, and the **public glimt** the
participants shared themselves. Photographs that carry a location appear on the map too, clustered
when zoomed out.

It is the answer to "what did you actually do last night?" asked by someone who was not there and
will never install the app.

## 2. Problem & Motivation

**What problem does this solve?** Two dead ends that meet at the same page.

- **The app has nothing after the finish line.** The moment a patrol finishes, the app is over,
  at exactly the point where participants are most interested in what just happened and most
  likely to show it to someone. A finished Nathejk is twelve hours of walking through a forest at
  night that leaves almost no trace for the people who did it.
- **The people they want to show it to cannot get in.** The app is install-first and behind a
  login (PRD 005). A grandparent is not going to install a PWA and verify a phone number. PRD 019
  already conceded this and built `/offentligt/glimt` for exactly them — and that page is
  currently the *only* thing on the public side, a single flat feed with no context around it.

**Why now?**

- **Possible:** the public surface exists and works. `/offentligt/glimt` is server-rendered Go
  templates, no bundle, no JavaScript, rate-limited by IP, `noindex`, with retention and
  anonymous reporting already in place (PRD 019, tasks 320–327). A frontpage is another page in
  that pattern, not a new product.
- **Possible:** the data exists. Tracks flow to `TELEMETRY.<year>.track.<personId>.reported` and
  are retained indefinitely (task 081), with **no reader at all** today. Scans are projected into
  the `scan` table (PRD 016). 2025 is fully represented on the stream, so all of this can be built
  and verified before the 2026 event.
- **Necessary:** **the consent bargain is one-sided.** The app asks a 12-year-old for continuous
  location access and everything it does with that goes to the organizers. PRD 002 §11.1 promised
  something back — *"så vi bagefter kan vise jer jeres egen rute"* — and until that exists the
  privacy page describes a benefit the app does not deliver. Note carefully: that promise was
  *"vise jer"*, to you. Keeping it on a **public** page is keeping a different promise than the one
  made, which is settled in §0b — and carries a copy change with it.

**Evidence.**

- Maintainer direction, 2026-09-19 (the three sections, the patrol page's contents, the merged
  track, the clustered photo locations).
- PRD 019 §11 Q10 asked whether the public feed shows everything or a curated selection and
  deferred it. This answers it: a curated set *and* the full feed, as two different sections doing
  two different jobs.
- PRD 019 §11 Q9 asked whether a patrulje's glimt collection *is* PRD 011's team artefact. It is
  part of it, not all of it — hence a patrol page that carries facts, a diploma and a map as well.
- The `diplom` repo already generates a per-patrol PDF diploma — background artwork, the Nathejk
  Impact font, the team's number and name, a finish time from the last checkpoint scan. Hardcoded
  to 2024 and reachable only as its own service. The idea is proven; the route to a visitor's
  browser is what is missing.
- Task 082's device run: participants get a **fragmented** track. Anything built here is either
  honest about that or it presents fiction as fact.

## 3. Goals

- Someone who was not at the event, on any device, with no login and no install, can see what it
  looked like.
- A patrol has **one URL** it can send to a family, that page says who they are and what they did,
  and it is worth sending.
- The estimated distance is a number a 12-year-old can repeat and an adult cannot catch out — so
  it is stated as a floor, not a measurement.
- The route is presented honestly, gaps included, at the precision §0a allows and no more.
- The curated albums give the event a public face that the raw feed cannot: chosen, ordered,
  captioned.
- The position track becomes something the participant gets back, so PRD 002 §11.1's consent is
  reciprocated — and the consent copy is corrected to describe where it now goes (§0b).
- Nothing here slows or complicates the during-race app. It is a separate surface.

## 4. Non-Goals

- **Live anything.** No live tracking, no live feed, no publication of a patrol's positions while it
  is still walking. A patrol that can watch itself navigates differently. Note the limit of this
  non-goal after §0b.3: a patrol's page opens the moment *that* patrol scans at the last checkgroup,
  so pages do appear while the event is still running — what is excluded is a patrol's own positions
  appearing before it has finished, not the event being over everywhere.
- **Ranking, scoring or comparison between patrols.** Nathejk is not scored that way, and a public
  page carrying a distance per patrol is one sort away from a leaderboard. Explicitly out: no
  ordering by distance, no "longest route", no percentile. If the maintainer wants a competition,
  that is a decision, not an emergent property of this page.
- **Any personal data about a person.** No names of people, no portraits, no phone numbers, and
  emphatically no `phoneParent` (`.rules` — hard rule). A patrol is attributed as a patrol, which
  is the rule the public glimt page already works under.
- **Per-member tracks.** Section 2 shows **one merged track per patrol**, with no attribution to a
  person. This is a **requirement, not an option**: the surface's whole privacy claim is that it
  names no person (§0b), and an attributed track breaks it. It would also disclose which member
  declined location (visibly absent from a track others are on) and which member was elsewhere
  (visibly apart) — two things nobody asked to publish.
- **Search engine indexing.** `noindex`, following PRD 019's reasoning: photographs shared publicly
  is not the same as asking to be findable by name years later.
- **A public surface for personnel** (postmandskab, guide, samarit, gøgler, crew). They have no
  patrol and no route walked as a patrol. Their experience is a separate question.
- **Uploading to albums from the app.** Album curation is an organizer job; where the tooling for
  it lives is §11 Q5, and it is not a participant-facing feature.
- **Editing or correcting scans, tracks or distances.** Read-only. A wrong scan is fixed upstream.
- **Replacing PRD 013's anonymous website.** The rules, the programme and the practical information
  stay there. This page is the event's *memory*; that page is its *manual*.

## 5. User Stories & Scenarios

- As a **parent**, I want to open a link and see where my child walked and that they finished, so
  that I understand what the weekend was.
- As a **spejder**, I want one URL for our patrol that I can send to my family, so that I do not
  have to explain it.
- As a **grandparent on an old laptop**, I want to look at the photographs without installing or
  signing in to anything.
- As a **leader**, I want to see which posts my patrols reached, so that I can talk to them about
  their night.
- As a **visitor who has never heard of Nathejk**, I want the albums to tell me what this is.

**Primary happy path**

1. A visitor opens the public frontpage. Three sections, in this order: **albums → find your
   patrol → glimt**.
2. The albums are 3–5 covers with titles. Opening one is a page of photographs, captioned.
3. "Find din patrulje" takes a patrol number (and only a number). It resolves to that patrol's
   page, once that patrol has crossed the finish line (§0b.3).
4. The patrol page: **patrol name, gruppe, korps** as the headline; **~distance** beside it; the
   **diploma thumbnail** to the right. Below, the **map of the race area** with the patrol's scans
   as pins and its merged track drawn through them.
5. The glimt section shows the most recent public glimt and links to the full page, which already
   exists.

**Edge cases and error scenarios**

- **A patrol number that does not exist**, or exists in another year, or has not finished yet. **One
  answer for all three**: "we cannot show that patrol yet". Distinguishing them tells a stranger
  which numbers are real and turns the URL space into a live finish-order feed (§11 Q2).
- **A patrol with no track at all** — nobody granted location, or everybody's phone was in a
  pocket. Per task 082 this is the *common* case, not the exception. The page must be worth
  opening with an empty map, or the feature is only for the lucky patrols.
- **A patrol with no scans** at all gets no page until the last checkpoint closes, at which point it
  gets one like everybody else (§0b.3, trigger 2) — with an empty map and no distance. It must read
  as a page about a patrol that was there.
- **A patrol that did not finish** (retired, or was driven home) has no scan at the last checkgroup,
  so trigger 1 never fires for them. They get their page when **the last checkpoint closes**, along
  with everyone else — which means nobody is excluded from the feature for not finishing, and nothing
  has to publish the fact that they retired in order to give them a page. §11 Q4 is now about what
  that page *says*, not whether it exists.
- **Scans with no position.** A post can register a patrol manually, so a scan is listable but not
  plottable — the `scan` table stores coordinates as strings and they can be absent or
  unparseable. Such a scan is still listed and still counts for nothing in the distance.
- **Several tracks from the same patrol** — one per member who recorded, and more than one per
  member if the app was killed and resumed (8 of 28 backgroundings, task 082). These are merged;
  see §6.
- **The event has not happened.** 2026 is empty today. Empty sections must read as "not yet",
  never as an error or a blank page — and section 2 will spend most of the year in exactly this
  state, because §0b.3 keeps a patrol's page shut until it finishes or the last post closes. The
  not-yet page is therefore a **first-class screen**, not an error path: it is what every link to a
  patrol page shows for most of the year.
- **A link shared before that patrol's gate opens.** A family given the URL in advance, or a patrol
  that sends it from the bus, arrives at not-yet and must be told *when* in a way that does not read
  as breakage — and must not be able to tell whether the number they were given is even real.
- **A patrol that finished but whose last scan was not attributed to a checkpoint.** Trigger 1 misses
  them, so their page arrives when the last checkpoint closes rather than at their finish. Late, not
  absent — the backstop is guaranteed (§0b.3) — which is why the override is a convenience rather than
  a safety net.
- **A photograph whose location is wrong** — EXIF from a phone with no fix, or a coordinate off in
  the North Sea. A pin in the wrong place on a public page is worse than no pin. Bounds-check
  against the race area and drop what falls outside.
- **Load.** This page is shared in family WhatsApp groups the morning after. It is the one surface
  where a hundred simultaneous visitors is normal.

## 6. Requirements

### Functional — section 1: curated albums

- [ ] The frontpage shows **3–5 albums**, each with a cover image, a title and an optional short
      description.
- [ ] An album opens to its photographs, in an order the curator set, each with an optional
      caption.
- [ ] Albums are **curated, not participant-submitted**: an organizer chooses what is in them.
      They are not the glimt feed with a filter on it. Curation is also where photo permission is
      enforced (§0b) — a photograph is in an album because somebody decided it may be shown.
- [ ] A curator can **remove** a photograph, or a whole album, and it leaves the public page
      promptly — within the page's cache window and no longer. Permission is withdrawable.
- [ ] A photograph **may** carry a location. When it does, it is plotted on the maps this PRD
      draws; when it does not, it is simply a photograph. Most will not have one.
- [ ] Located photographs are **clustered when zoomed out** and separate when zoomed in, so a
      post that fifty photographs were taken at reads as one marker, not a blot.
- [ ] A photograph's location is a **deliberate, reviewable field**, not metadata that happens to
      survive. The media pipeline strips all EXIF including GPS by re-encoding (PRD 019,
      `glimtmedia.go`) and **that must not change**; a coordinate is read before stripping and
      stored as a column a curator can see, correct and remove. See §8.
- [ ] Coordinates are bounds-checked against the race area; anything outside is stored but not
      plotted, and is visible to the curator as such.
- [ ] Albums render without JavaScript: real `<img>` tags, thumbnails, `loading="lazy"`, no
      carousel — the same decision and the same reasoning as PRD 019's public page (task 323).

### Functional — section 2: patruljens egen side

- [ ] One page per patrol per year, addressed by patrol number (§11 Q2 decides whether the
      address is guessable).
- [ ] The header carries **patrol name, gruppe and korps**, read from shared-go's `patrulje`
      projection (§0b.4). `korps` is rendered as its label via `types.CorpsSlug.Label()`, and
      `andet`/empty is omitted rather than printed.
- [ ] An **estimated distance**, derived from the patrol's scans and tracks per the Distance rules
      below.
- [ ] A **diploma thumbnail**, to the right of the header, linking to the full diploma. Which
      service renders it is §8 / §11 Q6.
- [ ] A page opened by the **backstop** rather than by a finish scan (§0b.3) has no finish, and
      therefore no diploma: the slot is **absent, not apologetic**, and no copy draws attention to
      what is missing. §11 Q4.
- [ ] A **map of the race area** below, carrying:
      - [ ] every scan the patrol collected that has a plottable position, as a pin,
      - [ ] the patrol's **merged track**,
      - [ ] any located album photograph in the area, clustered as above.
- [ ] **Tracks are merged into one.** A patrol has many source tracks: one per member who
      recorded, plus one per resume after an iOS kill. They are presented as a single track with
      **no per-person attribution** — §0b and §4. The merged track is the patrol's route; there is
      no person in it, and there must be no way to recover one from it.
- [ ] **Merging is a union of segments, not an interleaving of points.** Sorting every member's
      points into one timestamp-ordered polyline draws a line that jumps between members walking
      ten metres apart — a zig-zag that is pure artefact. Each source track is simplified and
      broken independently; the result is one *multi-segment* polyline in one colour.
- [ ] Duplicate points are collapsed on `(person, timestamp)`. **This is the contract task 083
      established**: a retry after a timeout can legitimately publish the same point twice, and
      the reader is the only place it can be removed.
- [ ] **Gaps are rendered as gaps.** Break a segment on a time delta above a stated multiple of
      the sampling interval. Joining two points either side of a two-hour hole draws a confident
      line through terrain nobody walked.
- [ ] The page **states what the track covers** — that it records where a phone had the app open,
      not where the patrol walked — in one sentence a parent will read. A gap must not be read as
      "they stood still here".
- [ ] Every scan is also **listed**, in race order, with what kind of registration it was and
      when — including the ones with no position. The map is not the only representation.
- [ ] Simplification is applied for legibility, and the layer that applies it is recorded
      (server-side before the response, or client-side before rendering).
- [ ] The map is legible on both topographic and aerial backgrounds.
- [ ] **A patrol with no track, or no scans, still gets a page** that reads as a page, not as an
      error or an empty frame.

#### Distance — the rules, because the obvious implementation is wrong

- [ ] The distance is presented as a **floor with a tolerance**, in Danish, e.g. *"mindst ~24 km"*
      — never a decimal. `23,47 km` claims a measurement that does not exist.
- [ ] It is **not** the sum of track segments. At 2% coverage (§0a) that sum is a fraction of the
      night and would under-report by an order of magnitude: a patrol that walked 30 km would be
      told it walked 600 m.
- [ ] The base is the **sum of straight-line distances between consecutive positioned scans, in time
      order**. Every leg is a real journey between two places the patrol demonstrably was, and a
      straight line is the shortest it can have been — so the sum is a true lower bound.
- [x] **Amended 2026-09-19 (task 339), on 2025 data: a leg implying more than a walking pace is
      excluded as a vehicle transfer.** The rule above is a floor on *distance travelled*, not on
      distance walked, and patrols are transported between sections of the course. Summing every leg gave
      2025 a median of 45.9 km and a **maximum of 157.9 km**; excluding vehicle legs gives 41.3 km and
      41 km, which is 3.4 km/h over a twelve-hour night. Excluded rather than capped — capping invents a
      walk of exactly the length the filter allows. A residual remains: a transfer slow enough to pass the
      filter cannot be told from a long walk, which is in tension with the word *mindst* and is flagged
      for the maintainer in task 339's log.
- [x] **Amended again 2026-09-21 (§0b.6, maintainer): the transfer premise was wrong, and the fix is
      status plus per-point outlier rejection.** There are no transfer sections. What produces a
      car-shaped track is a person who is **no longer active** in the race, so their points are excluded
      from that moment on (`person.memberStatusAt` → `cmd/api/patroltrack.go`); where the moment is
      unknown the member is excluded entirely, which errs low as *mindst* permits. Separately, a
      coordinate that is obviously wrong is dropped as a **single point** — reported accuracy over 250 m,
      or a fix that jumps implausibly far and comes straight back — never the leg around it.
      The 7 km/h leg filter stays as a **backstop**, because 2025 has no telemetry at all and speed is the
      only signal available there.
- [x] **And a third rule, found by measuring rather than reasoning (task 349): a leg longer than 4 hours
      *and* further than 10 km is not one walk.** The overstating tail turned out not to be fast legs but
      slow enormous ones — 38.7 km over 13.6 hours, and 10.4 km over **143 hours** — which a speed filter
      waves through by construction. Both conditions are needed: a long gap alone is a patrol resting, and
      2025 has six-hour legs covering two kilometres that are real walking. On 2025 this moves mean/max
      from 44.8/103.5 km to **38.1/63.6 km**, and patrols credited with over 60 km from 26 to 1.
- [ ] Within a leg where track points exist, the **measured track distance replaces the straight
      line if it is longer**, which it usually is. This is the only thing the track contributes to
      the number, and it can only move it up — never down.
- [ ] Legs whose endpoints have no position contribute nothing, and their absence must not make
      the number look complete. If a material share of scans is unplottable, the page says the
      figure is incomplete rather than quietly under-reporting.
- [ ] The rounding, the tolerance and the wording are **stated in one place in the code** and
      tested, because this is the number that will be screenshotted.
- [ ] Sanity-check against 2025: the computed figures must be in the same range as the course's
      actual planned length. A distance estimate that disagrees with the route plan by a factor is
      a bug, not a finding.

### Functional — section 3: public glimt

- [ ] The frontpage shows the most recent public glimt and links to the full page at
      `/offentligt/glimt`.
- [ ] It reuses PRD 019's projection, filtering, retention window, hidden-flag handling and
      reporting. **No second implementation of publicly-visible-glimt logic**; `publiclyVisible`
      stays the single gate.
- [ ] A glimt is attributed to its **hold**, never to a person — PRD 019's rule, unchanged.
- [ ] Public glimt carry **no location**, because their EXIF is stripped and nothing replaces it.
      They are not on the map, and that is deliberate: a participant tapping "Offentligt" agreed
      to share a photograph, not a position. Only curated album photographs are plotted. §11 Q7 if
      this is ever revisited.

### Functional — access, gating and safety

- [ ] **Everything on these pages is unauthenticated** and must ignore the session cookie
      entirely, so a signed-in member sees exactly what a parent sees. Structural, as with
      `/offentligt/glimt`: the routes do not use the auth middleware.
- [ ] There is a test asserting that **no response on this surface carries a person's name, a
      phone number, a portrait or a `phoneParent`** — the `.rules` invariant, enforced in the
      projection rather than trusted to the template.
- [ ] **A patrol's page does not exist publicly until one of two triggers fires** (§0b.3), whichever
      comes first:
      - [ ] that patrol has a scan attributed to a checkpoint in the last checkgroup — the
            non-deleted `checkgroup` with the highest `sortOrder`, which is the finish line, so this
            means "has finished"; or
      - [ ] **the last checkpoint has closed** — the greatest `checkpoint.openUntilUts` across the
            event's non-deleted checkpoints — at which point every patrol's page opens, finished or
            not.
- [ ] Trigger 2 fires **only when a closing instant exists**. The last checkpoint always has absolute
      opening hours (confirmed 2026-09-19), so this is an assertion of an invariant rather than an
      expected path — but `0` means "not set", not 1970, so an absent instant must leave the backstop
      unfired rather than opening every page at the epoch.
- [ ] The gate **fails closed**: neither trigger satisfied, or the underlying rows cannot be read,
      and the page stays shut. A page that opens early publishes a patrol's positions while it is
      still racing; a page that opens late disappoints somebody. Those are not comparable costs.
- [ ] A **manual override** exists so løbsledelsen can open a patrol's page early — the case being a
      patrol that finished but whose finish scan could not be attributed to a checkpoint (a normal
      outcome, not an error — §0b.3, §8), which otherwise waits for the backstop. A convenience, not a
      safety net: with the backstop guaranteed, no patrol can end up with no page at all.
- [ ] Before a patrol's gate opens, its URL answers as **not-yet**, not as 404 and not as a blank
      page — and the not-yet answer carries no patrol data at all, so it cannot be used to discover
      which numbers exist, to read a name early, or to infer which patrols have finished.
- [ ] The gate is **all-or-nothing** (§0b.3): a patrol page never appears partially. There is no
      element with its own reveal rule — no "pins later", no "track later" — so there is exactly one
      gate state per patrol to test.
- [ ] The gate's **assumptions are documented where it is implemented**: that the last checkgroup is
      the finish line, that one route sequence serves every patrol, and that the greatest
      `openUntilUts` is when the race ends. All three are facts about the event rather than the
      schema, and any of them changing silently invalidates the gate (§0b.3, §8).
- [ ] Sections 1 and 3 (albums, glimt) are **not** gated and may be live during the event.
- [ ] The gate is testable in both states, and the closed state is the default in a fresh
      environment.
- [ ] By-IP rate limiting on every new public route, following `publicGlimtReadLimiter`.
- [ ] `noindex, nofollow` on every page here.
- [ ] A visible route to "take this down" on the patrol page. Not because the design is in doubt
      — §0b settled that — but because a patrol whose page is wrong, or who has a reason we have
      not thought of, needs somewhere to write. The glimt page already has this affordance.
- [ ] All new endpoints carry OpenAPI annotations (repo rule), including the HTML ones, as
      `glimtpublic.go` already does.

### Non-Functional

- **Reach before polish.** This page exists so someone on a ten-year-old browser can see the
  weekend. The albums, the glimt, the patrol facts, the distance, the diploma thumbnail and the
  scan *list* must all work with **no JavaScript at all**. The **map is the one exception** and is
  therefore a progressive enhancement, not a requirement — see §8.
- **Honesty over polish.** Every requirement above about gaps, floors, non-finishers and missing
  tracks exists because the attractive rendering is the dishonest one. Participants were there and
  will notice; parents will not, which is worse.
- **Privacy.** This surface's privacy property is not "scoped correctly" but "has nothing to
  scope": it names no person. That is enforced in the projection (§8), not in the templates, and
  the one place it could be broken is an attributed track — which §6 forbids.
- **Performance.** One patrol of six at 30 s sampling is roughly **8,600 points across ~2,160
  stream messages**. Cheap once; not cheap a hundred times a minute on an unauthenticated route.
  Cache the rendered patrol page, and treat "the morning-after WhatsApp spike" as the design load.
- **Accessibility.** Every map has a list equivalent on the same page. Alt text on every
  photograph. Real headings and lists, so the content survives unstyled.
- **Typography.** Headlines use the Nathejk font (`.rules`). The public pages are outside the Vue
  bundle, so the font has to be reachable from them too, or the `font-nathejk` rule is quietly
  broken here.

## 7. UX / UI Notes

**Where this lives.** `/offentligt` as the public root, with `/offentligt/glimt` already under it:

| path | what |
|---|---|
| `/offentligt` | the frontpage: albums, find-your-patrol, recent glimt |
| `/offentligt/album/{slug}` | one album |
| `/offentligt/patrulje/{number}` | patruljens egen side *(address shape: §11 Q2)* |
| `/offentligt/glimt` | the full public feed *(exists, PRD 019)* |

`/desktop.html` (PRD 013) stays what it is and links here; this page links there for the rules and
the privacy text. Two surfaces, one each for *what the event is* and *what the event was*. The
duplication to avoid is content, not navigation.

**The frontpage, in order.** Albums first: they are the most inviting thing and need no input.
Then find-your-patrol, because the visitor who came for that came with a number in their hand.
Then the glimt, as a strip with a link — the full feed has its own page and does not need
reproducing.

**Patruljens egen side.** The maintainer's layout, taken literally:

```
┌──────────────────────────────────────────────┬──────────┐
│ Patrulje 42 · Ørnene                         │ ┌──────┐ │
│ 1. Søllerød Gruppe · Det Danske Spejderkorps │ │diplom│ │
│ mindst ~24 km                                │ └──────┘ │
├──────────────────────────────────────────────┴──────────┤
│                                                         │
│   map of the race area: scans, merged track, photos     │
│                                                         │
├─────────────────────────────────────────────────────────┤
│   the scans, in order, as a list                        │
└─────────────────────────────────────────────────────────┘
```

Patrol name in `font-nathejk` and large; gruppe and korps beneath it, smaller — a patrol is known
by its name, and the korps is context. The distance is the one number on the page, so it gets room.
The diploma thumbnail is the visual anchor and the thing a parent clicks.

**Before a patrol's gate opens.** Its URL shows a not-yet page: what the page will contain, and that
it appears when the patrol crosses the finish line — or, for those who do not, when the last post
closes. It is the most-served version of this route by far (§5), so it deserves to be written rather
than generated — and it must say the same thing for a patrol number that does not exist, or it becomes
a way to check which numbers are real and which patrols have finished.

**The map.** Same base layers as the app's (PRD 002) so the two are recognisably the same place.
One colour for the merged track, a distinguishable pin for a scan, a different marker for a
photograph, and clusters that show a count. Where the map cannot run, its place is taken by the
scan list and a plain line of text — not by a broken frame.

**Tone.** Danish, readable by a 12-year-old and their grandparent, the standard the `/privatliv`
page works under. The patrol page is the one place in this repo where a little ceremony is right;
it is also the place where an overclaim does the most damage.

## 8. Technical Considerations

**Surface: Go templates, not the Vue bundle.** The public pages are server-rendered
`html/template`, extending `glimtpublic.go`'s pattern. This is not a preference: a login-free page
whose purpose is reach cannot require the app's bundle to parse, and `.rules` puts the app's
baseline at Safari 16.4 / Chrome 111 precisely because the app needs push and service workers —
none of which this page uses. `html/template` escapes every interpolation, which matters because
captions here are participant- or curator-authored text on an unauthenticated page.

**Frontend (Vue 3 / TS): almost nothing.** The app gains, at most, a link from the post-race state
to a patrol's public page. `EventMap.vue` **cannot be reused** — it is a Vue component in the
bundle this page must not load — so the map here is a small island: Leaflet plus a marker-cluster
plugin, loaded only where JS runs, fed from a JSON endpoint, drawn into a container that already
contains a usable fallback. The base-layer configuration is worth sharing with the app; the
component is not shareable and pretending otherwise will produce the wrong architecture. **This is
the one place this PRD spends a new dependency** (clustering), and §11 Q8 asks whether that is the
right trade or whether a static server-rendered map image is.

**BFF (Go): five pieces, one of them genuinely new.**

1. **A patrol read model for the public page.** A projection carrying exactly what the page shows —
   number, name, gruppe, korps, finish time (null when they did not finish), the computed distance,
   and why the gate is open — and nothing else. **Why**, not just whether: a page opened by the finish
   scan shows a diploma and a page opened by the backstop does not (§0b.3), so the two states are not
   interchangeable and collapsing them to a boolean loses the distinction the template needs. **The
   projection is where the privacy invariant is enforced**: the public page cannot leak a name or a
   `phoneParent` if the row it reads has no such column. Note this is not theoretical — the source row
   in shared-go's `patrulje` carries `contactName` (§0b.4), so "project a purpose-built row" is what
   stops one `SELECT *` from putting a person's name on the open web.
2. **Gruppe and korps: available, and §0b.4 says where.** shared-go's `tables/patrulje` has
   `groupName` and `korps`; `types/corps.go` turns the slug into a label. So this is a consumer and
   a projection, not an upstream change — the question that was blocking approval is closed.
3. **The gate.** Two predicates OR'd together (§0b.3): "has this patrol scanned at the last
   checkgroup?" — the highest-`sortOrder` non-deleted `checkgroup`, joined to the patrol's scans
   through the scanner's `checkpersonnel` shift — and "has the last checkpoint closed?", the greatest
   `checkpoint.openUntilUts` over non-deleted rows. Every input exists (PRD 016); what is new is the
   query, the fail-closed default, the uncomputable-backstop case and the override. Cheap, but it is
   on an unauthenticated route, so compute the per-patrol half with the patrol row rather than per
   request. The closing instant is one value for the whole event and wants caching, not a join.

   **This function needs a comment recording why it is safe**, because the safety is not local to the
   code. It reads as "publish a patrol's positions once it scans here", which looks alarming in
   isolation; it is safe only because that checkgroup is the finish line, because one route sequence
   serves every patrol, and because the greatest `openUntilUts` really is when the race ends (§0b.3).
   All three are facts about the event, not about the schema — confirmed by the maintainer on
   2026-09-19, including that the last checkpoint always carries absolute opening hours — and a future
   event that gives patrols different routes, adds a post after the finish, or runs its final post on a
   `relative` window invalidates the gate without changing a line of this code. That is exactly the
   kind of assumption that has to be written down next to the thing depending on it.
4. **A track reader**, reading `TELEMETRY` by subject filter for the patrol's members, merging and
   simplifying. This is a **deliberate departure** from "reads come from projections" (PRD 008 §8)
   and the reasoning has to travel with it: projecting tracks means millions of points in MariaDB
   (827 participants × ~1,440 points) to serve a view opened once per patrol after the event. The
   exception is justified because the read is **bulk, cold and non-critical**. It is *less* true
   here than in the old version of this PRD, because a public URL can be opened repeatedly by
   strangers — so the merged, simplified result must be **cached or materialised per patrol**, and
   the cache, not the stream, is what the page reads. Do not generalise the exception.

   **⚠️ Superseded 2026-09-19 (task 340). This paragraph was wrong on both counts.** The points *are*
   projected, into `nathejk/table/trackpoint`, and there is no departure from PRD 008 §8 to justify.
   Two findings overturned it:

   - **The library cannot read a stream on demand.** `stream.Stream` offers push `Subscribe` and
     `LastMessage`, with no bounded fetch-by-subject. "Read one person's last twelve hours" would have
     meant dropping to `nats.go` beneath the abstraction every other read goes through.
   - **Projecting is better, for a reason this paragraph missed.** Task 083's contract is that a point
     is identified by `(person, timestamp)` because a retry can republish it, and the reader is the
     only place a duplicate can be removed. As a projection that is the **primary key** — dedup
     enforced by the database. On demand it would have been a million-point in-memory problem, per
     request, forever.

   The volume was also never measured: 1.19M rows of six small columns is ~60–100 MB, the same order as
   projections this service already carries. What this paragraph actually wanted — that a public page
   must not do bulk work per request — is kept: the merge, the gap-breaking and the simplification
   happen on read and are cached per patrol.
5. **Albums.** A new small table: album (slug, title, description, order, published) and album
   item (album, ordinal, blob refs, caption, optional lat/lng, bounds-check verdict). Media goes
   through the existing `internal/blob` and `internal/imaging` path, which content-addresses and
   makes a thumbnail. **The GPS question:** that path deliberately destroys EXIF by re-encoding, and
   must keep doing so. A curated photo's coordinate is therefore read from the original bytes
   *before* re-encoding and written to a column — visible, editable, deletable, bounds-checked. A
   coordinate in a column is a decision somebody made; a coordinate hidden in a file is a leak
   waiting to happen.

**Scans.** `internal/scans` has a real projection now (`projection.go` over the `scan` table, PRD
016), so the old version's first task is done. Two facts carry forward: coordinates are stored as
**strings** because that is how the event carries them, so parsing is a real step with a real
failure mode; and **a scan carries no checkpoint**, which is recovered by asking which post the
scanner was on shift at (`checkpersonnel.sql`). An unattributed scan is a normal outcome. The
kind (`checkpoint` / `bandit`) is modelled in `internal/scans` — whether it is *derivable* for
every scan is the residual part of the old scan-classification question; confirm against 2025 before
the list claims a kind it guessed.

**The diploma and the no-cross-service rule.** The org's architecture forbids a service calling
another service's HTTP API, so `hej`'s BFF **may not** call `diplom`. Three options (§11 Q6):

- **(a) Link the browser to `diplom`.** A link is not a service-to-service call, so the rule holds.
  Cheapest, reuses artwork that works. Costs: a second origin, and `diplom` is hardcoded to 2024.
  **The public frontpage makes this option better than it was** — there is no session to carry
  across origins any more, which was the main objection. But it still needs a thumbnail, so `hej`
  must either render one or `diplom` must serve one.
- **(b) Render the diploma in `hej`** from its own projections. One origin, one deployment. Costs:
  reimplementing generation (`go-pdf/fpdf` is small) and the artwork pipeline.
- **(c) Move generation into `hej` and retire `diplom`.** Cleanest end state, largest change,
  touches another repo.

The old recommendation was (b), on the grounds that bouncing a 12-year-old to another domain to log
in again would kill the feature. **That argument is gone with the login**, so **(a) is now the
better first ship** — provided a thumbnail exists and 2026 data reaches `diplom`.

**Decided 2026-09-21 (maintainer): (c).** *"a sibling repo 'diplom' can be used as guideline to how we
create a diploma — the diploma generation logic should be moved here, use an old graphic as mock, we
will replace before launch."* Shipped in task 345 as `internal/diploma`, and the argument that
settles it is not architectural but editorial: the diploma is one element on a page this repo already
renders, gated by a verdict it already computes, from a finish time it already has — so linking out
would have meant a second origin, a second deployment and a year hardcoded in another codebase, for a
PDF that is a background image and four lines of text.

**One thing did not come with the logic: the patrol photograph.** `diplom` places a `natpas` portrait
in the middle of the page. On this surface that would publish eight children's faces through no
consent gate, which §0b.2 forbids and which would make §8's "names no person" work pointless — a
photograph is a stronger identifier than any name. So the public diploma carries the patrol's name,
the event, the route and the finish minute, and nothing else. **The 2026 artwork should be designed
for a diploma without a photograph**, rather than around a hole where one used to be.

**API endpoints (all new, all unauthenticated, all requiring OpenAPI annotations):**

| endpoint | purpose |
|---|---|
| `GET /offentligt` | the frontpage (HTML) |
| `GET /offentligt/album/{slug}` | one album (HTML) |
| `GET /offentligt/patrulje/{number}` | patruljens egen side (HTML) |
| `GET /api/public/albums` | albums and their located items, for the map island |
| `GET /api/public/patrol/{number}` | the patrol's facts, distance and diploma link |
| `GET /api/public/patrol/{number}/map` | merged track + plottable scans, for the map island |
| `GET /api/public/albums/{id}/media/{ordinal}` | album media, mirroring the glimt media route |

The existing authenticated `GET /api/patrol/scans` is untouched. Note it is patrol-scoped from the
session; the public one is scoped by path parameter. The old §6 required "never from a request
parameter" and this cannot honour it — which is fine now the response carries nothing personal, but
it means the **projection is the only guard**, so §8.1 is not an optimisation.

**Where the gate lives.** In the handlers for the three patrol routes, as a single shared check —
not in the templates, and not in the map island. Three routes serve section 2 (the HTML page and two
JSON endpoints), and a gate applied in the template would leave the JSON open, which is the whole
course in machine-readable form. One function, called first in all three, tested in both states.

**Data / storage.** Two new tables (album, album item) plus a public patrol read model, each owned
by its projection (PRD 008 §8). ~~No storage for tracks;~~ **and a `track_point` projection — see
§8.4, which reversed that decision (task 340).** Media reuses the blob store, whose retention is
PRD 019's — album media is **organizer-owned and must not be subject to glimt retention**, which is a
real trap: the purge job walks the blob store.

**Dependencies & risks.**

- **2026 has no data yet; 2025 has all of it.** Build and verify against 2025, which is complete on
  the stream. That is an advantage and should be used rather than waited out.
- **Risk: the distance becomes a competition.** A public number per patrol is a leaderboard that
  somebody else will build in a spreadsheet. §4 forbids us shipping the sort; it cannot forbid the
  screenshot. Worth the maintainer's explicit acceptance.
- **Risk: the map island becomes a second frontend.** The line to hold: it draws what an endpoint
  gives it and holds no state. If it grows routing, layer switching and a store, it has become the
  app and should be in the app.
- **Risk: album curation has no home.** Without tooling, "curated" means "an organizer SSHes in" —
  and since curation is where photo permission is enforced (§0b), the tool is a safety control, not
  a convenience. §11 Q5.
- **Risk: the public surface's load profile is new.** Every other route in this repo is behind a
  login, which is a very effective rate limiter. This one is not.
- **PRD 007 (portraits) and PRD 003 (profile) are unrelated** and must not become prerequisites.
  Portraits in particular are exactly what must never reach this surface.

## 9. Success Metrics

- Patrols send their page to their families: measurable as public patrol pages opened per
  finishing patrol in the week after the event. If nobody sends it, the patrol page is decoration.
- The public frontpage is reachable and readable with **JavaScript disabled** and on the oldest
  device available (iPad mini 2, iOS 12.5.8) — verified by hand, as PRD 013 does.
- **Zero personal-data disclosures**, measured by the projection-level test, not by absence of
  complaints. The claim to be able to make afterwards: no name of a person appeared anywhere on
  this surface, by construction.
- **Zero pages opened early.** No patrol page, and no public endpoint behind one, answers with that
  patrol's data before one of §0b.3's two triggers has fired for it — asserted in a test against the
  closed state, which is the state that ships first and the state the system spends almost all its
  time in. Because trigger 1 *is* the finish, this also carries the race-integrity claim: nothing
  becomes public about a patrol while that patrol is still racing.
- The distances computed from 2025 agree with the course's planned length to within a stated
  tolerance.
- The reported track coverage on the patrol page is consistent with what task 082 measured on the
  device — i.e. the page is not quietly hiding gaps.
- PRD 002 §11.1's claim becomes true, and the privacy copy describes the public page accurately —
  **including that a patrol's merged route is on it.**

## 10. Rollout / Task Breakdown

Approved 2026-09-19; tasks **330–348** created in `roadmap/tasks/open/`. Sequenced so the gate lands
first (nothing in section 2 may be reachable without it) and so **sections 1 and 3 are shippable
independently of section 2**.

*Phase 0 — the gate and the copy that must precede publication*

- [ ] **330** — the section 2 gate: per-patrol finish scan OR last-checkpoint-closed backstop, failing
      closed, with the not-yet page, the invariant assertion, the override, the documented assumptions
      and all states tested (§0b.3)
- [ ] **331** — update `/privatliv` and PRD 002's consent copy to describe the patrol's merged route
      being public — **a prerequisite for shipping section 2**, per §0b.1

*Phase 1 — the public shell, albums and the glimt strip (no dependency on section 2)*

- [ ] **332** — `/offentligt` frontpage shell in Go templates, linking the existing glimt page
- [ ] **333** — album and album-item tables, media ingest reusing `blob`/`imaging`, with the
      GPS-before-strip coordinate column and the race-area bounds check
- [ ] **334** — album pages: no-JS, thumbnails, alt text
- [ ] **335** — curator removal of a photograph or an album, effective within the cache window (§0b.2)
- [ ] **336** — recent-public-glimt strip on the frontpage, reusing `publiclyVisible`
- [ ] **337** — assert in a test that no public response carries a name, phone, portrait or
      `phoneParent`

*Phase 2 — patruljens egen side*

- [ ] **338** — public patrol read model: number, name, gruppe and korps from shared-go's `patrulje`,
      finish time (nullable), distance, and *why* the gate is open; no `contactName`, asserted
- [ ] **339** — the estimated distance: scan-leg floor, track-raised legs, floor wording, 2025 sanity
      check, tested in one place
- [ ] **340** — merged track reader: `TELEMETRY` by subject filter, dedup on (person, timestamp),
      union-of-segments merge, gap breaks, simplification, cached per patrol
- [ ] Task: the patrol page — header, distance, diploma thumbnail, scan list, no-JS complete
      *(also carries the finish time, moved here from 338: it is derived from the last-checkgroup scan
      rather than projected, so it belongs with the page that renders it)*
- [ ] Task: the map island — Leaflet + clustering, scans, merged track, located photographs, with
      a working fallback where it cannot run
      *(also: give the map endpoint a **timestamped** point shape and wire
      `trackPointsForDistance` — task 341 left the track raising no distance leg, because
      `patroltrack.Point` carries no time and `distance.Compute` matches legs by time)*
- [ ] **343** — the takedown affordance on the patrol page
- [ ] **344** — confirm scan-kind classification against 2025 before the list labels a kind
- [ ] **349** — the distance, corrected per §0b.6: exclude points recorded after a member left the race,
      and drop suspicious coordinates one point at a time rather than filtering whole legs by speed

*Phase 3 — the diploma and the backstop-opened page*

- [ ] **345** — the diploma: implement the chosen option from §11 Q6, including the thumbnail
- [ ] **346** — the page for a patrol opened by the backstop rather than by a finish: copy and the
      absent diploma slot (§11 Q4)

*Throughout*

- [ ] **347** — by-IP rate limits and cache headers on every new public route
- [ ] **348** — verify the whole surface against **2025** data, on a phone viewport, with JS off, and
      on the oldest device available

## 11. Open Questions

Five questions that were here on 2026-09-19 were answered the same day and moved to **§0b**: whether
a patrol's track may be public, how photo permission is handled, when a patrol's page opens, whether
the reveal is staged, and where gruppe and korps come from. What follows is what is still genuinely
undecided.

1. ~~**Do the map and the scan pins wait for the field to clear, even though the facts open per
   patrol?**~~ **Resolved (§0b.3, 2026-09-19):** no — and the question was based on a
   misunderstanding rather than a trade-off. **The last checkgroup is the finish line**, so the gate
   opens only once a patrol has finished; its track then discloses nothing that can affect its own
   race, and nothing useful to patrols still out, who already know where the finish is and have
   simply not reached it. PRD 016's reveal model is satisfied rather than bypassed. The gate is
   all-or-nothing per patrol: no staged reveal, no second gate state.
2. **Is a patrol page's address guessable?** `/offentligt/patrulje/42` is memorable, sendable by
   voice, and lets a family find the page from the number on the patrol's sign; it also lets anyone
   enumerate every patrol in the event. Now that §0b makes the content public, enumeration reveals
   nothing that was not already public, so **the memorable number is the recommendation**. The one
   residual: because the gate is the finish (§0b.3), enumerating tells you **which patrols have
   finished**, live, during the race — a live results feed nobody asked for. §6 keeps the not-yet page
   indistinguishable from the page for a number that does not exist, which closes it at no cost.
3. ~~**Do gruppe and korps reach `hej`?**~~ **Resolved (§0b.4, 2026-09-19):** shared-go's
   `tables/patrulje` carries `groupName` and `korps`, and `types/corps.go` maps the slug to a label.
   A consumer and a projection, not an upstream change. Two notes carried into §0b.4: render the
   label rather than the slug, and do not project that table's `contactName`.
4. **What does a non-finishing patrol's page say?** No longer "do they get one" — §0b.3's second
   trigger settles that: when the last checkpoint closes, every patrol's page opens, so nobody is
   excluded from the feature for not finishing, and nothing has to announce that they retired in order
   to give them a page. That is a much better answer than any of the options this question used to
   list.

   What is left is copy and the diploma. Such a page has a track, scans and a distance, but no finish
   — so: does it show a diploma at all (`diplom` derives its finish time from the last checkpoint
   scan, which does not exist here), and does it say anything about not finishing, or simply present
   the night they had and stop? My recommendation is the latter: the page shows where they went and
   how far, the diploma slot is absent rather than apologetic, and no copy anywhere draws attention to
   what is missing. A patrol that walked seven hours and got driven home does not need a page
   explaining that.
5. **Who curates the albums, and with what?** "Curated" implies a person and a tool, and per §0b
   that tool is where photo permission is enforced — so it is a safety control, not a convenience,
   and "a directory on disk and a deploy" is a weaker answer than it looks. Options: a Team-section
   surface in the app (the moderation-queue precedent, PRD 019 tasks 300/308/309), or a small admin
   page on the public service. Related: the curator also needs the removal path §6 now requires.
6. ~~**Who owns the diploma** — link out to `diplom` (a), render in `hej` (b), or move generation
   here (c)?~~ **Resolved (§8, 2026-09-21, maintainer): (c).** Generation moved into
   `internal/diploma`, following `diplom`'s layout as a guideline; the 2024 poster stands in as a
   **mock** until 2026's artwork exists. What remains is a launch checklist rather than a question,
   and it is recorded in code as `diploma.ReplaceBeforeLaunch`: the artwork, the headline font (the
   Impact file `diplom` embeds is not ours to redistribute), and the route line. **The patrol
   photograph is not on the list — it is excluded on purpose** (§8, §0b.2).
7. **Should public glimt ever carry a location?** §6 says no, because EXIF is stripped and the
   composer's "Offentligt" never offered to share a place. Note this is a *different* permission to
   the one §0b records: album photographs are cleared by a curator, while a glimt is cleared by the
   member who posted it, and they agreed to share a photograph rather than a position. If located
   glimt are wanted later it needs a deliberate opt-in in the composer and its own copy — not a
   pipeline change.
8. ~~**Is a JS map island acceptable on a page whose virtue is being dumb?**~~ **Resolved (§0b.5,
   2026-09-21):** yes — the island stays, because **zoom is the feature**. Built and measured first
   (≈60 KB gzipped, task 342), and the measurement was put forward as an argument for the static
   image instead: comparable bytes, no JavaScript. The maintainer's answer reframed it — a PNG and an
   interactive map are different features, not two weights of one, and a family's whole use of this
   page is zooming in on the forest they know. No second implementation as a fallback either: where
   the script does not run, the scan list already carries the same facts.
9. **How long does this stay up** — until the next event, or indefinitely? `TELEMETRY` retention is
   indefinite (task 081), so the data outlives any decision here. A public page has a stronger
   argument for expiry than an in-app one did, and PRD 019 already has a public retention window
   (`GLIMT_PUBLIC_RETENTION_DAYS`) whose reasoning applies.
10. **Do klans, bandits and gøglere get a page?** A bandit's hold is a klan and their night is
    catches, not posts — the same three sections may work with the emphasis inverted (who *we*
    caught). Gøglere staff the night and are not a hold at all. §4 excludes them for now; the
    exclusion will be noticed by the bandits, who are also 14.
