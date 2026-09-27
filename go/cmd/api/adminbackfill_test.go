package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/internal/blob"
	"nathejk.dk/nathejk/table/photo"
)

// The 800px rendition backfill (task 433).
//
// What makes this worth testing carefully is that it is a **write path over existing data**, run by hand,
// against a library that is the event's only record. The two ways it could go wrong are both quiet: it could
// touch rows that did not need it — re-rendering the whole library on every run — or it could put the rendition
// somewhere that grows the backup, which is the thing tasks 429/430 arranged for it not to do.

// backfillApp is an admin app with a recording publisher and a class-recording blob store.
//
// The class store is the point of the setup: the assertion that matters most here is not that a rendition was
// produced but *which class it was stored in*, and nothing else in the response can show that.
func backfillApp(t *testing.T) (*application, *httptest.Server, *classStore, *cqrstest.Publisher) {
	t.Helper()

	app, srv := adminApp(t)
	store := newClassStore(app.blobs)
	app.blobs = store
	pub := &cqrstest.Publisher{}
	app.commands = commandsWithPublisher(t, pub)
	return app, srv, store, pub
}

// seedSource stores a real decodable image and returns its ref.
func seedSource(t *testing.T, app *application, w, h int) string {
	t.Helper()
	ref, err := app.blobs.Put(context.Background(), testImage(t, w, h))
	if err != nil {
		t.Fatalf("seeding a source: %v", err)
	}
	return ref.String()
}

// runBackfill posts one pass and decodes the counts.
func runBackfill(t *testing.T, srv *httptest.Server, query string) adminBackfillResponse {
	t.Helper()

	resp := postAdmin(t, srv, "/api/admin/photos/renditions"+query, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var out adminBackfillResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding the response: %v", err)
	}
	return out
}

// The whole point: a photograph with no medium rendition gets one, at the right size, stored as cache, with the
// ref published so the projection can record it.
func TestTheBackfillProducesTheMediumRendition(t *testing.T) {
	app, srv, store, pub := backfillApp(t)
	source := seedSource(t, app, 1600, 1200)
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{
		{ID: photoID("a"), Ref: source, BoundsVerdict: photo.BoundsNone},
	}}

	got := runBackfill(t, srv, "")
	if got.Examined != 1 || got.Produced != 1 || got.Failed != 0 {
		t.Fatalf("examined/produced/failed = %d/%d/%d, want 1/1/0", got.Examined, got.Produced, got.Failed)
	}

	if len(pub.Messages) != 1 {
		t.Fatalf("published %d events, want 1", len(pub.Messages))
	}
	var added photo.MediumAdded
	if err := pub.Messages[0].Body(&added); err != nil {
		t.Fatalf("decoding the event: %v", err)
	}
	if added.MediumRef == "" {
		t.Fatal("the event carries no mediumRef")
	}
	if added.MediumRef == source {
		t.Fatal("the rendition is the source object, so nothing was actually re-rendered")
	}
	if added.PhotoID != photoID("a") {
		t.Errorf("the event names %q, want the photograph it was produced for", added.PhotoID)
	}

	// The size, read from the bytes rather than from a name.
	rc, err := app.blobs.Get(context.Background(), blob.Ref(added.MediumRef))
	if err != nil {
		t.Fatalf("the rendition is not in the store: %v", err)
	}
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(rc); err != nil {
		t.Fatalf("reading it back: %v", err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("the rendition is not a decodable JPEG: %v", err)
	}
	if cfg.Width != mediumEdge {
		t.Errorf("rendition is %dpx wide, want %d", cfg.Width, mediumEdge)
	}

	// **Cache, not original.** A backfill that grew the backup by a rendition per photograph would defeat the
	// reason task 429 made a third rendition affordable at all.
	if !store.cached[blob.Ref(added.MediumRef)] {
		t.Error("the backfilled rendition was stored as an original, so every one of them would land in the " +
			"backup — exactly what tasks 429/430 arranged for it not to do")
	}
}

// Idempotence, and the property that makes re-running safe: a photograph that already has the rendition is
// never even examined. Without this, every run would re-render the entire library.
func TestTheBackfillIgnoresPhotographsThatAlreadyHaveOne(t *testing.T) {
	app, srv, _, pub := backfillApp(t)
	source := seedSource(t, app, 1600, 1200)
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{
		{ID: photoID("a"), Ref: source, MediumRef: photoID("m"), BoundsVerdict: photo.BoundsNone},
	}}

	got := runBackfill(t, srv, "")
	if got.Examined != 0 {
		t.Errorf("examined %d photographs, want 0: one that already has the rendition must not be re-rendered",
			got.Examined)
	}
	if len(pub.Messages) != 0 {
		t.Errorf("published %d events for nothing to do", len(pub.Messages))
	}
}

// A photograph whose source bytes are gone cannot gain a rendition, and never will. It must be counted, logged
// and skipped rather than stopping the pass.
//
// This is why `examined` and not `failed` is the documented loop condition: a library with one unreadable
// photograph settles at "examined 1, produced 0, failed 1" on every run, so an operator waiting for
// `failed == 0` would loop for ever.
func TestTheBackfillSurvivesAPhotographWhoseBytesAreGone(t *testing.T) {
	app, srv, _, pub := backfillApp(t)
	missing := seedSource(t, app, 1600, 1200)
	present := seedSource(t, app, 1400, 1000)
	if err := app.blobs.Delete(context.Background(), blob.Ref(missing)); err != nil {
		t.Fatalf("removing the source: %v", err)
	}
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{
		{ID: photoID("a"), Ref: missing, BoundsVerdict: photo.BoundsNone},
		{ID: photoID("b"), Ref: present, BoundsVerdict: photo.BoundsNone},
	}}

	got := runBackfill(t, srv, "")
	if got.Examined != 2 || got.Produced != 1 || got.Failed != 1 {
		t.Fatalf("examined/produced/failed = %d/%d/%d, want 2/1/1 — one missing source must not stop the pass",
			got.Examined, got.Produced, got.Failed)
	}
	if len(pub.Messages) != 1 {
		t.Errorf("published %d events, want 1 (only the photograph that could be rendered)", len(pub.Messages))
	}
}

// The batch is bounded. Each row is a decode, a resize and an encode, paid by the process every other request
// is sharing, so an unbounded batch is an unbounded amount of CPU in one request.
func TestTheBackfillBatchIsBounded(t *testing.T) {
	app, srv, _, _ := backfillApp(t)

	rows := make([]photo.LibraryPhoto, 0, 6)
	for _, c := range []string{"a", "b", "c", "d", "e", "f"} {
		rows = append(rows, photo.LibraryPhoto{
			ID: photoID(c), Ref: seedSource(t, app, 900, 700), BoundsVerdict: photo.BoundsNone,
		})
	}
	app.models.PhotoCurator = &libraryCurator{rows: rows}

	if got := runBackfill(t, srv, "?limit=2"); got.Examined != 2 {
		t.Errorf("examined %d with ?limit=2, want 2", got.Examined)
	}

	if adminBackfillDefaultBatch > adminBackfillMaxBatch {
		t.Error("the default batch is above its own ceiling")
	}
	if adminBackfillMaxBatch >= 1000 {
		t.Errorf("adminBackfillMaxBatch is %d; a ceiling that high is not a ceiling", adminBackfillMaxBatch)
	}
}

func TestTheBackfillRefusesANonsenseLimit(t *testing.T) {
	_, srv, _, _ := backfillApp(t)

	for _, q := range []string{"?limit=0", "?limit=-3", "?limit=abc"} {
		resp := postAdmin(t, srv, "/api/admin/photos/renditions"+q, "")
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s answered %d, want 400", q, resp.StatusCode)
		}
	}
}

// The event carries one field and the fold writes one column. That is the whole reason this is not a
// republished `Uploaded`, so it is worth a guard rather than a comment: an `Uploaded` here would have to echo
// six existing values back, and dropping any one of them silently corrupts a row.
func TestTheBackfillPublishesMediumAddedAndNotUploaded(t *testing.T) {
	src, err := os.ReadFile("adminbackfill.go")
	if err != nil {
		t.Fatalf("reading adminbackfill.go: %v", err)
	}
	text := string(src)

	if strings.Contains(text, "photo.Uploaded{") {
		t.Error("the backfill republishes photo.Uploaded. That fold also writes width, height, bytes, the " +
			"coordinate, the verdict and uploadedAt — so a backfill would have to echo six existing values " +
			"faithfully, and a dropped coordinate takes a photograph off the map with nothing to notice it")
	}
	if !strings.Contains(text, "photo.MediumAdded{") {
		t.Error("the backfill should publish photo.MediumAdded")
	}
	if !strings.Contains(text, "PutCache") {
		t.Error("the backfilled rendition must be stored as cache, or the backup grows by one rendition per " +
			"photograph")
	}
	// The bytes must be in the store before the event names them: an event referencing a ref that is not
	// stored yet is the unrecoverable order, while bytes with no event are an unreferenced object the next pass
	// reproduces and dedups for free.
	if strings.Index(text, "PutCache") > strings.Index(text, "photo.MediumAdded{") {
		t.Error("the rendition must be stored before the event is published")
	}
}
