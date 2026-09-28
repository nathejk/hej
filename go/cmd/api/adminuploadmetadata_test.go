package main

import (
	"strings"
	"testing"
	"time"

	"nathejk.dk/internal/eventtime"
	"nathejk.dk/nathejk/table/photo"
)

// The capture time on an upload (task 441, PRD 024 §6 R3).
//
// # Why this is a test about ordering, not about a field
//
// The value exists so an album can be sorted by when the photographs were taken. What makes that possible is
// not the column — it is that the read happens **before** `imaging.Prepare` re-encodes the bytes, because
// after that there is nothing left to read. That ordering is the whole feature, it is invisible in a diff,
// and it is exactly what a later "tidy-up" of `storeAlbumImage` could reverse while every other test kept
// passing. The GPS fix has the same property and the same guard (PRD 022 §8.4).

func TestTheUploadReadsTheCaptureTimeBeforeReEncoding(t *testing.T) {
	src := adminSource(t, "albummedia.go")

	read := strings.Index(src, "imaging.ReadShotAt(raw")
	prepare := strings.Index(src, "imaging.Prepare(raw")
	if read < 0 || prepare < 0 {
		t.Fatal("could not find both the capture-time read and the re-encode; this guard needs updating")
	}
	if read > prepare {
		t.Error("the capture time must be read before the bytes are re-encoded, or there is nothing left " +
			"to read: Prepare re-encodes from pixels and destroys all EXIF")
	}

	// And it is read in the event's own timezone, not the server's. EXIF carries no offset, so a UTC reading
	// of a photograph taken at 23:41 on a September night lands it on the previous day — which is the one
	// case that matters, because that is when Nathejk photographs are taken.
	if !strings.Contains(src, "imaging.ReadShotAt(raw, eventtime.Location())") {
		t.Error("the capture time must be read in the event's timezone; EXIF carries none")
	}
}

// A photograph whose file said nothing is not a photograph with a wrong time. The fold writes NULL, and
// every reader falls back to `uploadedAt` — PRD 024 §6 R3, and the reason the column is NULL-able.
func TestTheUploadedEventLeavesTheCaptureTimeUnsetWhenTheFileDidNotSay(t *testing.T) {
	var e photo.Uploaded
	if e.ShotAt != nil {
		t.Error("the zero event must have no capture time")
	}

	// A pointer rather than a zero time, so that "the file did not say" cannot be confused with an instant.
	// Asserted structurally because the distinction is the only reason the field is a pointer, and a future
	// change to `time.Time` would compile, pass everything else, and silently write year 1 into the column.
	shot := time.Date(2026, 9, 12, 23, 41, 7, 0, eventtime.Location())
	e.ShotAt = &shot
	if e.ShotAt == nil || !e.ShotAt.Equal(shot) {
		t.Error("the capture time must survive being set")
	}
}

// The fold must fill a gap and never open one (see handleUploaded).
//
// Re-dragging a card is the documented recovery procedure when a batch half-failed (task 372), and the id is
// the hash of the *stored rendition* — so the same photoId can arrive from a different file: the same pixels
// with the EXIF stripped, or re-saved by an editor. A plain `VALUES(shotAt)` would let that second file blank
// a capture time the first one supplied, and an album sorted by time would reorder underneath the curator for
// a photograph nobody meant to touch.
//
// Source-read because the fold builds a statement rather than executing one here; the statement is the
// behaviour. Comments stripped, since the paragraph above this in consumer.go names the thing being checked.
func TestTheUploadFoldNeverClearsACaptureTimeItAlreadyHas(t *testing.T) {
	src := stripGoComments(adminSource(t, "../../nathejk/table/photo/consumer.go"))

	if !strings.Contains(src, `"shotAt=COALESCE(VALUES(shotAt), shotAt), "`) {
		t.Error("a re-upload must not blank a capture time an earlier file supplied: a photograph can " +
			"legitimately arrive twice, the second time from a file with its EXIF stripped")
	}
	// The plain form is the bug this replaces, so it must not also be present.
	if strings.Contains(src, "shotAt=VALUES(shotAt)") {
		t.Error("shotAt is written with a plain VALUES(), which lets a stripped re-upload clear it")
	}
	// The insert still lists both, or a first arrival would never store either.
	if !strings.Contains(src, `"shotAt=%s, fileName=%s, "`) {
		t.Error("the insert must set shotAt and fileName, or the values only ever arrive on a duplicate")
	}
}

// PRD 022 §6's second exception, decided in task 448: the library keeps the name the photographer's file
// had, so an album can keep the order the card was in on their own computer.
//
// # What is worth testing about it
//
// Not that the field exists — the compiler covers that. What the exception **rests on** is worth testing,
// because those are the parts a later change can quietly remove while the feature keeps working:
//
//   - it is normalised before it is published, because the log is permanent and a projection cannot fix it;
//   - it goes onto the event and no further; nothing derives a person from it.
//
// The other bound — that it never reaches a public read — is held by `isPersonShaped` still flagging the
// word, and by `TestTheFilenameExceptionStopsAtTheAdminSurface`. It is deliberately not re-asserted here:
// one place per rule.
func TestTheUploadKeepsTheFilenameBounded(t *testing.T) {
	src := stripGoComments(adminSource(t, "adminupload.go"))

	if !strings.Contains(src, "FileName:   photo.NormalizeFileName(fileName),") {
		t.Error("the filename must be normalised at the point of publication: a value longer than the column " +
			"is a write MariaDB truncates or refuses depending on its mode, and the log cannot be corrected")
	}

	// `NormalizeFileName` is where every bound lives, so it must be the only thing that touches the value.
	// A second treatment here would be a second place for the rules to drift.
	for _, forbidden := range []struct{ needle, why string }{
		{"strings.Split(fileName", "a filename is never parsed for meaning (PRD 024 §6 R5)"},
		{"filepath.Ext(fileName", "nor read for an extension"},
		{"models.People", "and emphatically never joined to the person projection"},
	} {
		if strings.Contains(src, forbidden.needle) {
			t.Errorf("adminupload.go does something with the filename beyond storing it (%q): %s",
				forbidden.needle, forbidden.why)
		}
	}
}

// The fold applies the same bounding again, and that is not belt-and-braces for its own sake: it does not get
// to assume the publisher was this version of the publisher. An event written by an older binary, or by a
// future one with a bug, is still folded by this code.
func TestTheUploadFoldBoundsTheFilenameItself(t *testing.T) {
	src := stripGoComments(adminSource(t, "../../nathejk/table/photo/consumer.go"))

	if !strings.Contains(src, "quote(NormalizeFileName(body.FileName))") {
		t.Error("the fold must normalise the filename rather than trusting the event: a 300-character value " +
			"is the class of bug task 352 shipped and task 350 exists to prevent")
	}
	if !strings.Contains(src, `"fileName=IF(VALUES(fileName)=\"\", fileName, VALUES(fileName)), "`) {
		t.Error("a re-upload that carries no name must not blank the one already stored: the raw-body path " +
			"has no filename at all, and re-dragging a card is the documented recovery procedure (task 372)")
	}
}
