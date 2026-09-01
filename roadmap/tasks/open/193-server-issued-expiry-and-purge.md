# 193 — Server-issued expiry and post-event purge on the device

**Status:** open
**Priority:** medium
**Created:** 2026-09-01

## Description

PRD 009 §6. Cached personal data must not outlive the event. The server half exists —
`go/cmd/api/portraitpurge.go` runs on an interval and the person projection carries an expiry —
so this is about the device.

**Expiry is server-issued, never client-computed.** A device clock set wrongly (or set wrongly
on purpose) must not be able to extend the life of a cached directory. So the payload carries
its own deadline and the client honours it; it does not add a TTL to `Date.now()`.

**The dormant device is the honest limit.** A phone that never reopens the app keeps what it has
until the OS evicts it, and a service worker that never runs cannot purge anything. A baked-in
server-issued expiry is the only lever we actually hold, and it only fires when something runs.
Say that plainly in the PRD rather than implying a guarantee — PRD 009 §11.5 and PRD 007 §11.8
both already do.

Smaller than it once was: only adults' records are cached, and no spejder data reaches any
device (PRD 007). That reduces the exposure; it does not remove the requirement.

Shared with task 173, which verifies the whole loop end to end.

## Acceptance Criteria

- [ ] Sensitive cached payloads carry a **server-issued** expiry timestamp; the client refuses
      to use anything past it and clears it.
- [ ] Expiry is honoured on read as well as on a scheduled sweep, so a device that only ever
      opens the app cold still expires its data.
- [ ] Tested with a device clock set backwards — the case the rule exists for.
- [ ] The purge covers both the index and the portrait bytes, in whichever storage they live.
- [ ] The dormant-device limitation is documented where a reader will hit it, not papered over.
- [ ] Nothing `unrecoverable` is purged by this mechanism — an unshipped track is not sensitive
      personal data being retained, it is the user's own recording awaiting upload.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
