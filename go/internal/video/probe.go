// Package video wraps ffprobe and ffmpeg for the library's video items (PRD 029).
//
// The binaries are in the API image (task 491). Everything here shells out rather than linking a decoder: the
// inputs are phone recordings of every shape, ffmpeg is the tool that reads all of them, and a crash in a
// child process costs one job rather than the API.
package video

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

// Info is what ffprobe says about a file.
type Info struct {
	// FormatName is ffprobe's container name, e.g. "mov,mp4,m4a,3gp,3g2,mj2" or "matroska,webm".
	FormatName string
	DurationMs int
	// Width and Height are the stored frame size **before** rotation, as for a photograph original.
	Width  int
	Height int
	// Rotation is the display rotation in degrees (0, 90, 180, 270) a phone records instead of rotating pixels.
	Rotation int
	HasAudio bool
}

// ErrNotVideo means ffprobe read the file but found no video stream, or could not read it at all.
var ErrNotVideo = errors.New("video: not a video")

// Prober reads a file's Info. A func type so tests can supply one without ffprobe installed.
type Prober func(ctx context.Context, path string) (Info, error)

// Probe runs ffprobe.
func Probe(ctx context.Context, path string) (Info, error) {
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-print_format", "json",
		"-show_format", "-show_streams", path).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			// ffprobe ran and refused the file: that is a property of the file, not of the server.
			return Info{}, fmt.Errorf("%w: %s", ErrNotVideo, strings.TrimSpace(string(exit.Stderr)))
		}
		return Info{}, fmt.Errorf("video: ffprobe: %w", err)
	}
	return parseProbe(out)
}

type probeJSON struct {
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		CodecType   string            `json:"codec_type"`
		Width       int               `json:"width"`
		Height      int               `json:"height"`
		Tags        map[string]string `json:"tags"`
		Disposition map[string]int    `json:"disposition"`
		SideData    []struct {
			Rotation float64 `json:"rotation"`
		} `json:"side_data_list"`
	} `json:"streams"`
}

func parseProbe(out []byte) (Info, error) {
	var p probeJSON
	if err := json.Unmarshal(out, &p); err != nil {
		return Info{}, fmt.Errorf("video: parse ffprobe output: %w", err)
	}
	info := Info{FormatName: p.Format.FormatName}
	if d, err := strconv.ParseFloat(p.Format.Duration, 64); err == nil && d > 0 {
		info.DurationMs = int(math.Round(d * 1000))
	}
	found := false
	for _, s := range p.Streams {
		switch s.CodecType {
		case "audio":
			info.HasAudio = true
		case "video":
			// A cover image in an audio file, or a thumbnail track, is a "video" stream too. Skip it.
			if s.Disposition["attached_pic"] == 1 || found {
				continue
			}
			found = true
			info.Width, info.Height = s.Width, s.Height
			rot := 0.0
			for _, sd := range s.SideData {
				if sd.Rotation != 0 {
					rot = sd.Rotation
				}
			}
			if r, err := strconv.ParseFloat(s.Tags["rotate"], 64); err == nil && rot == 0 {
				rot = r
			}
			info.Rotation = ((int(math.Round(rot))%360 + 360) % 360)
		}
	}
	if !found || info.Width == 0 || info.Height == 0 {
		return Info{}, ErrNotVideo
	}
	return info, nil
}

// ContentType maps a probed container and the uploaded file name to a MIME type for the original.
func ContentType(info Info, fileName string) string {
	lower := strings.ToLower(fileName)
	switch {
	case strings.Contains(info.FormatName, "webm") || strings.Contains(info.FormatName, "matroska"):
		return "video/webm"
	case strings.Contains(info.FormatName, "mov") || strings.Contains(info.FormatName, "mp4"):
		if strings.HasSuffix(lower, ".mov") || strings.HasSuffix(lower, ".qt") {
			return "video/quicktime"
		}
		return "video/mp4"
	case strings.Contains(info.FormatName, "avi"):
		return "video/x-msvideo"
	}
	return "application/octet-stream"
}
