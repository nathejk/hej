package main

import (
	"strings"
	"testing"
)

// Every write in the admin tool waits for its own projection before reloading the grid (task 457).
//
// # The bug these guard against, and why it needs guarding at all
//
// A write here publishes an event; the grid reads a projection a jetstream consumer folds from it. The two are
// milliseconds apart in a healthy system and not ordered at all in principle, so a reload issued in the same tick
// as the response is a read with nothing behind it. Task 437 was that bug reported from use — a curator looking at
// a list of photographs they had just uploaded, above a grid that did not have them — and task 439 put the wait on
// the shared context so the rest of the tool could use it. For seven months' worth of features, the rest of the
// tool did not: `ctx.settled` had exactly one caller.
//
// **It is invisible in both directions, which is what makes a guard worth having.** The code does reload, so
// reading it nothing looks wrong; and the race is usually won on a developer's machine, where the fold is
// instant. What it costs is trust: a caption that comes back in its old form reads as a save that did not save,
// and the curator types it again.
//
// Source-read, because there is no JavaScript runtime in this suite. The reads being polled are covered by real Go
// tests — `TestAdminLibraryFiltersByID` for `ids=`, and the filter composition these waits depend on by
// `TestTheIDFilterAsksForExactlyThoseIDs` and the curator filter tests next to it.

// **Nothing reloads the grid without waiting for something first.**
//
// The blanket form is the point: a ninth action added next year gets this for free, which the per-write needles
// below cannot do. Written as "which files reload" rather than a list of names, so adding an action to the tool
// adds it to this test.
func TestEveryAdminScriptThatReloadsTheSheetWaitsFirst(t *testing.T) {
	waits := []string{"ctx.settled(", "ctx.settledFilter(", "ctx.settledRows(", "ctx.settledOrder("}

	for _, name := range adminPageScripts {
		src := stripJSLineComments(adminAsset(t, strings.TrimPrefix(name, "adminui/")))
		if !strings.Contains(src, "ctx.reloadSheet()") {
			continue
		}
		waited := false
		for _, w := range waits {
			if strings.Contains(src, w) {
				waited = true
			}
		}
		if !waited {
			t.Errorf("%s reloads the contact sheet but waits for nothing: a reload in the same tick as the "+
				"write's response reads a projection the event may not have reached, and the curator sees the "+
				"state they just changed (task 437). One of %v belongs here.", name, waits)
		}
	}
}

// And each write waits for **its own** condition.
//
// The blanket test above is satisfied by any wait at all, including the wrong one — and the wrong one is the
// mistake this tool has already made twice. `ctx.settled` asks whether photographs exist, which every one of these
// writes made true before it ran: it returns on the first ask and proves nothing. So each needle below names the
// predicate that is actually load-bearing for that write, and the `why` says what is wrong with the obvious
// alternative.
func TestEachAdminWriteWaitsForWhatItActuallyChanged(t *testing.T) {
	for _, want := range []struct{ file, needle, why string }{
		{"captionaction.js", "ctx.settledRows(ids, (row) => row && (row.caption || '') === caption)",
			"the caption is compared to the string that was sent; presence of the photographs says nothing, and " +
				"the `|| ''` is what makes clearing a caption settle too"},
		{"creditaction.js", "ctx.settledFilter(ids, '&credit=' + encodeURIComponent(proof), true)",
			"a credit may be stored as a crew id and rendered as a name, so the projection is asked in its own " +
				"terms rather than the browser guessing at the server's formatting"},
		{"albumaction.js", "ctx.settledFilter(ids, '&album=' + encodeURIComponent(albumId), true)",
			"membership in the album that was chosen — albumCount is non-zero for a photograph already filed " +
				"somewhere else, so it cannot answer this"},
		{"deleteaction.js", "ctx.settledFilter(ids, '&album=' + encodeURIComponent(albumId), false)",
			"a removal is proved by absence: the photographs still exist, which is the whole difference between " +
				"removing from an album and deleting"},
		{"deleteaction.js", "ctx.settledRows(ids, (row) => row && row.deleted === true)",
			"a delete is proved by the row saying so, not by absence — a photograph the library cannot see at " +
				"all would read as a successful delete"},
		{"patrolaction.js", "ctx.settledFilter(ids, '&tagged=yes', true)",
			"tagCount is a count and cannot say whether this tag landed; tagged=yes is what the grid's own mark " +
				"is drawn from"},
		{"positionaction.js", "ctx.settledFilter(ids, filter, true)",
			"location=yes|no plus the verdict the response reported: the coordinate itself is resolved " +
				"server-side for a checkpoint, so the browser has no numbers to compare"},
		{"albumorder.js", "ctx.settledOrder(albumId, out.order)",
			"a hand move changes positions, not existence, and the order waited for is the server's answer " +
				"rather than the browser re-deriving moveAlbumOrder"},
		{"vieweredit.js", "ctx.settledRows([w.id], (row) => row && (row.caption || '') === w.value)",
			"the viewer's reload is deferred to close, which narrows the race without closing it — captioning " +
				"and pressing Escape is one gesture"},
	} {
		src := stripJSLineComments(adminAsset(t, want.file))
		if !strings.Contains(src, want.needle) {
			t.Errorf("%s no longer has %q: %s", want.file, want.needle, want.why)
		}
	}
}

// The removal's proof cannot be the one the delete uses, and vice versa.
//
// Both live in `deleteaction.js` and they are the two acts the panel exists to keep apart (task 379). Getting
// their waits the wrong way round would be invisible: removing from an album would wait on a `deleted` flag no
// event ever set, time out, and tell every curator that every removal might be stale.
func TestTheRemovalAndTheDeleteWaitForDifferentThings(t *testing.T) {
	src := stripJSLineComments(adminAsset(t, "deleteaction.js"))

	// The negative filter belongs to the removal, and `false` is the whole assertion: with `true` it would wait
	// for the photographs to still be in the album it just took them out of.
	if !strings.Contains(src, "'&album=' + encodeURIComponent(albumId), false)") {
		t.Error("removing from an album must wait for the ids to leave that album's read")
	}
	if strings.Contains(src, "'&album=' + encodeURIComponent(albumId), true)") {
		t.Error("nothing in the delete panel adds photographs to an album, so no wait here should want presence")
	}
}

// The uploader's wait stays **live-only**, and the others do not.
//
// `settledRows` asks with `deleted=1` on purpose: it needs a row for every id so its predicate is about the
// photograph rather than about whether the read happened to include it, and the delete's proof is that row's
// `deleted` field. The uploader must not ask the same way. Re-uploading a photograph a curator deleted publishes
// the same content-derived id and the fold deliberately leaves `deleted` alone (PRD 022 §8.5), so a read including
// deleted rows would tell the uploader the photograph is in the grid when it is neither there nor supposed to be.
func TestTheUploadersWaitDoesNotAskForDeletedPhotographs(t *testing.T) {
	main := stripJSLineComments(adminAsset(t, "main.js"))

	if !strings.Contains(main, "ctx.settled = async (ids) => ctx.settledFilter(ids, '', true);") {
		t.Error("the uploader's wait must ask with no extra filter, which is the library's live-only default: " +
			"a re-uploaded photograph that was deleted must not read as present (PRD 022 §8.5)")
	}
	if !strings.Contains(main, "rowsFor(waiting, '&deleted=1')") {
		t.Error("settledRows must ask with deleted=1, so its predicate is asked about a row that exists — the " +
			"delete's proof is the row's deleted flag, which a live-only read cannot show")
	}
}

// One wording for "this view may be stale", provided once.
//
// Seven writes now say it. Seven wordings of the same caveat is how a curator learns to ignore all of them, and
// the drift would be silent — each sentence is correct on its own.
func TestTheStaleSheetCaveatIsSaidOneWay(t *testing.T) {
	main := adminAsset(t, "main.js")
	if !strings.Contains(main, "ctx.behindNote = 'Kontaktarket kan være et øjeblik bagud") {
		t.Fatal("main.js must provide the one sentence an action says when it gave up waiting")
	}

	// Nobody writes their own copy of it. The uploader is excepted by name: its tail names what the reader can
	// actually check ("hvis nogle mangler"), which is a better sentence in the one place the check is possible.
	for _, name := range adminPageScripts {
		file := strings.TrimPrefix(name, "adminui/")
		if file == "main.js" || file == "upload.js" {
			continue
		}
		src := adminAsset(t, file)
		if strings.Contains(src, "kan være et øjeblik bagud —") {
			t.Errorf("%s writes its own version of the stale-sheet caveat; use ctx.behindNote so the tool says "+
				"it one way", file)
		}
	}
}
