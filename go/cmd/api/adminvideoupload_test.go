package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/video"
	"nathejk.dk/nathejk/table/photo"

	"github.com/jrgensen/cqrs/cqrstest"
)

// The resumable video upload (PRD 029, task 494).

func videoUploadApp(t *testing.T, info video.Info, probeErr error) (*application, *cqrstest.Publisher, func(method, path string, body io.Reader, hdr ...string) *http.Response) {
	t.Helper()
	app, srv := adminApp(t)
	app.models.PhotoCurator = &libraryCurator{}
	pub := &cqrstest.Publisher{}
	app.commands = commandsWithPublisher(t, pub)
	app.videoProbe = func(context.Context, string) (video.Info, error) { return info, probeErr }

	do := func(method, path string, body io.Reader, hdr ...string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, srv.URL+path, body)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set(adminYearHeader, "2026")
		req.SetBasicAuth(testAdminUser, testAdminPass)
		for i := 0; i+1 < len(hdr); i += 2 {
			req.Header.Set(hdr[i], hdr[i+1])
		}
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = resp.Body.Close() })
		return resp
	}
	return app, pub, do
}

func decodeState(t *testing.T, resp *http.Response) videoUploadState {
	t.Helper()
	var s videoUploadState
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func startVideoUpload(t *testing.T, do func(string, string, io.Reader, ...string) *http.Response, size int) string {
	t.Helper()
	resp := do(http.MethodPost, "/api/admin/videos/uploads", strings.NewReader(fmt.Sprintf(`{"fileName":"IMG_1.MOV","size":%d}`, size)))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d", resp.StatusCode)
	}
	return decodeState(t, resp).UploadID
}

func putChunk(do func(string, string, io.Reader, ...string) *http.Response, id string, data []byte, start, total int) *http.Response {
	return do(http.MethodPut, "/api/admin/videos/uploads/"+id, bytes.NewReader(data),
		"Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, start+len(data)-1, total))
}

func TestVideoUploadResumesAndCompletes(t *testing.T) {
	app, pub, do := videoUploadApp(t, video.Info{FormatName: "mov,mp4", DurationMs: 90_000, Width: 1920, Height: 1080}, nil)
	clip := bytes.Repeat([]byte("frame"), 300)
	id := startVideoUpload(t, do, len(clip))

	if got := putChunk(do, id, clip[:500], 0, len(clip)).StatusCode; got != http.StatusOK {
		t.Fatalf("first chunk: %d", got)
	}
	// The client lost the response and resends from the wrong place: told where to continue, not corrupted.
	resp := putChunk(do, id, clip[200:700], 200, len(clip))
	if resp.StatusCode != http.StatusConflict || decodeState(t, resp).Offset != 500 {
		t.Fatalf("an out-of-order chunk should be 409 with offset 500, got %d", resp.StatusCode)
	}
	if got := decodeState(t, do(http.MethodGet, "/api/admin/videos/uploads/"+id, nil)).Offset; got != 500 {
		t.Fatalf("resume offset %d", got)
	}
	if got := putChunk(do, id, clip[500:], 500, len(clip)).StatusCode; got != http.StatusOK {
		t.Fatalf("second chunk: %d", got)
	}

	resp = do(http.MethodPost, "/api/admin/videos/uploads/"+id+"/complete", nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("complete: %d %s", resp.StatusCode, body)
	}
	var out adminUploadResponse
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out.Outcome != adminUploadStored || out.PhotoID != blob.ComputeRef(clip).String() {
		t.Errorf("got %+v", out)
	}
	if ok, _ := app.blobs.Exists(context.Background(), blob.ComputeRef(clip)); !ok {
		t.Error("the original was not stored")
	}
	subjects := pub.Subjects()
	if len(subjects) != 1 || !strings.HasSuffix(subjects[0], ".videouploaded") {
		t.Fatalf("published %v", subjects)
	}
	var ev photo.VideoUploaded
	_ = pub.Messages[0].Body(&ev)
	if ev.Original.ContentType != "video/quicktime" || ev.DurationMs != 90_000 || ev.FileName != "IMG_1.MOV" {
		t.Errorf("event %+v", ev)
	}
}

func TestVideoUploadRefusesOver30Minutes(t *testing.T) {
	_, pub, do := videoUploadApp(t, video.Info{DurationMs: 31 * 60 * 1000, Width: 1, Height: 1}, nil)
	id := startVideoUpload(t, do, 4)
	putChunk(do, id, []byte("abcd"), 0, 4)
	resp := do(http.MethodPost, "/api/admin/videos/uploads/"+id+"/complete", nil)
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusRequestEntityTooLarge || !strings.Contains(string(body), "30 minutter") {
		t.Errorf("want 413 naming the limit, got %d %s", resp.StatusCode, body)
	}
	if len(pub.Messages) != 0 {
		t.Error("published anyway")
	}
}

func TestVideoUploadRefusesOver4GB(t *testing.T) {
	_, _, do := videoUploadApp(t, video.Info{}, nil)
	resp := do(http.MethodPost, "/api/admin/videos/uploads", strings.NewReader(fmt.Sprintf(`{"size":%d}`, int64(maxVideoUpload)+1)))
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("want 413, got %d", resp.StatusCode)
	}
}

func TestVideoUploadRefusesANonVideo(t *testing.T) {
	_, _, do := videoUploadApp(t, video.Info{}, video.ErrNotVideo)
	id := startVideoUpload(t, do, 4)
	putChunk(do, id, []byte("abcd"), 0, 4)
	if got := do(http.MethodPost, "/api/admin/videos/uploads/"+id+"/complete", nil).StatusCode; got != http.StatusBadRequest {
		t.Errorf("want 400, got %d", got)
	}
}

func TestVideoUploadCannotCompleteEarly(t *testing.T) {
	_, _, do := videoUploadApp(t, video.Info{DurationMs: 1000, Width: 1, Height: 1}, nil)
	id := startVideoUpload(t, do, 8)
	putChunk(do, id, []byte("abcd"), 0, 8)
	if got := do(http.MethodPost, "/api/admin/videos/uploads/"+id+"/complete", nil).StatusCode; got != http.StatusBadRequest {
		t.Errorf("want 400, got %d", got)
	}
}

// A clip a curator took down is not put back by uploading it again.
func TestVideoUploadDoesNotRestoreADeletedClip(t *testing.T) {
	app, pub, do := videoUploadApp(t, video.Info{DurationMs: 1000, Width: 1, Height: 1}, nil)
	clip := []byte("taken down")
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{{ID: blob.ComputeRef(clip).String(), Deleted: true}}}
	id := startVideoUpload(t, do, len(clip))
	putChunk(do, id, clip, 0, len(clip))
	resp := do(http.MethodPost, "/api/admin/videos/uploads/"+id+"/complete", nil)
	var out adminUploadResponse
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out.Outcome != adminUploadPreviouslyDeleted || len(pub.Messages) != 0 {
		t.Errorf("got %+v, published %v", out, pub.Subjects())
	}
	if ok, _ := app.blobs.Exists(context.Background(), blob.ComputeRef(clip)); ok {
		t.Error("the deleted clip's bytes were put back on disk")
	}
}

func TestVideoUploadIDsCannotNameAPath(t *testing.T) {
	_, _, do := videoUploadApp(t, video.Info{}, nil)
	if got := do(http.MethodGet, "/api/admin/videos/uploads/..%2f..%2fetc", nil).StatusCode; got != http.StatusNotFound {
		t.Errorf("want 404, got %d", got)
	}
}
