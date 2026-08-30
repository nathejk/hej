# 147 — Lift to shared-go: the member-verification messages

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

## Description

The maintainer will add these to `shared-go` by hand and then bump the dependency in `hej`
(2026-08-30). This task holds the exact shapes so nothing is reconstructed from memory, and
records the reasoning behind each field.

Background: task 132 declared `member.verified` **inside `hej`**, following the portrait
precedent, because nothing outside `hej` consumed it and adding an unused export to a module three
repos depend on costs a version bump in each. The maintainer has now decided these belong in
shared-go, which also settles the question that task left open.

**Do not lift these until the guardian-correction shape below is agreed** — it adds a field to
`member.verified`, and adding a field to a message nothing consumes is free, whereas reshaping one
after `hq` subscribes is not.

---

## 1. `messages/member.go` — two new messages

House style followed: `types.MemberID` / `types.PhoneNumber` for the domain values, a
`// nathejk:<subject pattern>` annotation above each type, one struct per event.

```go
// nathejk:*.member.*.verified
//
// NathejkMemberVerified says that a member has looked at the guardian/emergency contact number
// held for them and acknowledged that it can be reached during the event (hej, PRD 005).
//
// Published by `hej` — the app the member uses — and by nothing else. It is the first member
// fact that originates with the member rather than with the register.
//
// # Why two phone numbers
//
// The acknowledgement is a claim about a *specific* number ("this number can be contacted during
// Nathejk"), so it is only meaningful alongside the number it was made about. And the member may
// acknowledge a number that is NOT the one on file: if they cannot recognise the registered
// number they are asked to supply the correct one and confirm that instead.
//
//	AcknowledgedPhone — the number the member says can be reached. Authoritative for contacting
//	                    a guardian during the event.
//	RegisteredPhone   — what the register held at that moment. Kept so two different questions
//	                    stay answerable:
//	                      * has the register changed since? (RegisteredPhone != current) → the
//	                        acknowledgement is stale and must be asked again
//	                      * did the member correct us? (AcknowledgedPhone != RegisteredPhone) →
//	                        the register is wrong and an organizer should fix it
//
// Collapsing them into one field makes "stale" and "corrected" indistinguishable, and they call
// for opposite responses: re-ask the member, versus update the register and leave the member alone.
type NathejkMemberVerified struct {
	MemberID types.MemberID `json:"memberId"`
	Year     string         `json:"year"`

	// AcknowledgedPhone is normalized. Never empty: a verification that names no number cannot
	// be checked for staleness later, so it would be a permanent tick for a phone nobody agreed
	// to — which is the expensive kind of wrong in an emergency-contact flow.
	AcknowledgedPhone types.PhoneNumber `json:"acknowledgedPhone"`

	// RegisteredPhone is what the register held when the member acknowledged. Empty is
	// meaningful: it says the register had no number and the member supplied one.
	RegisteredPhone types.PhoneNumber `json:"registeredPhone,omitempty"`

	// VerifiedAt is when the member acknowledged, in UTC.
	//
	// On the event rather than derived from delivery time, because delivery time changes on
	// every replay and this timestamp answers "how many members verified before arriving?".
	VerifiedAt time.Time `json:"verifiedAt"`
}

// nathejk:*.member.*.guardianReported
//
// NathejkMemberGuardianReported says that a member could not confirm the guardian number held for
// them and could not supply a replacement either (hej, PRD 005).
//
// A separate event from the absence of a verification, because absence is not a signal: most
// unverified members have simply not opened the app yet. This is a member actively saying
// something is wrong, which is what an organizer can act on before the event.
//
// The reason is a closed pair and the distinction is the point — they are different jobs:
//
//	wrong   — the member can see the number is not the right one → a record to FIX
//	unknown — the member does not know it → a record to CHECK
//
// Useful even when the number turns out to be correct: "this member could not confirm their
// guardian number" tells whoever is holding the phone at 02:00 not to rely on it without calling
// first, whatever the register says.
type NathejkMemberGuardianReported struct {
	MemberID types.MemberID `json:"memberId"`
	Year     string         `json:"year"`

	// Reason is "wrong" or "unknown". A string rather than a typed enum so an older consumer
	// meets an unfamiliar value rather than failing to decode; the publisher validates it.
	Reason string `json:"reason"`

	ReportedAt time.Time `json:"reportedAt"`
}
```

### On `time.Time` versus `types.UnixtimeString`

Several existing messages carry `types.UnixtimeString`. **These two use `time.Time`**, and the
reason is worth stating so it does not look like an oversight: `UnixtimeString` exists to model
payloads the legacy monolith already emits, where the wire format was fixed before the type. These
are new messages with no legacy shape to match, and an RFC3339 timestamp is self-describing when a
human reads the stream a year later, which is the main way anyone reads an event log.

Happy to switch if consistency wins — it is a two-line change on the `hej` side.

---

## 2. Subjects

Only `hej` publishes these, so the subject *builders* can stay in `hej`
(`nathejk/table/person/verified.go`). What must be in shared-go is the **pattern**, in the
annotation comments above, so a consumer can subscribe without reading another repo:

```
NATHEJK.<year>.member.<memberId>.verified
NATHEJK.<year>.member.<memberId>.guardianReported
```

Per person, deliberately: `nats stream purge --subject` can then erase one individual's history
and nothing else. That matters here because these events carry a parent's phone number.

On the `NATHEJK` stream because they are small, low-frequency domain facts about a member, and
`NATHEJK.>` already claims the subject — no broker topology change (contrast the position track,
whose volume forced a sibling stream).

---

## 3. Not now: the portrait messages

`hej` also defines `PortraitCaptured`, `PortraitPurged`, `PortraitThumb` and `PortraitOriginal`
locally (PRD 003, task 103), with the same "this package is bound for shared-go" note. They are
**deliberately not in this lift**: nothing outside `hej` consumes them yet, and PRD 007 — the
feature that reads portraits — is still in `draft/`. Lifting them now would freeze a shape before
its only consumer exists.

Worth lifting the moment `hq` wants to show a face at the check-in counter, and the shapes are
already documented in `nathejk/table/person/portrait.go`.

## 4. Cleanup in `hej` once the dependency lands

Not shared-go changes, but they become possible with it:

- **Drop the duplicated `MemberStatusRacing`.** `nathejk/table/person/querier.go` carries a local
  `const MemberStatusRacing = "racing"` with a comment saying the package cannot depend on
  `internal/`. True, but irrelevant: the package already imports `shared-go/messages`, so it can
  import `shared-go/types` and use `types.MemberStatusRacing` — the persisted string with the
  authoritative definition next to it.
- **Delete the local message declarations** in `nathejk/table/person/verified.go`, keeping the
  subject builders and the projection handler.

## Acceptance Criteria

- [ ] `NathejkMemberVerified` and `NathejkMemberGuardianReported` exist in shared-go's `messages`
      package with the fields above, the annotations, and the reasoning comments
- [ ] shared-go committed, pushed, version-bumped
- [ ] `hej`'s `go.mod` requires the new version and `GOWORK=off go build ./...` succeeds against
      it, not only a workspace build
- [ ] `hej` publishes and projects the shared-go types; the local declarations are removed
- [ ] `RegisteredPhone` is populated on publish, and the projection stores it (task 148)
- [ ] `types.MemberStatusRacing` replaces the duplicated local constant
- [ ] No `hq` changes (PRD 005 §4)

## Depends on

- **Task 148** — the guardian-correction flow, which is what `RegisteredPhone` exists for. Agree
  that shape before lifting: adding a field now is free, reshaping after `hq` subscribes is not.

## Progress Log

- 2026-08-30 — Task created to hold the exact shapes for the maintainer's shared-go update.
