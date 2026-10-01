package main

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/photo"

	"github.com/jrgensen/cqrs/cqrstest"
)

// A takedown must free the photographer's original too (PRD 027 R8, task 478).
//
// # Why this is the most consequential test in PRD 027
//
// "Deleted from the library" is what somebody means when they say *take it down* (PRD 022 §5). Every other copy of a
// photograph in this system is a metadata-stripped re-encode; the original is the photographer's file with its EXIF
// intact, **including the coordinate of where it was taken**.
//
// So a takedown that freed the renditions and left the original behind would take the photograph off every page while
// the most sensitive copy of it stayed on disk — inside the one volume that is backed up and never purged (PRD 022
// §11 Q2). That is a takedown in name only, and it is specifically a failure that the person who asked for the
// photograph to come down would never be able to detect.
//
// The opposite error is just as real and is why task 478 landed **before** anything stored an original: content
// addressing means two rows can legitimately name the same bytes, so freeing too eagerly destroys a file that is
// still in use — and an original, unlike a rendition, cannot be rebuilt (task 430 exists only for renditions).
//
// These two pull in opposite directions, so both are tested.

// deleteAdminPhoto sends the takedown with the credential and a reason.
func deleteAdminPhoto(t *testing.T, srv interface{ Client() *http.Client }, url, photoID string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodDelete, url+"/api/admin/photos/"+photoID,
		strings.NewReader(`{"reason":"en forælder har bedt om det"}`))
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set(adminYearHeader, "2026")
	req.SetBasicAuth(testAdminUser, testAdminPass)

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// The original's bytes are gone after a takedown.
func TestAdminDeleteFreesTheOriginal(t *testing.T) {
	app, srv := adminApp(t)

	full, err := app.blobs.Put(context.Background(), []byte("the 1600px display image"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	thumb, err := app.blobs.Put(context.Background(), []byte("its thumbnail"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	medium, err := app.blobs.Put(context.Background(), []byte("its 800px rendition"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	// The one that carries EXIF, and the one that cannot be produced again.
	original, err := app.blobs.Put(context.Background(), []byte("the photographer's file, GPS and all"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{{
		ID: full.String(), Ref: full.String(), ThumbRef: thumb.String(), MediumRef: medium.String(),
		OriginalRef: original.String(), OriginalContentType: "image/jpeg", OriginalBytes: 8_412_907,
		BoundsVerdict: photo.BoundsNone,
	}}}
	app.commands = commandsWithPublisher(t, &cqrstest.Publisher{})

	if got := deleteAdminPhoto(t, srv, srv.URL, full.String()).StatusCode; got != http.StatusNoContent {
		t.Fatalf("want 204, got %d", got)
	}

	// All four, and the original named separately in the failure so the message says which rule broke.
	for name, ref := range map[string]string{
		"display image": full.String(), "thumbnail": thumb.String(), "800px rendition": medium.String(),
	} {
		if exists, _ := app.blobs.Exists(context.Background(), blobRefOf(ref)); exists {
			t.Errorf("the %s should have been freed", name)
		}
	}
	if exists, _ := app.blobs.Exists(context.Background(), blobRefOf(original.String())); exists {
		t.Error("the **original** survived the takedown. It is the photographer's file with its EXIF intact, " +
			"including where the photograph was taken, and it sits in the volume that is backed up and never " +
			"purged — so a takedown that leaves it behind takes the photograph off every page while keeping the " +
			"most sensitive copy of it on disk. PRD 027 R8.")
	}
}

// A photograph with no original is a normal takedown, not a partial one.
//
// This is the state of **most of the library**: everything uploaded before PRD 027 shipped has no original and never
// will. A delete path that treated an empty ref as something to free, or as something to complain about, would make
// the ordinary case the broken one.
func TestAdminDeleteOfAPhotographWithNoOriginalIsOrdinary(t *testing.T) {
	app, srv := adminApp(t)

	full, err := app.blobs.Put(context.Background(), []byte("a photograph from before PRD 027"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	photos := &stubPhotos{}
	app.models.Photos = photos
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{{
		ID: full.String(), Ref: full.String(), BoundsVerdict: photo.BoundsNone,
	}}}
	app.commands = commandsWithPublisher(t, &cqrstest.Publisher{})

	if got := deleteAdminPhoto(t, srv, srv.URL, full.String()).StatusCode; got != http.StatusNoContent {
		t.Fatalf("want 204, got %d", got)
	}
	if exists, _ := app.blobs.Exists(context.Background(), blobRefOf(full.String())); exists {
		t.Error("the display image should have been freed")
	}
	// An empty ref must not reach the sharing check: `RefsInUse` would be asked whether "" is in use, and a ref is
	// the one string in this service that becomes a filesystem path.
	for _, asked := range photos.asked {
		for _, ref := range asked {
			if ref == "" {
				t.Error("an empty original ref was passed to the sharing check; it should not be in the list at all")
			}
		}
	}
}

// **The direction that destroys data.** An original shared with another live photograph is not freed.
//
// Two library rows naming the same original is not hypothetical: identical bytes are one object, so a photographer
// handing in the same file twice — or two photographers who were sent the same file — produces one stored original
// with two rows pointing at it. Freeing it on the first takedown would silently empty the second photograph's archive
// master, and there is nothing to rebuild it from.
func TestAdminDeleteKeepsAnOriginalAnotherPhotographStillHolds(t *testing.T) {
	app, srv := adminApp(t)

	full, err := app.blobs.Put(context.Background(), []byte("one of two display images"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	shared, err := app.blobs.Put(context.Background(), []byte("an original both rows name"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	// The library reports the shared original as still in use — which is exactly what the real `RefsInUse` does once
	// it knows about the column, and the reason task 477 added it to that query's column list.
	app.models.Photos = &stubPhotos{inUse: map[string]bool{shared.String(): true}}
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{{
		ID: full.String(), Ref: full.String(), OriginalRef: shared.String(), BoundsVerdict: photo.BoundsNone,
	}}}
	app.commands = commandsWithPublisher(t, &cqrstest.Publisher{})

	if got := deleteAdminPhoto(t, srv, srv.URL, full.String()).StatusCode; got != http.StatusNoContent {
		t.Fatalf("want 204, got %d", got)
	}

	if exists, _ := app.blobs.Exists(context.Background(), blobRefOf(shared.String())); !exists {
		t.Error("the shared original was freed while another live photograph still names it. An original cannot " +
			"be rebuilt — task 430's repair exists only for renditions — so this is the irrecoverable direction " +
			"of the same mistake task 368 is about.")
	}
	// The unshared display image still goes, so the test is about sharing rather than about nothing being freed.
	if exists, _ := app.blobs.Exists(context.Background(), blobRefOf(full.String())); exists {
		t.Error("the unshared display image should still have been freed")
	}
}

// The original reaches the sharing check at all.
//
// Asserted on the refs `RefsInUse` was **asked** about rather than only on the outcome, because the two failure modes
// look identical from outside: a path that never asks and a path that asks and is told "not in use" both end with the
// bytes gone. Only one of them is correct, and the incorrect one deletes a file somebody else still holds the moment
// a second row appears.
func TestAdminDeleteAsksWhetherTheOriginalIsShared(t *testing.T) {
	app, srv := adminApp(t)

	full, err := app.blobs.Put(context.Background(), []byte("display"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	original, err := app.blobs.Put(context.Background(), []byte("original"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	photos := &stubPhotos{}
	app.models.Photos = photos
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{{
		ID: full.String(), Ref: full.String(), OriginalRef: original.String(), BoundsVerdict: photo.BoundsNone,
	}}}
	app.commands = commandsWithPublisher(t, &cqrstest.Publisher{})

	if got := deleteAdminPhoto(t, srv, srv.URL, full.String()).StatusCode; got != http.StatusNoContent {
		t.Fatalf("want 204, got %d", got)
	}

	var sawOriginal bool
	for _, asked := range photos.asked {
		for _, ref := range asked {
			if ref == original.String() {
				sawOriginal = true
			}
		}
	}
	if !sawOriginal {
		t.Errorf("the original was never offered to the sharing check (asked: %v). Freeing it without asking "+
			"works until a second photograph names the same bytes, and then it destroys that one's archive "+
			"master with nothing to rebuild from.", photos.asked)
	}
}

// Every ref column the projection has is named by the delete path.
//
// # Why a count rather than a list
//
// Because the bug this prevents is **forgetting**, and a test that lists the columns by hand forgets in exactly the
// same way the code does — `mediumRef` was added to `RefsInUse` and to the takedown separately, and PRD 027 added a
// fifth column to the table. So this asserts the two lists agree in size with the table's ref columns, and fails with
// an instruction rather than a diff.
//
// Source-shaped for the reason the rest of this repo's structural guards are: `cqrs.Reader` cannot execute a query
// against a fake, so "does this SQL name every column" is not observable at runtime.
func TestEveryRefColumnIsNamedByTheDeletePathAndTheSharingCheck(t *testing.T) {
	// The ref columns the projection declares, read from the schema rather than listed here.
	schema := readRepoFile(t, "../../nathejk/table/photo/table.sql")
	var columns []string
	for _, line := range strings.Split(schema, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "--") {
			continue
		}
		// `xxxRef VARCHAR(64)` is how every one of them is declared.
		if i := strings.Index(line, "Ref VARCHAR(64)"); i > 0 {
			columns = append(columns, strings.Fields(line)[0])
		}
	}
	if len(columns) < 4 {
		t.Fatalf("found %d ref columns in table.sql (%v); expected at least blobRef, thumbRef, mediumRef and "+
			"originalRef. Has the declaration style changed?", len(columns), columns)
	}

	// The sharing check must ask about all of them.
	querier := readRepoFile(t, "../../nathejk/table/photo/querier.go")
	for _, col := range columns {
		if !strings.Contains(querier, `"`+col+`"`) {
			t.Errorf("photo.RefsInUse does not name %s. A ref column it does not ask about either orphans bytes "+
				"on disk forever or deletes a live object because nothing claimed it (task 368). Add it to the "+
				"`columns` list, which drives the clause, the arguments, the select list and the scan.", col)
		}
	}

	// And the takedown must offer all of them for freeing.
	del := withoutComments(readRepoFile(t, "admindelete.go"))
	for _, col := range columns {
		// `blobRef` is read as `p.Ref`; the others follow the field name.
		field := "p." + strings.ToUpper(col[:1]) + col[1:]
		if col == "blobRef" {
			field = "p.Ref"
		}
		if !strings.Contains(del, field) {
			t.Errorf("the library takedown does not free %s (looked for %s). Bytes left behind here are bytes "+
				"that outlive the photograph they belong to — and for originalRef that means an EXIF-bearing "+
				"file surviving its own takedown (PRD 027 R8).", col, field)
		}
	}
}

// readRepoFile reads a file relative to cmd/api, failing the test rather than the package.
func readRepoFile(t *testing.T, path string) string {
	t.Helper()

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(src)
}
