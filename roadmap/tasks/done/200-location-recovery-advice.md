# 200 — Recovery advice when location stays silent

**Status:** done
**Priority:** medium
**Created:** 2026-09-02
**Picked up by:** agent session (Zed)
**Started:** 2026-09-02
**Completed:** 2026-09-02

## Description

Third data point from the same iPad (2026-09-02): **a fresh install worked immediately.** Location was
granted and the map found the device, with no code change between the failing attempt and the working one.

That reframes tasks 197–199 honestly:

| hypothesis | status after this run |
|---|---|
| Location Services off device-wide caused the *first* silence | **Confirmed** (Apple Kort said so) |
| `getCurrentPosition` hangs in an installed iPadOS PWA as a rule | **Not supported.** A fresh install answers fine |
| The installed instance was left in a **stuck permission state** by asking while Location Services was off | **Best explanation.** Enabling Location Services device-wide did not clear it; reinstalling did |

So the likely story is per-origin state inside that installed app, not a WebKit code-path defect. The
sequence that produces it is ordinary and will happen at an event: a device arrives with Location Services
off, the app asks anyway during onboarding, and that installed instance is then unable to ask again — no
dialog, no error, for ever.

### What this means for task 199

The strategy race stands, but its motivating hypothesis is **unconfirmed**, and the log line it added is
how it gets confirmed or dismissed. It is defensible on its own terms — nothing chained off another call's
failure, the watch is what the map wants anyway, and the coupling it removed was real — but it should not
be described as *the* fix for this bug. The fix for this bug is telling the user what to do.

### Why the advice stops short of "reinstall"

Reinstalling clears the app's storage, which includes **an unshipped position track** — the one thing on
the device that exists nowhere else (PRD 009 §6). Telling a twelve-year-old in a car park to delete and
reinstall the app is advice that can silently destroy their own recorded route, and they cannot know that.

So the card gets the two cheap, safe steps, and the destructive one is documented for organisers instead,
with the precondition attached: get online first so the track ships.

## Acceptance Criteria

- [x] The `stuck` message names both: Stedtjenester for the device, and this app's own permission.
- [x] It suggests closing and reopening the app.
- [x] It does not suggest reinstalling — asserted by a test that scans every failure message for
      "slet", "afinstal", "geninstal" and "fjern app", so the advice cannot creep in later.
- [x] Recorded in `roadmap/offline-test-protocol.md` §6a as a four-step order for organisers, and as a
      **Finding** in PRD 002 §11 where the location design lives.

## Progress Log

- 2026-09-02 — Task created from the fresh-install result, which was the piece of evidence that separated
  "iPadOS cannot do this" from "that installation could not do this any more".

- 2026-09-02 — **The fresh install is the piece of evidence that separated two very different stories:**
  "iPadOS cannot do this" and "that installation could not do this any more". It is the second, which means
  tasks 198 and 199 were built on a hypothesis this run does not support. Recorded plainly rather than
  quietly left standing: 199's strategy race is still defensible on its own terms — nothing chained off
  another call's failure, the watch is the call the map wants anyway, and the coupling it removed was real —
  but it should not be described as the fix for this bug. The fix for this bug is telling the user what to
  do, which is this task.
- 2026-09-02 — **The advice deliberately stops one step short of what works.** Reinstalling recovered the
  iPad, and it is the one step the app will not suggest: it clears the app's storage, including a position
  track that has not been uploaded, which is the only data on the device that exists nowhere else. A
  participant in a car park cannot know that. So the card gets the two safe steps and organisers get the
  destructive one, with "go online and check nothing is pending first" attached.
- 2026-09-02 — Guarded by a test rather than a comment, because the pressure to add "prøv at geninstallere"
  will come from a real support conversation where it is the true answer.
- 2026-09-02 — **What remains genuinely unresolved** is whether the app should ask for location at all when
  the platform is not ready — asking while Location Services was off is what appears to have wedged the
  install. There may be no way to detect that beforehand: the Permissions API does not report it, and
  WebKit answers `prompt` for a granted permission anyway. Raised on PRD 002 §11 rather than guessed at.
- 2026-09-02 — ✅ Green: 2 new tests (29 in this store's spec), suite 386 across 31 files, `type-check` and
  `build` clean.
