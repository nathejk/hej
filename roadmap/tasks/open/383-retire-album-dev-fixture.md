# 383 — Retire the album dev fixture in favour of the real tool

**Status:** open
**Priority:** medium
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

## Description

Remove `go/cmd/api/devalbum.go` and its route registration in `go/cmd/api/routes.go`, once the admin
tool can do what it stood in for.

The file states its own expiry condition. It exists because "the only way to see an album before the
curation tool exists (PRD 011 §11 Q5) is to hand-write events onto a broker", and the comment above the
route in `routes.go` says the same. PRD 022 **is** the answer to Q5, so the fixture's reason to exist
ends when phase 3 lands. Leaving it would be worse than untidy: it publishes `album.itemadded` events,
which task 364 changed shape, so a fixture left behind is a generator of exactly the legacy events the
fold now has to ignore — it would manufacture the warnings §8.7 went to trouble to avoid.

Its removal is also what makes the honesty of §8.7's breaking change hold going forward. The argument
that changing a published event shape is acceptable rests on "no album has ever been created outside a
development fixture". That sentence stays true only if the fixture stops creating them.

Keep the **tolerance** in the fold (task 364) even after the fixture is gone. Developers have databases
and brokers with old fixture events in them, and a replay must still not fill the log with warnings.
Removing the producer does not remove the history.

Note this is a dev-only route (`devRoutesEnabled`, `ENV=development`), so removing it changes nothing in
production and cannot break a deploy — the only cost is that a developer wanting an album now uses the
admin tool, which is the point.

## Acceptance Criteria

- [ ] `devalbum.go` is deleted along with its route registration and any test fixture that depended on
      it
- [ ] The legacy-`itemadded` tolerance in the album fold remains, with a comment noting the producer is
      gone but the history is not
- [ ] A developer can create, fill and publish an album locally using only `/admin`, documented in one
      or two lines
- [ ] The comment in `routes.go` describing the fixture as a stand-in is removed, not left dangling
- [ ] The suite passes with no reference to the fixture's endpoint remaining
- [ ] Sequenced after phase 3 — this task must not be picked up before the tool can replace it
