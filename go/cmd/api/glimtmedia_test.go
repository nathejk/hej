package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
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
	"nathejk.dk/internal/ratelimit"
	"nathejk.dk/nathejk/table/glimt"
)

// Glimt media upload tests (task 303).

func multipartMedia(t *testing.T, field, filename string, data []byte) (string, io.Reader) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, filename)
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

func postMedia(t *testing.T, url, contentType string, body io.Reader, cookies []*http.Cookie) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, body)
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
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func decodeStoredMedia(t *testing.T, resp *http.Response) glimtMediaStored {
	t.Helper()
	defer resp.Body.Close()
	var out glimtMediaStored
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

// blobBytes reads a stored object. blob.Store.Get returns a ReadCloser, and these tests care
// about the bytes themselves — the dimensions of a thumbnail, the absence of a metadata marker.
func blobBytes(t *testing.T, app *application, ref string) []byte {
	t.Helper()
	rc, err := app.blobs.Get(context.Background(), blob.Ref(ref))
	if err != nil {
		t.Fatalf("get %s: %v", ref, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read %s: %v", ref, err)
	}
	return data
}

func TestUploadGlimtMedia_RequiresAuth(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	ct, body := multipartMedia(t, "media", "glimt.jpg", testImage(t, 64, 64))
	resp := postMedia(t, srv.URL+"/api/glimt/media", ct, body, nil)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestUploadGlimtMedia_StoresFullAndThumbnail(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	ct, body := multipartMedia(t, "media", "glimt.jpg", testImage(t, 2000, 1500))
	resp := postMedia(t, srv.URL+"/api/glimt/media", ct, body, cookies)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	got := decodeStoredMedia(t, resp)

	if got.Kind != glimt.MediaKindImage {
		t.Errorf("kind = %q, want image", got.Kind)
	}
	if got.ContentType != "image/jpeg" {
		t.Errorf("content type = %q", got.ContentType)
	}
	// Downscaled to the Glimt edge, aspect preserved. The client also downscales, but the
	// server cannot assume it did.
	if got.Width != maxGlimtEdge {
		t.Errorf("width = %d, want %d", got.Width, maxGlimtEdge)
	}
	if got.Height != maxGlimtEdge*3/4 {
		t.Errorf("height = %d, want %d", got.Height, maxGlimtEdge*3/4)
	}

	// Both objects are in the store, and they are different objects.
	for name, ref := range map[string]string{"full": got.Ref, "thumb": got.ThumbRef} {
		if ref == "" {
			t.Fatalf("%s ref is empty", name)
		}
		if !blob.Ref(ref).Valid() {
			t.Errorf("%s ref %q is not a content hash", name, ref)
		}
		if ok, _ := app.blobs.Exists(context.Background(), blob.Ref(ref)); !ok {
			t.Errorf("%s bytes are not in the store", name)
		}
	}
	if got.Ref == got.ThumbRef {
		t.Error("the thumbnail is the same object as the full image")
	}
	if got.Bytes <= 0 {
		t.Errorf("bytes = %d", got.Bytes)
	}
	// A still must not claim a duration.
	if got.DurationMs != 0 {
		t.Errorf("duration = %d, want 0 for a still", got.DurationMs)
	}
}

// TestUploadGlimtMedia_ThumbnailIsGridSized pins the constant the post-race browse depends on.
//
// The hold-collection grid pulls thumbnails by the thousand over a congested network, so the
// thumbnail must actually be small — an "optimisation" that raised this to the display size would
// not fail any other test.
func TestUploadGlimtMedia_ThumbnailIsGridSized(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	stored, err := app.storeGlimtImage(
		httptest.NewRequest(http.MethodPost, "/api/glimt/media", nil),
		testImage(t, 2000, 2000),
	)
	if err != nil {
		t.Fatalf("storeGlimtImage: %v", err)
	}

	full := blobBytes(t, app, stored.Ref)
	thumb := blobBytes(t, app, stored.ThumbRef)

	cfg, err := jpeg.DecodeConfig(bytes.NewReader(thumb))
	if err != nil {
		t.Fatalf("decode thumb: %v", err)
	}
	if cfg.Width != glimtThumbEdges[0] {
		t.Errorf("thumbnail is %dpx wide, want %d", cfg.Width, glimtThumbEdges[0])
	}
	if len(thumb) >= len(full) {
		t.Errorf("thumbnail (%d bytes) is not smaller than the full image (%d bytes)", len(thumb), len(full))
	}
}

// TestUploadGlimtMedia_StripsMetadata is the EXIF/GPS requirement as a property.
//
// The output is generated from decoded pixels, so nothing from the input file can survive — which
// is what makes the claim true by construction rather than by a scrubber we have to trust. A
// passthrough implementation would fail this.
func TestUploadGlimtMedia_StripsMetadata(t *testing.T) {
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 40, 40))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	tagged := append(pngBuf.Bytes(), []byte("GPSLatitudeSecretMarker")...)

	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	stored, err := app.storeGlimtImage(
		httptest.NewRequest(http.MethodPost, "/api/glimt/media", nil), tagged)
	if err != nil {
		t.Fatalf("storeGlimtImage: %v", err)
	}

	for name, ref := range map[string]string{"full": stored.Ref, "thumb": stored.ThumbRef} {
		data := blobBytes(t, app, ref)
		if bytes.Contains(data, []byte("SecretMarker")) {
			t.Errorf("appended metadata survived into the %s object", name)
		}
		if _, derr := jpeg.Decode(bytes.NewReader(data)); derr != nil {
			t.Errorf("%s object is not a decodable JPEG: %v", name, derr)
		}
	}
}

// TestUploadGlimtMedia_KeepsNoOriginal records a deliberate difference from the portrait.
//
// The portrait keeps a metadata-stripped original so renditions can be regenerated (task 111).
// Glimt does not: glimt are unbounded where portraits are one per member, and the blob store is the
// only thing in this service that cannot be rebuilt from the stream. Keeping originals would roughly
// double the only irreplaceable data we hold, to enable a re-render nobody has asked for.
func TestUploadGlimtMedia_KeepsNoOriginal(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	stored, err := app.storeGlimtImage(
		httptest.NewRequest(http.MethodPost, "/api/glimt/media", nil),
		testImage(t, 3000, 2000),
	)
	if err != nil {
		t.Fatalf("storeGlimtImage: %v", err)
	}

	// Exactly two objects for one upload: the display image and its thumbnail.
	full := blobBytes(t, app, stored.Ref)
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(full))
	if err != nil {
		t.Fatalf("DecodeConfig: %v", err)
	}
	if cfg.Width != maxGlimtEdge {
		t.Errorf("stored image is %dpx wide, want the display size %d — an original may have been kept",
			cfg.Width, maxGlimtEdge)
	}
}

func TestUploadGlimtMedia_RejectsNonMedia(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	// A PDF-ish blob with a plausible filename. The decode is the validation, and neither
	// the extension nor the declared content type is consulted.
	ct, body := multipartMedia(t, "media", "holiday.jpg", []byte("%PDF-1.7 not an image at all"))
	resp := postMedia(t, srv.URL+"/api/glimt/media", ct, body, cookies)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUploadGlimtMedia_RejectsOversize(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	oversize := bytes.Repeat([]byte{0xff}, maxGlimtUpload+1024)

	// Both transports, because the over-limit error surfaces from two different places: the
	// multipart parser and the raw read. The portrait endpoint shipped with only the raw
	// path tested and reported an 11.6 MB multipart upload as a 400 "missing field" — the
	// bug photo.go documents. Tested here from the start.
	t.Run("multipart", func(t *testing.T) {
		ct, body := multipartMedia(t, "media", "big.jpg", oversize)
		resp := postMedia(t, srv.URL+"/api/glimt/media", ct, body, cookies)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Errorf("status = %d, want 413", resp.StatusCode)
		}
	})

	t.Run("raw body", func(t *testing.T) {
		resp := postMedia(t, srv.URL+"/api/glimt/media", "image/jpeg", bytes.NewReader(oversize), cookies)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Errorf("status = %d, want 413", resp.StatusCode)
		}
	})
}

func TestUploadGlimtMedia_AcceptsARawBody(t *testing.T) {
	// The `<input capture>` fallback and shell-driven tests post the file as the body.
	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	resp := postMedia(t, srv.URL+"/api/glimt/media", "image/jpeg",
		bytes.NewReader(testImage(t, 100, 100)), cookies)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := decodeStoredMedia(t, resp); got.Ref == "" {
		t.Error("no ref returned")
	}
}

func TestUploadGlimtMedia_RejectsAnEmptyBody(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	resp := postMedia(t, srv.URL+"/api/glimt/media", "image/jpeg", bytes.NewReader(nil), cookies)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUploadGlimtMedia_WrongFieldNameIsAClearError(t *testing.T) {
	// The field is `media`, not `photo` — it carries video from task 322, and a field named
	// for one of the two would be a lie in half the requests. A client using the portrait's
	// field name should get a message naming the right one.
	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	ct, body := multipartMedia(t, "photo", "glimt.jpg", testImage(t, 64, 64))
	resp := postMedia(t, srv.URL+"/api/glimt/media", ct, body, cookies)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	payload, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(payload), "media") {
		t.Errorf("error does not name the expected field: %s", payload)
	}
}

func TestUploadGlimtMedia_RateLimited(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	// One upload, so the second is refused. Checked before the body is read, so a member at
	// the ceiling does not get to push 12 MiB up a mobile link first.
	app.glimtMediaLimiter = ratelimit.New(1, time.Hour)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	ct, body := multipartMedia(t, "media", "one.jpg", testImage(t, 64, 64))
	first := postMedia(t, srv.URL+"/api/glimt/media", ct, body, cookies)
	io.Copy(io.Discard, first.Body)
	first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first upload = %d, want 200", first.StatusCode)
	}

	ct, body = multipartMedia(t, "media", "two.jpg", testImage(t, 64, 64))
	second := postMedia(t, srv.URL+"/api/glimt/media", ct, body, cookies)
	defer second.Body.Close()
	if second.StatusCode != http.StatusTooManyRequests {
		t.Errorf("second upload = %d, want 429", second.StatusCode)
	}
}

// TestUploadGlimtMedia_IsContentAddressed is what makes a retry safe.
//
// The client's outbox retries after a dropped connection (task 314). Identical bytes must produce
// the same ref and no second object, or a flaky link would multiply storage.
func TestUploadGlimtMedia_IsContentAddressed(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/glimt/media", nil)
	img := testImage(t, 200, 200)

	first, err := app.storeGlimtImage(req, img)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := app.storeGlimtImage(req, img)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first.Ref != second.Ref || first.ThumbRef != second.ThumbRef {
		t.Errorf("the same bytes produced different refs:\n1: %+v\n2: %+v", first, second)
	}
}

func TestGlimtBlobRefRejectsPathShapedValues(t *testing.T) {
	// The create handler parses client-supplied refs with this before they reach a SQL
	// statement or a URL path. "../../etc/passwd" is a Ref-shaped string.
	for _, bad := range []string{
		"", "../../etc/passwd", strings.Repeat("a", 63),
		strings.Repeat("z", 64), "/" + strings.Repeat("a", 63),
	} {
		if _, ok := glimtBlobRef(bad); ok {
			t.Errorf("glimtBlobRef(%q) accepted it", bad)
		}
	}
	if _, ok := glimtBlobRef(strings.Repeat("a", 64)); !ok {
		t.Error("a valid content hash was rejected")
	}
}

// TestGlimtBlobRefRequiresTheCanonicalForm guards against a silent data loss.
//
// blob.Ref.Valid accepts uppercase hex — right for its own job of deciding what is safe to turn
// into a path — while blob.ComputeRef only emits lowercase and the glimt projection's validRef only
// accepts lowercase. Without this check the three facts combine badly: an uppercased ref passes
// upload validation, travels on the created event, and is then **dropped by the fold**, so a
// three-photo glimt arrives with two, with nothing logged and nothing for the member to retry.
func TestGlimtBlobRefRequiresTheCanonicalForm(t *testing.T) {
	upper := strings.ToUpper(strings.Repeat("a", 64))
	if blob.Ref(upper).Valid() != true {
		t.Fatal("precondition changed: blob.Ref.Valid no longer accepts uppercase, so this guard may be redundant")
	}
	if _, ok := glimtBlobRef(upper); ok {
		t.Error("an uppercase ref was accepted; the projection would silently drop that media item")
	}

	// Mixed case too, which is what a hand-edited request looks like.
	mixed := "AbCd" + strings.Repeat("a", 60)
	if _, ok := glimtBlobRef(mixed); ok {
		t.Error("a mixed-case ref was accepted")
	}

	// And the refs the upload path actually produces must pass, or nothing works at all.
	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	stored, err := app.storeGlimtImage(
		httptest.NewRequest(http.MethodPost, "/api/glimt/media", nil), testImage(t, 80, 80))
	if err != nil {
		t.Fatalf("storeGlimtImage: %v", err)
	}
	for _, ref := range []string{stored.Ref, stored.ThumbRef} {
		if _, ok := glimtBlobRef(ref); !ok {
			t.Errorf("a ref this service just produced was rejected: %q", ref)
		}
	}
}
