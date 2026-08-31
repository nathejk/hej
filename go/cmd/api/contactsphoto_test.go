package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"nathejk.dk/internal/blob"
	"nathejk.dk/nathejk/table/person"
)

// contactsPhotoApp wires an app whose projection can resolve one person by id, with the
// given portrait bytes in the blob store.
func contactsPhotoApp(t *testing.T, subject person.Person, thumb []byte) *application {
	t.Helper()
	app, stub := contactsTestApp(t, []person.Person{subject})
	stub.p = subject
	stub.found = true

	if thumb != nil {
		ref, err := app.blobs.Put(t.Context(), thumb)
		if err != nil {
			t.Fatalf("storing thumbnail: %v", err)
		}
		subject.PortraitRef = string(ref)
		subject.PortraitThumbRef = string(ref)
		stub.p = subject
		stub.listed = []person.Person{subject}
	}
	return app
}

func getPhoto(t *testing.T, app *application, srv *httptest.Server, phone, normalized, personID string) *http.Response {
	t.Helper()
	cookies := authedCookies(t, app, srv, phone, normalized)
	return getWithCookies(t, srv.URL+"/api/contacts/people/"+personID+"/photo", cookies)
}

func TestContactsPhoto_ServesPermittedThumbnail(t *testing.T) {
	jpeg := []byte("not-really-a-jpeg-but-bytes-are-bytes")
	subject := crewRow()
	app := contactsPhotoApp(t, subject, jpeg)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp := getPhoto(t, app, srv, "30000002", "+4530000002", subject.PersonID)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !bytes.Equal(body, jpeg) {
		t.Error("served bytes differ from the stored thumbnail")
	}
	if got := resp.Header.Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("content type = %q", got)
	}
	// Photographs of members must never sit in a shared cache.
	if cc := resp.Header.Get("Cache-Control"); !bytes.Contains([]byte(cc), []byte("private")) {
		t.Errorf("Cache-Control = %q, want private", cc)
	}
	if resp.Header.Get("ETag") == "" {
		t.Error("no ETag; the sync engine would refetch every image on every sync")
	}
}

func TestContactsPhoto_NotModified(t *testing.T) {
	subject := crewRow()
	app := contactsPhotoApp(t, subject, []byte("bytes"))
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000002", "+4530000002")
	first := getWithCookies(t, srv.URL+"/api/contacts/people/"+subject.PersonID+"/photo", cookies)
	io.Copy(io.Discard, first.Body)
	first.Body.Close()

	etag := first.Header.Get("ETag")
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/contacts/people/"+subject.PersonID+"/photo", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set("If-None-Match", etag)
	second, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	io.Copy(io.Discard, second.Body)
	second.Body.Close()

	if second.StatusCode != http.StatusNotModified {
		t.Errorf("status = %d, want 304", second.StatusCode)
	}
}

// The endpoint must not be usable to discover who exists or what they are. Every refusal
// looks the same as every absence — same status, same body.
func TestContactsPhoto_RefusalIsIndistinguishableFromAbsence(t *testing.T) {
	// A gøgler the viewing bandit may not see, with a real portrait.
	forbidden := goeglerRow()
	app := contactsPhotoApp(t, forbidden, []byte("bytes"))
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	refused := getPhoto(t, app, srv, "30000002", "+4530000002", forbidden.PersonID)
	refusedBody, _ := io.ReadAll(refused.Body)
	refused.Body.Close()

	// A person id that does not exist at all.
	app2, stub2 := contactsTestApp(t, nil)
	stub2.found = false
	srv2 := httptest.NewServer(app2.routes())
	defer srv2.Close()

	missing := getPhoto(t, app2, srv2, "30000002", "+4530000002", "no-such-person")
	missingBody, _ := io.ReadAll(missing.Body)
	missing.Body.Close()

	if refused.StatusCode != missing.StatusCode {
		t.Errorf("refusal status %d differs from absence status %d — the endpoint is an enumeration oracle",
			refused.StatusCode, missing.StatusCode)
	}
	if refused.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for both", refused.StatusCode)
	}
	if !bytes.Equal(refusedBody, missingBody) {
		t.Errorf("refusal body %q differs from absence body %q", refusedBody, missingBody)
	}
}

// A spejder's face is never available here, whoever asks — including crew, who reach it only
// through the (uncached) patrol lookup.
func TestContactsPhoto_NeverServesSpejderPortrait(t *testing.T) {
	spejder := spejderRow()
	app := contactsPhotoApp(t, spejder, []byte("a-minors-face"))
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, viewer := range []struct{ name, phone, normalized string }{
		{"bandit", "30000002", "+4530000002"},
		{"gøgler", "30000006", "+4530000006"},
		{"samarit", "30000005", "+4530000005"},
		{"crew", "30000007", "+4530000007"},
	} {
		t.Run(viewer.name, func(t *testing.T) {
			resp := getPhoto(t, app, srv, viewer.phone, viewer.normalized, spejder.PersonID)
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				t.Fatalf("%s was served a spejder portrait through the directory", viewer.name)
			}
			if bytes.Contains(body, []byte("a-minors-face")) {
				t.Error("spejder portrait bytes leaked into the response")
			}
		})
	}
}

func TestContactsPhoto_RefusesSpejderViewer(t *testing.T) {
	subject := crewRow()
	app := contactsPhotoApp(t, subject, []byte("bytes"))
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp := getPhoto(t, app, srv, "30000001", "+4530000001", subject.PersonID)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

func TestContactsPhoto_MissingBytesAreNotFound(t *testing.T) {
	// A row that references a portrait whose bytes are gone — PRD 008 §8 requires this to
	// degrade to "no photo" rather than fail.
	subject := crewRow()
	subject.PortraitRef = string(blob.Ref("0000000000000000000000000000000000000000000000000000000000000000"))
	subject.PortraitThumbRef = subject.PortraitRef

	app, stub := contactsTestApp(t, []person.Person{subject})
	stub.p = subject
	stub.found = true
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp := getPhoto(t, app, srv, "30000002", "+4530000002", subject.PersonID)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// Defaulting to the thumbnail is what keeps the pane from shipping full-resolution faces:
// /me/photo defaults to `full`, and this endpoint must not.
func TestContactsPhoto_DefaultsToThumbnail(t *testing.T) {
	full := []byte("FULL-RESOLUTION-PORTRAIT")
	thumb := []byte("thumb")

	app, stub := contactsTestApp(t, nil)
	fullRef, err := app.blobs.Put(t.Context(), full)
	if err != nil {
		t.Fatalf("put full: %v", err)
	}
	thumbRef, err := app.blobs.Put(t.Context(), thumb)
	if err != nil {
		t.Fatalf("put thumb: %v", err)
	}

	subject := crewRow()
	subject.PortraitRef = string(fullRef)
	subject.PortraitThumbRef = string(thumbRef)
	stub.p = subject
	stub.found = true
	stub.listed = []person.Person{subject}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp := getPhoto(t, app, srv, "30000002", "+4530000002", subject.PersonID)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if !bytes.Equal(body, thumb) {
		t.Errorf("served %q, want the thumbnail — the directory must not default to full size", body)
	}
}
