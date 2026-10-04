package video

import (
	"context"
	"strings"
	"testing"
)

type recorder struct{ calls [][]string }

func (r *recorder) run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	return nil, nil
}

func TestTranscodeShortClipMakesNoSDRendition(t *testing.T) {
	var r recorder
	out, err := Transcode(context.Background(), r.run, "/in.mov", "/tmp/x", 3*60*1000)
	if err != nil {
		t.Fatal(err)
	}
	if out.SD != "" || len(r.calls) != 2 {
		t.Errorf("want 720p + poster only, got %d calls, SD=%q", len(r.calls), out.SD)
	}
	hd := strings.Join(r.calls[0], " ")
	for _, want := range []string{"-map_metadata -1", "libx264", "-crf 23", "-maxrate 2.5M", "-bufsize 5M",
		"+faststart", "yuv420p", "aac", "min(720,ih)"} {
		if !strings.Contains(hd, want) {
			t.Errorf("720p command lacks %q:\n%s", want, hd)
		}
	}
}

func TestTranscodeLongClipAlsoMakesSD(t *testing.T) {
	var r recorder
	out, err := Transcode(context.Background(), r.run, "/in.mov", "/tmp/x", 22*60*1000)
	if err != nil {
		t.Fatal(err)
	}
	if out.SD == "" || len(r.calls) != 3 {
		t.Fatalf("want 720p, 480p and poster, got %d calls", len(r.calls))
	}
	sd := strings.Join(r.calls[1], " ")
	if !strings.Contains(sd, "-maxrate 1M") || !strings.Contains(sd, "min(480,ih)") || !strings.Contains(sd, "-i /tmp/x/hd.mp4") {
		t.Errorf("480p command:\n%s", sd)
	}
}
