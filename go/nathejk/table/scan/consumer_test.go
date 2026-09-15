package scan

import (
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/cqrstest"
	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
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

func single(t *testing.T, stmts []string) string {
	t.Helper()
	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d: %v", len(stmts), stmts)
	}
	return stmts[0]
}

func mustContain(t *testing.T, stmt string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(stmt, want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmt)
		}
	}
}

func TestScannedIsRecorded(t *testing.T) {
	body := messages.NathejkQrScanned{
		QrID:       "qr-17",
		TeamID:     "team-9",
		TeamNumber: "138",
		ScannerID:  "user-3",
	}
	body.Location.Latitude = "56.1382"
	body.Location.Longitude = "9.5521"

	stmt := single(t, fold(t, "NATHEJK.2026.qr.qr-17.scanned", body))

	mustContain(t, stmt, "INSERT IGNORE INTO scan",
		`"qr-17"`, `"2026"`, `"team-9"`, `"user-3"`, `"56.1382"`, `"9.5521"`)
}

// A scan is an immutable fact about a moment: INSERT IGNORE, not an upsert. A replay must not rewrite
// it, and there is nothing to update.
func TestScanIsInsertIgnoreNotUpsert(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.qr.qr-17.scanned",
		messages.NathejkQrScanned{QrID: "qr-17", TeamID: "team-9"}))

	if strings.Contains(stmt, "ON DUPLICATE KEY UPDATE") {
		t.Errorf("a scan must not be rewritten on replay\ngot: %s", stmt)
	}
	mustContain(t, stmt, "INSERT IGNORE")
}

// A scan with no team cannot appear in any patrol's list, but it is still stored: discarding it would
// remove the only evidence the scan happened, and an unattributable scan is exactly the diagnostic
// task 260 counts.
func TestScanWithoutATeamIsStillStored(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.qr.qr-17.scanned",
		messages.NathejkQrScanned{QrID: "qr-17"}))

	mustContain(t, stmt, "INSERT IGNORE INTO scan", `"qr-17"`)
}

// `qr.registered` is a handover and belongs to the maphandout projection. Consuming it here would show a
// patrol its map handouts as if they were post visits.
func TestRegisteredIsNotConsumedHere(t *testing.T) {
	subjects := make([]string, 0, 4)
	for _, s := range (consumer{}).Consumes() {
		subjects = append(subjects, s.Subject())
	}
	joined := strings.Join(subjects, " ")

	if strings.Contains(joined, "registered") {
		t.Errorf("qr.registered is a handout, not a scan: %s", joined)
	}
	for _, want := range []string{"qr.*.scanned", "checkpersonnel.*.added",
		"checkpersonnel.*.timespecified", "checkpersonnel.*.removed"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing subscription to %s: %s", want, joined)
		}
	}
}

func TestShiftAddedWithAWindow(t *testing.T) {
	start := time.Unix(1750000000, 0)
	end := time.Unix(1750010000, 0)

	stmt := single(t, fold(t, "NATHEJK.2026.checkpersonnel.shift-1.added",
		messages.NathejkCheckpersonnelAdded{
			UserID:       "user-3",
			CheckpointID: "cp-1",
			TimeRange:    &types.TimeRange{Start: start, End: end},
		}))

	mustContain(t, stmt, "INSERT INTO checkpersonnel", `"shift-1"`, `"user-3"`, `"cp-1"`,
		"1750000000", "1750010000")
}

// A shift added without hours is stored with a zero window, which matches no scan in the attribution
// join. So it attributes nothing — the safe direction. Guessing a window would attribute scans to a post
// on no evidence, and a wrong post name plus a wrong on-time verdict is worse for a patrol than a
// missing one.
func TestShiftWithoutAWindowAttributesNothing(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.checkpersonnel.shift-1.added",
		messages.NathejkCheckpersonnelAdded{UserID: "user-3", CheckpointID: "cp-1"}))

	for _, forbidden := range []string{"startUts", "endUts"} {
		if strings.Contains(stmt, forbidden) {
			t.Errorf("an hours-less shift must not write %s\ngot: %s", forbidden, stmt)
		}
	}
}

// `timespecified` may arrive before `added` on a replay, so the added handler is an upsert rather than an
// INSERT — otherwise the window set by the earlier event would be lost or the insert would fail.
func TestShiftAddedIsAnUpsert(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.checkpersonnel.shift-1.added",
		messages.NathejkCheckpersonnelAdded{UserID: "user-3", CheckpointID: "cp-1"}))

	mustContain(t, stmt, "ON DUPLICATE KEY UPDATE")
}

func TestShiftTimeSpecified(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.checkpersonnel.shift-1.timespecified",
		messages.NathejkCheckpersonnelTimeSpecified{
			Start: time.Unix(1750000000, 0),
			End:   time.Unix(1750010000, 0),
		}))

	mustContain(t, stmt, "UPDATE checkpersonnel SET startUts=1750000000, endUts=1750010000",
		`"shift-1"`)
}

// A hard DELETE, unlike the soft deletes elsewhere here: a removed shift must stop attributing scans
// immediately, and a flag would mean every attribution query had to remember to filter on it — the one
// that forgot would attribute scans to a post nobody was standing at.
func TestShiftRemovedIsAHardDelete(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.checkpersonnel.shift-1.removed", struct{}{}))

	mustContain(t, stmt, "DELETE FROM checkpersonnel", `"shift-1"`)
	if strings.Contains(stmt, "deleted=1") {
		t.Errorf("want a hard delete\ngot: %s", stmt)
	}
}

// The shift id is only in the subject — the bodies do not carry one — so the extraction is worth pinning
// directly. (A subject with an empty id segment does not match the subscription pattern at all, so it
// never reaches the handler; the guard there is for the day the pattern is widened.)
func TestShiftIDComesFromTheSubject(t *testing.T) {
	got := subjectEntityID(cqrs.SubjectFromStr("NATHEJK.2026.checkpersonnel.shift-1.removed"))
	if got != "shift-1" {
		t.Errorf("want shift-1, got %q", got)
	}
	if got := subjectEntityID(cqrs.SubjectFromStr("NATHEJK.2026")); got != "" {
		t.Errorf("want no id for a short subject, got %q", got)
	}
	if got := subjectYear(cqrs.SubjectFromStr("NATHEJK.2026.qr.qr-1.scanned")); got != "2026" {
		t.Errorf("want 2026, got %q", got)
	}
}

func TestStatementsAreIdempotent(t *testing.T) {
	body := messages.NathejkQrScanned{QrID: "qr-17", TeamID: "team-9", ScannerID: "user-3"}
	first := single(t, fold(t, "NATHEJK.2026.qr.qr-17.scanned", body))
	second := single(t, fold(t, "NATHEJK.2026.qr.qr-17.scanned", body))

	if first != second {
		t.Fatalf("folding the same event twice differed:\n%s\n%s", first, second)
	}
}

func TestUnrelatedSubjectIsIgnored(t *testing.T) {
	w := &cqrstest.Writer{}
	c := consumer{w: w}

	msg := cqrstest.NewMessage(cqrs.SubjectFromStr("NATHEJK.2026.spejder.member-1.updated"))
	if err := msg.SetBody(map[string]any{"memberId": "member-1"}); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	if err := c.HandleMessage(msg); err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if len(w.Statements) != 0 {
		t.Errorf("nothing should be written: %v", w.Statements)
	}
}
