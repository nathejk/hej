# 232 — PWA: attempt feedback, exhaustion, and the empty-number mode

**Status:** open
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

## Description

The client half of PRD 015, in `vue/src/components/onboarding/WelcomeStepConfirmProfile.vue`
and `vue/src/stores/profile.store.ts`.

Three behaviours:

1. **Remaining-attempts hint.** The existing 400 message ("De to cifre passer ikke til
   nummeret, vi har.") gains a hint on failures 1 and 2, driven by what the server reports
   (task 227 owns the count — the client must not count for itself).
2. **Exhaustion behaves like the skip path.** On the third failure the component emits `skip`
   and lets the member into the app, with a short kind explanation rather than an error: we
   will ask again next time, and they can tell their leader. It is not a failure state, and it
   must not look like one.
3. **Empty or blanked contact number opens the step directly in correction mode.** With
   `phone_parent: ""` (task 229) there are no digits to recall, so the recall UI must not
   render an empty number followed by two crosses. The step opens with the full-number field
   and the `+45` addon.

Also, and easy to miss: **after exhaustion a reload must not re-open the check.**
`confirmation_required` stays true — the member has not verified — so the check will be offered
again, but its endpoint now answers 409 (task 227). The PWA must treat that 409 as "carry on"
rather than surfacing an error.

**The Danish copy is part of this task, not a follow-up.** The remaining-attempts hint and the
exhaustion message need actual agreed wording, not paraphrases: they are the only two places in
the app where a member is told they got something wrong. Non-accusatory and written for a
13-year-old (PRD 005 §11) — not knowing a parent's number is expected, not a mistake. And no
copy anywhere may suggest the member's **own** number is an acceptable answer, which is the
whole reason task 229 exists.

Depends on tasks 227, 228 and 229 for the server-side behaviour it renders.

## Acceptance Criteria

- [ ] Failures 1 and 2 show a remaining-attempts hint; the count comes from the server response
- [ ] The third failure emits `skip`, lets the member into the app, and renders as an
      explanation rather than an error
- [ ] `phone_parent: ""` opens the step in correction mode with no recall input and no empty
      masked number
- [ ] A reload after exhaustion lands the member in the app: the 409 is handled as "carry on",
      with nothing shown that reads as a failure
- [ ] The Danish strings are agreed and recorded in this task's progress log, and none of them
      suggests the member's own number is acceptable
- [ ] Component and store tests cover all three failure counts, the exhaustion emit, the
      empty-number mode and the post-exhaustion 409

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
