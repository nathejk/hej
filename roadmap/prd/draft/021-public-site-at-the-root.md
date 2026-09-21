# PRD 021 — The public site at the root, and two binaries

**Status:** draft
**Author:** agent session (Zed)
**Created:** 2026-09-21
**Last updated:** 2026-09-21 (deferred — see §0)
**Approved:**
**Shipped:**
**Target users:** everyone who reaches `hej.nathejk.dk` without the app — parents, grandparents, participants on unsupported devices, anyone following a shared link — and, indirectly, the participants whose app becomes a separately deployable thing

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 0. Decision: deferred, with an interim step (2026-09-21, maintainer)

> *"we will put the public pages in a subfolder /2026 for now, and do the complete directory mangling in a
> few months when we have very little traffic."*

**This PRD stays in `draft/` and is not being implemented now.** Two decisions, and both are better than what
this document proposed:

**1. The full restructure waits for a low-traffic window — months, not weeks.** §2 argued for "now, post-event"
on the grounds that a stranded install self-heals cheaply. That reasoning is not wrong, but it weighed the
*install* cost and ignored the *attention* cost: the week after an event is when families are actually looking
at these pages, and it is also when the people who would have to fix a botched service-worker handover are
least available. A quiet month is strictly better for a change whose hardest part is a migration nobody can
fully predict (§8). Nothing in this PRD expires in the meantime.

**2. The public pages move to `/2026` in the meantime.** A small, reversible step that takes the public surface
out of `/offentligt` without touching the app's URL, its manifest, or its service-worker scope — so it carries
**none** of Phase 0's risk.

It is also a better interim address than `/offentligt` for a reason worth recording: **it is year-scoped.**
These pages are a record of one event, and an album or a patrol page is only meaningful with a year attached.
That prefigures the eventual structure rather than fighting it — the root site will want to be indexed by year
(`/2026/patrulje/42`, `/2027/...`), so this interim move is a step along the final path instead of a detour.

What the interim step still requires, none of it risky:

- ~~Permanent redirects from every `/offentligt*` path, which were already required by §6.~~ **Not needed
  (2026-09-21):** those URLs were never published anywhere, so there is nothing to keep compatible with and
  the paths were deleted outright. Worth recording because §6 below still lists the redirects as a
  requirement for the *eventual* move — and there the argument does hold, since `/2026` links will have been
  shared by then.
- The app's `navigateFallbackDenylist` swaps its `/offentligt` entries for the year prefix, **matched by
  shape** (`/^\/\d{4}/`) so next year needs no frontend change. The list survives until the full move deletes
  it — and task 332's bug (a missing bare-path entry silently serving the app shell) is the thing to check
  for on the way past.
- A request to an unknown path *under* the year prefix must get the public site's not-found page, not the app
  shell — the same failure, one directory along.
- The year in the path should be **validated against the configured event year**, not hard-coded, so next
  year's deploy needs no code change and a stale `/2026` link cannot quietly render 2027 data.
- **The app has to name the public site without knowing the year.** It cannot be told: the year is runtime
  configuration on the BFF, and the desktop gate decides during the router's first navigation, before
  anything is fetched. Task 351 derives it from the calendar — the same default the server uses — and accepts
  a bounded degradation if somebody overrides `EVENT_YEAR`: the visitor lands on the public site's own
  not-found page, which links onward, and cannot bounce back into the app because every year-shaped path
  belongs to the public site. **The full move to `/` removes this problem entirely**, which is one more small
  argument for it.
- PRD 011's §6 records `/offentligt/...` as the addresses; it needs updating when this lands.

See §10, Phase −0, for the task. Everything else in this document describes the end state and is unchanged.

## 1. Summary

Move the **public website to the root** of `hej.nathejk.dk` and the **installed app to `/app`**, then split
the Go BFF into **two binaries in the same repo**: `cmd/api` (the app's BFF, which also runs the projections)
and `cmd/public` (the public website, read-only).

This is a structural change, not a feature. Nothing a visitor can see is new; what changes is which URL serves
what, and which process — and therefore which *address space* — the public surface runs in.

## 2. Problem & Motivation

- **What problem does this solve?** Maintainer direction, 2026-09-21:

  > *"The site we are building now (photoalbum, patrolpage and public glimt) is not the same as the app. This
  > site should be available in the root of the domain hej.nathejk.dk, where as the app can be served from
  > /app. the two parts web+app are heavily related, but then again not so much. […] Key features needs to be
  > maintainability over time, and performance"*

  Three concrete problems sit behind that:

  1. **The front door is the wrong door.** `hej.nathejk.dk/` serves an app a browser visitor cannot use: the
     app is install-first (PRD 005), its login lives inside the installed app (task 143), and on a desktop
     `main.ts` redirects to `/desktop.html`. So the most memorable URL we own answers a parent with a dead
     end, while the thing they want — PRD 011's albums, patrol pages and glimt — is hidden under
     `/offentligt`.
  2. **The boundary between the two is maintained by a list.** `vite.config.ts`'s
     `navigateFallbackDenylist` enumerates the public paths the app's service worker must not swallow:
     `/desktop.html`, `/offentligt`, `/offentligt/`, `/api/`. Task 332 shipped a bug because the bare
     `/offentligt` was missing from it — an installed member following a link to the public frontpage got the
     app shell. Every public page added grows that list, and the failure mode is silent.
  3. **One process serves both, and it cannot be replicated.** `docker-compose.prod.yml` says so at length:
     the service runs projections, `jrgensen/stream` subscriptions have no queue group, so two instances
     would both apply every event to the same read model — *"DO NOT `docker compose up --scale hej=2`"*. It
     also names the way out: *"split the projectors into their own single-instance process, leaving the API
     stateless and freely replicable."* The public site is the one part of this system with a burst load
     profile (PRD 011 §8: a hundred simultaneous visitors is the *normal* case the morning after), and it is
     currently welded to the one part that must stay single-instance.

- **Why now?** **The 2026 event finished on 2026-09-20.** Moving the app's URL breaks installed home-screen
  instances until their service worker updates (see §8), so the cost of this change is a function of *when*
  it happens:

  | when | what it costs |
  |---|---|
  | now, post-event | a handful of installs self-heal on next launch; nobody is mid-race |
  | next August | a broken home-screen icon during onboarding, when install-first makes that icon the only way in |
  | during an event | unacceptable |

  There is one other timing argument: PRD 011 is nearly complete (17 of 21 tasks), so the public site exists
  and is worth putting at the root. Doing this before it shipped would have been speculative; doing it a year
  after means moving URLs people have shared.

- **Evidence.**
  - `vue/vite.config.ts`: `start_url: '/'`, `scope: '/'`, and the four-entry denylist.
  - `docker-compose.prod.yml` lines 232–265: the no-replicas rule and its stated remedy.
  - **The boundary already leaks in the copy.** The public site's footer links to `/privatliv`
    (`publicsite.go`, `glimtpublic.go`) — which is a *Vue route inside the app*. A parent on a laptop who
    clicks "Data og privatliv" on a patrol page today gets the app shell, which redirects them to
    `/desktop.html`, which says *"more to come…"*. The public surface links into a place its audience cannot
    go, and nothing catches it because both are served by one binary at one origin.
  - PRD 011's own privacy work (tasks 337, 338): the invariant "the public surface names no person" is held
    up by `requireAuth` being the only place a session enters the request context, plus a test that parses
    `routes.go`. Discipline, not structure.

## 3. Goals

- The root of `hej.nathejk.dk` serves something **any** visitor on **any** browser can read and use.
- The public surface's privacy invariant becomes **structural**: the process serving it has no session secret,
  no access to names or guardian numbers, and no code path that could acquire them.
- A burst on the public site cannot degrade the app **during an event**, when the app carries SOS, dispatch
  and contacts.
- The public site can be **replicated and cached** independently; the app keeps its single-instance
  projections without holding the public site hostage.
- Deploying one side does not interrupt the other. (Today `up -d` recreates stop-first: *"a few seconds of
  502"* — for both surfaces at once.)
- Every URL anyone has already shared keeps working.
- Maintainability measured as: **fewer cross-boundary rules to remember**, not fewer files.

## 4. Non-Goals

- **A second frontend application.** The public site is server-rendered Go templates plus one ~200-line
  vanilla island (PRD 011, task 342). That is the right shape for a page whose virtue is working with
  JavaScript off on a ten-year-old browser, and a second Vue app would reintroduce exactly the cost the
  current design avoids. No SPA, no bundle, no framework on the public side.
- **A second repository.** One Go module, one repo, two `main` packages. `internal/*` and `nathejk/table/*`
  stay shared.
- **A second database.** Both processes read one MariaDB. A read replica is a later option, not part of this.
- **Moving the projections into their own process.** The documented end state, and a bigger change; `cmd/api`
  keeps them. This PRD only requires that `cmd/public` **must not run any**.
- **A public-site sign-in.** Named by the maintainer as a likely future (*"probably with a signin of it's own
  at some point"*) and deliberately excluded: it would put a new population into the system and deserves its
  own PRD. This PRD only avoids foreclosing it.
- **New public features.** PRD 011 owns the content; this moves it.
- **Redesigning the app.** Its look, routes and behaviour are unchanged apart from the URL prefix.

## 5. User Stories & Scenarios

- As a **grandparent sent a link**, I want `hej.nathejk.dk` to show me the photographs and the patrol's page,
  so that I do not have to be told which sub-path to type.
- As a **parent on a laptop**, I want to read what the event does with my child's data without installing
  anything, so that "Data og privatliv" is a page and not a dead end.
- As a **participant with the app installed**, I want my home-screen icon to keep opening the app, so that a
  change I never asked for does not cost me my login.
- As a **maintainer**, I want the public site to be a process that *cannot* read a session, so that the
  privacy rule survives the next handler somebody adds in a hurry.
- As an **organizer during the race**, I want a link going round a parents' group chat not to slow down the
  samarit looking up a patrol.

### Primary path

1. A visitor opens `hej.nathejk.dk`. The public binary serves the frontpage: albums, find-your-patrol, glimt.
2. They follow `hej.nathejk.dk/patrulje/42`. Server-rendered, cached 60 s, no bundle.
3. A participant taps their home-screen icon. It opens `hej.nathejk.dk/app`, the app's service worker serves
   the shell from cache, and nothing about their session changed.
4. An old link — `hej.nathejk.dk/offentligt/patrulje/42` — answers **301** to `/patrulje/42`.

### Edge cases

- **An installed app from before the move.** Its `start_url` is `/` and its service worker is registered at
  scope `/`. See §8; this is the one genuinely hard case and it is settled by a device test, not by argument.
- **A visitor with JavaScript disabled** gets the whole public site, as today, and `/app` gets the app's
  existing unsupported-browser page rather than a blank screen.
- **A deploy of the app while the public site is being shared.** The public site keeps answering.
- **The public binary loses the database.** It must degrade the way the pages already do (PRD 011: "not
  available right now" rather than a stack trace), and must not answer as though a patrol does not exist —
  that distinction is already handled by `openPatrol` and must survive the move.
- **Two processes, one blob store.** Album media and glimt thumbnails are content-addressed files. The public
  binary needs read access to the same `BLOB_PATH`; it must never write there.

## 6. Requirements

### Functional

- [ ] `/` serves the public frontpage. `/album/:slug`, `/patrulje`, `/patrulje/:number`,
      `/patrulje/:number/anmeld` and `/glimt` serve what `/offentligt/...` serves today.
- [ ] Every old `/offentligt*` path answers **301 Moved Permanently** to its new location, permanently — those
      URLs were shared in family group chats and in PRD 011's own copy.
- [ ] The app is served from `/app` and `/app/*`, including its client-side routes on a hard reload.
- [ ] `/` no longer falls through to `index.html`. A typo'd public URL gets the public site's own answer, not
      the app shell. (Today `router.NotFound` serves the shell for everything unknown.)
- [ ] The public site has its **own** privacy page, server-rendered, and its footer links to that rather than
      into the app. The app keeps its `/privatliv` view for signed-in members; the two must not contradict
      each other, so one of them is the source (see §11 Q3).
- [ ] `/desktop.html` answers 301 to `/`, and PRD 013's content (rules, programme, practical information)
      becomes pages on the public site. **This PRD supersedes PRD 013's delivery mechanism**, not its content.
- [ ] `cmd/public` serves the public site and `/api/public/*`. It registers **no cqrs consumers** and holds no
      session secret.
- [ ] `cmd/api` serves the app, `/api/*` and the projections, exactly as today minus the public routes.
- [ ] Traefik routes by path prefix to the two services, with `/api/public/` taking priority over `/api/`.
- [ ] The dev stack routes the same way it does in production, so a path that works in dev works deployed.
- [ ] The session cookie is **not sent to the public site at all** — `Path=/api` rather than `Path=/`. The app
      shell is not secret and authentication happens over `/api`, so nothing needs it on a navigation.
- [ ] Installed instances from before the move end up in the app, without a member having to reinstall or log
      in again. Verified on a real iPhone and a real Android device, not reasoned about.
- [ ] The public binary's static assets (vendored Leaflet, `publicmap.js`, its stylesheet) ship **with that
      binary**, so it has no dependency on the SPA's build output.

### Non-Functional

- **Privacy (the point of the exercise).** `cmd/public` must not import the packages that can read a person's
  name, phone or `phoneParent`. A test asserts the import graph, so the property is checked by the compiler's
  view of the world rather than by review.
- **Performance.** Public HTML stays server-rendered with `Cache-Control: public, max-age=60` and media at a
  year immutable (task 347). `cmd/public` must be **replicable**: no in-process state that a second instance
  would contradict. Note its per-patrol track cache and single-flight (task 347) are per-process — correct
  under replication, just less effective.
- **Availability.** Deploying either service must not interrupt the other. The public site must survive the
  app being down entirely, and say so gracefully if it cannot.
- **Reach.** The public site keeps working with JavaScript disabled, on the oldest device available, as PRD
  011 and PRD 013 require. The restructure must not quietly make the root depend on script.
- **Security.** Two processes, two sets of credentials: `cmd/public` gets a database user with **SELECT only**
  (plus the broker publish it needs for takedown reports), so a hole in the public surface cannot write.
- **Observability.** Separate health endpoints and logs, or an incident on one side is invisible behind the
  other's noise.

## 7. UX / UI Notes

Visually, nothing changes: the public pages keep PRD 011's layout and the app keeps its shell. What changes is
navigation *between* them:

- **The public site gets a way into the app** — a plain link ("Åbn appen") for participants who arrive at the
  root, honest about the install-first flow rather than pretending a browser can use it.
- **The app keeps no link back** except where one already exists (the footer's rulebook link), because a
  member inside the app has no reason to leave it.
- **Old app URLs a member has bookmarked** (`/glimt`, `/kort`) now belong to the public site or to nothing.
  `/glimt` is the sharp one: it is a public page *and* an app route. Under this scheme the public glimt page
  owns `/glimt` and the app's is `/app/glimt`; a bookmark to the old `/glimt` lands on the public page, which
  is a reasonable answer for anyone and the wrong one for a member. Accepted rather than solved — the app is
  reached from its icon, not from a bookmark.
- Frontend routes affected: every app route gains the `/app` prefix via `import.meta.env.BASE_URL`, which
  `createWebHistory(import.meta.env.BASE_URL)` in `router/index.ts` already honours. No view changes.

## 8. Technical Considerations

### The service-worker handover — the one hard part

An installed instance has `start_url: '/'` and a service worker registered at **scope `/`**, which controls
navigations to `/`. So after the move, a home-screen launch goes to `/` and the *old* worker answers it from
its precached shell — the member keeps getting the old app until that worker updates. This cannot be fixed
with a redirect, because the redirect never reaches the network.

The intended mechanism, to be **verified on devices rather than trusted**:

1. Ship a **self-destroying service worker at the root scope** (`vite-plugin-pwa` has a `selfDestroying`
   option built for deprecating a PWA): it unregisters itself and clears its caches on activation. The browser
   checks `sw.js` on navigation, so an online launch picks it up.
2. The app's manifest moves to `start_url: '/app'`, `scope: '/app/'`, and the app registers a fresh worker
   there.
3. For a transition window, `/` carries a **minimal standalone-detect redirect** to `/app` — the only script
   on the public root, narrowly scoped, with a removal date. An installed launch that reaches the network
   therefore lands in the app; a browser visitor never sees it fire.
4. Net effect for an existing install: the old app once or twice, then the correct one. An offline-only launch
   keeps the cached old shell until it next has network, which is acceptable and must be stated in the task.

**Risks:** iOS is the one that matters (PRD 005's install-first flow is an iOS constraint), and iOS has
historically handled scope changes and `start_url` updates poorly. If the device test says the handover does
not work, the fallback is to keep `/` serving the app for one more event cycle and put the public site on
`/web` — ugly, and better than a broken icon. **The device test gates this PRD's approval into `doing/`.**

Everything that currently reasons about `start_url: '/'` must be revisited: `router/gates.ts`,
`config/runtime.ts`, `dev/devDevice.ts` (all three have comments about an installed launch dropping the query
string), `helpers/resumeProbe.ts`, and the `navigateFallbackDenylist` — which **disappears**, since the app's
scope no longer covers the public paths. That deletion is the maintainability win made concrete.

### Two binaries

- **`cmd/api`** — the app's BFF *and* the projector. Unchanged except that the public routes leave. Stays
  single-instance for the reason `docker-compose.prod.yml` documents.
- **`cmd/public`** — the public website and `/api/public/*`. **Read-only by construction:** it registers no
  consumers, so it cannot write a projection; its database user has SELECT only; its one write is publishing a
  takedown report to the broker (task 343), which `cmd/api`'s projector folds. Freely replicable.
- **The trap this avoids:** two processes both folding the same subjects into the same tables. Subscriptions
  have no queue group, so both would receive every message and both would replay from sequence zero on boot —
  *"fine for a strictly idempotent projection, silently wrong for an unconditional UPDATE"*. The requirement
  "`cmd/public` registers no consumers" is what keeps that impossible, and it wants a test.
- **Shared code** stays in `internal/*` and `nathejk/table/*`. The public binary takes the narrow read models
  PRD 011 already built (`publicpatrol`, `album`, `trackpoint`, `glimt`) and **not** `person` — which is how
  "names no person" stops being a rule and becomes an absence.
- **`go:embed` for the public binary's assets**, rather than a `WEB_ROOT`. One file to deploy, no dependency
  on the SPA's `dist/`, and no way for a missing mount to serve a half-working site.

### API endpoints

No new endpoints, and **no changed paths under `/api`** — `/api/public/*` keeps its URLs and moves process.
The `@Router` annotations for the page routes change (`/offentligt/patrulje/{number}` → `/patrulje/{number}`),
and the redirects are new documented responses. Per `.rules`, every endpoint keeps its OpenAPI annotations;
the two guards that parse `routes.go` (`glimtopenapi_test.go`, `publicprivacy_test.go`) must **follow the
routes into `cmd/public`**, or they will silently stop covering them — which would be the worst possible
outcome of a change made for privacy reasons.

### Data / storage

No schema changes. Two consumers of one database with different privileges; one blob store, read-only from
`cmd/public`.

### Dependencies & risks

| risk | mitigation |
|---|---|
| Installed PWAs strand on the old scope | self-destroying worker + standalone redirect, **device-tested on iOS and Android before approval** |
| Deep links already sent (SMS, email, printed) | permanent 301s from every `/offentligt*` path |
| The privacy guards stop covering the moved routes | move the guards with the routes in the same commit; assert the import graph |
| Two deployments drift in config | one compose file, shared env block, both images built from the same Dockerfile |
| A shared database means isolation is partial | separate connection pools sized per service; replica later if measured |
| Dev and prod route differently | move dev routing to Traefik path rules instead of the Vite proxy |

## 9. Success Metrics

- **Zero** installed instances stranded: after the handover, a home-screen launch reaches the app on both test
  devices, and no member reports having to reinstall.
- **Zero** old public URLs broken: every `/offentligt*` path in PRD 011's copy and in the last year's messages
  answers 301 to a working page.
- **The session cookie never reaches the public site**: verified by request inspection and by a test, not by
  reading the code.
- **`cmd/public` cannot write**: its database user fails an `INSERT` (checked once, deliberately).
- **Two replicas of `cmd/public` serve correctly** — the property `docker-compose.prod.yml` currently forbids.
- **A deploy of `cmd/api` costs the public site no failed requests**, measured with a request loop across a
  deploy (today: a few seconds of 502 for both).
- **The denylist is gone**: `navigateFallbackDenylist` no longer needs a public-path entry, so no future public
  page can be swallowed by the app's worker.

## 10. Rollout / Task Breakdown

**Sequencing matters, and it is: URLs first, processes second.** All the user-visible risk is in the URL move;
doing it while still one binary keeps that risk isolated. The binary split then becomes a pure refactor with no
visible change — the safest possible second step. Doing them in one release would mean debugging a
service-worker handover and a new process boundary at the same time.

*Phase −0 — the interim move (§0), independent of everything below*

- [x] **351** — serve the public pages under the event-year prefix (`/2026`) instead of `/offentligt`, with
      permanent redirects, a year validated against the configured event year, a public not-found page for
      unknown paths under the prefix, and the service-worker denylist updated. **Done 2026-09-21**, and it
      also moved the app's desktop gate off `/desktop.html` and gave the public site its own privacy page —
      see that task's log for the two link-loops it found on the way.

*Phase 0 — answer the question that gates the rest*

- [ ] Task: device-test the service-worker handover on iOS and Android (self-destroying worker, scope change,
      `start_url`), and record what actually happens. **Approval into `doing/` depends on this.**

*Phase 1 — the URL move, one binary*

- [ ] Task: serve the public site at `/` and the app at `/app`; root stops falling through to the app shell
- [ ] Task: permanent redirects for every `/offentligt*` path and for `/desktop.html`
- [ ] Task: the app's `base`, manifest (`start_url`, `scope`), service-worker handover and the transitional
      standalone redirect; delete `navigateFallbackDenylist`'s public entries
- [ ] Task: revisit every place that reasons about `start_url: '/'` (`gates.ts`, `runtime.ts`, `devDevice.ts`,
      `resumeProbe.ts`)
- [ ] Task: narrow the session cookie to `Path=/api`
- [ ] Task: the public site's own privacy page; footer stops linking into the app
- [ ] Task: dev-stack routing by path in Traefik, matching production

*Phase 2 — the split*

- [ ] Task: `cmd/public` — move the public handlers and `/api/public/*`, `go:embed` its assets, register no
      consumers
- [ ] Task: move the OpenAPI and privacy route guards into `cmd/public`, and add an import-graph assertion
      (no `person`, no session)
- [ ] Task: compose and Traefik for two services, including a SELECT-only database user and per-service pools
- [ ] Task: prove the properties — two replicas, no session cookie at the public binary, no write possible,
      no failed public requests during an app deploy

*Phase 3 — fold in PRD 013*

- [ ] Task: move PRD 013's content (rules, programme, practical) onto the public site and close that draft

## 11. Open Questions

1. **Does the iOS handover actually work?** Phase 0 exists to answer it. If not: keep `/` on the app for one
   more cycle and put the public site at `/web`, or accept a one-time reinstall announced in advance. This is
   the only question that can change the shape of the PRD.
2. **`/app` or `/app/`?** A trailing-slash scope is what service workers want; a bare path is what a person
   types. Both, with one redirecting, is fine — but it should be decided once rather than per route.
3. **Which privacy page is the source?** The public site needs one that a parent can read without an account;
   the app has one that explains member-specific behaviour (glimt audiences, task 321). One document with two
   renderings, or two documents with a shared section? PRD 013's "content has one source" goal applies.

   **Both now exist** (task 351): `/2026/privatliv` is server-rendered with the public-surface wording lifted
   from `PrivacyView.vue`. That was forced rather than chosen — the footer's link into the app became a loop
   the moment the desktop gate pointed at the public site — so the question is now about *keeping them in
   step*, not about whether to build the second one. Until it is answered, prefer changing both.
4. **Does `/api/public/*` keep its path?** Keeping it means a Traefik priority rule separating it from
   `/api/*`. Moving it (say to `/data/*`) makes the routing trivially prefix-based and breaks the island's
   URLs, which are in a checked-in asset. Recommendation: keep the path, accept the routing rule.
5. **Does the public site get a CDN?** Everything it serves is `public` and cacheable, so it is the one
   surface where a CDN is straightforward. It also puts a third party between a visitor and a page about
   children, which is exactly the reasoning that made us vendor Leaflet rather than use a CDN (task 342).
6. **Where does the eventual public sign-in live?** Out of scope here, but the answer affects whether
   `cmd/public` can stay replicable: the app's PIN store and push subscriptions are per-process in-memory
   maps, which is precisely why `cmd/api` cannot be replicated. A public login must not repeat that.
7. **Does PRD 011 finish before or after this?** Recommendation: finish 011 first (three tasks remain, two
   blocked on a dev stack and one on artwork), so this PRD moves a complete surface rather than a moving one.
