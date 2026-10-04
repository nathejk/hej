package video

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"
)

// TestASampleClip runs the real pipeline — ffprobe and ffmpeg — on a file named by VIDEO_SAMPLE (PRD 029, task 504).
// Skipped otherwise, because CI has no sample clips. Run it in the api-dev image, which has ffmpeg:
//
//	docker run --rm -v $PWD/go:/app -v /path/to/clips:/clips -e VIDEO_SAMPLE=/clips/x.mp4 \
//	  --entrypoint go hej-api-dev test -run TestASampleClip -v ./internal/video/
//
// VIDEO_SAMPLE_FORCE_SD=1 claims the clip is long, so the 480p rendition is made and checked too.
func TestASampleClip(t *testing.T) {
	in := os.Getenv("VIDEO_SAMPLE")
	if in == "" {
		t.Skip("VIDEO_SAMPLE not set")
	}
	ctx := context.Background()
	info, err := Probe(ctx, in)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	t.Logf("input: %+v", info)

	duration := info.DurationMs
	if os.Getenv("VIDEO_SAMPLE_FORCE_SD") != "" {
		duration = int(SDThreshold/time.Millisecond) + 1
	}
	dir := t.TempDir()
	started := time.Now()
	out, err := Transcode(ctx, nil, in, dir, duration)
	if err != nil {
		t.Fatalf("transcode: %v", err)
	}
	took := time.Since(started)
	t.Logf("transcode took %s for %d ms of video: %.2fx real time", took.Round(time.Millisecond), info.DurationMs,
		float64(info.DurationMs)/float64(took.Milliseconds()))

	for name, p := range map[string]string{"720p": out.HD, "480p": out.SD} {
		if p == "" {
			continue
		}
		r, err := Probe(ctx, p)
		if err != nil {
			t.Fatalf("%s: probe: %v", name, err)
		}
		st, _ := os.Stat(p)
		t.Logf("%s: %dx%d, %d ms, %d bytes (%.1f MB/min)", name, r.Width, r.Height, r.DurationMs, st.Size(),
			float64(st.Size())/1e6/(float64(r.DurationMs)/60000))
		short := min(r.Width, r.Height)
		want := HDShortEdge
		if name == "480p" {
			want = SDShortEdge
		}
		if short > want {
			t.Errorf("%s: short edge %d, want at most %d", name, short, want)
		}
		if d := r.DurationMs - info.DurationMs; d > 500 || d < -500 {
			t.Errorf("%s: duration %d ms, input %d ms", name, r.DurationMs, info.DurationMs)
		}
		if info.HasAudio && !r.HasAudio {
			t.Errorf("%s: the audio track was lost", name)
		}
		raw, _ := os.ReadFile(p)
		moov, mdat := bytes.Index(raw, []byte("moov")), bytes.Index(raw, []byte("mdat"))
		if moov < 0 || mdat < 0 || moov > mdat {
			t.Errorf("%s: not faststart (moov at %d, mdat at %d) — a browser would have to fetch the end first", name, moov, mdat)
		}
		for _, tag := range [][]byte{[]byte("\xa9xyz"), []byte("com.apple.quicktime.location")} {
			if bytes.Contains(raw, tag) {
				t.Errorf("%s: carries location metadata %q", name, tag)
			}
		}
	}
	poster, err := os.ReadFile(out.Poster)
	if err != nil || len(poster) < 1000 || !bytes.HasPrefix(poster, []byte{0xff, 0xd8}) {
		t.Errorf("poster is not a JPEG (%d bytes, %v)", len(poster), err)
	}
}
