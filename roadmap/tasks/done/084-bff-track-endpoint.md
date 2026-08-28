# 084 — BFF `POST /api/track` publishing to the telemetry stream

**Status:** done
**Priority:** high
**Created:** 2026-08-26
**Picked up by:** agent session (Zed)
**Started:** 2026-08-28
**Completed:** 2026-08-28

## Description

PRD 002 §11.1. Accept a batch of position points from the signed-in user and **publish**
it to the telemetry stream. No SQL: the BFF publishes, exactly as it does for every other
state change (PRD 008 §8).

**Blocked by task 081** — the stream must exist first. JetStream routes by subject, so
publishing to a subject no stream claims fails quietly in the least useful way: the
request succeeds and the data is nowhere.

## Security properties

Two, and both are the kind that are easy to leave out and hard to notice missing:

- **The person is resolved from the session, never from the request body.** Otherwise a
  member can report a track as somebody else — which in a race where position is
  competitively meaningful is not a theoretical concern.
- **The endpoint is rate limited.** A 2-minute cadence needs nothing like the login
  limiter's headroom, and an unbounded ingest endpoint is the one place where a client-side
  bug becomes a broker-capacity problem. Note the existing PIN limiter is per-IP; per-user
  is the more meaningful axis here, since participants will share networks.

## Subjects

Per person, per task 081's agreed pattern, so a later retention or erasure policy can be
applied to one individual. The year comes from the same config the directory uses
(`EVENT_YEAR`) rather than the clock, for the reasons in PRD 006 §11 Q7.

## Acceptance Criteria

- [x] `POST /api/track` behind `requireAuth`, with OpenAPI annotations (mandatory per
      `.rules`, matching the style in `go/cmd/api/auth.go`)
- [x] The person id comes from the session; a body that names a different person is
      ignored or rejected, and there is a test proving a member cannot report as another
- [x] The batch is published to the telemetry stream on a per-person subject
- [x] Writes no SQL
- [x] Rate limited, per user
- [x] Batch size is bounded, with a clear `413`/`400` rather than an unbounded read into
      memory
- [x] Malformed points (missing timestamp, out-of-range coordinates, absurd accuracy) are
      rejected or dropped deliberately, with the choice recorded — this stream is retained
      indefinitely, so junk in it is permanent
- [x] `202` on accept, `401` unauthenticated
- [x] Behaves sanely when the broker is unreachable: the endpoint must fail in a way the
      client can retry (see task 083), **not** accept the batch and lose it. Note
      `commands.ErrNoPublisher` already exists for exactly this state
- [x] Verified end to end against the dev broker: publish a batch, read it back off the
      stream

## Progress Log

- 2026-08-26 — Task created from PRD 002 §11.1.
- 2026-08-28 — Unblocked by task 081 (the `TELEMETRY` stream now exists, owned by the
  `nathejk` repo). Implemented as `POST /api/track` behind `requireAuth`.
- 2026-08-28 — Shape: `internal/track` holds the point type, the validation and the subject
  builder; `cmd/api/track.go` is the handler. Split that way because what is worth getting
  right — which points are junk, and what a subject looks like — is worth testing without a
  request, a session or a broker.
- 2026-08-28 — **The security property is structural rather than defensive.** `trackRequest`
  has no field naming a person, and `ReadJSON` sets `DisallowUnknownFields`, so a body that
  tries to name someone else is a **400** — the attempt cannot be expressed, rather than
  being accepted and overwritten. The test asserts both halves: four plausible field names
  (`personId`, `person_id`, `userId`, `uid`) are each rejected, **and** nothing was published
  on the victim's subject. The second assertion is what keeps the test meaningful if someone
  later adds such a field: the request would then parse, and the subject check would fail.
- 2026-08-28 — **Rate limited per user, not per IP**, and there is a test that fails for an
  IP-keyed limiter: after one user is throttled, a second user on the same connection must
  still be accepted. Participants share networks — a patrol on one phone's hotspot, a klan
  behind one carrier NAT — so an IP limit would punish groups while still letting one client
  flood. 20/minute is ~40× the 2-minute cadence; the headroom exists because a client
  shipping an offline backlog sends several chunks in quick succession (task 083), which is
  exactly when throttling would drop the data it is most important to keep.
- 2026-08-28 — **Malformed points are dropped and counted, not rejected with the batch**, and
  this is the decision most worth recording. The client retries a batch until the server
  accepts it, so failing a whole batch over one bad point would put a member's entire track
  behind a poison pill: the same request would fail forever and every later point would queue
  behind it. One junk point costs one junk point. The response carries
  `{"accepted":N,"dropped":M}` so it is visible rather than silent.
- 2026-08-28 — What counts as junk, and why each: a zero or missing timestamp; a timestamp
  before 2020 or more than 24 h ahead (a phone with a wrong clock is real, and this stream is
  retained indefinitely, so such a point would sit in someone's route permanently);
  coordinates out of range; exactly (0, 0), which is Null Island, the signature of a failed
  fix reported as a success. Deliberately **kept**: a poor accuracy — a 4.8 km cell-tower fix
  is bad data but real, and 086 can filter on it, whereas discarding it here throws away the
  only evidence of where someone was. Only ≥100 km is refused, because that is not a fix.
- 2026-08-28 — An **all-junk batch answers 202 with `accepted: 0`**, not an error. There is
  nothing to publish, but also nothing the client can fix by retrying, and a 4xx would make
  it retry forever. Nothing is published in that case — asserted.
- 2026-08-28 — Oversized batches get **413**, not 400, because a client can act on "too big"
  by splitting (which is what 083's chunking does) and cannot act on a generic bad request.
  The bound is 2,000 points: a full 12-hour race at 30 s is ~1,440, so someone offline for
  the whole event still ships their backlog in one request. Added
  `app.PayloadTooLargeResponse` rather than hand-rolling the response, per the repo rule to
  go through the transport helpers.
- 2026-08-28 — **The subject validates the person id instead of trusting it.** An id
  containing a dot would split into extra tokens; the publish would still *succeed* (it
  matches `TELEMETRY.>`), but the per-person purge pattern would no longer match it, silently
  making that person's track unerasable. Ids are UUIDs in practice — this keeps that true.
- 2026-08-28 — Broker down → **503**, never 202. The batch exists only in the phone's
  IndexedDB until the server takes it, so a 202 would tell the client to delete the only copy
  that exists. Both paths are tested: `commands.ErrNoPublisher`, and any other publish error.
- 2026-08-28 — Substantiated the failure mode task 081 warned about rather than trusting the
  warning: read `jrgensen/stream`'s jetstream publish and found it uses the **acked**
  `js.Publish`, then proved the behaviour on the broker — a request to an unclaimed subject
  answers *"No responders are available"*, while a core `pub` to the same subject reports
  success. So "the request succeeds and the data is nowhere" cannot happen through this path,
  and the 503 mapping covers it.
- 2026-08-28 — Verified end to end against the **dev broker**, signed in as a real person
  from the projection (the mock directory is not in play when a database is present):
  * `POST /api/track` with 2 good and 2 junk points → `202 {"accepted":2,"dropped":2}`;
  * the message landed on
    `TELEMETRY.2026.track.c9ce6de0-c12c-4ec8-9de2-22645fb6f898.reported`, matching the
    session's `user_id` exactly;
  * the body read back off the stream carries `personId`, `year` and **only the two good
    points**, with `meta.producer: hej-api`;
  * **`NATHEJK` stayed at 29,136 messages** — the check that this writes to the sibling
    stream and nothing else;
  * unauthenticated → 401; 2,001 points → 413 with `batch exceeds 2000 points`.
  * The test message was purged afterwards, so the dev stream is empty for 083.
- 2026-08-28 — Writes no SQL: the handler holds only `app.commands`, which can publish and
  nothing else, so the rule is enforced by what the handler can reach rather than by review.
- 2026-08-28 — Gates green both ways (`gofmt`, `build`, `vet`, `staticcheck`, `test`, and the
  same with `GOWORK=off`). PRD 002's two BFF-side §11.1 criteria ticked.
- 2026-08-28 — ✅ All criteria complete. Moving to done. **Task 083 is now unblocked** — and
  with it the privacy-page copy becomes true, which is the honesty gap flagged earlier.
