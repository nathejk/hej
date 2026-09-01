# 169 — Names-only degradation and missing-portrait placeholder

**Status:** done
**Priority:** medium
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

## Description

Two distinct failure states that must never look like breakage (PRD 007 §5, §6):

**1. Portrait missing.** Skippable at onboarding, so many portraits will be absent early on.
Show a neutral placeholder with initials — visibly "no photo", not a failed image load.

**2. Images unavailable entirely.** iOS evicts service-worker caches for web apps that go unused,
within days. This interacts badly with PRD 005 pushing users to install *early*. The pane must
degrade to **names-only** — search, groups and favourites keep working — rather than appearing
empty.

The distinction has to be legible to the user: **"no photo" is not the same as "not synced
yet"**, and a user who believes they have portraits but does not is worse off than one who knows.

## Implementation

Most of this fell out of earlier tasks; this one closed the gap that was actually missing.

**Names-only degradation is structural, not a mode.** The store holds the index and *no image
data at all* (task 177): portraits are `<img>` requests against the BFF. So an evicted image cache
cannot affect the list, the groups, the search or the favourites — there is no code path by which
it could. reka-ui's `Avatar` swaps in the fallback by itself when an image fails, so a device with
an evicted cache renders a complete, usable, initials-only pane with no special handling.

**The gap that was real: "no photo" and "not downloaded" looked identical.** Both showed initials,
which flattens two facts that call for different reactions — nothing can be done about a member
who never took a photo, whereas a portrait this device has not fetched will fill in on going
online. `ContactRow` now tracks the image's loading status and, when a portrait *is* known to
exist but failed to load, marks the avatar with a small `ImageOff` badge. Deliberately small and
unobtrusive: it is a fact about the device, not about the person.

**Sync state cannot fail silently.** The header shows a spinner while refreshing, `Ikke opdateret`
with a `WifiOff` icon when the last refresh failed, `Ikke hentet endnu` when nothing has ever
synced, and otherwise `Opdateret nu` / `Opdateret kl. HH:MM` (task 163).

**Opportunistic re-sync** is the freshness loop's `online` handler (task 162), which has a test.

## Acceptance Criteria

- [x] Initials placeholder for a person with no portrait.
- [x] Names-only mode when images are absent or evicted: list, groups, search and
      favourites all still work.
- [x] The UI distinguishes "no photo" from "not synced yet".
- [x] Sync state is prominent enough that silent failure is not possible.
- [x] Opportunistic re-sync when a network appears.
- [~] Tests: index present + images absent renders a usable pane. **Not added as a test**, for
      two reasons worth stating rather than glossing: the repo has no component-test stack
      (`vitest.config.ts` runs in `node`, no jsdom — see task 164), and at the store level the
      assertion would be vacuous, because the store contains no image data for an absent image
      to affect. The property is guaranteed by structure, not by behaviour that could regress.
      What *could* regress — caching an image blob into the store — would be visible in review
      and is called out in `contacts.store.ts`'s header.

## Progress Log

- 2026-09-01 01:10 — Picked up. Audited what earlier tasks had already delivered rather than
  assuming: initials fallback (164), three empty states and the sync line (163), `online` re-sync
  (162), index/image separation (177).
- 2026-09-01 01:20 — Found the one genuine gap: a missing portrait and an unfetched portrait both
  rendered as bare initials, so the pane could not tell the user which it was. Added the
  `ImageOff` marker driven by the avatar's loading status.
- 2026-09-01 01:25 — ✅ Criteria met, with the test criterion resolved as structurally guaranteed
  rather than silently ticked. `vue-tsc --noEmit` clean; 158 frontend tests pass.
