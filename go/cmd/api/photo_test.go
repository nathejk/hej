package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/data"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/person"
)

// stubPeople is a person.Queries returning one fixed row, so the GET path can be tested
// without a database.
type stubPeople struct {
	p     person.Person
	found bool
	err   error
	asked []string

	// expired is what ExpiredPortraits returns, and expiredCutoffs records what it was
	// asked for — the cutoff is the whole retention policy, so a test asserts on it.
	expired        []person.ExpiredPortrait
	expiredErr     error
	expiredCutoffs []time.Time
}

func (s *stubPeople) Get(year, personID string) (person.Person, bool, error) {
	s.asked = append(s.asked, year+"/"+personID)
	return s.p, s.found, s.err
}

func (s *stubPeople) Lookup(string, string) ([]person.Person, error) { return nil, nil }

func (s *stubPeople) ExpiredPortraits(_ string, before time.Time, _ int) ([]person.ExpiredPortrait, error) {
	s.expiredCutoffs = append(s.expiredCutoffs, before)
	if s.expiredErr != nil {
		return nil, s.expiredErr
	}
	// Returned once: the real query stops finding a row after its purge event is
	// projected, and a stub that kept returning it would hide a runaway loop.
	out := s.expired
	s.expired = nil
	return out, nil
}

// testImage builds a real JPEG, because the handler validates by decoding: a fixture of
// arbitrary bytes would test nothing.
func testImage(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	return buf.Bytes()
}

func multipartPhoto(t *testing.T, data []byte) (string, io.Reader) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("photo", "portrait.jpg")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return writer.FormDataContentType(), &body
}

func putWithCookies(t *testing.T, url, contentType string, body io.Reader, cookies []*http.Cookie) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	return resp
}

func photoTestApp(t *testing.T, pub *cqrstest.Publisher, people person.Queries) *application {
	t.Helper()
	app := portraitTestApp(t, pub)
	app.models = data.NewModels(users.NewMockDirectory(), scans.NewMockSource(), nil, people)
	return app
}

func TestUploadPhoto_RequiresAuth(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	ct, body := multipartPhoto(t, testImage(t, 64, 64))
	resp := putWithCookies(t, srv.URL+"/api/me/photo", ct, body, nil)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestUploadPhoto_StoresAndPublishes(t *testing.T) {
	pub := &cqrstest.Publisher{}
	app := photoTestApp(t, pub, nil)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	ct, body := multipartPhoto(t, testImage(t, 64, 64))
	resp := putWithCookies(t, srv.URL+"/api/me/photo", ct, body, cookies)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	got := decodeProfile(t, resp)

	ref := blob.Ref(got["ref"].(string))
	if !ref.Valid() {
		t.Fatalf("returned ref %q is not a content hash", got["ref"])
	}
	if ok, _ := app.blobs.Exists(t.Context(), ref); !ok {
		t.Error("bytes were not stored under the returned ref")
	}
	if subjects := pub.Subjects(); len(subjects) != 1 ||
		!strings.HasPrefix(subjects[0], "NATHEJK.2026.portrait.") {
		t.Errorf("subjects = %v", subjects)
	}
}

// A raw body (no multipart) must work too: the `<input capture>` fallback and any
// non-browser client send the file as the body.
func TestUploadPhoto_AcceptsARawBody(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	resp := putWithCookies(t, srv.URL+"/api/me/photo", "image/jpeg",
		bytes.NewReader(testImage(t, 32, 32)), cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

// The stored bytes must be a JPEG regardless of what arrived, and must not carry the
// upload's metadata. This is the EXIF/GPS-stripping requirement expressed as a property:
// the output is generated from pixels, so nothing from the input file can survive.
func TestUploadPhoto_ReEncodesToJpegAndDropsMetadata(t *testing.T) {
	// A PNG carrying a text chunk. If the bytes were stored as-is, the chunk would still
	// be there — which is what a passthrough implementation would do.
	var pngBuf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	tagged := append(pngBuf.Bytes(), []byte("GPSLatitudeSecretMarker")...)

	encoded, meta, err := normalizePortrait(tagged)
	if err != nil {
		t.Fatalf("normalizePortrait: %v", err)
	}
	if meta.ContentType != "image/jpeg" {
		t.Errorf("content type = %q, want image/jpeg", meta.ContentType)
	}
	if bytes.Contains(encoded, []byte("SecretMarker")) {
		t.Error("appended metadata survived re-encoding")
	}
	if _, err := jpeg.Decode(bytes.NewReader(encoded)); err != nil {
		t.Errorf("stored bytes are not a decodable JPEG: %v", err)
	}
}

// Oversized images are downscaled server-side. The client also does this, but the server
// cannot assume it did — and PRD 007 sizes its offline cache against what is stored.
func TestUploadPhoto_DownscalesToTheEdgeLimit(t *testing.T) {
	encoded, meta, err := normalizePortrait(testImage(t, 2000, 1000))
	if err != nil {
		t.Fatalf("normalizePortrait: %v", err)
	}
	if meta.Width != maxPortraitEdge {
		t.Errorf("width = %d, want %d", meta.Width, maxPortraitEdge)
	}
	// Aspect ratio preserved, not squashed to a square.
	if meta.Height != maxPortraitEdge/2 {
		t.Errorf("height = %d, want %d", meta.Height, maxPortraitEdge/2)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeConfig: %v", err)
	}
	if cfg.Width != meta.Width || cfg.Height != meta.Height {
		t.Errorf("stored image is %dx%d but the event says %dx%d",
			cfg.Width, cfg.Height, meta.Width, meta.Height)
	}

	// And a thumbnail comes out of the same call (task 104).
	thumbCfg, err := jpeg.DecodeConfig(bytes.NewReader(meta.Thumb))
	if err != nil {
		t.Fatalf("thumbnail is not a JPEG: %v", err)
	}
	if thumbCfg.Width != thumbnailEdge {
		t.Errorf("thumbnail width = %d, want %d", thumbCfg.Width, thumbnailEdge)
	}
}

// `?size=thumb` serves the thumbnail; without it, the full image.
func TestShowPhoto_ServesTheThumbnailWhenAsked(t *testing.T) {
	people := &stubPeople{found: true}
	app := photoTestApp(t, &cqrstest.Publisher{}, people)

	full := []byte("full image bytes")
	thumb := []byte("thumb")
	fullRef, err := app.blobs.Put(t.Context(), full)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	thumbRef, err := app.blobs.Put(t.Context(), thumb)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	people.p = person.Person{PortraitRef: fullRef.String(), PortraitThumbRef: thumbRef.String()}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	for query, want := range map[string][]byte{
		"":            full,
		"?size=thumb": thumb,
		// Case-insensitive, because a client writing `Thumb` is not making a mistake
		// worth answering with the wrong image.
		"?size=THUMB": thumb,
		"?size=full":  full,
		// An unrecognised value falls back to the full image rather than erroring.
		"?size=weird": full,
	} {
		resp := getWithCookies(t, srv.URL+"/api/me/photo"+query, cookies)
		got, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !bytes.Equal(got, want) {
			t.Errorf("%q served %q, want %q", query, got, want)
		}
	}
}

// A portrait captured before thumbnails existed must serve the full image when a
// thumbnail is requested — a client asking for something small would rather have
// something large than nothing.
func TestShowPhoto_FallsBackWhenThereIsNoThumbnail(t *testing.T) {
	people := &stubPeople{found: true}
	app := photoTestApp(t, &cqrstest.Publisher{}, people)

	full := []byte("full image bytes")
	ref, err := app.blobs.Put(t.Context(), full)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	people.p = person.Person{PortraitRef: ref.String()}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := getWithCookies(t, srv.URL+"/api/me/photo?size=thumb", cookies)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, full) {
		t.Error("want the full image as a fallback")
	}
}

// A small image is left at its own size rather than being blown up.
func TestNormalizePortraitDoesNotUpscale(t *testing.T) {
	_, meta, err := normalizePortrait(testImage(t, 80, 60))
	if err != nil {
		t.Fatalf("normalizePortrait: %v", err)
	}
	if meta.Width != 80 || meta.Height != 60 {
		t.Errorf("got %dx%d, want the original 80x60", meta.Width, meta.Height)
	}
}

// Non-image bytes must be refused. Note the content type says image/jpeg: the handler
// must not believe it.
func TestUploadPhoto_RejectsNonImages(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	resp := putWithCookies(t, srv.URL+"/api/me/photo", "image/jpeg",
		strings.NewReader("MZ\x90\x00 this is an executable, honestly"), cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUploadPhoto_RejectsOversizedBodies(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	oversized := bytes.Repeat([]byte{0xff}, maxPortraitUpload+1024)
	resp := putWithCookies(t, srv.URL+"/api/me/photo", "image/jpeg",
		bytes.NewReader(oversized), cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", resp.StatusCode)
	}
}

// With no broker the upload must fail as retryable, not report success: the bytes are
// stored but nothing references them, so the portrait is not on file.
func TestUploadPhoto_FailsRetryablyWithoutABroker(t *testing.T) {
	app := photoTestApp(t, nil, nil)
	app.commands = commandsWithNoPublisher()
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	ct, body := multipartPhoto(t, testImage(t, 32, 32))
	resp := putWithCookies(t, srv.URL+"/api/me/photo", ct, body, cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

func TestShowPhoto_RequiresAuth(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, &stubPeople{})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/me/photo")
	if err != nil {
		t.Fatalf("GET photo: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestShowPhoto_ServesTheOwnersPortrait(t *testing.T) {
	data := testImage(t, 48, 48)
	people := &stubPeople{found: true}
	app := photoTestApp(t, &cqrstest.Publisher{}, people)

	ref, err := app.blobs.Put(t.Context(), data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	people.p = person.Person{PortraitRef: ref.String()}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	resp := getWithCookies(t, srv.URL+"/api/me/photo", cookies)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("content type = %q", ct)
	}
	// A photograph of a minor must not be cacheable by anything shared.
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "private") {
		t.Errorf("Cache-Control = %q, want it to be private", cc)
	}
	served, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(served, data) {
		t.Error("served bytes differ from the stored ones")
	}

	// The lookup is keyed by the configured year and the *session's* user, never by
	// anything from the request.
	if len(people.asked) != 1 || !strings.HasPrefix(people.asked[0], "2026/") {
		t.Errorf("looked up %v", people.asked)
	}
}

func TestShowPhoto_404WithoutAPortrait(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, &stubPeople{found: true})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	resp := getWithCookies(t, srv.URL+"/api/me/photo", cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// PRD 008 §8: a reference whose bytes have gone missing must degrade to "no photo",
// never to an error page.
func TestShowPhoto_MissingBytesDegradeTo404(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, &stubPeople{
		found: true,
		p:     person.Person{PortraitRef: strings.Repeat("a", 64)},
	})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	resp := getWithCookies(t, srv.URL+"/api/me/photo", cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// A ref that is not a content hash must never reach the blob store: it is the one value
// on this path that travels from a database row into a filesystem lookup.
func TestShowPhoto_RefusesANonHashRef(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, &stubPeople{
		found: true,
		p:     person.Person{PortraitRef: "../../../etc/passwd"},
	})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	resp := getWithCookies(t, srv.URL+"/api/me/photo", cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// Without a database, portraits are unavailable rather than absent — and the difference
// matters, because "you have no photo" would be a lie that stops the client nudging.
func TestShowPhoto_NoProjectionIs503(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	resp := getWithCookies(t, srv.URL+"/api/me/photo", cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}
