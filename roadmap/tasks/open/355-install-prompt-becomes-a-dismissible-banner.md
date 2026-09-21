# 355 — After the race: the install wall becomes a dismissible banner

**Status:** open
**Priority:** medium
**Created:** 2026-09-21
**Picked up by:** —
**Started:** —
**Completed:** —

## Description

Maintainer decision, 2026-09-21, while finishing the public frontpage (PRD 011):

> We don't want to push the app so hard. After race we will reduce the install prompt to a
> closeable banner on top.

**Deliberately not done now.** It is a post-event change. The scope question it was written to
ask has since been **settled** by the maintainer, the same day:

> this repo has two parts, a public website and an install-only PWA — the PWA and all pages from
> the PWA should only work when installed to home screen. The thing that changes now is that all
> devices should be able to access the public website.

So it is **reading (a)** below: the banner belongs on the **public website**, and the PWA stays
install-only. Reading (b) is **not** wanted, and PRD 005 needs no amendment. Both are kept, because
the distinction is what makes the scope clear.

The access half of that clarification is already done — **task 356** stopped the install wall from
answering the root in a mobile browser. What remains here is the *invitation*: the website carries
a call-to-action box today (task 143), and after the race it should be a dismissible top banner.

### What exists today

`vue/src/views/InstallView.vue` is a **wall**, not a prompt. `router/gates.ts`
`deviceAndInstallGates()` sends every mobile, non-standalone navigation to `/install`
unconditionally, and the wall's only forward action is the website. That is not incidental
styling — task 143 made it deliberate:

> the website is anonymous, no login — login is only for pwa

So today there are exactly two modes: the anonymous public site, and the installed app. A
browser tab is not a degraded app; it is the website.

### The consequence that had to be decided first (now decided: (a))

A dismissible banner implies **something to dismiss it onto**. If the banner sits on top of
the app in a browser tab, then the app has to work in a browser tab — including **login**,
which task 143 removed on purpose and PRD 005 §6/§8 records as given up. Reinstating it is a
PRD amendment, not a task: it changes who can hold a session and where.

Two readings, and the difference is the whole size of the job:

- **(a) The banner lives on the public site.** Post-race the public site is the destination a
  link lands on, and a visitor reading a patrol page has no reason to be pushed into an app
  they no longer need. A dismissible top banner — *"Hej Nathejk findes som app"* → `/app` —
  is entirely within PRD 011/021's surface, needs no login change, and is small.
- **(b) The banner replaces the wall inside the app.** The app becomes usable in a browser
  tab, the wall stops existing, and login-outside-standalone comes back. This reverses task
  143 and touches PRD 005.

Reading (a) is what the sentence most likely means in the context it was said in, but it is
**not** what "the install prompt" names in this codebase, so it is written down rather than
assumed.

### Notes for whoever picks this up

- `install_gate` (`GET /api/config`, `env.go`) already switches the whole gate off at runtime,
  no redeploy. Turning it off post-race removes the wall — but leaves **no** prompt at all, so
  it is the blunt version of this task, available on day one if needed.
- A dismissal must be **per-browser and forgettable**: a `localStorage` key, and note the
  constraint recorded in `config/gates.ts` — an installed launch drops the query string, so
  nothing may depend on a URL parameter surviving.
- Whatever the banner is, it must not overlap the update banner (fixed top, z-60, see
  `App.vue`), and on the app's map it must not cover the edge arrows — the keep-out zones in
  `config/map.ts` exist because that bug shipped twice (tasks 264/276).
- PRD 021 (draft) is the natural home if this turns out to be (b): it already owns the
  public-site/app split and the `/app` prefix.

## Acceptance Criteria

- [x] Maintainer has chosen (a) or (b) — **(a)**, 2026-09-21; PRD 005 stays as it is
- [ ] The prompt is dismissible, and the dismissal persists per browser without a URL parameter
- [ ] Dismissing it leaves the user somewhere that works — no dead end, no screen that needs a
      login the browser cannot obtain
- [ ] The banner does not overlap the update banner or the map's edge arrows
- [ ] A test pins that a dismissed banner stays dismissed across a reload
