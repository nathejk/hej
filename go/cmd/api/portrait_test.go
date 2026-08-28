package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/commands"
	"nathejk.dk/internal/imaging"
	"nathejk.dk/nathejk/table/person"
)

// commandsWithPublisher wires a test publisher into the write facade.
func commandsWithPublisher(t *testing.T, p cqrs.Publisher) commands.Commands {
	t.Helper()
	holder := commands.NewPublisherHolder()
	holder.Set(p)
	return commands.New(holder)
}

// commandsWithNoPublisher is the "broker never arrived" state handlers must cope with.
func commandsWithNoPublisher() commands.Commands {
	return commands.New(commands.NewPublisherHolder())
}

// publishedPortrait decodes the one event a test expects to have been published.
//
// It decodes rather than inspecting a captured struct, so the assertions run through
// the same JSON round-trip a real consumer does — which is what makes them able to
// catch a wrong or missing field tag.
func publishedPortrait(t *testing.T, pub *cqrstest.Publisher) person.PortraitCaptured {
	t.Helper()
	if len(pub.Messages) != 1 {
		t.Fatalf("published %d events, want 1", len(pub.Messages))
	}
	var body person.PortraitCaptured
	if err := pub.Messages[0].Body(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

// portraitTestApp is an app with a test publisher and a fixed event year.
//
// The year is set here rather than in `newTestApp` to follow the existing convention
// (see `trackTestApp`, `raceAreaApp`): tests that care about the year state it, so it is
// never unclear which value an assertion depends on.
func portraitTestApp(t *testing.T, pub cqrs.Publisher) *application {
	t.Helper()
	app := newTestApp(t)
	app.config.eventYear = "2026"
	app.commands = commandsWithPublisher(t, pub)
	return app
}

func TestStorePortraitStoresBytesAndPublishesTheRef(t *testing.T) {
	pub := &cqrstest.Publisher{}
	app := portraitTestApp(t, pub)

	data := []byte("not really a jpeg, but bytes are bytes")
	ref, err := app.storePortrait(context.Background(), "member-1", data,
		portraitMeta{ContentType: "image/jpeg", Width: 1024, Height: 1024})
	if err != nil {
		t.Fatalf("storePortrait: %v", err)
	}

	// Content addressed: the ref is the hash of the bytes, not an opaque id.
	if want := blob.ComputeRef(data); ref != want {
		t.Errorf("ref = %q, want the content hash %q", ref, want)
	}
	if ok, _ := app.blobs.Exists(context.Background(), ref); !ok {
		t.Error("bytes were not stored")
	}

	if got := pub.Subjects(); len(got) != 1 || got[0] != "NATHEJK.2026.portrait.member-1.captured" {
		t.Errorf("subjects = %v", got)
	}

	body := publishedPortrait(t, pub)
	if body.Ref != ref.String() {
		t.Errorf("event ref = %q, want %q", body.Ref, ref)
	}
	if body.PersonID != "member-1" || body.Year != "2026" {
		t.Errorf("event identifies %q/%q", body.PersonID, body.Year)
	}
	if body.Bytes != len(data) {
		t.Errorf("event bytes = %d, want %d", body.Bytes, len(data))
	}
	if body.ContentType != "image/jpeg" || body.Width != 1024 || body.Height != 1024 {
		t.Errorf("metadata lost in transit: %+v", body)
	}
	// The retention job (task 109) works from this timestamp, so its presence is a
	// requirement rather than a nicety.
	if body.CapturedAt.IsZero() {
		t.Error("capturedAt must be set")
	}
}

// Storing the same photo twice must converge on one object and one reference. This is
// what makes a projection replay free of side effects.
func TestStorePortraitIsContentAddressedAcrossCalls(t *testing.T) {
	app := portraitTestApp(t, &cqrstest.Publisher{})

	data := []byte("same bytes")
	first, err := app.storePortrait(context.Background(), "member-1", data, portraitMeta{})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := app.storePortrait(context.Background(), "member-1", data, portraitMeta{})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first != second {
		t.Errorf("refs differ for identical bytes: %q vs %q", first, second)
	}
}

// A failed publish must fail the call. The bytes are stored but nothing references
// them, so as far as the app is concerned the portrait was not saved — and reporting
// success would stop nudging a member for a photo nobody can look up.
func TestStorePortraitFailsWhenThePublishFails(t *testing.T) {
	app := portraitTestApp(t, &cqrstest.Publisher{Err: errors.New("broker gone")})

	if _, err := app.storePortrait(context.Background(), "member-1", []byte("x"), portraitMeta{}); err == nil {
		t.Fatal("want an error when the event cannot be published")
	}
}

// With no broker at all the outcome must be the same: not saved.
func TestStorePortraitFailsWithNoPublisher(t *testing.T) {
	app := newTestApp(t)
	app.config.eventYear = "2026"
	app.commands = commandsWithNoPublisher()

	_, err := app.storePortrait(context.Background(), "member-1", []byte("x"), portraitMeta{})
	if !errors.Is(err, commands.ErrNoPublisher) {
		t.Fatalf("err = %v, want ErrNoPublisher", err)
	}
}

// The thumbnails are separate content-addressed objects, referenced from the same event
// **with their own sizes**, so PRD 007 can budget an offline cache before downloading
// anything.
func TestStorePortraitStoresEveryRendition(t *testing.T) {
	pub := &cqrstest.Publisher{}
	app := portraitTestApp(t, pub)

	small := []byte("pretend this is a 96px jpeg")
	medium := []byte("pretend this is a 256px jpeg, which is longer")
	_, err := app.storePortrait(context.Background(), "member-1", []byte("the full image"),
		portraitMeta{
			ContentType: "image/jpeg",
			Thumbs: []imaging.Rendition{
				{Name: "thumb256", Bytes: medium, Width: 256, Height: 192},
				{Name: "thumb96", Bytes: small, Width: 96, Height: 72},
			},
		})
	if err != nil {
		t.Fatalf("storePortrait: %v", err)
	}

	body := publishedPortrait(t, pub)
	if len(body.Thumbs) != 2 {
		t.Fatalf("event carries %d thumbnails, want 2", len(body.Thumbs))
	}

	for i, want := range []struct {
		name          string
		data          []byte
		width, height int
	}{
		{"thumb256", medium, 256, 192},
		{"thumb96", small, 96, 72},
	} {
		got := body.Thumbs[i]
		if got.Name != want.name {
			t.Errorf("thumb %d named %q, want %q", i, got.Name, want.name)
		}
		if got.Ref != blob.ComputeRef(want.data).String() {
			t.Errorf("%s ref = %q, want its content hash", got.Name, got.Ref)
		}
		// The whole reason for the list: size metadata per rendition.
		if got.Bytes != len(want.data) || got.Width != want.width || got.Height != want.height {
			t.Errorf("%s = %d B %dx%d, want %d B %dx%d",
				got.Name, got.Bytes, got.Width, got.Height, len(want.data), want.width, want.height)
		}
		if got.ContentType != "image/jpeg" {
			t.Errorf("%s content type = %q", got.Name, got.ContentType)
		}
		if ok, _ := app.blobs.Exists(context.Background(), blob.Ref(got.Ref)); !ok {
			t.Errorf("%s bytes were not stored", got.Name)
		}
		if got.Ref == body.Ref {
			t.Errorf("%s must be its own object, not the full image's hash", got.Name)
		}
	}

	// The deprecated single-ref field is no longer written.
	if body.ThumbRef != "" {
		t.Errorf("thumbRef = %q, want it unset on new events", body.ThumbRef)
	}
}

// The original is stored as its own object and referenced from the event, with the
// orientation recorded outside the file (task 111).
func TestStorePortraitStoresTheOriginal(t *testing.T) {
	pub := &cqrstest.Publisher{}
	app := portraitTestApp(t, pub)

	original := []byte("the untouched-pixel original, minus its metadata")
	_, err := app.storePortrait(context.Background(), "member-1", []byte("display image"),
		portraitMeta{
			ContentType:         "image/jpeg",
			Original:            imaging.Rendition{Name: "original", Bytes: original, Width: 3000, Height: 4000},
			OriginalContentType: "image/jpeg",
			Orientation:         6,
		})
	if err != nil {
		t.Fatalf("storePortrait: %v", err)
	}

	body := publishedPortrait(t, pub)
	if body.Original == nil {
		t.Fatal("the event carries no original")
	}
	if body.Original.Ref != blob.ComputeRef(original).String() {
		t.Errorf("original ref = %q, want its content hash", body.Original.Ref)
	}
	if body.Original.Ref == body.Ref {
		t.Error("the original must be its own object, not the display image's hash")
	}
	if body.Original.Bytes != len(original) ||
		body.Original.Width != 3000 || body.Original.Height != 4000 {
		t.Errorf("original metadata lost: %+v", body.Original)
	}
	// The one piece of EXIF worth keeping, kept in the log rather than in the file.
	if body.Original.Orientation != 6 {
		t.Errorf("orientation = %d, want 6 — a re-render could not turn it upright without this",
			body.Original.Orientation)
	}
	if ok, _ := app.blobs.Exists(context.Background(), blob.Ref(body.Original.Ref)); !ok {
		t.Error("original bytes were not stored")
	}
}

func TestStorePortraitWithoutAnOriginal(t *testing.T) {
	pub := &cqrstest.Publisher{}
	app := portraitTestApp(t, pub)

	if _, err := app.storePortrait(context.Background(), "member-1", []byte("display"),
		portraitMeta{ContentType: "image/jpeg"}); err != nil {
		t.Fatalf("storePortrait: %v", err)
	}
	if body := publishedPortrait(t, pub); body.Original != nil {
		t.Errorf("original = %+v, want none", body.Original)
	}
}

func TestStorePortraitRefusesBadInput(t *testing.T) {
	app := portraitTestApp(t, &cqrstest.Publisher{})

	if _, err := app.storePortrait(context.Background(), "", []byte("x"), portraitMeta{}); err == nil {
		t.Error("want an error with no person")
	}
	if _, err := app.storePortrait(context.Background(), "member-1", nil, portraitMeta{}); err == nil {
		t.Error("want an error with no data")
	}
	// A person id that cannot be a subject token must be refused rather than published:
	// it would still match NATHEJK.> but no longer match the per-person purge pattern,
	// quietly making that person's portrait unerasable.
	if _, err := app.storePortrait(context.Background(), "member.1", []byte("x"), portraitMeta{}); err == nil {
		t.Error("want an error for an id that is not a single subject token")
	} else if !strings.Contains(err.Error(), "subject token") {
		t.Errorf("error should explain the subject-token rule, got: %v", err)
	}
}
