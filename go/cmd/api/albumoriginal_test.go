package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/imaging"
	"nathejk.dk/nathejk/table/photo"

	"github.com/jrgensen/cqrs/cqrstest"
)

// Storing the photographer's file (PRD 027 R1/R12, task 479).
//
// # The whole of PRD 027 in one sentence, and this file is the test of it
//
// **The archive gains metadata; no reader does.**
//
// `cmd/api/albummedia.go` used to say "**Never** change the pipeline to preserve EXIF because this feature wants a
// coordinate", and PRD 027 changes that. The reason it is safe is that the prohibition was protecting two different
// things which only looked like one while the archive master and the served bytes were the same object:
//
//   - no reader may be handed unexamined metadata — unchanged, and `TestStoreAlbumImageReadsTheCoordinateAndStripsIt`
//     plus `TestTheRenditionsStillCarryNoMetadata` below are what hold it;
//   - the archive may not hold metadata — this is what changed, because an archive that has lost the capture time,
//     the camera and the lens is a worse archive and none of it can be recovered later.
//
// So the two halves are tested together, in one file, deliberately. Seeing only one of them is how somebody
// concludes the rule was simply dropped.

// failingPutStore refuses to store originals, and nothing else.
//
// Only `Put` fails: the renditions go through `PutCache`, so this isolates "the one object whose loss is
// unrecoverable could not be written" from "the volume is broken".
type failingPutStore struct {
	blob.Store
	err error
}

func (s *failingPutStore) Put(context.Context, []byte) (blob.Ref, error) { return "", s.err }

// readStoredBlob returns an object's bytes.
//
// Shared with the download tests: both halves of PRD 027 are assertions about bytes being passed through *unchanged*,
// so both need to compare against what is actually on disk rather than against a re-derivation.
func readStoredBlob(t *testing.T, app *application, ref string) []byte {
	t.Helper()

	raw, err := app.readBlob(context.Background(), blob.Ref(ref))
	if err != nil {
		t.Fatalf("reading the stored object: %v", err)
	}
	return raw
}

// The stored original is the uploaded file, byte for byte, with its GPS intact.
func TestTheOriginalIsTheUploadedFileWithItsMetadata(t *testing.T) {
	app := newTestApp(t)
	app.models.RaceAreas = stubRaceAreas{area: testRaceArea(), ok: true}

	// A real coordinate in the race area, so the fixture is the interesting case rather than a file with no EXIF.
	raw := jpegWithTestGPS(t, 55, 43, 59.74, 'N', 12, 15, 53.35, 'E')
	if _, _, ok := imaging.ReadGPS(raw); !ok {
		t.Fatal("the fixture should carry a coordinate to begin with")
	}

	prepared, err := app.storeAlbumImage(context.Background(), app.config.eventYear, raw)
	if err != nil {
		t.Fatalf("storeAlbumImage: %v", err)
	}
	if prepared.Original == nil {
		t.Fatal("no original was stored")
	}

	stored := readStoredBlob(t, app, prepared.Original.Ref)

	// Byte for byte. Asserted as equality rather than by sampling properties, because "unchanged" is the
	// requirement and any re-encode — even a lossless one — would be a different file with different metadata.
	if !bytes.Equal(stored, raw) {
		t.Errorf("the stored original is not the uploaded file (%d bytes stored, %d uploaded). PRD 027 R1: no "+
			"re-encode, no resize, no strip. Anything else is our rendering of the photograph rather than the "+
			"photograph.", len(stored), len(raw))
	}
	// And the consequence, stated separately so the failure says which rule broke.
	if _, _, ok := imaging.ReadGPS(stored); !ok {
		t.Error("the stored original carries no GPS. That is the previous behaviour, and PRD 027 reversed it: " +
			"an archive master without the capture time, the camera and the coordinate is a worse archive, and " +
			"none of it can be recovered later. The guard that makes this safe is originalboundary_test.go — " +
			"nothing outside requireAdmin may resolve this ref.")
	}

	// The ref is the content hash of those exact bytes, which is what keeps an original verifiable — `blob.PutAs`
	// refuses to overwrite one for the same reason.
	if want := blob.ComputeRef(raw).String(); prepared.Original.Ref != want {
		t.Errorf("the original's ref is %q, want the content hash %q", prepared.Original.Ref, want)
	}
}

// **The other half, and it must never move.** Every rendition is still stripped.
//
// `TestStoreAlbumImageReadsTheCoordinateAndStripsIt` covers the display image and was deliberately left untouched by
// PRD 027 — if it had needed editing, the change would have gone wrong. This extends the same assertion to the other
// two renditions, which that test does not reach and which are what a reader is actually served: the viewer's `srcset`
// offers the 800px copy to a phone, and the contact sheet and public grid use the thumbnail.
func TestTheRenditionsStillCarryNoMetadata(t *testing.T) {
	app := newTestApp(t)
	app.models.RaceAreas = stubRaceAreas{area: testRaceArea(), ok: true}

	raw := jpegWithTestGPS(t, 55, 43, 59.74, 'N', 12, 15, 53.35, 'E')
	prepared, err := app.storeAlbumImage(context.Background(), app.config.eventYear, raw)
	if err != nil {
		t.Fatalf("storeAlbumImage: %v", err)
	}

	for name, ref := range map[string]string{
		"display image":   prepared.Ref,
		"800px rendition": prepared.MediumRef,
		"320px thumbnail": prepared.ThumbRef,
	} {
		if ref == "" {
			t.Fatalf("no %s was produced, so this test asserts nothing about it", name)
		}
		if _, _, ok := imaging.ReadGPS(readStoredBlob(t, app, ref)); ok {
			t.Errorf("the %s still carries GPS. This is the half of the old rule that does not move: the "+
				"renditions are what readers are served, on the public album page and in the PWA, and a "+
				"photograph of a child must not carry where it was taken inside a file nobody has looked at "+
				"(PRD 003 §6, PRD 027 R12).", name)
		}
	}
}

// Every upload gets an original — there is no size or pixel-count condition.
//
// `imaging.Prepare` declines to keep a portrait original that has no more pixels than the display image, measured in
// production on 2026-08-29 at 1.9x the storage for nothing. That measurement was about a **stripped** same-size copy,
// which really does carry no additional information. Once the metadata stays, the premise is gone: the capture time,
// camera, lens and coordinate are facts no rendition has. So a 320px upload gets an original too — and `originalRef`
// means one thing rather than "present, unless one of two conditions you have to go and read".
func TestEveryUploadGetsAnOriginalEvenASmallOne(t *testing.T) {
	app := newTestApp(t)

	for _, size := range []struct{ w, h int }{
		{320, 240},   // smaller than every rendition
		{800, 600},   // exactly the medium edge
		{1600, 1200}, // exactly the display edge, so nothing is resized at all
		{4000, 3000}, // the real case
	} {
		prepared, err := app.storeAlbumImage(context.Background(), app.config.eventYear,
			testImage(t, size.w, size.h))
		if err != nil {
			t.Fatalf("%dx%d: %v", size.w, size.h, err)
		}
		if prepared.Original == nil || prepared.Original.Ref == "" {
			t.Errorf("a %dx%d upload kept no original. Every upload gets one (PRD 027 R1): the "+
				"\"only if it has more pixels\" rule belongs to stripped portrait originals, where a same-size "+
				"copy adds nothing, and does not apply once the metadata is retained.", size.w, size.h)
			continue
		}
		if got := prepared.Original.Width; got != size.w {
			t.Errorf("a %dx%d upload recorded width %d", size.w, size.h, got)
		}
	}
}

// A store that cannot take the original fails the upload.
//
// Every other object in this path is recoverable: a thumbnail rebuilds, the display image rebuilds from the original,
// and a failed publish leaves bytes a retry reuses for free. The original rebuilds from nothing. So accepting the
// photograph anyway would leave one frame quietly un-archivable, discovered years later for no visible reason — the
// same call the portrait path already makes.
func TestAFailureStoringTheOriginalFailsTheUpload(t *testing.T) {
	app := newTestApp(t)
	wanted := errors.New("no room on the volume")
	app.blobs = &failingPutStore{Store: app.blobs, err: wanted}

	_, err := app.storeAlbumImage(context.Background(), app.config.eventYear, testImage(t, 2000, 1500))
	if err == nil {
		t.Fatal("the ingest succeeded without storing an original. Keeping the photograph and silently " +
			"dropping its original is the one outcome PRD 027 §5 rules out: nothing would ever report it, and " +
			"there is no second chance to produce the file.")
	}
	if !errors.Is(err, wanted) {
		t.Errorf("the failure should carry the store's error for the log, got %v", err)
	}
}

// An unreadable file is refused before anything is stored.
//
// The decode is the validation (PRD 022 §8.4), and the header read now happens first — so a `.mov`, a raw file or a
// `Thumbs.db` off the same card is rejected without putting bytes into the volume that is backed up and never purged.
// It must still surface as the same error, because the handler turns exactly one error into exactly one Danish
// sentence a photographer reads in a table of 300 rows.
func TestANonImageIsRefusedBeforeTheOriginalIsStored(t *testing.T) {
	app := newTestApp(t)
	store := newClassStore(app.blobs)
	app.blobs = store

	_, err := app.storeAlbumImage(context.Background(), app.config.eventYear, []byte("this is not an image"))
	if !errors.Is(err, errGlimtNotMedia) {
		t.Fatalf("want errGlimtNotMedia so the handler's one Danish message still covers it, got %v", err)
	}
	if len(store.cached) != 0 {
		t.Errorf("%d objects were stored for a file that is not an image; nothing should reach the store",
			len(store.cached))
	}
}

// The published event carries the original, so the projection can hold it.
//
// Asserted on the event rather than only on the blob store, because those are two separate failures with the same
// appearance: bytes stored but never named by an event are bytes nothing can ever find again — orphaned in the volume
// that is backed up and never purged, invisible to every read, and not even reachable by a takedown.
func TestAdminUploadPublishesTheOriginal(t *testing.T) {
	app, srv := uploadApp(t, &stubPhotoCurator{})
	pub := &cqrstest.Publisher{}
	app.commands = commandsWithPublisher(t, pub)

	raw := jpegWithTestGPS(t, 55, 43, 59.74, 'N', 12, 15, 53.35, 'E')
	if resp := postPhoto(t, srv, "photo", raw); resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	if len(pub.Messages) != 1 {
		t.Fatalf("want 1 event, got %d", len(pub.Messages))
	}
	var uploaded photo.Uploaded
	if err := pub.Messages[0].Body(&uploaded); err != nil {
		t.Fatalf("decoding: %v", err)
	}

	if uploaded.Original == nil {
		t.Fatal("the event carries no original. The bytes are in the store either way, which is the trap: an " +
			"object no event names is unreachable by every read *and* by the takedown that would free it — so it " +
			"sits in the backup scope forever with nothing pointing at it.")
	}
	if want := blob.ComputeRef(raw).String(); uploaded.Original.Ref != want {
		t.Errorf("the event names original %q, want the uploaded bytes' hash %q", uploaded.Original.Ref, want)
	}
	if uploaded.Original.Bytes != len(raw) {
		t.Errorf("the event says %d bytes, want %d", uploaded.Original.Bytes, len(raw))
	}
	if uploaded.Original.Width == 0 || uploaded.Original.Height == 0 {
		t.Error("the event carries no dimensions for the original; a reader cannot tell what it is holding")
	}
	// And the display image is still the id, which is what keeps the upload path idempotent (PRD 022 §8.5). If the
	// original's hash ever became the id, re-uploading a stripped copy of the same photograph would create a second
	// row for one photograph.
	if uploaded.PhotoID != uploaded.Ref {
		t.Errorf("photoId %q is not the display rendition's ref %q: the id must stay the hash of the stored "+
			"rendition, or a re-upload of a re-saved file becomes a second photograph", uploaded.PhotoID, uploaded.Ref)
	}
}

// # Why this is a source guard
//
// Because switching `keepOriginal` on here would *look* like the right way to implement PRD 027 — the parameter is
// literally called that — and it would be wrong in two ways at once that no behavioural test would obviously catch:
// it strips the metadata PRD 027 exists to keep, and it declines a same-size original. The result would be a library
// that appears to store originals and silently stores censored ones.
//
// So the rule is stated where somebody would reach for it, and the portrait side is asserted too: this must not be
// read as "keepOriginal is deprecated". It is right for portraits, where stripping is still the rule, because a
// portrait is a photograph of a person and often of a minor.
func TestTheLibraryDoesNotUseKeepOriginal(t *testing.T) {
	library, err := os.ReadFile("albummedia.go")
	if err != nil {
		t.Fatalf("reading albummedia.go: %v", err)
	}
	body := withoutComments(string(library))

	if !strings.Contains(body, "imaging.Prepare(raw, maxGlimtEdge, libraryThumbEdges, glimtJPEGQuality, false)") {
		t.Error("the library's imaging.Prepare call no longer passes keepOriginal=false.\n" +
			"Switching it on would look like the obvious way to implement PRD 027 and is wrong twice: that " +
			"path strips the metadata PRD 027 exists to keep, and it declines an original with no more pixels " +
			"than the display image. The library's original is the raw uploaded bytes, stored by " +
			"storeLibraryOriginal.")
	}

	portrait, err := os.ReadFile("photo.go")
	if err != nil {
		t.Fatalf("reading photo.go: %v", err)
	}
	if !strings.Contains(withoutComments(string(portrait)), "keepOriginal)") {
		t.Error("the portrait path no longer passes keepOriginal through. PRD 027 did not deprecate it: " +
			"stripping is still right for a photograph of a person, often a minor (PRD 003 §6).")
	}
}
