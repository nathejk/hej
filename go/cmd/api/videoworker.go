package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/imaging"
	"nathejk.dk/internal/video"
	"nathejk.dk/nathejk/table/photo"
)

// The transcode worker (PRD 029, task 495).
//
// # One goroutine, one job at a time, in this binary
//
// PRD 029 §11 Q6: a thread of its own in the same binary. One job at a time because a transcode already uses every
// core ffmpeg is given, and two would only halve each other's speed while doubling the scratch disk. ffmpeg runs at
// `nice 10`, so the API stays responsive while it works.
//
// # The projection is the queue
//
// There is no job table. A video in `processing` *is* a job, so the queue survives a restart by construction: a
// worker killed mid-encode finds the same row on the next boot and starts it again. Every output is
// content-addressed, so a repeated job converges on the same objects rather than duplicating them.
//
// The one thing the projection cannot say is "I just finished this": the `videotranscoded` event takes a moment to
// fold, and a worker that polled in that moment would encode the clip twice. `recent` covers that window.

const (
	// videoWorkerPoll is the fallback interval. Uploads and retries wake the worker directly; this only catches
	// what arrived while it was busy, or a requeue from another path.
	videoWorkerPoll = time.Minute
	// videoRecentWindow is how long a finished job is not picked up again, covering projection lag. Also the
	// back-off after a server-side failure (disk, store), which is retried rather than marked failed.
	videoRecentWindow = 15 * time.Minute
	// videoJobTimeout bounds one job. A 30-minute 4K clip at well under real time fits with room to spare; a job
	// that runs past it is stuck, not slow.
	videoJobTimeout = 3 * time.Hour
)

// startVideoWorker starts the worker if this process can run one.
func (app *application) startVideoWorker(ctx context.Context, logger *slog.Logger) {
	if app.videoFiles() == nil || app.models.PhotoCurator == nil {
		logger.Warn("video transcode worker not started: needs a blob store that holds files and the photo library")
		return
	}
	app.videoWake = make(chan struct{}, 1)
	go app.runVideoWorker(ctx, videoWorkerPoll)
	logger.Info("video transcode worker started")
}

func (app *application) runVideoWorker(ctx context.Context, poll time.Duration) {
	recent := map[string]time.Time{}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		for app.processNextVideo(ctx, recent) {
			if ctx.Err() != nil {
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-app.videoWake:
		case <-ticker.C:
		}
	}
}

// processNextVideo runs one pending job, if there is one not recently attempted. It reports whether it did.
func (app *application) processNextVideo(ctx context.Context, recent map[string]time.Time) bool {
	now := time.Now()
	for id, at := range recent {
		if now.Sub(at) > videoRecentWindow {
			delete(recent, id)
		}
	}
	pending, err := app.models.PhotoCurator.ProcessingVideos(10)
	if err != nil {
		app.Logger.Error("video worker: reading the queue", "err", err)
		return false
	}
	for _, v := range pending {
		if _, seen := recent[v.PhotoID]; seen {
			continue
		}
		recent[v.PhotoID] = now
		jobCtx, cancel := context.WithTimeout(ctx, videoJobTimeout)
		app.transcodeVideo(jobCtx, v)
		cancel()
		return true
	}
	return false
}

// transcodeVideo runs one job and publishes its outcome. A failure that is the file's fault is published as
// `videofailed`; one that is the server's (disk, store, a killed process) is logged and retried after
// videoRecentWindow, because marking a good clip failed for a full disk would make the curator retry by hand.
func (app *application) transcodeVideo(ctx context.Context, v photo.PendingVideo) {
	started := time.Now()
	log := app.Logger.With("photoId", v.PhotoID, "year", v.Year)
	log.Info("video worker: transcoding", "durationMs", v.DurationMs)

	files := app.videoFiles()
	staging, err := files.StagingDir()
	if err != nil {
		log.Error("video worker: staging", "err", err)
		return
	}
	work, err := os.MkdirTemp(staging, "transcode-")
	if err != nil {
		log.Error("video worker: scratch dir", "err", err)
		return
	}
	defer os.RemoveAll(work)

	in, err := app.videoInput(ctx, files, blob.Ref(v.OriginalRef), work)
	if errors.Is(err, blob.ErrNotFound) {
		app.publishVideoFailed(v, "originalen findes ikke længere på serveren")
		return
	}
	if err != nil {
		log.Error("video worker: reading the original", "err", err)
		return
	}

	out, err := video.Transcode(ctx, app.videoRunner, in, work, v.DurationMs)
	if errors.Is(err, video.ErrTranscode) {
		log.Warn("video worker: the clip could not be transcoded", "err", err)
		app.publishVideoFailed(v, err.Error())
		return
	}
	if err != nil {
		log.Error("video worker: transcode", "err", err)
		return
	}

	probe := app.videoProbe
	if probe == nil {
		probe = video.Probe
	}
	info, err := probe(ctx, out.HD)
	if err != nil {
		log.Error("video worker: probing the 720p rendition", "err", err)
		return
	}
	hdInfo, err := os.Stat(out.HD)
	if err != nil {
		log.Error("video worker: stat 720p", "err", err)
		return
	}
	poster, err := os.ReadFile(out.Poster)
	if err != nil {
		log.Error("video worker: reading the poster", "err", err)
		return
	}

	// The renditions are cache class: all of them can be produced again from the original (PRD 029 §6).
	hdRef, err := files.PutFile(ctx, out.HD, true)
	if err != nil {
		log.Error("video worker: storing 720p", "err", err)
		return
	}
	sdRef := blob.Ref("")
	if out.SD != "" {
		if sdRef, err = files.PutFile(ctx, out.SD, true); err != nil {
			log.Error("video worker: storing 480p", "err", err)
			return
		}
	}

	// The poster goes through the photograph pipeline, so a video's grid tile and viewer placeholder are the same
	// sizes, made the same way, as every photograph's.
	thumbRef, mediumRef := "", ""
	var prepared imaging.Portrait
	if perr := withDecodeSlot(ctx, func() error {
		var e error
		prepared, e = imaging.Prepare(poster, maxGlimtEdge, libraryThumbEdges, glimtJPEGQuality, false)
		return e
	}); perr != nil {
		// A video without a poster still plays; the grid falls back as it does for a photograph without a
		// thumbnail.
		log.Warn("video worker: no poster", "err", perr)
	} else {
		thumbRef = app.storeRendition(ctx, prepared, glimtThumbEdges[0], "thumbnail")
		mediumRef = app.storeRendition(ctx, prepared, mediumEdge, "medium")
	}

	// The displayed dimensions: a portrait phone clip is stored landscape with a rotation, and the 720p rendition
	// has had the rotation applied. `Probe` reports stored dimensions, so a rendition with no rotation tag is
	// already the right way up.
	width, height := info.Width, info.Height
	if info.Rotation == 90 || info.Rotation == 270 {
		width, height = height, width
	}
	subject, err := photo.Subject(v.Year, v.PhotoID, photo.VerbVideoTranscoded)
	if err != nil {
		log.Error("video worker: subject", "err", err)
		return
	}
	if err := app.commands.Publish(subject, photo.VideoTranscoded{
		PhotoID: v.PhotoID, Year: v.Year,
		Ref: hdRef.String(), SdRef: sdRef.String(), ThumbRef: thumbRef, MediumRef: mediumRef,
		Width: width, Height: height, Bytes: int(hdInfo.Size()), DurationMs: info.DurationMs,
		TranscodedAt: time.Now().UTC(),
	}); err != nil {
		log.Error("video worker: publishing the result", "err", err)
		return
	}
	log.Info("video worker: done", "took", time.Since(started).Round(time.Second), "hdBytes", hdInfo.Size(),
		"sd", out.SD != "")
}

// videoInput returns a path ffmpeg can read the original from: the object itself when the store has one, otherwise
// a copy in the scratch directory.
func (app *application) videoInput(ctx context.Context, files blob.Files, ref blob.Ref, work string) (string, error) {
	if p, ok, err := files.LocalPath(ctx, ref); err != nil {
		return "", err
	} else if ok {
		return p, nil
	}
	src, err := files.Open(ctx, ref)
	if err != nil {
		return "", err
	}
	defer src.Close()
	p := filepath.Join(work, "original")
	dst, err := os.Create(p)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return "", err
	}
	return p, dst.Close()
}

func (app *application) publishVideoFailed(v photo.PendingVideo, reason string) {
	subject, err := photo.Subject(v.Year, v.PhotoID, photo.VerbVideoFailed)
	if err == nil {
		err = app.commands.Publish(subject, photo.VideoFailed{
			PhotoID: v.PhotoID, Year: v.Year, Reason: reason, FailedAt: time.Now().UTC(),
		})
	}
	if err != nil {
		app.Logger.Error("video worker: publishing a failure", "photoId", v.PhotoID, "err", err)
	}
}

// requeueVideo puts a video back in `processing` and wakes the worker: the curator's retry, and the rebuild after
// a rendition went missing from the cache.
func (app *application) requeueVideo(year, photoID string) error {
	subject, err := photo.Subject(year, photoID, photo.VerbVideoQueued)
	if err != nil {
		return err
	}
	if err := app.commands.Publish(subject, photo.VideoQueued{
		PhotoID: photoID, Year: year, QueuedAt: time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("requeue video: %w", err)
	}
	app.notifyVideoWorker()
	return nil
}
