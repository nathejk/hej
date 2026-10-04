package video

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The renditions a library video gets (PRD 029 §6).
//
// Two MP4s and a poster. Both MP4s are H.264/AAC with the index at the front (`+faststart`), so a browser starts
// playing from the first bytes and seeks with Range requests; that is what makes one file per quality enough and
// HLS unnecessary (PRD 029 §8.2).
const (
	// HDShortEdge is the 720p rendition's short edge: 1280×720 landscape, 720×1280 portrait.
	HDShortEdge = 720
	// SDShortEdge is the 480p rendition's, produced only for clips longer than SDThreshold.
	SDShortEdge = 480
	// SDThreshold is the duration above which a clip also gets the 480p rendition.
	SDThreshold = 5 * time.Minute
)

// Renditions are the files a transcode wrote, in the directory it was given.
type Renditions struct {
	HD     string
	SD     string // "" when the clip is not long enough to need one
	Poster string // a JPEG frame at most HDShortEdge on its short side
}

// Runner runs one command and returns its combined output. A variable so tests can substitute one.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// ExecRunner runs the command at reduced priority, so a 22-minute encode never starves the API that shares its
// CPU (PRD 029 §6).
func ExecRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	nice, err := exec.LookPath("nice")
	if err != nil {
		return exec.CommandContext(ctx, name, args...).CombinedOutput()
	}
	return exec.CommandContext(ctx, nice, append([]string{"-n", "10", name}, args...)...).CombinedOutput()
}

// scaleFilter fits the short edge to edge without upscaling, keeping the long edge even. ffmpeg has already
// applied the container's rotation by the time the filter runs, so "short edge" is the displayed one.
func scaleFilter(edge int) string {
	e := strconv.Itoa(edge)
	return "scale='if(gt(iw,ih),-2,min(" + e + ",iw))':'if(gt(iw,ih),min(" + e + ",ih),-2)'"
}

// encodeArgs is one rendition's ffmpeg command line.
func encodeArgs(in, out string, edge int, maxrate, audio string) []string {
	return []string{
		"-hide_banner", "-loglevel", "error", "-y", "-i", in,
		// No metadata from the phone survives into anything a reader is served: no location, no device.
		"-map_metadata", "-1", "-map", "0:v:0", "-map", "0:a:0?",
		"-vf", scaleFilter(edge),
		"-c:v", "libx264", "-preset", "medium", "-crf", "23", "-profile:v", "high",
		"-maxrate", maxrate, "-bufsize", doubleRate(maxrate), "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", audio, "-ac", "2",
		"-movflags", "+faststart",
		out,
	}
}

func doubleRate(r string) string {
	n, err := strconv.ParseFloat(strings.TrimSuffix(r, "M"), 64)
	if err != nil {
		return r
	}
	return strconv.FormatFloat(n*2, 'f', -1, 64) + "M"
}

// Transcode writes the renditions for in into dir.
//
// durationMs decides whether the 480p rendition is made; pass the probed value.
func Transcode(ctx context.Context, run Runner, in, dir string, durationMs int) (Renditions, error) {
	if run == nil {
		run = ExecRunner
	}
	out := Renditions{
		HD:     filepath.Join(dir, "hd.mp4"),
		Poster: filepath.Join(dir, "poster.jpg"),
	}
	if b, err := run(ctx, "ffmpeg", encodeArgs(in, out.HD, HDShortEdge, "2.5M", "128k")...); err != nil {
		return Renditions{}, failure("720p", err, b)
	}
	if time.Duration(durationMs)*time.Millisecond > SDThreshold {
		out.SD = filepath.Join(dir, "sd.mp4")
		// From the 720p rendition rather than the original: a quarter of the pixels to decode, and the
		// phone's HEVC/HDR quirks have already been dealt with once.
		if b, err := run(ctx, "ffmpeg", encodeArgs(out.HD, out.SD, SDShortEdge, "1M", "96k")...); err != nil {
			return Renditions{}, failure("480p", err, b)
		}
	}
	// The poster from the 720p rendition too, one second in (or the first frame of a shorter clip): the very first
	// frame of a phone clip is often black or blurred by the hand pressing record.
	at := "1"
	if durationMs < 2000 {
		at = "0"
	}
	if b, err := run(ctx, "ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-ss", at, "-i", out.HD,
		"-frames:v", "1", "-q:v", "3", out.Poster); err != nil {
		return Renditions{}, failure("poster", err, b)
	}
	return out, nil
}

// ErrTranscode wraps a failure that is the file's fault rather than the server's.
var ErrTranscode = errors.New("video: transcode failed")

func failure(step string, err error, output []byte) error {
	var exit *exec.ExitError
	msg := strings.TrimSpace(string(output))
	if len(msg) > 200 {
		msg = msg[len(msg)-200:]
	}
	if errors.As(err, &exit) {
		return fmt.Errorf("%w: %s: %s", ErrTranscode, step, msg)
	}
	return fmt.Errorf("video: %s: %w", step, err)
}
