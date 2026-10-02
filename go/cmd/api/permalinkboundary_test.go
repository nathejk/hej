package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nathejk.dk/internal/blob"
)

// The boundary a shortened permalink must not cross (task 486 R4/R5).
//
// # Why shortening the ref needed a guard at all
//
// PRD 022 §8.4's narrowed invariant is that **a blob is never addressed by a ref taken off the wire**: the path names
// an entity, a projection read applies the visibility filter, and only then does a `blob.Ref` appear. The permalink
// has always satisfied that — it resolves a ref against the album's own items and redirects, never touching the store.
//
// Task 486 made the ref in that URL a *prefix*, which introduces a value that **looks like a ref and is not one**. If
// such a value ever reached the store, two things would be true at once: `blob.Ref.Valid()` would refuse it, so
// nothing would break visibly; and the code path that passed it would have stopped applying the invariant, silently.
// The backstop working is exactly what would hide the mistake.
//
// So this pins the boundary rather than trusting the backstop.

// A 12-character prefix is not a usable blob ref, and the store refuses it.
//
// Stated first because everything below depends on it: the reason a leak here would be *quiet* rather than dangerous
// is that `Valid()` requires 64 hex characters. That is a good backstop and a bad boundary.
func TestAShortenedRefIsNotAValidBlobRef(t *testing.T) {
	full := strings.Repeat("3f", 32)
	if !blob.Ref(full).Valid() {
		t.Fatal("the fixture is not a valid ref, so this test asserts nothing")
	}
	if blob.Ref(shortPhotoRef(full)).Valid() {
		t.Errorf("a %d-character prefix passes blob.Ref.Valid(). The whole reason a shortened ref cannot "+
			"become a filesystem path is that the store refuses it; if that stops being true, task 486's "+
			"boundary needs to become a check rather than a property.", albumPermalinkRefLen)
	}
}

// The permalink handler resolves against the projection and never reaches the store.
//
// Asserted by reading the handler, because "it did not call the blob store" is not observable in a 302. The handler is
// a redirect: it should name the album read and `http.Redirect`, and nothing else.
func TestThePermalinkHandlerNeverTouchesTheBlobStore(t *testing.T) {
	body := withoutComments(handlerBody(t, "albumpage.go", "albumPhotoPermalinkHandler"))

	for _, forbidden := range []string{"app.blobs", "blob.Ref", "streamGlimtMedia", "readBlob"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("albumPhotoPermalinkHandler mentions %s. Since task 486 the ref in that URL is a "+
				"**prefix** — a value that looks like a ref and is not one — so this handler must stay a "+
				"projection lookup and a redirect. PRD 022 §8.4.", forbidden)
		}
	}
	// And it does resolve through the album read, so the test is about *how* it resolves rather than about the
	// handler having been emptied.
	if !strings.Contains(body, "app.models.Albums.BySlug") {
		t.Error("the permalink handler no longer resolves through the album projection; this guard needs updating")
	}
}

// The media route's selector is still an exact match, so a permalink prefix cannot address bytes.
//
// # Why this is the case that would actually have bitten
//
// `albumMediaHandler` takes `{selector}`, which is a ref **or** an ordinal, and it does reach the blob store. It is
// also `immutable`-cached for a year. If its matching had been loosened to a prefix along with the permalink's — an
// easy thing to do for consistency, since both scan the same items — then a 12-character prefix would address bytes,
// and an ambiguous one would address *whichever item came first* under a year-long immutable cache.
//
// That is why task 486 R5 left this route alone, and why the asymmetry is tested rather than remembered.
func TestTheMediaRouteRefusesAPermalinkPrefix(t *testing.T) {
	app, store := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	ref := firstItemRef(t, store)

	// The full ref is served, so the test is about the prefix and not about the route being broken.
	full, err := srv.Client().Get(srv.URL + "/api/public/albums/al-1/media/" + ref)
	if err != nil {
		t.Fatalf("GET the full ref: %v", err)
	}
	full.Body.Close()
	if full.StatusCode != http.StatusOK {
		t.Fatalf("the full ref should be served, got %d", full.StatusCode)
	}

	short, err := srv.Client().Get(srv.URL + "/api/public/albums/al-1/media/" + shortPhotoRef(ref))
	if err != nil {
		t.Fatalf("GET the prefix: %v", err)
	}
	short.Body.Close()
	if short.StatusCode == http.StatusOK {
		t.Error("the media route served a 12-character prefix. It reaches the blob store and is cached " +
			"immutable for a year, so prefix matching there would mean an ambiguous prefix pinning whichever " +
			"photograph happened to be first. Task 486 R5 keeps albumItemIs an exact compare.")
	}
}

// handlerBody returns one handler's source, from its signature to the next top-level declaration.
//
// Comment lines are stripped by the caller: this file's prose quotes `app.blobs` and `blob.Ref` while explaining why
// they must not appear, which is the mistake `membershipsafety_test.go` records happening four times in one session.
func handlerBody(t *testing.T, file, name string) string {
	t.Helper()

	src := readRepoFile(t, file)
	start := strings.Index(src, "func (app *application) "+name+"(")
	if start < 0 {
		t.Fatalf("%s no longer declares %s; this guard needs updating", file, name)
	}
	// Sliced to the next top-level `func`, not to the next comment heading: a guard that sliced to a heading once
	// read the following function's body and passed.
	rest := src[start+1:]
	if end := strings.Index(rest, "\nfunc "); end >= 0 {
		return src[start : start+1+end]
	}
	return src[start:]
}
