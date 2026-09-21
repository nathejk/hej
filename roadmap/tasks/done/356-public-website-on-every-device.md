# 356 — The public website is for every device; the wall is only for app pages

**Status:** done
**Priority:** high
**Created:** 2026-09-21
**Picked up by:** agent session (Zed)
**Started:** 2026-09-21
**Completed:** 2026-09-21

## Description

Maintainer clarification, 2026-09-21, while task 355 was being written down:

> this repo has two parts, a public website and an install-only PWA — the PWA and all pages
> from the PWA should only work when installed to home screen. The thing that changes now is
> that all devices should be able to access the public website.

The first half is unchanged and stays unchanged: **every app page is install-only** (PRD 005,
task 143), and there is no login outside the installed app. The second half was not quite true.

### What was wrong

`router/gates.ts` step 3 sent **every** mobile, non-standalone navigation to `/install`. A
desktop visitor got the website (step 2), but a phone in Safari typing `hej.nathejk.dk` got an
add-to-home-screen wall — even though the thing they had come for, the public site, works
perfectly in that browser.

`/` is the address people type and the address that gets shared. Answering it with a wall
pushes the app at a visitor who only wanted to read a patrol page.

### The change

One branch, in `deviceAndInstallGates()`:

- Arrived at the **root** in a browser → `LEAVE_APP`, i.e. the website, exactly as a desktop
  does.
- Asked for an **app page by name** (`/kort`, `/sos`, `/profil`, …) in a browser → the install
  wall, as before. Those pages are install-only and saying so is honest.
- `/install` itself stays reachable, which is what the website's install invitation links to.

Two details worth keeping:

- `/` is a **redirect to `maps`** in the route table, so by the time the guard runs the root is
  only visible as `to.redirectedFrom`. A test pins that; verified by breaking it — dropping the
  `redirectedFrom` check fails exactly that test and nothing else.
- **This cannot move to the server.** The root is also the installed app's `start_url`, and no
  request tells the BFF whether it came from a home screen. So a mobile browser at `/` still
  loads the bundle before leaving — one wasted round trip, which the root move in PRD 021 is
  what actually fixes.

## Acceptance Criteria

- [x] A mobile browser at `/` lands on the public website
- [x] A mobile browser asking for an app page still gets the install wall
- [x] An installed launch (start_url `/`) is unaffected
- [x] `/install` is still reachable in a browser, for the website's install invitation
- [x] The redirect chain still terminates for every device/install/onboarding/session
      combination (the exhaustive fixpoint test)
