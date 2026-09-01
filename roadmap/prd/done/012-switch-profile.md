# PRD 012 — Switch profile from the app bar

**Status:** done
**Author:** agent session (Zed)
**Created:** 2026-09-01
**Last updated:** 2026-09-01 (candidate detail, after the switcher showed five identical rows)
**Approved:** 2026-09-01
**Shipped:** 2026-09-01
**Target users:** anyone whose phone number carries more than one profile — in practice a member with duplicate registrations, and a parent or sibling sharing a handset

---

## 1. Summary

A **Skift profil** action in the app bar's user menu, for a phone number that carries several
profiles. It ends the current session and starts one as the chosen profile, **without asking for
a new SMS code** — the holder already proved control of that number when they signed in.

## 2. Problem & Motivation

- **What problem does this solve?** Choosing a profile happens **only at login** today (PRD 006
  §11 Q1, task 079). Once signed in, the only way to reach another profile on the same number is
  to sign out and complete a full PIN round-trip.
- **Why it matters during the race.** The SMS path is the least reliable thing in the app at the
  moment it is needed: a member in woodland with one bar may wait minutes for a code, or not
  receive one at all. Making a routine switch depend on it means the app is least usable exactly
  when someone is standing at a checkpoint trying to show the right profile.
- **Why now?** The population is known and it is not small. A full replay of the real event stream
  (task 072/078) found **213 phone numbers shared by more than one person**, against ~1,610
  members with a number at all — roughly one in eight. Restricted to the 2026 event, **70 of 85**
  shared numbers carry rows with the *same name*: duplicate registrations of one person, with the
  largest cluster being nine rows for a single member. So the common case is not a parent juggling
  siblings; it is **one person whose account is fragmented**, needing to reach the profile that
  happens to hold their patrol, arm number or portrait.
- **Evidence.** PRD 006 §11 Q1 and its task 078 measurements, quoted above. Task 079 built the
  login-time chooser and explicitly scoped switching afterwards out.

## 3. Goals

- A member can move between the profiles on their own number in two taps, offline-tolerant.
- Switching costs no SMS and no re-entry of a code.
- The control is invisible to the (large) majority who have exactly one profile.
- Switching leaves no trace of the previous profile in the app: no cached directory, no
  favourites, no role-gated page still reachable.

## 4. Non-Goals

- **Being signed into two profiles at once.** One session, one identity. Anything else would mean
  two of every role-scoped cache and a "which one am I acting as" question on every screen.
- **Switching to a profile on a *different* number.** That is a different person as far as this app
  can tell, and it needs the PIN path.
- **Merging or de-duplicating profiles.** The duplicate registrations this feature makes navigable
  are an upstream data problem (PRD 006 §11 Q9); a switcher makes them survivable, not fixed.
- **A profile picker at every screen.** The app bar's user menu is where identity already lives.
- **Re-verifying by SMS on switch.** Deliberate; see §8.

## 5. User Stories & Scenarios

- As a **member with two registrations**, I want to switch to the profile that has my patrol on it,
  without waiting for a code I may not receive.
- As a **parent with two children's profiles on my phone**, I want to switch between them at a
  checkpoint quickly.
- As a **member with exactly one profile**, I want not to be offered a control that does nothing.

### Primary path

1. The member opens the user menu in the app bar and taps **Skift profil**.
2. The app asks the BFF for the profiles on their number and shows the same candidate list the
   login chooser uses — name plus patrol or section, which is what actually disambiguates.
3. They pick one. The BFF issues a session for that profile, replacing the previous one.
4. Role-scoped client state is cleared and the app lands on the map as the new profile.

### Edge cases

- **Only one profile on the number.** The menu item is absent, and the endpoint refuses — a direct
  caller gets a 409 rather than a list of one.
- **No number on file.** Some members have none (PRD 006 §11 Q13). No switch is possible; same
  refusal.
- **Offline.** The switch needs the BFF, so it fails with a clear "kræver forbindelse" and the
  current session is left untouched. Nothing half-switched.
- **The chosen profile has a different role.** Expected — a duplicate registration may be a
  spejder on one row and crew on another. The nav, the router guard and every role-gated cache
  must follow the new role, which is what §6's cache requirement is about.
- **The other profile was deleted upstream between listing and choosing.** `/auth/choose` re-checks
  ownership against the directory, so it refuses; the user stays signed in as they were.
- **Two profiles, one device, over time.** Whatever the previous profile cached must not be visible
  to the next one. Today the contacts directory and favourites are keyed per *device*, which is a
  latent leak the switcher would make routine — see §6.

## 6. Requirements

### Functional

- [ ] A **Skift profil** item in the app bar's user menu (`UserMenu.vue`), beside Min profil and
      Log ud.
- [ ] Shown **only** when the caller's number carries more than one profile, so it is invisible to
      the majority. `GET /api/me` reports this.
- [ ] Picking a profile issues a session for it, **replacing** the current one — no signed-out gap,
      and no second session.
- [ ] The candidate list is the **same component** the login chooser uses: name plus patrol or
      section. Two lists that could disagree about how a person is identified is one too many.
- [ ] The BFF refuses to mint a switch for a number with fewer than two profiles, and for a caller
      with no number on file.
- [ ] A switch may only ever offer profiles on **the caller's own number**, re-checked server-side
      against the directory rather than trusted from the request.
- [ ] **All role-scoped client state is dropped on switch**: the cached contacts directory, its
      version, favourites, and the profile store. Per-profile storage keys rather than a clear
      call, so the guarantee does not depend on remembering to clear.
- [ ] Offline, or on any failure, the current session survives unchanged.

### Non-Functional

- **No new SMS traffic.** The point of the feature.
- **Two taps** from anywhere with an app bar.
- **Auditable**: a switch is logged server-side with both profile ids, since it is an identity
  change made without a fresh proof of the number.
- **No new secret.** Reuses the existing signed choice token and `/auth/choose`.
- Baseline stays iOS/iPadOS Safari 16.4+ / Chrome 111+ per `.rules`.

## 7. UX / UI Notes

- The item lives in the existing shadcn-vue `dropdown-menu` in `UserMenu.vue`, with a Lucide
  `Users` (or `Repeat`) icon, above **Log ud**.
- Candidates appear in a `dialog` rather than by navigating away: switching is a two-tap action, and
  routing through the onboarding flow to reach a list the user is already looking at would be a
  worse answer. The onboarding chooser keeps its own placement.
- Not available on `/maps`, which has no app bar — the same accepted limitation as Min profil and
  Log ud, for the same reasons (see `UserMenu.vue`).
- After switching, land on `/maps` rather than staying put: the previous route may be role-gated and
  no longer permitted, and the guard bouncing the user somewhere would read as a glitch.

## 8. Technical Considerations

### Why no re-verification, stated plainly

This is the one decision in the PRD, and it is a security decision, so it belongs in writing rather
than in a commit message.

`/auth/choose` is safe today because of three properties its handler documents: the choice token is
minted only after a successful PIN verification, it is bound to the verified number, and the chosen
user is re-checked against that number's current owners. Switching keeps two of the three exactly
as they are — the binding and the re-check — and replaces "minted after a PIN verification" with
**"minted for the number the caller is already authenticated as"**.

That is not a weakening in any way this app can act on:

- the holder proved PIN control of that number at login, and **every profile on the number is
  reachable by that same holder** through the login chooser already;
- so the capability granted — reach another profile on your own number — is one the holder already
  has. The switch removes an SMS round-trip, not a barrier.

The honest residual: a session lasts 7 days, so a switch six days later is not re-proven by SMS.
Someone holding an unlocked phone could move to a sibling's profile without receiving a code. Weighed
against the alternative, that is thin — the same person already has full access to the *current*
profile, including its portrait and guardian-number confirmation, and the other profiles are the
same household or (usually) the same person. Making the switch require SMS would not protect that
data; it would only make the feature fail in a forest.

What that residual does justify: **logging every switch** with both profile ids, so an identity
change made without a fresh proof is at least visible afterwards.

### BFF (Go)

- `POST /api/auth/switch`, behind `requireAuth`. Resolves the caller, reads their own normalized
  number, calls `Users.LookupAll`, and returns `{choice_token, candidates}` in exactly the shape
  `/auth/verify` returns for a shared number — so the client reuses one code path.
- Refuses with `409` when the number carries fewer than two profiles, or the caller has no number.
- `GET /api/me` gains a count (or boolean) so the client can decide whether to draw the item. It is
  a fact about the caller's own number, so it discloses nothing new.
- `/auth/choose` is **unchanged**. It already re-checks ownership and issues the session, and
  leaving it untouched is what keeps the two paths from drifting.
- OpenAPI annotations on both changed endpoints, per `.rules`.

### Frontend (Vue 3 / TS)

- `session.store`: a `switchProfile()` action that fetches candidates + token, reusing the existing
  `choiceToken` / `choiceCandidates` state and `choose()`.
- Extract the login chooser's candidate list into a shared component so both surfaces render
  identically.
- `UserMenu.vue`: the menu item plus the dialog.

### Per-profile storage keys, and a latent bug this fixes

The contacts directory (`hej.contacts.v1`) and favourites (`hej.contacts.favourites.v1`) are keyed
per **device**. That is already wrong today — sign out, sign in as a sibling, and the previous
profile's cached directory and favourites are what you see — but it is rare enough to have gone
unnoticed. A switcher makes it routine, and the contacts cache holds names, phone numbers and
portraits of colleagues.

So both become **per profile**: `hej.contacts.v1.<userId>`. That is stronger than clearing on
switch, because it holds even if some future path forgets to clear, and it means a switch back finds
its own cache intact rather than an empty pane.

Note `pruneAgainstDirectory` would eventually drop out-of-scope favourites anyway — but only after a
sync, and only for people no longer visible. It is not a substitute for keying.

### Dependencies & risks

- **Depends on** task 079's chooser and PRD 006's directory. Both shipped.
- **Risk: a role change mid-session surprises the client.** The new profile may hold a different
  role, so anything cached per role must go. Handled by the storage keys plus landing on `/maps`.
- **Risk: the menu grows.** Three items is fine; a fourth should prompt a rethink rather than
  another line.
- **Risk: the audit log is the only trace.** If switching is ever abused, the log is what tells us.
  Cheap to add now, awkward to add retrospectively.

## 9. Success Metrics

- Switching profile requires **zero** SMS messages, verified by test.
- A member with one profile never sees the control — asserted in tests, not by inspection.
- No client state from the previous profile is readable after a switch: cached directory, its
  version, and favourites are all per-profile keys.
- A switch cannot reach a profile on another number — asserted server-side.
- Every switch appears in the BFF log with both profile ids.

## 10. Rollout / Task Breakdown

Server first, since the client cannot draw the control until `/api/me` reports the count.

- [x] Task 179: `POST /api/auth/switch` + `profile_count` on `/api/me`, with tests
- [x] Task 180: per-profile storage keys for the contacts directory and favourites
- [x] Task 181: shared candidate-list component, reused by the login chooser
- [x] Task 182: `Skift profil` in `UserMenu` — dialog, switch, land on `/maps`

All four shipped 2026-09-01. Two things worth carrying forward from the build:

- **The switch completes with a full page load**, not a router push. Persisted state is keyed per
  profile, but in-memory stores are not, and a reload is the only move that cannot leave a stale one
  behind through a path somebody later forgets to reset.
- **`profileCount` is also set by the login chooser**, from its candidate count, so the switcher is
  not missing for the rest of the session that just disambiguated.

## 11. Open Questions

1. **Should a switch require a fresh PIN after some age of session?** Not built. The residual in §8
   is a 7-day window; a "re-verify if the session is older than N days" rule would close most of it
   at the cost of the failure mode the feature exists to avoid. Recorded rather than decided.
2. **Should the previous profile's session be revoked server-side?** Sessions are stateless signed
   cookies, so the old cookie remains valid until it expires if it were ever captured. Revocation
   would need session state, which this app deliberately does not have. Worth naming as a known
   property of the cookie design rather than something the switcher introduces.
3. **Do gøglere and crew ever need this?** They have numbers too, and the duplicate-registration
   data is spejder-heavy. No reason to exclude them, and no reason to expect much use.
4. **Does the *login* chooser need the same enrichment?** Raised 2026-09-01 by a screenshot of the
   switcher showing five rows reading "Klaus" — one `postmandskab` profile named "Klaus Jørgensen"
   and four `gøgler` profiles named "Klaus".

   The switcher now shows full names and the role, which is defensible there: the caller is signed in
   on that number and can already reach every one of those profiles, so nothing is withheld that they
   could not obtain by choosing. **Login was deliberately left alone** — PRD 006 decided the login
   candidate carries no surname and no role, on the grounds that whoever holds the phone may be a
   sibling being shown somebody else's details, and
   `TestVerifySharedNumberAsksToChoose` enforces it.

   But the same member logging in sees the same unanswerable list, which PRD 006 §11 Q9 already
   predicted: "82% of the time it will offer several identical names, which is not a question a user
   can answer." Numbering the rows ("profil 1", "profil 2") keeps the dialog usable on both surfaces,
   and that is as far as this PRD goes.

   The open decision is whether login should also carry the role — arguably the least sensitive field
   on the record, and a population rather than a personal detail. It is PRD 006's rule to relax, not
   this one's, which is why it is recorded rather than changed.
5. **When does the duplicate registration itself get fixed?** Numbering identical rows makes the
   list navigable; it does not make it *meaningful*. Nine rows for one member (PRD 006 §11 Q1) is an
   upstream data problem, and until it is de-duplicated a member has to pick one of several profiles
   that differ in nothing they can see, with their patrol or portrait possibly attached to a
   different one. That is PRD 006 §11 Q9's territory and the real fix.
