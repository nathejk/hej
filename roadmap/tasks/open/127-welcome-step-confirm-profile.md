# 127 — WelcomeStepConfirmProfile: masked guardian number

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 005 §5 step 2, §6 and §7. Add
`vue/src/components/onboarding/WelcomeStepConfirmProfile.vue`: the first-login step where a
member looks at the details Nathejk holds for them and acknowledges that the
parent/guardian emergency contact number can actually be reached.

**Spejder only.** `PhoneParent` exists on `spejder` and on no other population — confirmed
by the shared-go survey and by PRD 006's `person` projection, where it is nullable and null
means *not applicable* (PRD 005 §5, §8). Bandits, gøgler and crew therefore skip the
guardian confirmation entirely. They may still be shown their own details to check. The step
must **never** render an empty guardian field as though the data were missing: an
absent number is not a data-quality problem for a bandit, and presenting it as one produces
support calls and, worse, teaches organizers to ignore the real flag.

`profile.store` already exposes `details.phoneParent: string | null` from
`GET /api/me/profile` (PRD 003, shipped), so eligibility is readable client-side — but
whether the step is *required* is not. Per PRD 005 §8 the BFF computes
`confirmation_required` from "has verified" **OR** "has started the event"
(`types.MemberStatusRacing` onwards), and **the client must not reimplement that rule**.

## The masked number

The number is shown masked to its last two digits — `11 22 33 **` — as **text, never as an
editable field**. The user types the two digits that are *not* on screen into a 2-digit
`Input` with `inputmode="numeric"`, and ticks a `Checkbox` labelled *"Dette nummer kan
kontaktes i løbet af Nathejk"*. Both are required to advance (PRD 005 §6).

Masking the last two rather than the first digits is what makes this a recall check instead
of a copying exercise: a user who cannot complete it has discovered that the number on file
is not one they know (PRD 005 §11, 2026-08-25).

**It is a recognition device, not a confidentiality control.** PRD 005 §11 settled this on
2026-08-30: `GET /api/me/profile` still returns `phone_parent` in full to its owner, exactly
as PRD 003 shipped it, so a determined user can read the two hidden digits out of the network
response. That is accepted — nobody is being authenticated here, and the number is the user's
own guardian's, not a secret being kept from them. An earlier draft of the PRD required the
full number never to reach the client and called the alternative "theatre"; that framing is
withdrawn. Practical consequences for this task:

- Do **not** change PRD 003's endpoint, and do not add a masked-only variant of it.
- Do **not** build, comment or document this step as tamper-proof. Someone will read those
  comments later and treat the flag as an identity check, which it is not.
- The digits are still verified server-side (task 135), so the acknowledgement is recorded
  against a real answer rather than against a shrug.

## The copy

The step must explain **both** reasons the number matters (PRD 005 §6, §11 2026-08-25):
Nathejk must be able to reach an adult **if something happens**, *and* **if the member
resigns mid-event and a pickup has to be arranged**. "I nødstilfælde" alone understates how
routinely the number is used and invites exactly the shrug this step exists to prevent — a
14-year-old who does not expect to get hurt has no reason to care about an emergency number,
but does understand going home early.

Danish throughout; headline in `font-nathejk`.

## Skipping, and what skipping does not mean

Skipped when the user has already started the event or confirmed previously — starting
implies the data was verified at the counter (PRD 005 §11). **Skipping this step must not
skip the portrait step** (task 129). The two are independent facts: verification says
something about the guardian number and nothing about whether there is a face on file.
Conflating them would remove the photo nudge from precisely the cohort that needs it most.

Confirmation state is server-side per user (PRD 005 §6), never `localStorage`, so a reinstall
or a new phone does not re-prompt a participant mid-event.

Details are **read-only** here — in-app editing is out of scope (PRD 005 §6, §12 Q4). If
editing ever lands, a number change should probably invalidate the verification; task 135's
storage already provides for keeping the acknowledged number so that is possible.

Components: shadcn-vue `Card`, `Input`, `Checkbox`, `Label` (PRD 005 §7). `checkbox` is not
yet generated in `vue/src/components/ui/` — task 122 generates it.

The failure paths ("nummeret er forkert" / "jeg kender ikke nummeret") are **task 128**.

## Acceptance Criteria

- [ ] `components/onboarding/WelcomeStepConfirmProfile.vue` renders the registered details
      read-only, with no editing affordance
- [ ] The guardian number is displayed masked as `11 22 33 **`, as text, never in an input
- [ ] A 2-digit `Input` with `inputmode="numeric"` plus the *"Dette nummer kan kontaktes i
      løbet af Nathejk"* `Checkbox` are **both** required to advance
- [ ] The step is skipped entirely for non-spejder, and never renders an empty guardian
      field as missing data
- [ ] Whether confirmation is required comes from the BFF's `confirmation_required`; the
      "verified OR started the event" rule is not reimplemented client-side
- [ ] Skipping this step does **not** skip the portrait step
- [ ] Copy explains both reasons the number matters — emergencies **and** pickup on
      mid-event resignation
- [ ] No change to `GET /api/me/profile`; nothing in the code or comments presents the
      masking as a security control
- [ ] Confirmation state is read from and written to the BFF, never persisted in
      `localStorage`
- [ ] Built from shadcn-vue `Card`/`Input`/`Checkbox`/`Label`; headline uses `font-nathejk`;
      copy in Danish
- [ ] `vue-tsc` and `npm run build` clean

## Depends on

- **Task 135** — `POST /api/me/profile/confirm` with the server-side digit check, and the
  `confirmation_required` / `verified_at` fields on `GET /api/me/profile`.
- **Task 122** — the `checkbox` shadcn-vue primitive.
- **Task 118** / **task 124** — the step machine and shell.
- **Task 128** — the non-punitive failure paths, which share this component's copy.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
