# 383 — Retire the album dev fixture in favour of the real tool

**Status:** done
**Priority:** medium
**Created:** 2026-09-22
**Picked up by:** agent
**Started:** 2026-09-23
**Completed:** 2026-09-23

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

- [x] `devalbum.go` is deleted along with its route registration and any test fixture that depended on
      it
- [x] The legacy-`itemadded` tolerance in the album fold remains, with a comment noting the producer is
      gone but the history is not
- [x] A developer can create, fill and publish an album locally using only `/admin`, documented in one
      or two lines
- [x] The comment in `routes.go` describing the fixture as a stand-in is removed, not left dangling
- [x] The suite passes with no reference to the fixture's endpoint remaining
- [x] Sequenced after phase 3 — this task must not be picked up before the tool can replace it

## What was done

`go/cmd/api/devalbum.go` deleted, its route and the comment above it removed, and the two tests that
asserted things about it removed with it.

**Two functions had to move rather than go.** `publishAlbum` and `writeAlbumPublishFailure` lived in the
fixture because the fixture was once the only thing in the app that created an album; the curator's tool,
the in-app takedown and the reorder path all use them now. They are in a new `albumpublish.go` rather than
in a corner of `adminalbum.go`, and that choice is the interesting part: a shared helper sitting inside the
admin file reads as admin-owned, and the next writer makes a second copy of it. That is the shape PRD 022
§8.9 argues against for the blob purge, and the argument does not change for a subject builder — `album.Subject`
validates the tokens, and an album id with a dot in it publishes successfully while quietly never matching
the per-album patterns again.

**The tolerance stays, with the reason written down.** `handleItemAdded` still skips a `photoId`-less event
silently, and `consumer.go` now says why the branch outlives its producer: removing the producer does not
remove the history. Developers hold databases and brokers full of events the fixture published, and every
one arrives on every replay — deleting the branch would trade one boot-time wall of warnings for another,
which is precisely what §8.7 was careful about. It can go when those streams are gone.

**The two deleted tests were not losses.** `TestDevAlbumFixtureSlugsAreUsableAsSubjectTokens` and
`TestDevAlbumFixtureCoversTheStatesWorthSeeing` described the shape of a hand-written development
convenience, not a property of the product — with the convenience gone there is nothing for them to be true
or false about. Every state they enumerated (an unpublished album, an out-of-bounds coordinate, an item with
no caption, both orientations) is asserted against the real reads by task 382's
`publicvisibility_test.go`, from a fixture that file builds itself. A note in `albummedia_test.go` records
where they went, so the next reader does not conclude the coverage was dropped.

The README gained a short "Photo albums (PRD 022)" section: open `/admin`, log in, drag photographs in,
create an album, add them, tick **Udgivet**. The point of saying so is that the states worth looking at are
now produced the way a real user produces them — which is what makes the fixture's deletion an improvement
rather than a subtraction. It also warns about `PUBLIC_ALBUMS=false`, since in dev every album surface
including the media bytes answers 404 until it is turned on.

## Verified live

```
POST /api/dev/album-fixture -> 404
POST /api/dev/glimt-fixture -> 401
```

The pair is the evidence: the album fixture is gone while its sibling still answers "authenticate", so the
404 is absence rather than a dev-routes block. No legacy `itemadded` warning appeared in the boot log, which
is the tolerance doing its job quietly.

## Notes

- **§8.7's honesty claim now holds going forward.** The argument that changing a published event shape was
  acceptable rests on "no album has ever been created outside a development fixture". That sentence was true
  when written and stays true only because the fixture has stopped creating them.
- **Filed [386](../open/386-album-item-added-deadletters-after-reorder.md) while verifying this.** Every boot
  deadletters two `handleItemAdded` upserts with `Duplicate entry … for key 'album_photo'`. `ON DUPLICATE KEY
  UPDATE` cannot resolve a row that conflicts with **two different rows** on two different unique keys, which
  is what an `itemAdded` replayed onto an already-reordered album does. Not fixed here — it is a fold change
  with its own reasoning and its own tests, and it is unrelated to retiring the fixture.
