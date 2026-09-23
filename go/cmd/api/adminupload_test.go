package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/internal/imaging"
	"nathejk.dk/nathejk/table/photo"
)

// The admin upload path (PRD 022 §8.5, §8.9, task 372).

// stubPhotoCurator answers the one read the upload path needs, and records what it was asked.
//
// Deliberately minimal: the upload handler uses exactly `Photo`, and a stub that implemented the whole
// interface faithfully would be a second implementation of the library to keep in step with the first.
type stubPhotoCurator struct {
	// present maps a photo id to whether it exists and whether it is deleted.
	present map[string]bool
	deleted map[string]bool
	err     error

	asked []string
}

func (s *stubPhotoCurator) Library(string, photo.Filter, int, int) ([]photo.LibraryPhoto, error) {
	return nil, s.err
}

func (s *stubPhotoCurator) Counts(string) (photo.Counts, error) { return photo.Counts{}, s.err }

func (s *stubPhotoCurator) Photo(_, photoID string) (photo.LibraryPhoto, bool, error) {
	s.asked = append(s.asked, photoID)
	if s.err != nil {
		return photo.LibraryPhoto{}, false, s.err
	}
	if !s.present[photoID] {
		return photo.LibraryPhoto{}, false, nil
	}
	return photo.LibraryPhoto{ID: photoID, Deleted: s.deleted[photoID]}, true, nil
}

func (s *stubPhotoCurator) Tags(string, string) ([]photo.Tag, error) { return nil, s.err }

// uploadApp returns an admin app whose library is the given stub, with a working event stream.
//
// The publisher matters: `newTestApp` deliberately has none, which is the "broker never arrived" state — so
// without this every upload would answer 503 and the tests would all be testing the same degraded path. The
// test that *wants* that path replaces the facade itself.
func uploadApp(t *testing.T, curator *stubPhotoCurator) (*application, *httptest.Server) {
	t.Helper()

	app, srv := adminApp(t)
	app.models.PhotoCurator = curator
	app.models.RaceAreas = stubRaceAreas{area: testRaceArea(), ok: true}
	app.commands = commandsWithPublisher(t, &cqrstest.Publisher{})
	return app, srv
}

// postPhoto uploads one file as multipart, the way the browser will.
func postPhoto(t *testing.T, srv *httptest.Server, field string, body []byte) *http.Response {
	t.Helper()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile(field, "IMG_0001.JPG")
	if err != nil {
		t.Fatalf("building the multipart body: %v", err)
	}
	if _, err := part.Write(body); err != nil {
		t.Fatalf("writing the file part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("closing the multipart writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/admin/photos", &buf)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Forwarded-Proto", "https")
	req.SetBasicAuth(testAdminUser, testAdminPass)

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("POST /api/admin/photos: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func decodeUpload(t *testing.T, resp *http.Response) adminUploadResponse {
	t.Helper()

	// Fail loudly on a non-200 rather than decoding an error envelope into a zero-valued response. Without
	// this a 503 decodes "successfully" into an empty struct, and every assertion downstream reports a
	// confusing symptom instead of the cause — which is exactly what happened while writing these tests.
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 200 before decoding an upload response, got %d: %s", resp.StatusCode, body)
	}

	var out adminUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding the upload response: %v", err)
	}
	return out
}

// capturePublisher points the app's command facade at a recording publisher and returns it.
//
// A thin wrapper over `commandsWithPublisher` (portrait_test.go) because three tests here need it and each
// cares about a different thing: that exactly one event is published, that none is, and which subject it
// carried. `commandsWithNoPublisher`, for the broken-broker case, is portrait_test.go's too.
func capturePublisher(t *testing.T, app *application) *cqrstest.Publisher {
	t.Helper()

	pub := &cqrstest.Publisher{}
	app.commands = commandsWithPublisher(t, pub)
	return pub
}

// The photo id and the blob ref are the same 64 hex characters by construction (PRD 022 §8.5). Tests below use
// `blobRefOf` from glimtfeed_test.go for the cast, which is worth noting rather than hiding: the equality is
// the whole idempotency mechanism, not a coincidence to paper over.

func TestAdminUploadStoresAPhotograph(t *testing.T) {
	app, srv := uploadApp(t, &stubPhotoCurator{})

	resp := postPhoto(t, srv, "photo", devFixtureImage(800, 600, 0, 0, "upload"))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	out := decodeUpload(t, resp)
	if out.Outcome != adminUploadStored {
		t.Errorf("want outcome %q, got %q", adminUploadStored, out.Outcome)
	}
	if len(out.PhotoID) != 64 {
		t.Errorf("want a content-hash id, got %q", out.PhotoID)
	}
	if out.Width == 0 || out.Height == 0 {
		t.Error("want the stored dimensions")
	}

	// The bytes reached the store, and the id addresses them. That equality is the whole idempotency
	// mechanism — see PRD 022 §8.5.
	exists, err := app.blobs.Exists(context.Background(), blobRefOf(out.PhotoID))
	if err != nil || !exists {
		t.Errorf("the photograph's own id must address its stored bytes (exists=%v err=%v)", exists, err)
	}
}

// One event per upload, carrying the derived id.
func TestAdminUploadPublishesOneEvent(t *testing.T) {
	app, srv := uploadApp(t, &stubPhotoCurator{})
	pub := capturePublisher(t, app)

	resp := postPhoto(t, srv, "photo", devFixtureImage(800, 600, 0, 0, "one event"))
	out := decodeUpload(t, resp)

	subjects := pub.Subjects()
	if len(subjects) != 1 {
		t.Fatalf("want exactly 1 event, got %d: %v", len(subjects), subjects)
	}
	want := "NATHEJK.2026.photo." + out.PhotoID + ".uploaded"
	if subjects[0] != want {
		t.Errorf("want subject %s, got %s", want, subjects[0])
	}
}

// **Re-dragging a folder is routine, not an error.** The same bytes must yield one library row, and the
// response must say so rather than claiming a fresh success — otherwise a duplicated card is undiagnosable.
func TestAdminUploadOfTheSameBytesIsNotADuplicate(t *testing.T) {
	curator := &stubPhotoCurator{present: map[string]bool{}, deleted: map[string]bool{}}
	app, srv := uploadApp(t, curator)
	pub := capturePublisher(t, app)

	first := decodeUpload(t, postPhoto(t, srv, "photo", devFixtureImage(800, 600, 0, 0, "same")))
	if first.Outcome != adminUploadStored {
		t.Fatalf("first upload: want stored, got %q", first.Outcome)
	}

	// The fold is asynchronous in production; the stub stands in for it having run.
	curator.present[first.PhotoID] = true

	second := decodeUpload(t, postPhoto(t, srv, "photo", devFixtureImage(800, 600, 0, 0, "same")))
	if second.PhotoID != first.PhotoID {
		t.Errorf("identical bytes must yield the same id: %q then %q", first.PhotoID, second.PhotoID)
	}
	if second.Outcome != adminUploadAlreadyPresent {
		t.Errorf("want outcome %q, got %q", adminUploadAlreadyPresent, second.Outcome)
	}
	if !strings.Contains(strings.ToLower(second.Message), "allerede") {
		t.Errorf("the message must say it was already uploaded, got %q", second.Message)
	}

	// And only the first upload published. A second event would be noise on a log that is never rewritten.
	if got := len(pub.Subjects()); got != 1 {
		t.Errorf("want 1 event across two uploads of the same bytes, got %d", got)
	}
}

// **The rule with the most consequence in this file.** A curator deleted this photograph — possibly because
// somebody objected — and a re-upload is not a decision to put it back.
//
// Nothing is published, and the response says so plainly. Silence would look like a bug to the one person who
// could explain it; an error would suggest something needs fixing.
func TestAdminUploadDoesNotResurrectADeletedPhotograph(t *testing.T) {
	curator := &stubPhotoCurator{present: map[string]bool{}, deleted: map[string]bool{}}
	app, srv := uploadApp(t, curator)

	// Establish the id by uploading once, then mark it deleted as a curator would have.
	first := decodeUpload(t, postPhoto(t, srv, "photo", devFixtureImage(800, 600, 0, 0, "objected")))
	curator.present[first.PhotoID] = true
	curator.deleted[first.PhotoID] = true

	pub := capturePublisher(t, app)
	second := decodeUpload(t, postPhoto(t, srv, "photo", devFixtureImage(800, 600, 0, 0, "objected")))

	if second.Outcome != adminUploadPreviouslyDeleted {
		t.Errorf("want outcome %q, got %q", adminUploadPreviouslyDeleted, second.Outcome)
	}
	if len(pub.Subjects()) != 0 {
		t.Errorf("a re-upload of a deleted photograph must publish nothing, got %v", pub.Subjects())
	}
	if !strings.Contains(strings.ToLower(second.Message), "slettet") {
		t.Errorf("the message must say the photograph was deleted, got %q", second.Message)
	}
}

// The decode is the validation; the declared content type is not trusted. Every one of these is a real thing
// that comes off a photographer's card.
func TestAdminUploadRefusesWhatIsNotAPhotograph(t *testing.T) {
	_, srv := uploadApp(t, &stubPhotoCurator{})

	for name, body := range map[string][]byte{
		"a QuickTime movie":      []byte("\x00\x00\x00\x14ftypqt  \x00\x00\x00\x00wide"),
		"a Canon raw file":       []byte("II*\x00\x10\x00\x00\x00CR\x02\x00 and then some bytes"),
		"a Windows thumbnail db": []byte("\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1 Thumbs.db contents"),
		"a text file":            []byte("this is not an image, it is a sentence"),
	} {
		resp := postPhoto(t, srv, "photo", body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", name, resp.StatusCode)
		}
	}
}

// A zero-byte file is a real thing on a card pulled mid-write, and it would otherwise fail with a confusing
// "not an image" rather than the truth.
func TestAdminUploadRefusesAnEmptyFile(t *testing.T) {
	_, srv := uploadApp(t, &stubPhotoCurator{})

	resp := postPhoto(t, srv, "photo", []byte{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400 for an empty file, got %d", resp.StatusCode)
	}
}

// Over the ceiling is 413, not 400: the client is told the file is too big rather than that its request was
// malformed. The distinction matters because one is fixable by the photographer and the other is not.
func TestAdminUploadRefusesAnOversizeFile(t *testing.T) {
	_, srv := uploadApp(t, &stubPhotoCurator{})

	// Just over the ceiling, and incompressible so the multipart body really is this big.
	huge := bytes.Repeat([]byte{0x17, 0x2b, 0x9f, 0xc4}, (maxAdminUpload/4)+1024)
	resp := postPhoto(t, srv, "photo", huge)

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("want 413 for a file over %d MB, got %d", maxAdminUpload>>20, resp.StatusCode)
	}
}

// The ceiling must clear a real camera JPEG. Glimt's 12 MiB does not, which is why this endpoint has its own
// number — a photographer whose ordinary files are refused would reasonably conclude the tool is broken.
func TestAdminUploadCeilingClearsAFullFrameJPEG(t *testing.T) {
	// 25 MiB is a generous but realistic maximum-quality full-frame JPEG.
	const realisticMax = 25 << 20
	if maxAdminUpload <= realisticMax {
		t.Errorf("maxAdminUpload is %d MB, which does not clear a %d MB camera JPEG",
			maxAdminUpload>>20, realisticMax>>20)
	}
	if maxGlimtUpload >= maxAdminUpload {
		t.Error("the admin ceiling must be higher than glimt's: the input device is a camera, not a phone")
	}
}

// **The ordering this whole feature depends on.** The coordinate is read from the original bytes and then
// destroyed by re-encoding. Asserted on the stored object, not on the response.
func TestAdminUploadReadsGPSThenDestroysIt(t *testing.T) {
	app, srv := uploadApp(t, &stubPhotoCurator{})

	raw := jpegWithTestGPS(t, 55, 43, 59.74, 'N', 12, 15, 53.35, 'E')
	if _, _, ok := imaging.ReadGPS(raw); !ok {
		t.Fatal("the fixture should carry a coordinate to begin with")
	}

	out := decodeUpload(t, postPhoto(t, srv, "photo", raw))

	if out.Location == nil {
		t.Fatal("the coordinate must be carried out of the ingest and returned to the curator")
	}
	if out.Location.BoundsVerdict != photo.BoundsInside {
		t.Errorf("a coordinate at the event should be inside, got %q", out.Location.BoundsVerdict)
	}

	// And the stored bytes carry none of it.
	rc, err := app.blobs.Get(context.Background(), blobRefOf(out.PhotoID))
	if err != nil {
		t.Fatalf("reading back the stored object: %v", err)
	}
	defer rc.Close()
	storedBytes, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("reading the stored object: %v", err)
	}
	if _, _, ok := imaging.ReadGPS(storedBytes); ok {
		t.Fatal("the stored image still carries GPS: reading the coordinate must never mean keeping it")
	}
}

// An out-of-bounds coordinate is returned to the curator *as rejected*, because PRD 011 §6 requires it to be
// visible as such rather than merely absent from the map.
func TestAdminUploadReportsAnOutOfBoundsCoordinate(t *testing.T) {
	_, srv := uploadApp(t, &stubPhotoCurator{})

	raw := jpegWithTestGPS(t, 40, 42, 46.0, 'N', 74, 0, 21.6, 'W')
	out := decodeUpload(t, postPhoto(t, srv, "photo", raw))

	if out.Location == nil {
		t.Fatal("an out-of-bounds coordinate must still be reported, not dropped")
	}
	if out.Location.BoundsVerdict != photo.BoundsOutside {
		t.Errorf("want the outside verdict, got %q", out.Location.BoundsVerdict)
	}
	if photo.Plottable(out.Location.BoundsVerdict) {
		t.Error("an out-of-bounds coordinate must not be plottable")
	}
}

// A broken stream must not answer 200. The projection is downstream of the log, so a silent publish failure
// would look like success until the curator reloaded and found the photograph missing.
func TestAdminUploadWithNoStreamDoesNotClaimToHaveSaved(t *testing.T) {
	app, srv := uploadApp(t, &stubPhotoCurator{})
	// A commands facade with no publisher, which is what a missing broker looks like.
	app.commands = commandsWithNoPublisher()

	resp := postPhoto(t, srv, "photo", devFixtureImage(800, 600, 0, 0, "no stream"))
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("want 503 with no event stream, got %d", resp.StatusCode)
	}
}

// With no library read there is no way to answer "was this already here", which is the endpoint's whole
// contract — so it is a 503 rather than a degraded success that cannot tell a duplicate from a new file.
func TestAdminUploadWithNoLibraryIsUnavailable(t *testing.T) {
	app, srv := adminApp(t)
	app.models.PhotoCurator = nil

	resp := postPhoto(t, srv, "photo", devFixtureImage(800, 600, 0, 0, "no library"))
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("want 503 with no library read, got %d", resp.StatusCode)
	}
}

// The upload is behind the credential like everything else on the surface, and its refusal carries the same
// headers. Cheap to assert, and it is the one endpoint where an unauthenticated success would be worst: an
// anonymous write into the one store that cannot be rebuilt from the log.
func TestAdminUploadRequiresTheCredential(t *testing.T) {
	_, srv := uploadApp(t, &stubPhotoCurator{})

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("photo", "IMG_0001.JPG")
	_, _ = part.Write(devFixtureImage(400, 300, 0, 0, "anon"))
	_ = mw.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/admin/photos", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Forwarded-Proto", "https")
	// No credential.

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("an anonymous upload must be refused, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("want no-store on the refusal, got %q", got)
	}
}

// A raw body, without multipart, is also accepted — which is what a `curl --data-binary` does and what makes
// the endpoint testable by hand during an event.
func TestAdminUploadAcceptsARawBody(t *testing.T) {
	_, srv := uploadApp(t, &stubPhotoCurator{})

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/admin/photos",
		bytes.NewReader(devFixtureImage(400, 300, 0, 0, "raw body")))
	req.Header.Set("Content-Type", "image/jpeg")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.SetBasicAuth(testAdminUser, testAdminPass)

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200 for a raw image body, got %d", resp.StatusCode)
	}
}

// The response must not carry blob refs beyond the id.
//
// The id *is* a content ref, which is unavoidable and fine — it is the photograph's name and the uploader just
// supplied the bytes. What must not appear is the **thumbnail** ref, which is a second capability with no
// reason to leave the server: the browser addresses a thumbnail through the photograph's id.
func TestAdminUploadDoesNotReturnTheThumbnailRef(t *testing.T) {
	_, srv := uploadApp(t, &stubPhotoCurator{})

	resp := postPhoto(t, srv, "photo", devFixtureImage(800, 600, 0, 0, "refs"))
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the response: %v", err)
	}
	for _, leak := range []string{"thumbRef", "thumb_ref", "blobRef", "ref\":"} {
		if strings.Contains(string(body), leak) {
			t.Errorf("the response carries %q; a thumbnail ref is a capability with no reason to leave the "+
				"server\ngot: %s", leak, body)
		}
	}
}

// Uploading is idempotent in the sense that matters operationally: a re-drag after a dropped connection is a
// correct recovery procedure. Asserted across a whole small "batch" with an overlap, because that is the real
// shape — a photographer re-drags the folder, not the one file that failed.
func TestAdminUploadMakesARedragSafe(t *testing.T) {
	curator := &stubPhotoCurator{present: map[string]bool{}, deleted: map[string]bool{}}
	_, srv := uploadApp(t, curator)

	files := [][]byte{
		devFixtureImage(400, 300, 0, 1, "a"),
		devFixtureImage(400, 300, 0, 2, "b"),
		devFixtureImage(400, 300, 0, 3, "c"),
	}

	// First pass: two land, then the "connection drops".
	ids := map[string]bool{}
	for _, f := range files[:2] {
		out := decodeUpload(t, postPhoto(t, srv, "photo", f))
		if out.Outcome != adminUploadStored {
			t.Fatalf("want stored on the first pass, got %q", out.Outcome)
		}
		curator.present[out.PhotoID] = true
		ids[out.PhotoID] = true
	}

	// Second pass: the whole folder again.
	stored, already := 0, 0
	for _, f := range files {
		out := decodeUpload(t, postPhoto(t, srv, "photo", f))
		switch out.Outcome {
		case adminUploadStored:
			stored++
			curator.present[out.PhotoID] = true
		case adminUploadAlreadyPresent:
			already++
			if !ids[out.PhotoID] {
				t.Errorf("reported already-present for an id the first pass never stored: %s", out.PhotoID)
			}
		default:
			t.Errorf("unexpected outcome %q", out.Outcome)
		}
	}

	if stored != 1 || already != 2 {
		t.Errorf("re-dragging three files after two landed should store 1 and skip 2; got stored=%d already=%d",
			stored, already)
	}
}

// The deadline is extended before the body is read, or it extends nothing. Structural, because a timeout is
// not usefully testable in a unit test.
func TestAdminUploadExtendsTheDeadlineBeforeReadingTheBody(t *testing.T) {
	src := adminSource(t, "adminupload.go")

	deadline := strings.Index(src, "SetReadDeadline")
	read := strings.Index(src, "readAdminUpload(w, r)")
	if deadline < 0 || read < 0 {
		t.Fatal("could not find both the deadline and the body read; this guard needs updating")
	}
	if deadline > read {
		t.Error("the read deadline must be set before the body is read, or it extends nothing")
	}
	if adminUploadTimeout <= glimtUploadTimeout {
		t.Errorf("the admin timeout (%v) should exceed glimt's (%v): the file is larger and the person is "+
			"uploading three hundred of them", adminUploadTimeout, glimtUploadTimeout)
	}
}

// Every write logs what happened, to what, and from where. With a shared credential the log is the only audit
// trail there is (PRD 022 §8.2), so its presence is a requirement rather than a convenience.
func TestAdminUploadLogsEveryOutcome(t *testing.T) {
	src := adminSource(t, "adminupload.go")

	// Each of the three outcomes logs, and each log carries the id and the client IP.
	if got := strings.Count(src, "app.Logger.Info"); got < 3 {
		t.Errorf("want a log line for each of the three upload outcomes, found %d", got)
	}
	for _, field := range []string{`"photoId", photoID`, `"ip", clientIP(r)`} {
		if strings.Count(src, field) < 3 {
			t.Errorf("every upload log line must carry %s: with a shared credential the log is the only "+
				"audit trail", field)
		}
	}
	_ = time.Now
}
