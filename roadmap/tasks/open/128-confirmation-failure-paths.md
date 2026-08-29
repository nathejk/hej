# 128 — Non-punitive "wrong number" and "I don't know it" paths

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 005 §5 (edge cases) and §6. The confirmation step (task 127) asks a member to supply two
digits of a number they do not control and may never have memorised. Failing that is
**expected and not rare**, and the PRD lists why: young scouts who simply do not know the
number, a guardian who recently changed number, and households where the two numbers on file
belong to two different parents.

So this is not an error state and **must not be worded as one**. No red, no "forkert kode",
no framing that suggests the member did something wrong. What has actually happened is that
the app has discovered a data-quality problem — which is a *success* of the step, not a
failure of the user.

Two distinct exits, offered plainly alongside the confirm action:

- **"Nummeret er forkert"** — the member recognises the number and knows it is wrong or out
  of date.
- **"Jeg kender ikke nummeret"** — the member cannot say either way.

Both must exist. Collapsing them into one loses the distinction organizers actually need:
"reported wrong" is a record to fix, "could not confirm" is a record to check. Both are
reported to `POST /api/me/profile/report-incorrect` with a reason distinguishing the two
(PRD 005 §8). That endpoint is **required, not optional** — a guardian number nobody can
confirm is an operational problem that has to reach a human before the event rather than dead-end
in the UI.

Note that the signal is valuable **even when the number turns out to be correct** (PRD 005
§5). "This member could not confirm their guardian number" tells an organizer that at 02:00
this is not a number to rely on without a phone call first, whatever the register says.

**The user gets into the app either way.** Both paths, and a wrong-digits response from
task 135's endpoint, let the member continue. PRD 005 §5 is blunt about the trade-off:
blocking a participant out of a safety app over a stale guardian number is the worse failure.
Only login is mandatory (PRD 005 §6). Note task 135 rate-limits wrong digits like the PIN
endpoint — not as a secrecy measure, just so it cannot be hammered — so the UI needs to
handle a 429 without implying the member is in trouble.

Also surface the **out-of-band correction channel**, so a member who reports a wrong number
knows what happens next rather than being thanked and moved along.

## Blocker on the copy

**PRD 005 §12 Q2 is open:** what the correction channel actually *is* has not been decided —
a phone number, an email address, the patrol leader, or purely the in-app flag. PRD 003 has
the same open question, and answering it there answers it here.

This blocks the final wording, not the implementation. Build the paths, the reason codes and
the endpoint call; leave the channel sentence as the one thing to fill in, and do not invent a
number. If the answer lands as "purely the in-app flag", the copy still has to say something
honest — something to the effect that Nathejk has been told and will check it — because
"thanks" with no stated consequence reads as a dead end and is why people stop reporting
things.

There is already a nødtelefon in `config/contact.ts` (used by the login step for "we don't
seem to know you"). It is a plausible answer to Q2 but must not be assumed to be one: the
nødtelefon is for emergencies during the event, and pointing pre-event data corrections at it
is a decision for whoever answers that phone, not for this task.

## Acceptance Criteria

- [ ] Both **"nummeret er forkert"** and **"jeg kender ikke nummeret"** are offered from the
      confirmation step
- [ ] Neither is styled or worded as an error or a failure; no punitive copy anywhere in the
      step, including after wrong digits
- [ ] Each posts to `POST /api/me/profile/report-incorrect` with a reason distinguishing
      "reported wrong" from "could not confirm"
- [ ] The user continues into the app on every path — reported wrong, could not confirm, and
      wrong digits — and is never blocked
- [ ] A 429 from the rate-limited confirm endpoint is handled without implying wrongdoing
- [ ] The correction channel is surfaced, so a report has a visible consequence
- [ ] Copy Danish, plain, and readable on a phone; headline (if any) uses `font-nathejk`
- [ ] The report call failing (offline, server error) still lets the user continue, and the
      report is not silently reported as sent
- [ ] The open channel question is recorded in this task's log rather than resolved by
      inventing a contact

## Depends on

- **Task 127** — the confirmation step these paths live in.
- **Task 135** — the confirm endpoint's rate limiting and error shape.
- **Task 136** — `POST /api/me/profile/report-incorrect`, including the reason codes these
  paths send.
- **PRD 005 §12 Q2 / PRD 003's matching question** — blocks the final copy only.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
