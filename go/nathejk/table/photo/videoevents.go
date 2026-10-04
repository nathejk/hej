package photo

import (
	"fmt"
	"time"

	"github.com/jrgensen/cqrs"
)

// The video event shapes (PRD 029, task 493).
//
// A video is a row in the same table as a photograph (see table.sql's "Video" note), but it reaches that row in
// two steps rather than one: the upload stores the original and the transcode worker produces everything a reader
// is served. So the facts are three, one event each, for the package comment's reason — the log should say what
// happened, and "the upload was accepted" and "the renditions exist" happen minutes apart.
//
// # The id is the original's hash
//
// A photograph's id is the hash of its display rendition, which exists the moment the upload finishes. A video's
// display rendition does not exist until the worker has run, so its id is the hash of the **uploaded file**. The
// property is the same one PRD 022 §8.5 wanted: re-uploading the same clip lands on the same row, and does not
// transcode it again.

// VideoUploaded puts one video in the year's library, in `processing`.
type VideoUploaded struct {
	PhotoID string `json:"photoId"`
	Year    string `json:"year"`

	// Original is the uploaded file. Required: for a video it is the only thing that exists yet.
	Original Original `json:"original"`

	// DurationMs is ffprobe's reading at upload, used for the length cap and shown while processing. The
	// transcode reports it again from the rendition.
	DurationMs int `json:"durationMs,omitempty"`

	FileName   string    `json:"fileName,omitempty"`
	UploadedAt time.Time `json:"uploadedAt"`
}

// VideoTranscoded records that the worker produced a video's renditions, making it `ready`.
type VideoTranscoded struct {
	PhotoID string `json:"photoId"`
	Year    string `json:"year"`

	// Ref is the 720p MP4. Never empty.
	Ref string `json:"ref"`
	// SdRef is the 480p MP4, produced only for clips over five minutes.
	SdRef string `json:"sdRef,omitempty"`
	// ThumbRef and MediumRef are the poster at the photograph rendition sizes.
	ThumbRef  string `json:"thumbRef,omitempty"`
	MediumRef string `json:"mediumRef,omitempty"`

	// Width/Height/Bytes describe the 720p rendition, as they describe the display image for a photograph.
	Width      int `json:"width,omitempty"`
	Height     int `json:"height,omitempty"`
	Bytes      int `json:"bytes,omitempty"`
	DurationMs int `json:"durationMs,omitempty"`

	TranscodedAt time.Time `json:"transcodedAt"`
}

// VideoFailed records that the worker could not transcode a video.
type VideoFailed struct {
	PhotoID string `json:"photoId"`
	Year    string `json:"year"`

	// Reason is shown to the curator, never publicly. Truncated by the fold.
	Reason string `json:"reason"`

	FailedAt time.Time `json:"failedAt"`
}

// VideoQueued puts a video back in `processing`: the curator's retry after a failure, or a rendition rebuilt
// after a cache miss (task 495).
type VideoQueued struct {
	PhotoID  string    `json:"photoId"`
	Year     string    `json:"year"`
	QueuedAt time.Time `json:"queuedAt"`
}

func (c consumer) handleVideoUploaded(msg cqrs.Message, year string) error {
	var body VideoUploaded
	if err := msg.Body(&body); err != nil {
		return err
	}
	photoID := body.PhotoID
	if photoID == "" {
		photoID = subjectEntityID(msg.Subject())
	}
	if photoID == "" {
		return fmt.Errorf("video uploaded with no photoId")
	}
	// Refused, not blanked: without its original a video row would be a placeholder nothing can ever fill.
	if !validRef(body.Original.Ref) {
		return fmt.Errorf("video uploaded with an invalid original ref")
	}
	if body.UploadedAt.IsZero() {
		return fmt.Errorf("video uploaded with no uploadedAt")
	}

	// # A re-upload changes nothing but the file name
	//
	// The id is the original's hash, so a duplicate is the same clip. Its status, renditions and failure are
	// the worker's to decide, and a re-upload resetting a `ready` video to `processing` would take it out of
	// every published album until a transcode nobody needed had finished. `deleted` is left alone for the
	// reason table.sql gives.
	return c.w.Consume(fmt.Sprintf(
		"INSERT INTO photo SET photoId=%s, year=%s, kind=\"video\", status=\"processing\", "+
			"originalRef=%s, originalContentType=%s, originalBytes=%d, originalWidth=%d, originalHeight=%d, "+
			"durationMs=%d, caption=\"\", credit=\"\", boundsVerdict=%s, fileName=%s, uploadedAt=%s "+
			"ON DUPLICATE KEY UPDATE "+
			"fileName=IF(VALUES(fileName)=\"\", fileName, VALUES(fileName))",
		quote(photoID), quote(year),
		quote(body.Original.Ref), quote(body.Original.ContentType), body.Original.Bytes,
		body.Original.Width, body.Original.Height,
		body.DurationMs, quote(BoundsNone), quote(NormalizeFileName(body.FileName)),
		quote(formatTime(body.UploadedAt)),
	))
}

func (c consumer) handleVideoTranscoded(msg cqrs.Message, year string) error {
	var body VideoTranscoded
	if err := msg.Body(&body); err != nil {
		return err
	}
	photoID := body.PhotoID
	if photoID == "" {
		photoID = subjectEntityID(msg.Subject())
	}
	if photoID == "" {
		return fmt.Errorf("video transcoded with no photoId")
	}
	// The 720p rendition is the event; without it there is nothing to say.
	if !validRef(body.Ref) {
		return fmt.Errorf("video transcoded with an invalid ref")
	}
	// The rest cost themselves, not the video, as a bad thumbnail ref does on upload.
	blank := func(r string) string {
		if r != "" && !validRef(r) {
			return ""
		}
		return r
	}

	// An UPDATE, and scoped to `kind="video"`: this event must not be able to conjure a row, nor turn a
	// photograph into something that plays.
	return c.w.Consume(fmt.Sprintf(
		"UPDATE photo SET status=\"ready\", failReason=\"\", blobRef=%s, videoRef=%s, videoSdRef=%s, "+
			"thumbRef=%s, mediumRef=%s, width=%d, height=%d, bytes=%d, "+
			"durationMs=IF(%d=0, durationMs, %d) "+
			"WHERE photoId=%s AND year=%s AND kind=\"video\"",
		quote(body.Ref), quote(body.Ref), quote(blank(body.SdRef)),
		quote(blank(body.ThumbRef)), quote(blank(body.MediumRef)), body.Width, body.Height, body.Bytes,
		body.DurationMs, body.DurationMs,
		quote(photoID), quote(year),
	))
}

func (c consumer) handleVideoFailed(msg cqrs.Message, year string) error {
	var body VideoFailed
	if err := msg.Body(&body); err != nil {
		return err
	}
	photoID := body.PhotoID
	if photoID == "" {
		photoID = subjectEntityID(msg.Subject())
	}
	if photoID == "" {
		return fmt.Errorf("video failed with no photoId")
	}

	// Only a video still `processing` can fail. A rebuild that fails after a cache miss must not take a
	// published video down: its renditions are what failed to come back, and the queued event that preceded
	// it is what put it in `processing` if that was ever right.
	return c.w.Consume(fmt.Sprintf(
		"UPDATE photo SET status=\"failed\", failReason=%s "+
			"WHERE photoId=%s AND year=%s AND kind=\"video\" AND status=\"processing\"",
		quote(truncateRunes(body.Reason, 255)), quote(photoID), quote(year),
	))
}

func (c consumer) handleVideoQueued(msg cqrs.Message, year string) error {
	var body VideoQueued
	if err := msg.Body(&body); err != nil {
		return err
	}
	photoID := body.PhotoID
	if photoID == "" {
		photoID = subjectEntityID(msg.Subject())
	}
	if photoID == "" {
		return fmt.Errorf("video queued with no photoId")
	}
	return c.w.Consume(fmt.Sprintf(
		"UPDATE photo SET status=\"processing\", failReason=\"\" "+
			"WHERE photoId=%s AND year=%s AND kind=\"video\"",
		quote(photoID), quote(year),
	))
}
