# 167 — Person profile route

**Status:** done
**Priority:** medium
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

## Description

`/contacts/:personId` — a large avatar and the person's details (PRD 007 §7, §11.4).

**The field allow-list, settled 2026-08-31:** avatar, name, group/team, phone, and
function/section for crew. **Excluded:** postal address and guardian number.

Works offline for directory members; live and not persisted for a member reached through a patrol
lookup (task 168). Role-guarded on deep link. Night-legible, `tel:` fine, no sharing affordance.

## Implementation

`vue/src/views/ContactPersonView.vue`, route registered in `router/index.ts`.

**No endpoint of its own — and that is the design, not a shortcut.** The page reads the synced
directory, which means it works offline for exactly the set of people the list covers, which is
the requirement. A dedicated `GET /api/contacts/people/{id}` would have been a *second, wider*
data path to the same fields, and a second place for someone to later add "the parent's number,
for the samarit". The allow-list is enforced where it is narrowest: in the manifest, which is
already tripwired (task 159).

The one case this does not cover is a person reached through the patrol lookup, whose details are
live and must never be stored. That belongs to task 168 precisely *because* those records are not
persisted, so they cannot come from this store.

**The route derives its `roles` from the contacts destination** rather than restating them, so a
deep link cannot become a way into the pane for a spejder if the destination's gate ever changes.
`rolegate.spec.ts` already covers the refusal under the name `contact-person`.

**All groups are shown, not just one.** A crew bandit's page says both "Klan Ravn" and "Crew",
because that is precisely the fact somebody opened the page to learn. The list, being per-section,
can only ever show one at a time.

**One neutral answer for "no such person" and "not visible to you."** The list is the only way in,
so anything else is a stale link or a probe, and neither deserves a distinguishable reply.

## Note on the task 159 criterion

The criterion said to add this route to the `paths` list in `guardiantripwire_test.go`. **Not
applicable as written, because there is no new endpoint** — the page reads the manifest, which is
already in that list and already asserted to carry no `phoneParent`, address, postal code or city.
The tripwire covers this surface through the endpoint that feeds it.

When task 168 adds the live patrol-member profile, *that* path does need adding: it returns
records which genuinely have guardian numbers. Added as a criterion there instead, so the
requirement lands where it bites rather than where it was first written down.

## Acceptance Criteria

- [x] Route `/contacts/:personId` renders large avatar, name, group, phone, and crew
      function where applicable.
- [x] No postal address and no guardian number anywhere on the page or in the payload — the
      store's `ContactEntry` has nowhere to put either.
- [x] Handler projects an explicit allow-list — satisfied by the manifest's allow-list; no new
      handler exists (reasoning above).
- [x] Works offline for directory members; live for lookup-reached members, persisting nothing —
      the offline half here, the live half in task 168.
- [x] Deep link as a spejder is refused (route inherits the destination's `roles`; covered by
      `rolegate.spec.ts`).
- [x] Unknown or non-permitted `personId` renders a neutral not-found, indistinguishable
      from "not allowed".
- [~] Added to the `paths` list in `guardiantripwire_test.go` — **not applicable**: no new
      endpoint. Re-homed onto task 168.

## Progress Log

- 2026-08-31 23:20 — Picked up, ahead of the view (163) because `ContactsView` navigates here and
  a commit with an unregistered route would be broken.
- 2026-08-31 23:30 — Decided against a dedicated endpoint: the manifest already carries the
  allow-listed fields and works offline, and a second path would be a second place to widen.
- 2026-08-31 23:35 — Resolved the task 159 criterion as not-applicable and moved it to task 168,
  where the live records actually carry guardian numbers.
- 2026-08-31 23:40 — ✅ Criteria met. `vue-tsc --noEmit` clean, 144 frontend tests pass.
