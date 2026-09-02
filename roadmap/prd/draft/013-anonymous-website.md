# PRD 013 — The anonymous website: what everyone who cannot use the app gets instead

**Status:** draft
**Author:** agent session (Zed)
**Created:** 2026-09-02
**Last updated:** 2026-09-02
**Approved:**
**Shipped:**
**Target users:** anyone who cannot or should not use the installed app — a participant on an old phone, a parent on a laptop, a leader looking something up, a curious visitor

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 1. Summary

A plain public website at `/desktop.html` that works on **any** browser — no bundle, no framework, no
login — carrying the information a participant or parent actually needs: the rules, the programme,
practical details, and who to ask. It is the single answer for three audiences who all currently get a
dead end: browsers too old to run the app, desktop visitors, and anyone who has not installed anything.

**It is not a second version of the app.** It has no account, holds no personal data, and offers nothing
that needs a session.

## 2. Problem & Motivation

- **What problem does this solve?** Maintainer direction, 2026-09-02: *"we can create a completely
  different website for non supported browsers — only thing we can't do is a blank page."* Task 204 stopped
  the blank page on an iPad mini 2 (iOS 12.5.8) with a static apology, which is honest and empty: it tells
  someone their device is too old and gives them nothing to do about it. The rules, the programme and the
  practical information are not device-dependent, and withholding them from a 12-year-old with an old phone
  is a choice we did not consciously make.
- **Three audiences, one dead end.** They look different and want the same thing:
  1. **Legacy browsers** — below the iOS 16.4 / Chrome 111 baseline. The app's bundle will not parse, so no
     gate, no routing, nothing of ours runs (PRD 005 §10a).
  2. **Desktop visitors** — deliberately excluded from the app (PRD 005), sent to `/desktop.html`, which
     today reads **"more to come…"**.
  3. **The not-yet-installed** — the app is install-first and its login lives inside the installed app
     (task 143), so a browser visitor has no way in and nothing to read.
- **Why now?** Two of the three pieces already exist and are unfinished rather than absent:
  `/desktop.html` is a real static page, already anonymous by decision, with a placeholder where its content
  should be; and task 204 just put a fallback in front of exactly the visitors who need it. This is
  finishing something, not starting something.
- **Evidence.** The iPad mini 2 run (task 204). `vue/public/desktop.html` line 128: `<p>more to come…</p>`.
  PRD 005 task 143, which removed the login from the website on purpose.

## 3. Goals

- **No dead ends.** Every browser that reaches `hej.nathejk.dk` gets something readable, on any device, with
  or without JavaScript.
- A participant on an unsupported device can still find out **what the rules are, when things happen, and
  what to bring** — the parts of the app that were never device-specific.
- A parent can read what the event is and what the app does with their child's data, without installing
  anything or being asked to log in.
- The content has **one source**, so the website and the app cannot disagree about the rules.
- The site is cheap enough to keep correct that it does not become a second product with its own decay.

## 4. Non-Goals

- **Not a second app, and not a downlevelled build of this one.** No login, no session, no role scoping,
  no personal data, no offline, no push, no map tiles, no position track. See §11.1 for the four options
  considered and why this is the shape.
- **No participant data of any kind.** Not a directory, not a name, not a patrol list, and emphatically no
  `phoneParent` (`.rules`). This is not a limitation to work around later — it is the property that makes
  the site cheap and safe. See §8.
- **Not an SOS or operational surface.** Crew work needs a supported phone; the radio remains the primary
  channel (PRD 007). §5 states this limitation plainly rather than hiding it.
- **Not a marketing site for Nathejk as an organisation.** That is `nathejk.dk`'s job. This is the app's
  front door for people who cannot come in.
- **Not a replacement for the start-area briefing**, which reaches participants; this reaches the people the
  briefing does not, which is mostly parents.

## 5. User Stories & Scenarios

- As a **spejder with an old phone**, I want to read the rules and the programme, so that not having a new
  phone does not put me at a disadvantage.
- As a **parent on a laptop**, I want to understand what this app records about my child and for how long,
  without installing it or being asked who I am.
- As a **participant who has not installed anything yet**, I want to know what the app is for before I put
  it on my home screen.
- As a **leader**, I want a URL I can give to a family that answers the usual questions without me
  answering them again.

### Primary path

1. Any browser opens `hej.nathejk.dk`.
2. A browser that can run the app is gated as it is today (PRD 005) — this PRD changes nothing for them.
3. A browser that cannot gets the static fallback from task 204, whose link leads here.
4. The website renders as plain HTML: rules, programme, practical information, data and privacy, and who to
   contact. Nothing on the page requires JavaScript.
5. On a device that *could* install the app, the existing install banner appears — progressive enhancement,
   already built.

### Edge cases

- **JavaScript off entirely.** The page is complete without it; the install banner is the only thing that
  disappears, and it is the only thing that should.
- **A supported phone arrives here by accident** (a shared link, a misread device). The install banner is
  the route back into the app, which is what it already exists for.
- **A crew member on an unsupported device.** They get the rules and the programme and *not* the crew
  directory, the patrol lookup or SOS. That is a real hole and the honest answer is organisational: crew
  bring a supported phone, and the radio is the channel that always works.
- **Someone treats this as the app.** The page must say what it is not, or a parent will conclude the app
  simply does not do much.
- **A very old browser with broken CSS support.** The content must be readable as unstyled HTML: semantic
  headings and lists, not layout carried by classes.

## 6. Requirements

### Functional

- [ ] The website is served at `/desktop.html` and is reachable by any browser, with no bundle, framework or
      build-time JavaScript required to read it.
- [ ] It carries, at minimum: **the rules**, **the programme**, **what to bring / practical information**,
      **data and privacy**, and **who to ask**.
- [ ] **The rules have one source.** They live in `vue/src/config/rulebook.ts` today, deliberately, so the
      app can read them offline. The website must render *that* data rather than a copy of the text — see
      §8 for how — so a rule change cannot land in one place and not the other.
- [ ] The page states plainly what it is and what it is not, so nobody mistakes it for the whole app.
- [ ] The existing install banner behaviour is preserved: shown to devices that qualify, absent otherwise.
- [ ] Semantic HTML — real headings, lists and links — so the content survives with CSS or JavaScript
      disabled.
- [ ] Task 204's fallback links here, and this page does not link back into the app for a browser that
      cannot run it.

### Non-Functional

- **Works on anything.** No syntax, CSS feature or API newer than the oldest browser we might plausibly
  see. That means hex colours, no `oklch()`, no CSS nesting, no modules — the constraints task 204 already
  works under.
- **Anonymous.** No session, no cookie that identifies a person, no request that needs authentication. The
  BFF's own rule already says this page is public by definition.
- **Privacy by construction.** With no auth there is no permitted set to scope, so there is no place for
  `phoneParent` or any other personal field to leak. That is the design, not a happy accident.
- **Cheap to keep correct.** A second surface is a second thing to update; anything that would need
  updating per-feature belongs in the app, not here.
- **Fast on a slow device.** The audience includes a 2013 tablet on rural mobile data.

## 7. UX / UI Notes

- One page, or a very small number of pages, in the existing visual language of `desktop.html` — which
  already uses inline CSS with hex colours and CSS variables it defines itself.
- Danish, and readable by a 12-year-old *and* their parent, which is the same rule the `/privatliv` page
  works under (PRD 002 task 085).
- The **data and privacy** content is the part parents come for, and a version of it already exists in the
  app at `/privatliv`. The website's version must not be a summary that quietly says less.
- No app chrome: no bottom nav, no avatar, nothing suggesting a session.
- Headings first: someone scanning on a phone in a car park should find "hvad skal jeg have med" without
  reading prose.

## 8. Technical Considerations

- **Frontend (Vue 3 / TS):** none at runtime. This page is outside the SPA on purpose (task 140) and must
  stay outside it, or it inherits the bundle it exists to avoid.
- **The single-source problem, and the proposed answer: generate it.** The rulebook is TypeScript in
  `src/config/rulebook.ts` because the app needs it offline; the website cannot import that without
  becoming an app. So a **build-time generator** should render the same data into static HTML, following the
  pattern already established by `vue/scripts/generate-icons.sh` and `generate-splash.sh`. One source, two
  renderings, no runtime coupling, and a rule change updates both by construction. *This is the single most
  important decision in this PRD: the alternative — copying the text — guarantees the two disagree, and the
  rules are exactly the content where that matters.*
- **BFF (Go):** nothing required. The page is a static file the Go binary already serves in production.
  **If** dynamic content is ever wanted here (see §11.2), it would be Go templates plus `htmx`, and it
  should be an enhancement over working HTML rather than a requirement for it.
- **API endpoints:** none new. Nothing on this page may require an authenticated call, so no endpoint gains
  an HTML representation and there is no OpenAPI change. If that ever changes, the annotation requirement in
  `.rules` applies as usual.
- **Data / storage:** none. No session, no storage, nothing cached beyond what the browser does for a static
  file.
- **Dependencies & risks:**
  - **Risk: it becomes a second product.** Every future feature would invite "and on the website?" The
    mitigation is the anonymity rule: if a feature needs to know who you are, it is not for this site. That
    single line answers most of those questions before they are asked.
  - **Risk: content drift.** Addressed for the rules by generation; the programme and practical information
    have no in-app source today, so whichever place they land in first becomes the source, and it should be
    a decision rather than an accident.
  - **Risk: `htmx` reintroduces the original problem.** htmx 2 targets modern browsers; htmx 1.x is ES5.
    A tool chosen to serve old browsers must be checked against the oldest browser we care about, or the
    blank page comes back wearing a different hat.
  - **Risk: it undermines install-first.** PRD 005 deliberately makes the app the only way in. A website
    that is too useful invites participants to stay in the browser and miss push, offline and the map. The
    line to hold: the website carries what does not change during the race; the app carries what does.

## 9. Success Metrics

- No browser, on any device, receives a blank or contentless page from `hej.nathejk.dk`. Verifiable by
  hand on the oldest device we have (iPad mini 2, iOS 12.5.8) and with JavaScript disabled.
- The rules shown on the website match the app's, by construction rather than by review — demonstrated by
  changing the rulebook source and seeing both change.
- A parent's questions about data can be answered by sending one URL.
- No authenticated request is ever made from this page: assertable in a test, and the thing that keeps the
  site's privacy surface at zero.

## 10. Rollout / Task Breakdown

Content before mechanism: the page already exists and already renders, so the first useful step is putting
real words on it, not building a generator.

Proposed tasks for `roadmap/tasks/open/`:

- [ ] Task: decide and write the website's content set — rules, programme, practical, privacy, contact
      *(needs løbsledelsen for anything not already written down)*
- [ ] Task: build-time generator rendering `config/rulebook.ts` into static HTML for the website
- [ ] Task: privacy and data section, derived from the app's `/privatliv` copy without weakening it
- [ ] Task: state on the page what it is and is not, and check task 204's fallback links here
- [ ] Task: verify on the oldest device available, and with JavaScript and CSS disabled
- [ ] Task: assert in a test that the page makes no authenticated request and contains no personal data

## 11. Open Questions

1. **Which of the four shapes is this?** Recorded because the choice is the PRD:

   | | shape | capability | cost |
   |---|---|---|---|
   | A | Apology page only *(shipped, task 204)* | none | ~0 |
   | **B** | **Anonymous static site — this PRD** | **the non-personal content** | **low** |
   | C | Server-rendered subset with login (Go + htmx) | most of the app, minus offline/push | high, and duplicated privacy rules |
   | D | Legacy ES5 build of the same SPA | looks complete | highest, and the worst failure mode |

   **Recommended: B.** C is the one that sounds right and carries the drift PRD 009 documented — a second
   renderer would need every privacy invariant re-enforced, including the `phoneParent` rule that `.rules`
   calls a hard rule rather than a per-feature judgement. D is worse than it looks: Tailwind v4 cannot be
   downlevelled to a 2018 Safari, there is no service worker, and shipping something that *looks* like the
   app but behaves differently is how a participant comes to believe the map works offline when it does not.

2. **Does the website ever need dynamic content?** Race-day updates are the candidate. Note the tension: if
   news appears on an anonymous page it is public, which may be exactly wrong for an operational message —
   and if it needs a login, it is not this site. Possibly the answer is that public notices and participant
   messages are two different things.

3. **Who owns the copy?** The rules are løbsledelsen's and authoritative (`rulebook.ts` says so). The
   programme and practical information have no owner in this repo yet.

4. **Is `/desktop.html` the right URL** for something that is no longer only about desktops? The name is now
   wrong in a way that will confuse the next reader, but renaming it touches PRD 005's gate, the fallback
   from task 204 and any link already shared. Cheap to do now, annoying later.

5. **Should the programme live in the app first?** If it does, the website can be generated from it, as with
   the rules. If it lands on the website first, the app will eventually copy it — the drift problem in the
   opposite direction.
