# 085 — Update the location consent copy to cover recording and upload

**Status:** done
**Priority:** high
**Created:** 2026-08-26
**Picked up by:** agent
**Started:** 2026-08-26
**Completed:** 2026-08-26

## Description

PRD 002 §11.1 changed what the app does with a member's location. It is no longer read
locally to draw a marker — it is **recorded, uploaded to the organizers, and retained
indefinitely**. The consent copy has to say so.

The existing pre-prompt (`MapsView.vue`, and PRD 005's onboarding) currently says:

> Vi bruger din placering til at vise dig på kortet under løbet. Du kan altid slå det fra
> igen.

That was accurate when it was written. It is now a narrower description than the actual
use, and asking for a permission under a narrower description than the use is the specific
thing to avoid — particularly here, where most of the people granting it are minors and
the number being recorded is their physical location through the night.

## Scope

Copy and where it appears, not mechanism. Small task, high consequence.

## Acceptance Criteria

- [x] The location pre-prompt states that the route is recorded and sent to the
      organizers, and roughly why (safety/analysis, and the team can see its own track
      afterwards — which is the part that makes it feel like a feature rather than
      surveillance)
- [x] Copy is Danish and consistent with the rest of the app's voice
- [ ] Updated in **both** places it is asked for: the map view's pre-prompt and PRD 005's
      onboarding step — they must not drift into two different promises.
      *The map half is done. PRD 005's onboarding does not exist yet (still in `draft/`), so
      there is no second place to update. The shared `PermissionPrompt` component now carries
      the link, so onboarding gets it by using the component — but PRD 005 must still write
      its own message text, and §6 of that PRD should require it.*
- [x] The profile or settings surface makes it discoverable *after* onboarding, so consent
      granted once at 22:00 is not the only time it is ever visible
- [ ] Wording reviewed by the maintainer before it ships — this is a promise to
      participants and their parents. *Drafted, not yet reviewed.*

## Progress Log

- 2026-08-26 — Task created from PRD 002 §11.1.
- 2026-08-26 — Done, per the maintainer: *"create it and link it from the profile, the use and
  intro to this app will be followed by educational talk in the start area."*

  **Two layers, because the prompt cannot carry the whole story.** The pre-prompt message is
  now honest about the thing that changed — *"Appen viser dig på kortet og gemmer din rute, som
  sendes til arrangørerne"* — and links to a fuller page rather than trying to explain
  retention in two lines on a phone in the dark. `PermissionPrompt` gained optional
  `moreTo`/`moreLabel` props so any prompt that asks for more than it can describe has
  somewhere to point.

  **The fuller page is `/privatliv` ("Data og privatliv")**, covering location, portrait,
  profile data and retention in plain Danish. Written to be readable by a 12-year-old *and* a
  parent, and deliberately not legalese — a text nobody reads is not consent.

  **It is in the navigation, not only linked from the prompt.** The maintainer asked for it on
  the profile, and PRD 003's profile page does not exist yet (still `draft/`). Waiting would
  have left the page reachable *only* from a prompt — which makes it unreachable the moment
  someone taps "Ikke nu", and this is precisely the page a worried parent goes looking for
  afterwards. So it is a nav destination now (landing in the overflow sheet; the bottom nav is
  unchanged at 4 primary slots plus "Mere"), and PRD 003 should link it from the profile when
  that lands.

  **On the start-area talk.** It is recorded as reinforcement, not as the consent mechanism,
  because of the sequencing: the permission is granted during onboarding, *before* anyone
  reaches the start area. The written copy therefore has to stand on its own at the moment of
  asking — which is what this task delivers — and the talk then goes deeper with a room full
  of participants.

  Worth flagging once: **the talk reaches participants, not parents.** For a location track of
  minors kept without an end date, `/privatliv` is the only account a parent will ever see. That
  is the argument for it being discoverable and plainly written rather than buried, and it is
  why the retention section says "we have not set an end date yet" instead of implying one.

  Two criteria left unticked with reasons above: PRD 005's onboarding has no step to update
  yet, and the wording has not been reviewed — **it should not ship without you reading it.**

  `vue-tsc` and `npm run build` clean; all four changed modules transform through the dev
  server (200). Precache 24 → 26 entries, +5 KiB.

## Files

- `vue/src/views/PrivacyView.vue` (new) — the fuller explanation
- `vue/src/config/navigation.ts`, `vue/src/router/index.ts` — the `/privatliv` destination
- `vue/src/components/PermissionPrompt.vue` — optional link to a fuller explanation
- `vue/src/views/MapsView.vue` — honest location prompt copy + the link
