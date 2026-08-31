# 173 — Post-event purge verification

**Status:** open
**Priority:** low
**Created:** 2026-08-31

## Description

Portraits and directory data must not outlive the event (PRD 007 §6, §9).

The **server side already exists**: `go/cmd/api/portraitpurge.go` runs a purge on an
interval and the person projection carries an expiry. This task is about verifying the
whole loop rather than building it:

- server storage is actually empty afterwards;
- the cached directory expires on devices, driven by a **server-issued** expiry — server-
  issued so a wrong device clock cannot defeat it;
- the patrol lookup needs nothing, since it stores nothing (task 157).

The open problem is the **dormant device** (PRD 007 §11.8): a phone that never reopens
the app keeps its cached directory until the OS evicts it, and a service worker that
never runs cannot purge anything. A baked-in expiry timestamp is the only lever we
actually hold. Smaller than it was — only adults' records are cached now, and no spejder
data is on any device — but it should be stated honestly rather than assumed solved.
Shared with PRD 009.

## Acceptance Criteria

- [ ] Purge verified end to end after an event (or a simulated one): server storage
      empty.
- [ ] Client expiry verified on a sample of devices, including one that was offline at
      the expiry moment.
- [ ] Expiry proven server-issued: a device with a skewed clock still expires.
- [ ] The dormant-device limitation documented in the PRD as a known residual, with
      whatever mitigation we settle on.
- [ ] Confirmed that a patrol lookup leaves nothing to purge (cross-check task 170).

## Progress Log

- 2026-08-31 — Task created from PRD 007 §6 / §9 / §11.8.
