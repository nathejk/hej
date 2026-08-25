# 055 — xstream.Mux and the projector registration pattern

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 008 §8. Add an `xstream.Mux` that fans subjects to projectors, and establish
the three-way registration `hq` uses for every table entity:

1. construct it
2. add it to the `projections` slice registered on the mux
3. pass it into `data.NewModels(...)` and/or the write facade

There are no projectors yet (PRD 006 adds the first). This task lands the
pattern and an empty-but-wired mux, so adding one is mechanical.

## Acceptance Criteria

- [ ] `xstream.Mux` constructed from the JetStream connection
- [ ] A `projections []cqrs.Consumer` slice exists and is registered on the mux
- [ ] The registration pattern is documented in a comment so the next entity
      follows it rather than inventing one
- [ ] Empty projection set does not error at startup
- [ ] `go build`, `go vet`, `go test ./...` green

## Progress Log

- 2026-08-25 — Task created from PRD 008.
