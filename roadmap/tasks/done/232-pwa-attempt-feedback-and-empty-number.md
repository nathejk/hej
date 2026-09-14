# 232 — PWA: attempt feedback, exhaustion, and the empty-number mode

**Status:** done
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

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

- [x] Failures 1 and 2 show a remaining-attempts hint; the count comes from the server response
- [x] The third failure emits `skip`, lets the member into the app, and renders as an
      explanation rather than an error
- [x] `phone_parent: ""` opens the step in correction mode with no recall input and no empty
      masked number
- [x] A reload after exhaustion lands the member in the app: the 409 is handled as "carry on",
      with nothing shown that reads as a failure
- [x] The Danish strings are agreed and recorded in this task's progress log, and none of them
      suggests the member's own number is acceptable
- [x] Component and store tests cover all three failure counts, the exhaustion emit, the
      empty-number mode and the post-exhaustion 409

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
- 2026-09-12 — `HttpError` gained a `body` field. The attempt count is *data* in a failure
  response, and without it the component would have to parse Danish prose or count failures
  itself — a client-side copy of a server-side rule, cleared by every reload. Typed `unknown`,
  because a shared union of every error body in the app is a type nobody could keep true.
- 2026-09-12 — `contactCheckFailure(err)` in the store module reads the structured 400 and returns
  null for anything else, so an older BFF, a 429 or a 503 falls back to the caller's own message
  rather than being rendered as a failed attempt with an invented count.
- 2026-09-12 — `skipContactCheck()` **never throws**, including on 409 (already over) and on a
  network failure. Login is the only mandatory step, so no signal in a forest must not turn giving
  up into a dead end — the outcome is lost instead, which PRD 015 accepts.
- 2026-09-12 — The exhaustion path deliberately does *not* call the skip endpoint: the server
  recorded the outcome as part of the third rejection, so calling it would publish a second event
  for one give-up. Asserted by a test that reads the source between the branch and the emit.
- 2026-09-12 — **Agreed Danish copy** (criterion 5):
  - after miss 1: server's *"de to cifre passer ikke"* + *"Du kan prøve to gange mere — eller
    skrive et andet nummer."*
  - after miss 2: same, with *"Du kan prøve én gang mere — eller skrive et andet nummer."*
  - on the third: *"vi spurgte tre gange — du kan komme videre uden at bekræfte nummeret"* (from
    the BFF, so one wording rather than two), shown as the step ends rather than as an error.
  - no number on file: *"Vi har ikke noget nummer på en voksen for dig."*
  Each names what happens next instead of what went wrong, and the existing *"ikke dit eget"* in
  the correction hint is the only mention of the member's own number — as an exclusion. A test
  fails if that stops being true.
- 2026-09-12 — The attempts hint sits inside the `role="alert"` wrapper with the error, so a
  screen-reader user does not hear "det passer ikke" without "du kan prøve to gange mere", which
  is the worse half on its own.
- 2026-09-12 — 409 on `/confirm` now means "carry on" for four separate reasons (already
  confirmed elsewhere, double submit, check closed earlier in this session, number settled at
  check-in). Listed at the branch, because a future reader will otherwise assume it is only the
  first.
- 2026-09-12 — Also hid "Tilbage — jeg prøver de to cifre igen" when there is no number on file:
  it would return the member to a mode with nothing to type.
- 2026-09-12 — Tests: this project has no jsdom and no `@vue/test-utils`, so the component cannot
  be mounted. Split the coverage the way `offlineIndicator.spec.ts` does — rules as functions and
  store actions, plus structural assertions against the component source for the ones that only
  live in the template. Recorded here because "why is there no mount test?" is the obvious
  question. 10 new tests; 482 pass overall, `vue-tsc` clean.
