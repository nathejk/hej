# 194 — Prefetch user data quietly; never gate the map cache on a prompt

**Status:** done
**Priority:** medium
**Created:** 2026-09-01
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

## Description

**Rewritten 2026-09-01 on maintainer direction.** This was specified as an onboarding step that
asked the user to prepare everything for offline use, with a ~500 MB estimate. Three corrections,
each of which removes work rather than adding it:

1. **Not every role has the contacts pane.** Spejdere do not get it and are not in it (PRD 007), so
   a step promising "kontakter og portrætter" is wrong for the largest group of users, and a prefetch
   that ignores role would put a few hundred devices on a 403 loop.
2. **User data is small enough that asking costs more than the bytes.** Under ~1 MB for the largest
   role, index and thumbnails together (tasks 078, 104). A screen asking permission to spend a
   megabyte is a screen that trains people to tap past screens. So: **prefetch it, do not ask.**
3. **If the user does not want the whole map, the map still caches as they browse.** That already
   works and is unconditional (task 087's cheap half). It must stay that way — declining a bulk
   download, or never being offered one, must not switch off the caching that happens while somebody
   uses the map along the route.

There is a fourth point that changes *when* rather than *whether*: **details and photos churn most
early**, until everyone has added a portrait and checked their own record. A copy synced three weeks
out is the least accurate copy the device will ever hold. So the interesting work is not the first
fetch — it is catching up afterwards, on a device whose owner may never open `Kontakter` at all.

### What this means for the step

The step largely dissolves. Once the data is prefetched without asking, and the map caches itself
while being used, there is no decision left for a screen to present. The only thing that would
warrant one is the **bulk map download** — a genuine ~324 MB choice — and that is task 087's second
half, which does not exist yet.

So: no onboarding step, and PRD 005's slot stays reserved for the bulk map action when 087 lands.
A step whose entire content is "we have done something for you" costs a tap and earns nothing.

## Acceptance Criteria

- [x] The directory and its portraits are prefetched **without a prompt**, for roles that have the
      pane, and not at all for roles that do not.
- [x] A spejder generates **no** contacts request from this path — gated client-side on role via
      `hasContactsPane`, with the BFF's `403` as a second stop.
- [x] The catch-up runs at app level (`useQuietPrefetch` in `App.vue`), not only inside `Kontakter`.
- [x] It adds **no continuous traffic**: `intervalSeconds: 0`, so foreground and reconnect only.
      Asserted by a test that no interval is started.
- [x] Portraits stay lazily fetched with the rows that show them. No bulk portrait prefetch.
- [x] Opportunistic map caching is verifiably not conditional on any prompt, setting or consent flag
      — `tileCaching.spec.ts`.
- [x] PRD 005's step-7 slot and PRD 009 §7 updated to describe what actually happens.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
- 2026-09-01 — **Rewritten before implementation**, on maintainer direction: prefetch user data
  without asking, keep the role gate, and never make the browse-time map cache depend on a prompt.
  The size estimate and the consent screen that motivated the original step apply only to the bulk
  map download, which is not built. Recorded above rather than quietly dropped, because the deleted
  screen is the interesting part of the decision.
- 2026-09-01 — **`useQuietPrefetch` reuses task 190's loop with `intervalSeconds: 0`.** That is not a
  trick: zero already means "keep the event-driven checks, drop the timer", which is exactly the
  behaviour wanted here. Foreground and reconnect catch a device up; the during-race interval stays
  scoped to the pane, where somebody is actually reading the result. First use of the extracted
  convention by a second consumer, which is what it was extracted for.
- 2026-09-01 — **`refreshIfStale` does double duty**: it fetches outright when nothing is held and
  otherwise costs one small version request. So the first launch prefetches and every launch after is
  a cheap check — no separate "do I have it yet" branch to get wrong.
- 2026-09-01 — **Role gate is client-side and it matters at scale.** Without it every spejder device
  asks, on every foreground, for a directory the BFF will always refuse. `hasContactsPane` mirrors
  `users.MayUseContacts`; a null or unknown role answers false, because prefetching on a guess is
  worse than not prefetching.
- 2026-09-01 — **No bulk portrait prefetch, and the churn is the reason.** Maintainer: photos and
  details change most at the beginning, until everyone has added a photo and checked their record. A
  bulk fetch three weeks out spends a participant's mobile data on bytes that are stale before the
  race. The index makes the pane usable; faces fill in as it is used, and `ContactRow` already
  distinguishes "no photo" from "not on this device".
- 2026-09-01 — **The map guard is a structural test, not a comment.** "Still cache while browsing" is
  easy to agree with and easy to break: the plausible next change is a "spar data" toggle, and wiring
  the runtime cache to it would look like a courtesy while removing the only offline map most
  participants will ever have. Nobody declining a 324 MB download is asking for the tiles they are
  *looking at* to be discarded. `tileCaching.spec.ts` asserts the route consults no preference,
  consent flag or data-saver setting — and strips comments first, for the reason `layout.spec.ts`
  gives: the config documents why it avoids `purgeOnQuotaError`, and a naive scan flags the
  explanation as the offence it warns about.
- 2026-09-01 — ✅ All criteria complete. 10 new tests (prefetch 6, tile caching 4); suite 323 across
  27 files; `type-check` and `build` clean. PRDs 005 and 009 updated, including closing §11.6 — the
  "skipped first sync" question largely dissolves once nothing is asked.
