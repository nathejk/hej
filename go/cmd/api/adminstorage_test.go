package main

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"nathejk.dk/internal/blob"
)

// The disk floor under an organizer upload (PRD 022 §8.11, §11 Q5; task 384).
//
// # What is being asserted, and what cannot be
//
// The capacity question itself is operational — somebody looking at a real volume, which is the bulk of task
// 384. What is testable is the *mechanism*: that the check exists, that it is against the result of the
// upload rather than its current state, that it refuses with a 507 and not a 400, and above all **that it
// fails open**. That last one is the property most likely to be inverted by a well-meaning edit, and
// inverting it means a photographer in a field losing a hand-in because a `statfs` call returned an error.

// spaceStore wraps a blob store with a free-space answer, or a failure to give one.
//
// A wrapper rather than a fake store, so the bytes still go somewhere real: an upload that is *allowed* must
// still succeed, and a store that only answers the capacity question would make the allow-path untestable.
type spaceStore struct {
	blob.Store
	free uint64
	err  error

	calls int
}

func (s *spaceStore) FreeBytes() (uint64, error) {
	s.calls++
	if s.err != nil {
		return 0, s.err
	}
	return s.free, nil
}

// A volume with no room refuses the upload, and says so as a 507.
func TestAnAdminUploadIsRefusedWhenTheVolumeIsAtItsFloor(t *testing.T) {
	app, srv := uploadApp(t, &stubPhotoCurator{})
	app.config.adminDiskFloorBytes = 1 << 30
	app.blobs = &spaceStore{Store: app.blobs, free: 1 << 20} // a megabyte left under a gigabyte floor

	resp := postPhoto(t, srv, "photo", devFixtureImage(800, 600, 0, 0, "floor"))
	if resp.StatusCode != http.StatusInsufficientStorage {
		t.Fatalf("want 507, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	// The message must not invite a retry and must not blame the photographer, because neither would be
	// true. Asserted on the substantive half of the Danish: keep the card.
	if !strings.Contains(string(body), "Behold kortet") && !strings.Contains(string(body), "behold kortet") {
		t.Errorf("the refusal must tell the photographer to keep the card: %s", body)
	}
}

// The check is against the *result*, not the current state.
//
// Otherwise an upload arriving with one byte more than the floor free would be allowed to write fifty
// megabytes — the same mistake `checkGlimtStorageCeiling` documents avoiding, and the same fix.
func TestTheDiskFloorCountsWhatIsAboutToBeWritten(t *testing.T) {
	app, _ := uploadApp(t, &stubPhotoCurator{})
	app.config.adminDiskFloorBytes = 1000

	// Exactly on the floor with nothing incoming: fine.
	app.blobs = &spaceStore{Store: app.blobs, free: 1000}
	if got := app.checkAdminDiskFloor(0); got != adminDiskOK {
		t.Errorf("free == floor with nothing incoming must be allowed, got %v", got)
	}
	// The same free space, with one byte arriving: refused. A check against the current state would pass.
	if got := app.checkAdminDiskFloor(1); got != adminDiskFull {
		t.Errorf("an upload that would breach the floor must be refused, got %v", got)
	}
}

// **The load-bearing one.** An unmeasurable volume allows the upload.
//
// The asymmetry against the moderation checks is deliberate and is stated in `adminstorage.go`: wrongly
// allowing an upload costs some disk, which an operator can see; wrongly refusing one costs a photographer's
// hand-in, possibly after their card has been wiped. A floor is a safety margin, not an authorization.
func TestTheDiskFloorFailsOpen(t *testing.T) {
	app, _ := uploadApp(t, &stubPhotoCurator{})
	app.config.adminDiskFloorBytes = 1 << 40 // a terabyte: nothing could satisfy it

	// A failing measurement.
	failing := &spaceStore{Store: app.blobs, err: errors.New("statfs: no")}
	app.blobs = failing
	if got := app.checkAdminDiskFloor(1); got != adminDiskOK {
		t.Errorf("a failing free-space read must allow the upload, got %v", got)
	}
	if failing.calls != 1 {
		t.Errorf("want the store asked exactly once, got %d", failing.calls)
	}

	// A store with no volume at all — the in-memory one, and whatever object store comes later. Not an
	// error: there is genuinely nothing to measure, which is different from being unable to measure it.
	app.blobs = failing.Store
	if got := app.checkAdminDiskFloor(1); got != adminDiskOK {
		t.Errorf("a store that cannot report free space must allow the upload, got %v", got)
	}
}

// Zero disables it, like every other ceiling in this service, and disabling it must not consult the store.
//
// Asserted on the call count rather than only on the verdict: a check that asked and then ignored the answer
// would behave identically today and would be one edit away from a surprise.
func TestADiskFloorOfZeroIsOff(t *testing.T) {
	app, _ := uploadApp(t, &stubPhotoCurator{})
	app.config.adminDiskFloorBytes = 0
	store := &spaceStore{Store: app.blobs, free: 0}
	app.blobs = store

	if got := app.checkAdminDiskFloor(1 << 30); got != adminDiskOK {
		t.Errorf("a floor of zero must allow anything, got %v", got)
	}
	if store.calls != 0 {
		t.Errorf("a disabled floor must not measure the volume, got %d calls", store.calls)
	}
}

// Room means the upload proceeds normally. The floor must not become a thing that refuses everything.
func TestAnAdminUploadSucceedsWithRoomToSpare(t *testing.T) {
	app, srv := uploadApp(t, &stubPhotoCurator{})
	app.config.adminDiskFloorBytes = 1 << 20
	app.blobs = &spaceStore{Store: app.blobs, free: 1 << 40}

	resp := postPhoto(t, srv, "photo", devFixtureImage(800, 600, 0, 0, "floor"))
	out := decodeUpload(t, resp)
	if out.PhotoID == "" {
		t.Error("an upload with room available must be stored")
	}
}

// The real store can actually answer the question.
//
// Everything above runs against a wrapper, so without this the production path could fail to implement
// `FreeSpacer` at all and every test here would still pass — the floor would be permanently off, failing open
// exactly as designed and protecting nothing.
func TestTheFileStoreReportsFreeSpace(t *testing.T) {
	store, err := blob.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	spacer, ok := any(store).(blob.FreeSpacer)
	if !ok {
		t.Fatal("the file store must report free space, or the floor silently never applies in production")
	}
	free, err := spacer.FreeBytes()
	if err != nil {
		t.Fatalf("FreeBytes: %v", err)
	}
	// Not an exact number — this is somebody's laptop or a CI runner. A megabyte is a floor no machine that
	// could run this test is below, and it catches the answer that would matter: zero.
	if free < 1<<20 {
		t.Errorf("got %d free bytes, which cannot be right; a zero would disable the floor by accident", free)
	}
}
