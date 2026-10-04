package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"nathejk.dk/internal/video"
	"nathejk.dk/nathejk/table/photo"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/cqrstest"
)

// The transcode worker (PRD 029, task 495). ffmpeg is faked: what is under test is the job's bookkeeping — what
// is stored where, what is published, and what a failure costs.

func posterJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1280, 720))
	for x := 0; x < 1280; x++ {
		img.Set(x, x%720, color.RGBA{200, 80, 10, 255})
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// fakeFFmpeg writes the output file each command names, or fails like ffmpeg does.
func fakeFFmpeg(t *testing.T, fail bool) video.Runner {
	poster := posterJPEG(t)
	return func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if fail {
			return []byte("moov atom not found"), &exec.ExitError{}
		}
		out := args[len(args)-1]
		data := []byte("mp4 for " + out)
		if strings.HasSuffix(out, ".jpg") {
			data = poster
		}
		return nil, os.WriteFile(out, data, 0o600)
	}
}

func workerApp(t *testing.T, fail bool, durationMs int) (*application, *cqrstest.Publisher, photo.PendingVideo) {
	t.Helper()
	app := newTestApp(t)
	original, err := app.blobs.Put(context.Background(), []byte("the phone's .mov"))
	if err != nil {
		t.Fatal(err)
	}
	v := photo.LibraryPhoto{ID: original.String(), OriginalRef: original.String(), Kind: "video",
		Status: "processing", DurationMs: durationMs}
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{v}}
	pub := &cqrstest.Publisher{}
	app.commands = commandsWithPublisher(t, pub)
	app.videoRunner = fakeFFmpeg(t, fail)
	app.videoProbe = func(context.Context, string) (video.Info, error) {
		return video.Info{Width: 1280, Height: 720, DurationMs: durationMs}, nil
	}
	return app, pub, photo.PendingVideo{Year: "2026", PhotoID: v.ID, OriginalRef: v.OriginalRef, DurationMs: durationMs}
}

func TestVideoWorkerPublishesEveryRendition(t *testing.T) {
	app, pub, _ := workerApp(t, false, 22*60*1000)
	if !app.processNextVideo(context.Background(), map[string]time.Time{}) {
		t.Fatal("the pending video was not picked up")
	}
	if s := pub.Subjects(); len(s) != 1 || !strings.HasSuffix(s[0], ".videotranscoded") {
		t.Fatalf("published %v", s)
	}
	var ev photo.VideoTranscoded
	_ = pub.Messages[0].Body(&ev)
	if ev.Ref == "" || ev.SdRef == "" || ev.ThumbRef == "" || ev.MediumRef == "" {
		t.Errorf("a 22-minute clip needs 720p, 480p and both poster sizes: %+v", ev)
	}
	for _, ref := range []string{ev.Ref, ev.SdRef, ev.ThumbRef, ev.MediumRef} {
		if ok, _ := app.blobs.Exists(context.Background(), blobRefOf(ref)); !ok {
			t.Errorf("%s was published but not stored", ref)
		}
	}
	if ev.Width != 1280 || ev.Height != 720 {
		t.Errorf("dimensions %dx%d", ev.Width, ev.Height)
	}
}

func TestVideoWorkerShortClipHasNoSDRendition(t *testing.T) {
	app, pub, _ := workerApp(t, false, 40_000)
	app.processNextVideo(context.Background(), map[string]time.Time{})
	var ev photo.VideoTranscoded
	_ = pub.Messages[0].Body(&ev)
	if ev.SdRef != "" {
		t.Error("a 40-second clip got a 480p rendition")
	}
}

func TestVideoWorkerPublishesAFailureTheCuratorCanSee(t *testing.T) {
	app, pub, _ := workerApp(t, true, 40_000)
	app.processNextVideo(context.Background(), map[string]time.Time{})
	if s := pub.Subjects(); len(s) != 1 || !strings.HasSuffix(s[0], ".videofailed") {
		t.Fatalf("published %v", s)
	}
	var ev photo.VideoFailed
	_ = pub.Messages[0].Body(&ev)
	if !strings.Contains(ev.Reason, "moov atom not found") {
		t.Errorf("the reason should carry ffmpeg's words: %q", ev.Reason)
	}
}

// The projection lags the event, so a job just finished must not be picked up again in the meantime.
func TestVideoWorkerDoesNotRepeatARecentJob(t *testing.T) {
	app, pub, v := workerApp(t, false, 40_000)
	recent := map[string]time.Time{v.PhotoID: time.Now()}
	if app.processNextVideo(context.Background(), recent) || len(pub.Messages) != 0 {
		t.Error("a job finished moments ago was run again")
	}
}

func TestVideoWorkerFailsAVideoWhoseOriginalIsGone(t *testing.T) {
	app, pub, v := workerApp(t, false, 40_000)
	_ = app.blobs.Delete(context.Background(), blobRefOf(v.OriginalRef))
	app.processNextVideo(context.Background(), map[string]time.Time{})
	if s := pub.Subjects(); len(s) != 1 || !strings.HasSuffix(s[0], ".videofailed") {
		t.Fatalf("published %v", s)
	}
}

func TestRetryRequeuesOnlyAFailedVideo(t *testing.T) {
	app, srv := adminApp(t)
	pub := &cqrstest.Publisher{}
	app.commands = commandsWithPublisher(t, pub)
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{
		{ID: "failed1", Kind: "video", Status: "failed"},
		{ID: "ready1", Kind: "video", Status: "ready"},
		{ID: "photo1", Kind: "photo", Status: "ready"},
	}}
	post := func(id string) int {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/admin/videos/retry/"+id, nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set(adminYearHeader, "2026")
		req.SetBasicAuth(testAdminUser, testAdminPass)
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	if got := post("failed1"); got != http.StatusAccepted {
		t.Errorf("failed video: %d", got)
	}
	if got := post("ready1"); got != http.StatusConflict {
		t.Errorf("ready video: %d", got)
	}
	if got := post("photo1"); got != http.StatusNotFound {
		t.Errorf("photograph: %d", got)
	}
	if s := pub.Subjects(); len(s) != 1 || !strings.HasSuffix(s[0], ".videoqueued") {
		t.Errorf("published %v", s)
	}
}

// lagCurator is a library whose queue is empty until `ready` is set, as a projection is before it folds an event.
type lagCurator struct {
	*libraryCurator
	ready atomic.Bool
}

func (c *lagCurator) ProcessingVideos(limit int) ([]photo.PendingVideo, error) {
	if !c.ready.Load() {
		return nil, nil
	}
	return c.libraryCurator.ProcessingVideos(limit)
}

// lockedPublisher is cqrstest.Publisher made safe to read while the worker goroutine publishes.
type lockedPublisher struct {
	mu sync.Mutex
	*cqrstest.Publisher
}

func (p *lockedPublisher) Publish(msg cqrs.Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.Publisher.Publish(msg)
}

func (p *lockedPublisher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.Messages)
}

// A wake that arrives before the projection has the row still finds the job within seconds, not at the next poll
// (task 504: an upload's job waited 56 s for the minute's tick).
func TestVideoWorkerFindsAJobThatLagsItsWake(t *testing.T) {
	app, _, v := workerApp(t, false, 40_000)
	lag := &lagCurator{libraryCurator: app.models.PhotoCurator.(*libraryCurator)}
	app.models.PhotoCurator = lag
	pub := &lockedPublisher{Publisher: &cqrstest.Publisher{}}
	app.commands = commandsWithPublisher(t, pub)
	app.videoWake = make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.runVideoWorker(ctx, time.Hour)

	app.notifyVideoWorker()
	time.Sleep(300 * time.Millisecond)
	lag.ready.Store(true)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && pub.count() == 0 {
		time.Sleep(100 * time.Millisecond)
	}
	if pub.count() == 0 {
		t.Fatalf("the job for %s was not picked up within 5 s of its wake", v.PhotoID)
	}
}
