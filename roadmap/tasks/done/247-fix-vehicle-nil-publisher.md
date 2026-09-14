# 247 — Fix: vehicle registration panicked on a nil publisher

**Status:** done
**Priority:** high
**Created:** 2026-09-14
**Picked up by:** agent session (Zed)
**Started:** 2026-09-14
**Completed:** 2026-09-14

## Description

Registering a vehicle in the dev stack answered 500 and left a panic in the api log:

```
http: panic serving 172.19.0.5:60794: runtime error: invalid memory address or nil pointer dereference
github.com/nathejk/shared-go/tables/vehicle.commander.Register(...)
	/shared-go/tables/vehicle/commands.go:150
main.(*application).registerVehicleHandler(...)
	/app/cmd/api/vehicles.go:266
```

Introduced by task 235's wiring: `main.go` constructed the entity with
`vehicle.New(ev.publisherOrNil(), …)`.

The broker is connected in the **background** (PRD 008 §6, so startup does not block on
it), and entities are constructed before that happens — so `publisherOrNil()` returned
nil, the commander stored it, and `c.p.MessageFunc()` dereferenced nil on the first
registration.

Two defects, and the second is the worse one:

1. **The panic.** A 500 with a stack trace instead of an answer.
2. **It could never recover.** Construction always precedes connection, so the captured
   nil was permanent: `POST /api/me/vehicles` was broken for the life of the process, and
   a restart would not have helped. Registration had in fact *never* worked outside the
   tests.

`internal/commands.PublisherHolder` exists precisely for this, and `app.commands` has
always gone through it — a shared-go entity cannot, because its constructor takes a
`cqrs.Publisher` and keeps it.

**Why the test suite was green.** Every handler test injects a fake `app.vehicles`, which
is right for testing handler behaviour — and meant nothing exercised what `main.go`
actually hands the entity. The wiring was the one part with no test.

## Acceptance Criteria

- [x] The entity is given a publisher that resolves through the `PublisherHolder`, so it
      picks up the broker when it arrives without being rebuilt
- [x] A registration with no broker fails cleanly rather than panicking
- [x] `ErrNoPublisher` answers `503`, not `500` — the request is fine, the stream is not
- [x] A publisher disappearing between the check and the publish does not panic either
- [x] Reads still work with no broker (PRD 008 §5)
- [x] Tests exercise the **real** entity over the real wiring, not a fake command side
- [x] The regression tests fail against the old wiring, reproducing the panic
- [x] The client distinguishes a `503` from a generic failure
- [x] Verified end to end against the running dev stack, with the plate from the report

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Reported from the dev stack with a screenshot and the api log: the form said
  "Kunne ikke gemme køretøjet" and the log carried a nil-pointer panic in
  `commander.Register`.
- 2026-09-14 — Root cause found in one read of `main.go`: `ev.publisherOrNil()` at wiring
  time is nil, always, because the broker connects in the background afterwards. Confirmed
  by the comment two lines above it, which claims a registration "fails at the command side
  rather than being prevented here" — true only if the publisher is non-nil.
- 2026-09-14 — Fixed with `lazyPublisher` (`cmd/api/lazypublisher.go`): a `cqrs.Publisher`
  that resolves the real one per call through the holder. This is the property
  `app.commands` already had — writes start working the moment the broker arrives.
- 2026-09-14 — Its fallback `MessageFunc` returns an `unpublishableMessage` rather than
  nil. Returning nil would have *moved* the panic to the next line rather than removed it,
  including in the genuinely reachable case where the broker drops between the handler's
  availability check and the publish. Not `streamtest.Message`: that lives in a test
  package, and product code importing it ships a test double to production.
- 2026-09-14 — `ErrNoPublisher` now answers `503` via `vehicleWriteError`, shared by all
  three write handlers. The two failures mean different things to the person holding the
  phone: "try again in a moment" versus "this will not work until somebody fixes it".
  Mirrored in the store as a distinct `unavailable` kind with its own sentence.
- 2026-09-14 — `vehiclewiring_test.go` builds the **real** `vehicle.New(...)` over the same
  `lazyPublisher` `main.go` uses and drives it through HTTP. Four tests: fails cleanly with
  no broker, works once the broker arrives, survives the publisher going away, and reads
  still work with no broker.
- 2026-09-14 — My first version passed `nil` as the reader and panicked inside
  `cqrs.EnsureColumn` — the `kind` migration from task 243 queries INFORMATION_SCHEMA
  through it. Checked whether production can reach that too: it cannot, `openEventing`
  refuses a nil database outright, so `ev != nil` implies a reader. Test now uses `sqlmock`.
- 2026-09-14 — **Verified the regression tests against the old wiring**: swapped
  `lazyPublisher{…}` back for `holder.Get()` and both fail with the same
  `invalid memory address or nil pointer dereference`. The second one failing is the
  important half — it is what proves the bug was permanent rather than a startup race.
- 2026-09-14 — ✅ End to end in the running dev stack, with the reported car: logged in over
  `/api/dev/pin`, `POST /api/me/vehicles` answered **201** with
  `license_plate: "DK+CA63640"`, `GET` read it back, the projected row shows
  `custodianUserId = driverUserId` (the projector's rule) and `kind = car`, and re-posting
  the same plate typed `ca 63 640` answered **409** — normalisation and duplicate detection
  both working against real data.
- 2026-09-14 — Full Go suite, `go vet`, `staticcheck`, `GOWORK=off` build/test, and the 520
  frontend tests all green.
- 2026-09-14 — Found while reading the projected rows: an existing row has
  `licensePlate = EC16795`, unprefixed, because `hq`'s organiser CRUD stores plates raw.
  Raised as task 248 rather than fixed here — it is a cross-repo decision, not a bug in this
  fix.
