package photo

import (
	"strings"
	"testing"
)

// The video folds (PRD 029, task 493).

func TestVideoUploadedInsertsAProcessingVideo(t *testing.T) {
	stmts := fold(t, "PHOTO.2026.photo."+hash("a")+".videouploaded", VideoUploaded{
		PhotoID: hash("a"), Year: "2026",
		Original:   Original{Ref: ref("a"), ContentType: "video/quicktime", Bytes: 3_100_000_000, Width: 3840, Height: 2160},
		DurationMs: 22 * 60 * 1000, FileName: "/Users/x/IMG_0001.MOV", UploadedAt: at,
	})
	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	for _, want := range []string{`kind="video"`, `status="processing"`, `originalRef="` + ref("a") + `"`,
		`durationMs=1320000`, `fileName="IMG_0001.MOV"`} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("missing %s\n%s", want, stmts[0])
		}
	}
}

// A re-upload is the same clip. Resetting it to processing would pull a ready video out of every published album.
func TestVideoReuploadTouchesNothingButTheFileName(t *testing.T) {
	stmts := fold(t, "PHOTO.2026.photo."+hash("a")+".videouploaded", VideoUploaded{
		PhotoID: hash("a"), Year: "2026", Original: Original{Ref: ref("a")}, UploadedAt: at,
	})
	update := stmts[0][strings.Index(stmts[0], "ON DUPLICATE KEY UPDATE"):]
	for _, col := range []string{"status", "blobRef", "videoRef", "deleted", "originalRef", "kind"} {
		if strings.Contains(update, col+"=") {
			t.Errorf("the re-upload branch writes %s\n%s", col, update)
		}
	}
}

func TestVideoUploadedRefusesAMissingOriginal(t *testing.T) {
	if err := foldErr(t, "PHOTO.2026.photo."+hash("a")+".videouploaded", VideoUploaded{
		PhotoID: hash("a"), Year: "2026", Original: Original{Ref: "../../etc/passwd"}, UploadedAt: at,
	}); err == nil {
		t.Error("an invalid original ref was accepted")
	}
}

func TestVideoTranscodedMakesItReady(t *testing.T) {
	stmts := fold(t, "PHOTO.2026.photo."+hash("a")+".videotranscoded", VideoTranscoded{
		PhotoID: hash("a"), Year: "2026", Ref: ref("b"), SdRef: ref("c"), ThumbRef: ref("d"), MediumRef: "bad",
		Width: 1280, Height: 720, Bytes: 400_000_000, DurationMs: 1_320_000, TranscodedAt: at,
	})
	s := stmts[0]
	if !strings.HasPrefix(s, "UPDATE photo") {
		t.Errorf("the transcode fold must not be able to create a row\n%s", s)
	}
	for _, want := range []string{`status="ready"`, `blobRef="` + ref("b") + `"`, `videoRef="` + ref("b") + `"`,
		`videoSdRef="` + ref("c") + `"`, `thumbRef="` + ref("d") + `"`, `mediumRef=""`, `kind="video"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s\n%s", want, s)
		}
	}
}

func TestVideoTranscodedRefusesAnInvalidRef(t *testing.T) {
	if err := foldErr(t, "PHOTO.2026.photo."+hash("a")+".videotranscoded", VideoTranscoded{
		PhotoID: hash("a"), Year: "2026", Ref: "", TranscodedAt: at,
	}); err == nil {
		t.Error("a transcode with no rendition was accepted")
	}
}

// A failed rebuild must not take a published video down.
func TestVideoFailedOnlyFailsAProcessingVideo(t *testing.T) {
	stmts := fold(t, "PHOTO.2026.photo."+hash("a")+".videofailed", VideoFailed{
		PhotoID: hash("a"), Year: "2026", Reason: strings.Repeat("x", 400), FailedAt: at,
	})
	if !strings.Contains(stmts[0], `status="processing"`) || !strings.Contains(stmts[0], `status="failed"`) {
		t.Errorf("want failed, guarded on processing\n%s", stmts[0])
	}
	if strings.Contains(stmts[0], strings.Repeat("x", 256)) {
		t.Error("the reason was not truncated to the column")
	}
}

func TestVideoQueuedReturnsItToProcessing(t *testing.T) {
	stmts := fold(t, "PHOTO.2026.photo."+hash("a")+".videoqueued", VideoQueued{
		PhotoID: hash("a"), Year: "2026", QueuedAt: at,
	})
	if !strings.Contains(stmts[0], `status="processing", failReason=""`) || !strings.Contains(stmts[0], `kind="video"`) {
		t.Errorf("unexpected statement\n%s", stmts[0])
	}
}
