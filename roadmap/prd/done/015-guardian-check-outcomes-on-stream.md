# PRD 015 — Guardian check outcomes on the stream

**Status:** done
**Author:** agent session (Zed), with maintainer direction 2026-09-12
**Created:** 2026-09-12
**Last updated:** 2026-09-12
**Approved:** 2026-09-12
**Shipped:** 2026-09-12
**Target users:** participant (spejder), organizer / nødtelefon (as consumers of the stream)

---

## 1. Summary

Turn the first-login guardian check (PRD 005 §5 step 2) into a **check-in fast track**:
what the member verifies in the app is published to the stream, so the counter does not
have to ask again. Both numbers count — the member's own, already proven by the SMS PIN
they just typed, and the contact number they confirm or supply. A member who does not
manage it is not chased and not flagged; they are simply asked at check-in, as everybody
is today. After check-in every member has a recorded contact number, and from that point
the app no longer lets them change it — once a member has started, the number is settled.

## 2. Problem & Motivation

**What problem does this solve?**

1. **Check-in asks everybody, including the members who already answered.** The contact
   number is established at the counter today, one member at a time, at the moment a queue
   is forming. Yet by then many members have already sat with the app and confirmed the
   number — the app just has no way to tell the counter. **This is the point of the
   feature:** every verification that happens before arrival is a conversation the counter
   does not have to have.

2. **Only success is recorded today.** `POST /api/me/profile/confirm` and
   `/me/profile/guardian` publish `nathejk:*.member.*.verified` on success (see
   `go/cmd/api/verification.go`). A member who cannot produce the two digits, or who taps
   "spring over", produces *nothing* — survivable, since they get asked at check-in, but it
   also means the counter cannot see who is already done.

3. **The step throws away a fact it just established.** To reach this page the member
   received a PIN by SMS on the number we hold for them and typed it back
   (`WelcomeStepLogin.vue`). Their *own* number is therefore verified, by a real
   challenge-response, at no extra cost to anybody — another question the counter can skip.

4. **Some members registered their own number as their guardian's.** Observed in the
   register. Such a record passes every check we have — well-formed, instantly recognised,
   verifies perfectly — while being worthless in the situation it exists for: an injured or
   withdrawing 13-year-old's phone is the phone we are trying not to depend on.
   Fast-tracking one would be worse than not fast-tracking at all, because the counter
   would then skip the one member whose record actually needs fixing.

   The same argument extends to a patrol-mate's number — the patrol is one group in one
   place, so a number inside it is not an escalation path out of it — but comparing
   against patrol-mates is **out of scope** (maintainer decision 2026-09-12, §4). Only
   the member's own number has evidence behind it, and it is the case that can be checked
   without querying the patrol on a hot read path.

**Why now?** The check itself shipped in PRD 005 and the correction path in task 148, so
the interaction is settled; what is missing is the publish the check-in side needs, and a
read side for it is being built shortly in another repo (maintainer, 2026-09-12).

**Evidence.** Register inspection (own number as `phoneParent`); PRD 005 §11 2026-08-25 on
the recall check being a discovery device; task 148 on correction being the better outcome
than a flag.

## 3. Goals

- A member who verified in the app is not asked again at check-in.
- Both verifiable numbers — the member's own and the contact number — are recorded as
  facts the check-in side can read.
- A contact number that is really the member's own number is never fast-tracked: it is
  treated as no number at all, so the counter still asks.
- A member who cannot complete the check is not trapped in it, not chased about it, and
  not flagged — they are simply asked at check-in.
- After the member has started, the recorded contact number is settled and the app cannot
  change it.

## 4. Non-Goals

- **Not** a report on *why* a number was wrong. The register's earlier value, and whether
  the member corrected us or the register moved, are explicitly uninteresting (maintainer,
  2026-09-12): the only question is whether we have a verified contact number for this
  member yet. This is what allows the message to carry the acknowledged number alone.
- **Not** an escalation or follow-up path for members who did not verify. Check-in is the
  follow-up.
- **Not** overwriting the register. As in PRD 005/task 148, `phoneParent` stays
  projected from upstream; this PRD publishes verifications, never a write back to the
  source of truth.
- **Not** changing the login mechanism, PIN length, cooldown or anti-enumeration
  behaviour (PRD 005 §4 still holds).
- **Not** comparing the contact number against other members of the patrol (maintainer
  decision 2026-09-12). Only the member's **own** number is checked. A patrol-mate's
  number is no better an emergency contact in principle, but it has no evidence behind it
  in the register and checking it means a patrol query on the profile read path. If it
  turns out to be common, it is a separate decision — and arguably an organizer-side
  report rather than something this app should silently blank.
- **Not** an organizer-facing screen. This PRD ends at the stream; the read side is
  coming shortly in another repo, and per `.rules` any surface carrying a guardian number
  belongs outside this app anyway.
- **Not** blocking a member from entering the app. Login remains the only mandatory
  step (PRD 005 §6); every outcome here lets them through.
- **Not** validating that a supplied guardian number *answers*. No SMS is sent to a
  guardian, and this PRD does not introduce one.

## 5. User Stories & Scenarios

- As a **spejder**, I want to get on with the app when I genuinely do not know my
  parent's number, so that I am not stuck retyping guesses at a form.
- As the **nødtelefon**, I want to know *before* an incident which patrols have no
  reachable adult behind them, so that "we cannot get hold of anyone" is not
  discovered at 03:00.
- As an **organizer**, I want to see which members corrected our register, so that the
  register gets fixed rather than re-verified.

**Happy path.** Member logs in with a PIN → lands on the check → the number on file
reads `11 22 33 ××` with the last two digits to fill in → they type them, tick the
acknowledgement, continue. Stream learns: own phone verified (from login), guardian
number confirmed against the register.

**Correction.** They do not recognise the number → tap "Skriv andet nummer" → type a
full number → continue. Stream learns: own phone verified, and the contact number the
member supplied — which a consumer can compare against the register's `phoneParent` to
see that we were corrected (but see §11 Q2 on doing that safely).

**Exhaustion (new).** They try the two digits and get them wrong three times. On the
third failure the step ends by itself, exactly as if they had tapped "spring over": they
are let into the app, and the stream learns the same thing a skip records — own phone
verified, contact number not. (That the member *looked and could not place it* is a
stronger signal than a plain skip, but under the agreed message shape it is not
separately recorded; see §6.)

**Skip.** They tap "Jeg kender ikke nummeret — spring over" at any point. Stream learns:
own phone verified, contact number not verified.

**Own-number collision (new).** The register's `phoneParent` for this member equals the
member's own `phone`. Before the page is ever rendered the number is blanked — the member
is asked to supply one as though the register held none.

## 6. Requirements

### The message contract (decided 2026-09-12)

One message type carries both verifications, with both numbers optional:

```go
type NathejkMemberVerified struct {
	MemberID     types.MemberID    `json:"memberId"`
	Phone        types.PhoneNumber `json:"phone,omitempty"`
	PhoneContact types.PhoneNumber `json:"phoneContact,omitempty"`
	VerifiedAt   time.Time         `json:"verifiedAt"`
}
```

published on

```
NATHEJK.<year>.spejder.<memberId>.verified      (consumed as NATHEJK.*.spejder.*.verified)
```

`Phone` is the member's own number, proven by the SMS challenge at login. `PhoneContact`
is the contact number the member acknowledged. Either may be absent, which is what lets
the two verifications be published independently and on their own schedules — the login
event carries only `Phone`, the guardian check carries `PhoneContact` (and `Phone`, which
is equally true at that point). `PhoneContact` uses the register's own field name
(`NathejkScoutUpdated.PhoneContact`), not the projection's `phoneParent`.

**Skip and exhaustion are this same event with an empty `PhoneContact`** (decided
2026-09-12). They are not a different kind of fact: the member's own number *is* verified
— the PIN proved it — and the contact number is not. So the event states exactly what is
true, and the zero value is the statement "not verified", not a missing field.

What that buys, and what it costs:

- No second message type, no outcome enum, and no vocabulary for workflow steps on a
  stream that records facts about members. This is the strongest argument for it.
- The projection must therefore treat `PhoneContact` as **authoritative when present and
  when explicitly empty on an outcome publish** — which is in tension with the general
  rule that an absent field says nothing (below). The resolution: only the guardian-check
  endpoints publish an outcome, and only they may assert an empty `PhoneContact`; the
  login publish never sends the field at all. Two publishers, two different meanings for
  the same zero value, which is a real sharp edge and must be commented at both ends.
- *Skipped* and *abandoned-after-three-attempts* become indistinguishable from each
  other, and from "logged in and never got to the check". §2 called the third of those
  the problem to be fixed; with this shape the fix is partial — the absence is now
  timestamped and attributable, but the reason is not on the stream. Accepted unless
  §11 Q1 says otherwise.

`VerifiedAt` stays on the body (decided 2026-09-12): delivery time changes on every
replay, the projection cannot store a zero timestamp, and the timestamp answers "how many
members verified before arriving?".

`Year` is **not** on the body — it is the second subject token, and the consumer already
receives it separately (`handleMemberVerified(msg, year)`). It must be the member's own
year, i.e. the year of the row being verified, not "the current event year" resolved
independently: those are the same value today and a publish that derives it from config
rather than from the member is how they stop being.

This replaces the shipped shape (`PhoneParentAcknowledged`, `PhoneParentRegistered`,
`Year`) and moves the subject from `.member.` to `.spejder.`, which aligns it with the
member-lifecycle events already on `NATHEJK.{year}.spejder.{memberId}.{event}`. Both are
breaking; see §8 for what has to change with them and what is lost.

### Who publishes a verification (decided 2026-09-12)

- **Spejder** — both halves: own phone at login, contact number from the check.
- **Bandit (klan)** — own phone at login. They have no contact number
  (`phoneParent == nil`), so they never publish `PhoneContact` and are never asked.
- **Crew and gøgler** — nothing. Their own numbers are already verified through another
  route, so a login event would restate a known fact.

### Functional

- [ ] The successful-login step publishes `NathejkMemberVerified{MemberID, Phone}` — the
      member's own number, proven by SMS challenge-response. **Once per member per year**
      (maintainer decision 2026-09-12): the fact does not change between logins, and an
      event per login would put login volume on the stream to say the same thing again.
- [ ] If a member's own number *does* change and a later login proves a different one,
      the later verification supersedes the earlier — **last verification wins**
      (maintainer decision 2026-09-12). So "once per year" means "once per verified
      number", not "the first one we ever saw": the publish is suppressed only while the
      number already recorded as verified is the number the PIN just proved. A spejder
      is not expected to have verified their own number earlier in the year at all, so
      in practice the first login is the publish and every later one is silent.
- [ ] Neither phone field may be written by an event that did not establish it. A login
      event must not send `PhoneContact` **at all** — absent means "says nothing", while
      an empty value on a guardian-check publish is the assertion "not verified". A login
      must never be able to un-verify a contact number.
- [ ] `POST /me/profile/confirm` counts failed digit attempts **per member,
      server-side**, scoped to the **login session** (maintainer decision 2026-09-12):
      a member who comes back tomorrow having asked their mother gets a fresh three.
      The third failure ends the step: the response tells the client the check is over
      and the BFF publishes an *abandoned* outcome. Further attempts in that session are
      refused as "nothing to confirm" (the existing 409 shape).
- [ ] A digit match publishes `NathejkMemberVerified{MemberID, PhoneContact}` with the
      registered number — today's *confirmed* outcome, keeping its meaning.
- [ ] A member-supplied number publishes the same message with the number the member
      typed. The event does **not** say what the register held, and does not need to: what
      is being recorded is "we have a verified contact number for this member", not a
      diagnosis of the register (§4).
- [ ] **After the member has started, the app cannot change the contact number.**
      `Person.HasStarted()` (`MemberStatusRacing`) is the signal (maintainer, 2026-09-12).
      `POST /me/profile/guardian` must refuse for a started member; `/me/profile/confirm`
      already does, because `confirmationRequired` is false once started (PRD 005 §8).
      This reverses task 148's deliberate ungating of `/guardian`, which was written before
      check-in was the backstop: the correction path stays open right up to the start and
      closes there. A member replacing the number afterwards would leave staff holding a
      number nobody validated.
- [ ] The profile response must tell the client the number is settled, so the PWA can
      render it read-only rather than offering an edit that will be refused.
- [ ] **The settled number is the one check-in recorded**, not the register's.
      `handleTeamStarted` already receives `Phone` and `PhoneGuardian` for every member who
      started (`messages.NathejkTeamStarted_Member`) and currently discards both, writing only
      `memberStatus`. It must record them, so that after the start the app shows the number
      staff actually hold rather than a register value that may differ. Showing the register's
      number while the counter holds another one is worse than showing nothing.
- [ ] A member who started **without** verifying in the app therefore also ends up with a
      recorded contact number, arriving through the same event. No separate backfill is
      needed, and no further prompting: §6's fast track is about who can be *skipped* at the
      counter, not about chasing anybody afterwards.
- [ ] "Spring over" and exhaustion after three attempts publish
      `NathejkMemberVerified{MemberID, Phone, VerifiedAt}` with an **empty**
      `PhoneContact`: the member's own number is verified, the contact number is not.
      The PWA must call an endpoint for the skip — a skip that only happens in the client
      is the absence this PRD exists to remove.
- [ ] The outcome publish **always fires**, even when the member's own number was already
      recorded as verified. The once-per-number suppression applies only to the login-side
      publish; suppressing an outcome would put us back to silence for exactly the members
      this PRD is about.
- [ ] Outcomes are idempotent per member per check: a double submit or a retry after a
      dropped connection must not read as two members' worth of signal.
- [ ] The projection keeps **own-phone verification and contact verification in separate
      columns**. Today `verifiedAt` means "contact number confirmed" and
      `Person.IsVerified` gates `confirmation_required` on it. A skip/exhaustion event
      carries a `VerifiedAt` too, so writing it into the same column would mark every
      member who gave up as verified — the exact opposite of this PRD's purpose.
- [ ] `handleMemberVerified` must **stop rejecting an event with no contact number**. It
      currently returns an error for an empty `PhoneParentAcknowledged`, which under the
      new shape would dead-letter every login and every skip. The rule it was protecting
      (never record a contact verification that names no number) survives as: an event
      without `PhoneContact` writes only the own-phone column.
- [ ] After exhaustion the member must not be re-shown the check on a reload **within the
      same session**. `confirmation_required` stays true (they have not verified), so
      either the profile response also expresses "this session's check is closed" or the
      client owns that state — and if the client owns it, a reload re-opens a check whose
      endpoint now answers 409, which the PWA must handle as "carry on" rather than as an
      error.
- [ ] A cached profile response on a device may still hold a colliding number from before
      the blanking shipped. The client data layer (PRD 009) must not serve it: either the
      sync version is bumped so cached profiles are refetched, or the blanking is applied
      on read as well. Projecting the field out server-side does nothing about a copy
      already written to a device (`.rules`).
- [ ] A skip whose POST cannot reach the BFF (no signal after login, broker down) must
      still let the member into the app — login is the only mandatory step. The outcome is
      then lost unless it is retried; see §11 Q3.
- [ ] `GET /me/profile` blanks `phone_parent` to `""` (expected but not registered —
      **not** `null`, which means "this population has no guardian number") when the
      registered guardian number equals the member's own number after normalization.
- [ ] The blanking is applied by the BFF in **every** response that could carry the
      value, not by the client choosing not to render it (`.rules`).
- [ ] The collision is reported somewhere an organizer can act on, so the register gets
      fixed. **The agreed message type has no vehicle for this** — it carries verified
      numbers, not findings about the register. Either a separate message is needed, or
      the finding is left implicit: the member is asked for a number as though the register
      held none, and whatever they supply arrives as a `PhoneContact` that differs from
      `phoneParent`. See §11 Q2, which is the same problem from the other end.
- [ ] With a blanked (`""`) guardian number the check opens directly in "type a number"
      mode: there are no digits to recall, and the recall UI must not render an empty
      number with two crosses after it.
- [ ] `confirmation_required` stays true for a blanked number (a spejder with `""` is
      exactly the record an organizer wants to hear about — PRD 005 §8, unchanged).

### Non-Functional

- **Privacy.** No new surface carries a guardian number. The stream already does, on a
  per-person purgeable subject (`person.VerifiedSubject`); every event added here must
  use the same subject discipline so a per-person purge reaches it.
- **Availability.** A failed publish fails the request, as today
  (`verification.go`): an outcome the log never saw did not happen. The exception is
  the login-side own-phone finding, which must **never** fail a login — it is a
  by-product, and a broker outage must not stop people getting into the app.
- **Attempt counting is per login session**, so it lives naturally with the session
  rather than in a member-keyed store with its own expiry. It must survive a page
  reload (the session does) but need not survive a BFF restart: the cost of a restart
  is a member getting three more tries, which is acceptable, and the cost of
  client-side counting is that the limit does not exist at all.
- **No enumeration.** The refusal after three attempts must not reveal the digits.
- **Rate limiting.** The per-IP `confirmLimiter` stays as it is, and the three-attempt
  rule is orthogonal to it. Worth remembering that a whole patrol on one campsite wifi
  shares an IP, so the IP limiter must not be tightened to enforce the attempt rule.
- **Copy.** All new strings are Danish, non-accusatory, and written for a 13-year-old
  (PRD 005 §11). The remaining-attempts hint and the exhaustion message need actual
  wording agreed, not paraphrases — they are the two places a member is told they got
  something wrong.
- **Tests.** The guardian tripwire (`cmd/api/guardiantripwire_test.go`) exists to fail if a
  guardian number reaches a response it should not. A field newly named `phoneContact`
  must be covered by the same reasoning, and the blanking rule wants a test of its own
  — `phoneParent == phone` is easy to write and easy to regress silently.

## 7. UX / UI Notes

`vue/src/components/onboarding/WelcomeStepConfirmProfile.vue`:

- Attempt feedback: the existing 400 message ("De to cifre passer ikke til nummeret, vi
  har.") gains a remaining-attempts hint on attempts 1 and 2. Wording must stay
  non-accusatory (PRD 005 §11): not knowing the number is expected.
- On the third failure the component behaves as the skip path does today — emits `skip`
  and lets the member into the app — with a short, kind explanation rather than an
  error: we will ask again next time, and they can tell their leader.
- Consider offering the correction field *first* on the second failure rather than
  after exhaustion, since a member who missed twice is unlikely to succeed on the
  third. Open question.
- Blanked/empty guardian number: the highlighted number block renders no digits and no
  recall input; the step opens in correction mode with the full-number field
  (`InputGroup` with the `+45` addon). No copy anywhere may suggest the member's own
  number is an acceptable answer.

## 8. Technical Considerations

- **Frontend (Vue 3 / TS).** `WelcomeStepConfirmProfile.vue` (attempt state, exhaustion
  → `skip`, empty-number mode), `WelcomeStepLogin.vue` (unchanged behaviour; the
  own-phone finding is published server-side, not requested by the client),
  `stores/profile.store.ts` (a `skip` action, an exhausted signal).
- **BFF (Go).** `cmd/api/verification.go` grows the new publishes and keeps owning the
  write path (no SQL in handlers — PRD 008 §8). `cmd/api/profile.go` handlers gain the
  attempt counter, the skip endpoint and the started-member refusal on `/guardian`. The
  collision rule is a small pure helper — compare `PhoneParent` against `Phone` on the same
  `person.Person`, both normalized with `internal/phone` — applied wherever a profile is
  projected. No extra query, and it can be unit-tested without a database. In the projection,
  `handleTeamStarted` starts recording the two phone numbers it already receives. The existing
  per-IP `confirmLimiter` stays; per-member attempt counting is a *different* thing and must
  not be conflated with it.
- **API endpoints.**
  - `POST /me/profile/confirm` — changed: attempt counting, a distinct response when the
    check has been abandoned. Annotations must be updated.
  - `POST /me/profile/skip` (name TBD) — new: records that the member gave up. Needs
    full OpenAPI annotations, like every endpoint in this repo (`.rules`).
  - `GET /me/profile` — unchanged shape, changed semantics for `phone_parent`. The
    description annotation must state the collision-blanking rule, because a client
    author cannot infer it.
- **Data / storage.** The projection (`nathejk/table/person`) needs to consume the new
  events: a `phoneVerifiedAt` (own number) alongside today's guardian columns, a
  check-outcome column, and a collision flag. `Person.IsVerified` and
  `confirmationRequired` must be re-read in light of "abandoned" — abandoning is **not**
  verifying, and must not silence the question on the next login.

### What the new message shape costs (§6)

The reshape is not free, and each loss should be an accepted decision rather than a
discovery during implementation:

- **`PhoneParentRegistered` disappears, and that is now intended.** The field existed to
  keep "the register moved since" apart from "the member corrected us", which
  `Person.IsVerified` and `Person.GuardianCorrected` read (`querier.go`, task 148). Per the
  maintainer (2026-09-12) neither question matters: the purpose is a check-in fast track, so
  the only thing worth knowing is whether a verified contact number exists. Consequences to
  accept deliberately:
  - `GuardianCorrected` loses its basis and should be **removed** rather than left
    returning a value it can no longer support.
  - `IsVerified` stops invalidating on a register change. A verification therefore stands
    even if `phoneParent` later moves — which is correct under the new framing (the member
    verified *a* reachable number, and that is what check-in wanted), and would have been
    wrong under the old one. The `verifiedAgainstPhone` column becomes dead and should go
    with it, not linger as a NULL nobody reads.
- **`omitempty` and "explicitly empty" are the same bytes.** This is the sharpest
  consequence of carrying skip/exhaustion as an empty `PhoneContact`: with
  `json:"phoneContact,omitempty"` an empty value is *omitted*, so a skip event and a
  login event serialize to identical JSON. The distinction described in §6 therefore does
  not exist on the wire as tagged. Either
  - drop `omitempty` from `PhoneContact` so `"phoneContact": ""` is transmitted and the
    assertion is real, or
  - accept that the two are the same event — which is coherent (both say "own number
    verified, contact number not") and means the skip endpoint's only job is to publish
    the same fact a login would, at a moment that proves the member reached the check.

  The second reading is simpler and probably right, but it must be chosen deliberately,
  because the first is what most readers will assume from the field being present in the
  struct.
- **`VerifiedAt` stays on the event** (decided 2026-09-12), for the reason its original doc
  gives: delivery time changes on every replay, a zero timestamp is not storable in
  MariaDB, and the timestamp answers "how many verified before arriving?".
- **`Year` disappears from the body** but survives as the second subject token, and the
  consumer already receives `year` separately (`handleMemberVerified(msg, year)`), so
  this one is a genuine simplification. The publisher must take the year from the member
  being verified rather than resolving it independently from config.
- **The subject moves** from `NATHEJK.<year>.member.<id>.verified` to
  `...spejder.<id>.verified`. Per the maintainer (2026-09-12) nothing has been published
  on the old subject, so there is nothing to migrate and no dual subscription — worth
  confirming against the live stream before the old subject builder is deleted, since the
  cost of being wrong is a silently unprojected verification. Per-person purgeability is
  preserved: the member id remains its own token.
- **`spejder` in the subject is a token, not a population.** Bandits publish their
  own-phone verification on it too (§6), so a consumer must not read the token as "this
  member is a spejder". The member-lifecycle events on the same prefix already work this
  way, so the convention is inherited rather than invented — but it belongs in the message
  doc, because role is exactly what a reader will otherwise infer from the subject.
- `messages.NathejkMemberVerified` is a shipped type in `shared-go` with a documented
  contract; changing it in place is a breaking change for any consumer, and its doc
  comment (the two-phone rationale) has to be rewritten rather than trimmed.
- **Dependencies & risks.**
  - **Blocking:** the message vocabulary lives in `github.com/nathejk/shared-go`
    (`messages/member.go`), a separate repo. The reshaped `NathejkMemberVerified` (§6)
    must land **there** first. That is the critical path, and it is a contract other
    consumers read.
  - Suppressing the own-phone publish requires reading the current state before
    publishing, which makes it read-then-write and therefore racy under two
    simultaneous logins. Acceptable: the events are idempotent in meaning (same member,
    same number, same claim), and the projection takes the latest. Do not add locking
    for it.
  - Append-only log: fields omitted at first publish can never be backfilled for those
    events (the reasoning already recorded in `verification.go` for
    `PhoneParentRegistered`). Get the payload right before shipping, not after.
  - The collision check needs no query and cannot fail: both numbers are already on the
    `person` row the profile read has loaded.

## 9. Success Metrics

None of these has a dashboard, and building one is a non-goal (§4). They are measured by
querying the stream (`nats stream view` per subject) or the `person` projection directly;
saying so here is what keeps them from being aspirational.

- Every login of a spejder with a contact number produces exactly one terminal
  outcome on the stream: a `verified` event carrying a `PhoneContact`, or one carrying
  only `Phone`. Target: 100%; a member with no event at all is a bug, not a data point.
- Own-phone-verified is recorded for every member who logs in at least once, and
  published at most once per member per verified number.
- Share of members arriving at check-in with a verified contact number — the fast-track
  rate, and the metric this feature exists to move.
- Number of members whose contact number is their own number: reported at all, which is
  the change — today it is zero because nobody looks.
- Share of *corrected* outcomes: the useful signal for how wrong the register is.
- No member reports being unable to leave the check screen.

## 10. Rollout / Task Breakdown

Sequencing is forced by the dependency: the message contract lands upstream first,
then the BFF, then the client. The collision rule is independent of the contract work
and can go in parallel.

- [x] Task: reshape `NathejkMemberVerified` in `shared-go` to `{MemberID, Phone,
      PhoneContact, VerifiedAt}` and rewrite its doc contract (blocking) — **task 222**
- [x] Task: move the publish subject to `NATHEJK.<year>.spejder.<memberId>.verified`,
      taking the year from the member; confirm the old `.member.` subject is empty first —
      **task 223**, and it was empty
- [x] Task: decide `omitempty` vs. transmitted-empty for `PhoneContact` (§11 Q1) — kept
      `omitempty`; a skip and a login are the same fact, so being byte-identical costs nothing
- [x] Task: publish own-phone-verified on successful PIN login for spejder and bandits,
      once per verified number (never fails the login) — **task 226**
- [x] Task: per-member attempt counter on `/me/profile/confirm`, third failure ends the
      check — **task 227**
- [x] Task: `POST /me/profile/skip` endpoint with OpenAPI annotations — **task 228**
- [x] Task: publish skip/exhaustion as a `verified` event with a zero `PhoneContact` —
      **task 228**, with task 227 asserting both paths publish the same fact
- [x] Task: project the new events in `nathejk/table/person` — separate own-phone and
      contact verification columns, stop rejecting events with no contact number, and
      re-read `IsVerified` / `confirmationRequired` for "abandoned" — **tasks 224 and 225**
- [x] Task: record `Phone` and `PhoneGuardian` from `NathejkTeamStarted` in
      `handleTeamStarted`, and serve the settled number from there after the start — **task 230**
- [x] Task: refuse contact-number changes once `HasStarted()`, and tell the client it is
      settled — **task 231**
- [x] Task: remove `GuardianCorrected` and `verifiedAgainstPhone`, and stop invalidating a
      verification when the register moves — **task 225**
- [x] Task: agree the Danish copy for the attempts hint and the exhaustion message — folded
      into **task 232**, where the agreed strings are recorded
- [x] Task: blank `phone_parent` when it equals the member's own number — **task 229**
- [x] Task: PWA — attempt feedback, exhaustion → skip, empty-number mode opens in
      correction mode — **task 232**
- [x] Task: make sure a cached profile cannot serve a blanked-away number (sync version or
      read-time blanking) — **task 233**, which found nothing caches the profile and left a
      regression guard instead of a fix

## 11. Open Questions

**Resolved 2026-09-12 (maintainer):**

- **Message shape and subject.** One `NathejkMemberVerified{MemberID, Phone,
  PhoneContact, VerifiedAt}`, both numbers `omitempty`, on
  `NATHEJK.<year>.spejder.<memberId>.verified` (`NATHEJK.*.spejder.*.verified`). Folded
  into §6.
- **`VerifiedAt` stays on the body**; `Year` does not — it comes from the subject, and
  must be the member's own year rather than a separately resolved "current" year.
- **Who publishes:** spejder (own phone + contact number), bandits (own phone only — no
  contact number exists for them), and neither crew nor gøgler, whose own numbers are
  already verified by another route. The `spejder` subject token therefore does not
  denote the population.
- **The old `.member.` subject is empty**, so no migration and no dual subscription.
- **Own-phone and contact verification share one message type**, distinguished by which
  field is present, rather than being separate types.
- **Own-phone-verified is once per member per year**, not once per login, with **last
  verification winning** if a later login proves a different number. A spejder is not
  expected to have verified their own number earlier in the year, so the first login is
  normally the publish. Folded into §6.
- **Three attempts per login session**, reset by a new login. Folded into §6.
- **Only the member's own number is checked for collision**, not patrol-mates' numbers
  (§4). No patrol query, no read-path cost.
- **Skip and exhaustion carry no new message type.** Both publish the same
  `verified` event with the member's own `Phone` and a zero `PhoneContact`: the own number
  *is* verified by the PIN, the contact number is not. Folded into §6.

- **Own-phone verification is write-only here on purpose.** A read side is coming shortly
  in a different repo (maintainer, 2026-09-12), so the column exists to be consumed there
  rather than by this app. That also settles why the fact belongs in `shared-go` rather
  than staying local to `hej`.
- **The purpose is a check-in fast track**, and *why* a number was wrong is explicitly not
  interesting. That closes the `PhoneParentRegistered` question: the field is dropped on
  purpose, `GuardianCorrected` and `verifiedAgainstPhone` go with it, and a verification is
  no longer invalidated by the register moving (§8).
- **`omitempty` on `PhoneContact` is fine.** A skip event being byte-identical to a login
  event costs nothing when the only question a consumer asks is "is there a verified
  contact number for this member?" — both events answer "not yet".
- **Once the member has started (`MemberStatusRacing`) the number is settled** and the app
  must refuse to change it (§6).

- **"Settled" means `Person.HasStarted()`** — `MemberStatusRacing` (maintainer,
  2026-09-12). Once a member has started, the contact number is no longer editable from the
  app. Convenient rather than coincidental: `confirmationRequired` already returns false for
  a started member (PRD 005 §8), so `/confirm` refuses today and the only endpoint that has
  to change is `/guardian`.

- **Yes, the check-in values come back** — in `messages.NathejkTeamStarted`, whose
  `Members[]` carry `Phone` and `PhoneGuardian` per member (maintainer, 2026-09-12). And
  `hej` **already consumes that event**: `person.consumer.handleTeamStarted`
  (`NATHEJK:*.patrulje.*.started`) writes `memberStatus=racing` from it and ignores both
  phone fields. So the lock signal and the authoritative post-check-in numbers arrive in the
  same message, in a handler that already exists. Folded into §6.

**Resolved during implementation:**

1. **The four names for one number** (`phoneContact` in the register, `phoneGuardian` on the start
   event, `phoneParent` in this projection, `startedPhoneContact` for what check-in recorded) —
   **documented rather than aligned** (task 230). The mapping is written in `table.sql` and at
   `handleTeamStarted`, where a reader tracing the number will be standing. Renaming across three
   repos was out of scope.
2. **`omitempty` on `PhoneContact`** — **kept** (task 222). A skip event and a login event are
   byte-identical on the wire, which costs nothing because both answer the only question a consumer
   asks with "not yet". Recorded in the message's doc comment so it is not "fixed", and pinned by a
   projection test asserting the two produce identical SQL.
3. **Does a lost skip get retried?** — **no** (tasks 228, 232). `skipContactCheck()` never throws
   and never queues: the outcome is lost and check-in is the backstop. The offline layer's first
   queued mutation was not worth introducing for a low-stakes fact.
4. **Does an abandoned check keep asking on the next login?** — **yes.** The attempt counter is
   keyed by the session's expiry, so a new login is a fresh three, and giving up leaves
   `confirmation_required` untouched.
5. **Does the collision rule apply to klan (bandit) records?** — **no-op, asserted** (task 229):
   `phoneParent` stays `nil` rather than becoming `""`.
6. **Should a collision also invalidate an existing contact verification?** — **partly, and this
   is the one loose end.** A colliding number can no longer be *confirmed* — task 229 blanks it
   before the digit check, so no new verification can be recorded against one. An acknowledgement
   recorded *before* this shipped is left alone: rewriting history in the projection is a different
   kind of change, and after the start check-in's number supersedes it anyway. **Worth a follow-up
   if any such rows turn out to exist.**
7. **Second-failure behaviour** (offer the correction field early) — **not done, and not needed.**
   The hint after each miss ends with "eller skrive et andet nummer" and the button is on screen
   throughout, which covers it without moving the member somewhere they did not ask to go.

**Left behind deliberately:**

- `verifiedAgainstPhone` still exists as a NULL column on deployed databases. The boot-time drift
  list is additive by design, so dropping a column needs a real migration (task 225).
