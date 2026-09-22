package patrolphoto

import (
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/cqrstest"
)

const (
	refA  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	refB  = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	thumb = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
)

func fold(t *testing.T, subject string, body any) []string {
	t.Helper()

	w := &cqrstest.Writer{}
	c := consumer{w: w}
	msg := cqrstest.NewMessage(cqrs.SubjectFromStr(subject))
	if err := msg.SetBody(body); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	if err := c.HandleMessage(msg); err != nil {
		t.Fatalf("HandleMessage(%s): %v", subject, err)
	}
	return w.Statements
}

func foldErr(t *testing.T, subject string, body any) error {
	t.Helper()

	w := &cqrstest.Writer{}
	c := consumer{w: w}
	msg := cqrstest.NewMessage(cqrs.SubjectFromStr(subject))
	if err := msg.SetBody(body); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	return c.HandleMessage(msg)
}

func aPhotograph() photographed {
	return photographed{
		TeamID:      "team-42",
		Year:        "2026",
		Type:        "start",
		Ref:         refA,
		ContentType: "image/jpeg",
		Width:       1600,
		Height:      1200,
		Renditions: []rendition{
			{Name: "thumb256", Ref: thumb, ContentType: "image/jpeg", Bytes: 20_000, Width: 256, Height: 192},
			{Name: "display1600", Ref: refB, ContentType: "image/jpeg", Bytes: 400_000, Width: 1600, Height: 1200},
		},
		CapturedAt: time.Date(2026, time.September, 18, 19, 0, 0, 0, time.UTC),
	}
}

func TestPhotographedIsRecordedWithItsSmallestRendition(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.patrulje.team-42.photographed", aPhotograph())

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d: %v", len(stmts), stmts)
	}
	for _, want := range []string{
		"INSERT INTO patrol_photo", "ON DUPLICATE KEY UPDATE",
		`year="2026"`, `teamId="team-42"`, `type="start"`, "ref=" + `"` + refA + `"`,
		// The 256px rendition, not the 1600px one: smallest by width.
		"thumbRef=" + `"` + thumb + `"`,
		"width=1600", "height=1200", "attention=0",
		`capturedAt="2026-09-18 19:00:00"`,
	} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %q:\n%s", want, stmts[0])
		}
	}
}

// **The original and the source must reach no column.** foto keeps the uploaded file with its EXIF — possibly the
// location a child was photographed in — and never serves it. This app has no column for it, so the strongest
// available assertion is that the fold's SQL never mentions one even when the event carries it.
func TestTheOriginalAndTheSourceReachNoColumn(t *testing.T) {
	// A body with the fields this app does not declare, as JSON the way foto publishes it.
	raw := map[string]any{
		"teamId": "team-42", "year": "2026", "type": "start", "ref": refA,
		"contentType": "image/jpeg", "width": 1600, "height": 1200,
		"original": map[string]any{
			"ref":         refB,
			"contentType": "image/heic",
			"orientation": 6,
		},
		"source": map[string]any{
			"url":  "https://kamera.example.invalid/upload/12345.heic",
			"kind": "kamera-webhook",
		},
	}

	stmts := fold(t, "NATHEJK.2026.patrulje.team-42.photographed", raw)
	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	for _, forbidden := range []string{"original", "kamera.example.invalid", "heic", "source", "orientation"} {
		if strings.Contains(strings.ToLower(stmts[0]), forbidden) {
			t.Errorf("the fold wrote %q, which this projection must not hold:\n%s", forbidden, stmts[0])
		}
	}
}

// A ref is this row's identity, a blob-store key and later a URL segment. `../../etc/passwd` is a ref-shaped
// string, so a malformed one is refused rather than stored.
func TestAnInvalidRefIsRefused(t *testing.T) {
	for _, ref := range []string{"", "nope", "../../etc/passwd", strings.ToUpper(refA)} {
		body := aPhotograph()
		body.Ref = ref
		if err := foldErr(t, "NATHEJK.2026.patrulje.team-42.photographed", body); err == nil {
			t.Errorf("ref %q should have been refused", ref)
		}
	}
}

// A photograph with no renditions is a normal record from before rendition sets existed. It must store an empty
// thumbRef rather than an unusable one — the read falls back to the display ref.
func TestNoRenditionsMeansAnEmptyThumbRef(t *testing.T) {
	body := aPhotograph()
	body.Renditions = nil

	stmts := fold(t, "NATHEJK.2026.patrulje.team-42.photographed", body)
	if !strings.Contains(stmts[0], `thumbRef=""`) {
		t.Errorf("want an empty thumbRef:\n%s", stmts[0])
	}
}

// A rendition whose ref is malformed is skipped, not stored: a gallery reading it would build a broken URL, and
// empty has a defined fallback.
func TestAnInvalidRenditionRefIsSkipped(t *testing.T) {
	body := aPhotograph()
	body.Renditions = []rendition{{Name: "thumb256", Ref: "not-a-hash", Width: 256}}

	stmts := fold(t, "NATHEJK.2026.patrulje.team-42.photographed", body)
	if !strings.Contains(stmts[0], `thumbRef=""`) {
		t.Errorf("want the invalid rendition skipped:\n%s", stmts[0])
	}
}

// **A purge deletes exactly what it names, and clears a cover pointing at it.** This is the mechanism by which a
// request to remove a child's photograph is honoured, so both halves matter: a cover still naming a purged ref
// would leave the diploma asking for bytes that must no longer be shown.
func TestPurgeRemovesTheNamedPhotographsAndTheCover(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.patrulje.team-42.photopurged", photoPurged{
		TeamID: "team-42", Year: "2026", Refs: []string{refA},
	})

	if len(stmts) != 2 {
		t.Fatalf("want a delete and a cover clear, got %d: %v", len(stmts), stmts)
	}
	if !strings.Contains(stmts[0], "DELETE FROM patrol_photo") || !strings.Contains(stmts[0], refA) {
		t.Errorf("want the photograph deleted:\n%s", stmts[0])
	}
	if !strings.Contains(stmts[1], "patrol_photo_cover") || !strings.Contains(stmts[1], `ref=""`) {
		t.Errorf("want the cover cleared:\n%s", stmts[1])
	}
}

// **A purge naming nothing usable must delete nothing.** The caller who meant "this blurred one" would otherwise
// erase the patrol's whole season — so this errors rather than running an unbounded DELETE.
func TestAPurgeWithNoValidRefDeletesNothing(t *testing.T) {
	w := &cqrstest.Writer{}
	c := consumer{w: w}
	msg := cqrstest.NewMessage(cqrs.SubjectFromStr("NATHEJK.2026.patrulje.team-42.photopurged"))
	if err := msg.SetBody(photoPurged{TeamID: "team-42", Year: "2026", Refs: []string{"nope", ""}}); err != nil {
		t.Fatalf("SetBody: %v", err)
	}

	if err := c.HandleMessage(msg); err == nil {
		t.Error("a purge naming no valid ref should be an error")
	}
	if len(w.Statements) != 0 {
		t.Errorf("nothing should have been written, got %v", w.Statements)
	}
}

func TestCoverSelectionIsStoredAndCanBeCleared(t *testing.T) {
	chosen := fold(t, "NATHEJK.2026.patrulje.team-42.photocoverselected", coverSelected{
		TeamID: "team-42", Year: "2026", Ref: refA,
		SelectedAt: time.Date(2026, time.September, 20, 8, 0, 0, 0, time.UTC),
	})
	if !strings.Contains(chosen[0], "INSERT INTO patrol_photo_cover") || !strings.Contains(chosen[0], refA) {
		t.Errorf("want the choice stored:\n%s", chosen[0])
	}

	// An empty ref is a valid event: it is how a selection is undone.
	cleared := fold(t, "NATHEJK.2026.patrulje.team-42.photocoverselected", coverSelected{
		TeamID: "team-42", Year: "2026", Ref: "",
	})
	if !strings.Contains(cleared[0], `ref=""`) {
		t.Errorf("want the choice cleared:\n%s", cleared[0])
	}
}

// The ids come from the body, and from the subject when the body omits them — so a message that names neither is
// refused rather than written under an empty key, where no read could ever find it.
func TestTheSubjectSuppliesMissingIds(t *testing.T) {
	body := aPhotograph()
	body.TeamID, body.Year = "", ""

	stmts := fold(t, "NATHEJK.2026.patrulje.team-99.photographed", body)
	if !strings.Contains(stmts[0], `year="2026"`) || !strings.Contains(stmts[0], `teamId="team-99"`) {
		t.Errorf("want the subject's ids used:\n%s", stmts[0])
	}
}

// A capturedAt that is missing must be NULL, not a zero date: zero sorts as the oldest photograph ever taken,
// which is the wrong answer for a read that picks the newest.
func TestAMissingCaptureTimeIsNull(t *testing.T) {
	body := aPhotograph()
	body.CapturedAt = time.Time{}

	stmts := fold(t, "NATHEJK.2026.patrulje.team-42.photographed", body)
	if !strings.Contains(stmts[0], "capturedAt=NULL") {
		t.Errorf("want NULL:\n%s", stmts[0])
	}
}

// The subscription must not swallow the other patrulje verbs this stream carries — there are several, and a
// projection folding `updated` into a photo table would write nonsense on every replay.
func TestTheSubscriptionMatchesOnlyPhotographVerbs(t *testing.T) {
	c := consumer{}
	patterns := c.Consumes()

	for _, subject := range []string{
		"NATHEJK.2026.patrulje.team-42.updated",
		"NATHEJK.2026.patrulje.team-42.numberassigned",
		"NATHEJK.2026.updated",
	} {
		for _, p := range patterns {
			if cqrs.SubjectFromStr(subject).Match(p.Subject()) {
				t.Errorf("%q must not match %q", subject, p.Subject())
			}
		}
	}
	// And the three that must match, or the projection silently folds nothing.
	for _, subject := range []string{
		"NATHEJK.2026.patrulje.team-42.photographed",
		"NATHEJK.2026.patrulje.team-42.photopurged",
		"NATHEJK.2026.patrulje.team-42.photocoverselected",
	} {
		matched := false
		for _, p := range patterns {
			if cqrs.SubjectFromStr(subject).Match(p.Subject()) {
				matched = true
			}
		}
		if !matched {
			t.Errorf("%q matches none of the subscriptions", subject)
		}
	}
}
