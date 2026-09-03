# 205 — every telemetry message carries the reporter's user type

**Status:** done
**Priority:** medium
**Created:** 2026-09-03
**Picked up by:** agent session (Zed)
**Started:** 2026-09-03
**Completed:** 2026-09-03

## Description

Anything this service publishes to the telemetry stream must name the *population* the
reporter belonged to, not just the person. Today `POST /api/track` is the only publisher on
`TELEMETRY.>` (task 084), and its event body carried `personId`, `year` and `points` — a
consumer could tell *who* reported a track but not *what they were*.

Two reasons that matters, and neither is cosmetic:

- **The stream is retained indefinitely; roles are not.** A spejder becomes a bandit, crew
  get reclassified when a section slug is fixed. A reader that joins last year's positions
  against today's directory would silently reinterpret history. The role has to be stamped
  at publish time, because publish time is the only moment it is known to be true.
- **A reader should not need this service to make sense of a message.** The post-race view
  reads the stream back by subject filter (PRD 002 §8); filtering or scoping by population
  should be a property of the message, not a fan-out of lookups into `hej`.

The value is the app role from the session (`users.Role`) — this repo's own vocabulary, and
deliberately not shared-go's `UserType`/`TeamType`, which do not line up one-to-one
(PRD 006 §8 owns that mapping).

## Acceptance Criteria

- [x] `track.Reported` carries a `userType` field and it is populated on every publish.
- [x] The value comes from the **session**, never from the request body.
- [x] A body attempting to set it is a 400 rather than a silently-ignored field.
- [x] Tests assert the published value for more than one role, and assert the spoofing
      attempt is refused.

## Progress Log

- 2026-09-03 — Audited every `commands.Publish` call in the repo first, to be sure this was
  one change and not a pattern to apply in five places: `portrait.go`, `portraitpurge.go`
  and `verification.go` all publish on `NATHEJK.>`, and `track.go` is the only publisher on
  the telemetry stream. So the rule lands in one handler and one event struct.
- 2026-09-03 — **Kept the user type out of the subject, and put it in the body.** Tempting
  to make it a token — it would let a consumer filter with a wildcard and no decode. But the
  subject is keyed per person *because that is the erasure unit*: `nats stream purge
  --subject TELEMETRY.<year>.track.<personId>.reported` is how one individual's track is
  removed (PRD 002 §11.1). Put a mutable attribute in a routing address and the day someone's
  role changes, their track splits across two subject shapes and the purge pattern stops
  matching half of it — quietly making that person's data unerasable. A field that changes
  belongs in the payload; only the identity belongs in the address.
- 2026-09-03 — An empty role is a **500, not a publish**. It cannot happen — every session
  this service issues carries one — so it is our bug rather than the caller's, and an
  unlabelled message on an indefinitely-retained stream is not something a later migration
  can repair (there would be nothing to repair it *from*). Better to fail the request, which
  leaves the points in the phone's IndexedDB, than to write a permanent hole.
- 2026-09-03 — Spoofing is refused for free: `trackRequest` has no such field and `ReadJSON`
  sets `DisallowUnknownFields`, so `userType`/`user_type`/`role` in a body is a 400. Asserted
  anyway, because "cannot be expressed" is a property that a future field addition could
  quietly remove.
- 2026-09-03 — ✅ Green: `go build ./...`, `go test ./internal/track/ ./cmd/api/`.
- 2026-09-03 — **Open item for the `nathejk` repo:** messages published before today have no
  `userType` and decode as `""`. Nothing in `hej` reads the stream (086 is unbuilt), so no
  consumer breaks now, but whatever reads it must treat the empty value as "unknown" rather
  than as a role.
