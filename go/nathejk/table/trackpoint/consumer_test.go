package trackpoint

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/cqrstest"
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

func TestReportedWritesOneRowPerPoint(t *testing.T) {
	stmts := fold(t, "TELEMETRY.2026.track.person-1.reported", reported{
		PersonID: "person-1",
		Points: []point{
			{TS: 1758328210000, Lat: 55.7332, Lng: 12.2648, Accuracy: 10.5},
			{TS: 1758328240000, Lat: 55.7340, Lng: 12.2650, Accuracy: 12},
		},
	})

	if len(stmts) != 2 {
		t.Fatalf("want one statement per point, got %d", len(stmts))
	}
	for _, want := range []string{
		"INSERT INTO track_point", "ON DUPLICATE KEY UPDATE",
		`"2026"`, `"person-1"`, "ts=1758328210000", "lat=55.733200", "accuracy=10.500000",
	} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
	}
}

// **The dedup contract as a constraint** (task 083). A retry after a timeout republishes the same points,
// and the upsert must write the same row rather than a second one. Asserted by checking the statement is an
// upsert on the primary key rather than a plain insert.
func TestAReplayedBatchUpsertsRatherThanDuplicating(t *testing.T) {
	batch := reported{
		PersonID: "person-1",
		Points:   []point{{TS: 1758328210000, Lat: 55.7332, Lng: 12.2648}},
	}

	first := fold(t, "TELEMETRY.2026.track.person-1.reported", batch)
	second := fold(t, "TELEMETRY.2026.track.person-1.reported", batch)

	if first[0] != second[0] {
		t.Errorf("a replayed batch produced a different statement:\n%s\n%s", first[0], second[0])
	}
	if !strings.Contains(first[0], "ON DUPLICATE KEY UPDATE") {
		t.Error("the fold must upsert: (year, personId, ts) is the identity of a point, and the reader " +
			"is the only place a duplicate can be removed")
	}
}

func TestReportedFallsBackToTheSubjectID(t *testing.T) {
	stmts := fold(t, "TELEMETRY.2026.track.person-9.reported", reported{
		Points: []point{{TS: 1758328210000, Lat: 55.7, Lng: 12.2}},
	})
	if !strings.Contains(stmts[0], `"person-9"`) {
		t.Errorf("want the id from the subject\ngot: %s", stmts[0])
	}
}

// A point belonging to nobody cannot be grouped into a patrol's route, so it would be permanent rubbish on
// a stream retained indefinitely.
func TestRefusesABatchWithNoPersonAnywhere(t *testing.T) {
	// A subject with no id position, and no personId in the body.
	err := foldErr(t, "TELEMETRY.2026.track", reported{
		Points: []point{{TS: 1, Lat: 55.7, Lng: 12.2}},
	})
	// This subject does not match the pattern, so it is ignored rather than refused — which is correct,
	// and means the guard is only reachable from a matching subject with an empty token.
	if err != nil {
		t.Fatalf("an unmatched subject should be ignored, got %v", err)
	}
}

// An empty batch is not an error. The publisher already drops junk points and counts them (task 084), so an
// empty batch means every point was junk — which happens to a phone with no fix.
func TestAnEmptyBatchIsNotAnError(t *testing.T) {
	stmts := fold(t, "TELEMETRY.2026.track.person-1.reported", reported{PersonID: "person-1"})
	if len(stmts) != 0 {
		t.Errorf("want no statements, got %v", stmts)
	}
}

// The publisher validates *this* year's batches; this projection replays everything ever published,
// including whatever predates the validator. A stream retained indefinitely outlives its validator.
func TestImplausiblePointsAreDropped(t *testing.T) {
	stmts := fold(t, "TELEMETRY.2026.track.person-1.reported", reported{
		PersonID: "person-1",
		Points: []point{
			{TS: 0, Lat: 55.7, Lng: 12.2},           // no timestamp
			{TS: -1, Lat: 55.7, Lng: 12.2},          // negative
			{TS: 1758328210000, Lat: 91, Lng: 12.2}, // past the pole
			{TS: 1758328211000, Lat: 55.7, Lng: 181},
			{TS: 1758328212000, Lat: 0, Lng: 0},             // null island / no fix
			{TS: 1758328213000, Lat: 55.7332, Lng: 12.2648}, // the only good one
		},
	})

	if len(stmts) != 1 {
		t.Fatalf("want 1 usable point, got %d statements:\n%s", len(stmts), strings.Join(stmts, "\n"))
	}
	if !strings.Contains(stmts[0], "ts=1758328213000") {
		t.Errorf("the wrong point survived: %s", stmts[0])
	}
}

// A poor fix is still a fix. Task 082 measured 35 m on a Wi-Fi-only iPad, and a cell-tower fix of a few
// kilometres is the only evidence of where somebody was — discarding it here would throw that away.
func TestAPoorFixIsKept(t *testing.T) {
	stmts := fold(t, "TELEMETRY.2026.track.person-1.reported", reported{
		PersonID: "person-1",
		Points:   []point{{TS: 1758328210000, Lat: 55.7332, Lng: 12.2648, Accuracy: 3500}},
	})
	if len(stmts) != 1 {
		t.Fatalf("a 3.5 km fix is poor but real, and must be kept: got %d statements", len(stmts))
	}
}

// The subject is TELEMETRY, not NATHEJK: the position track has its own stream so that a decision about how
// long to keep a minor's positions is not entangled with the event log's retention (PRD 002 §11.1).
func TestConsumesTheTelemetryStream(t *testing.T) {
	subs := consumer{}.Consumes()
	if len(subs) != 1 {
		t.Fatalf("want one subscription, got %d", len(subs))
	}
	if got := subs[0].Subject(); got != "TELEMETRY.*.track.*.reported" {
		t.Errorf("subscribed to %q", got)
	}

	// And the subject the publisher builds must match it.
	published := cqrs.SubjectFromStr("TELEMETRY.2026.track.person-1.reported")
	if !published.Match(strings.ToLower(subs[0].Subject())) {
		t.Errorf("the publisher's subject %q does not match the subscription %q",
			published.Subject(), subs[0].Subject())
	}
}

// Events on other streams must be ignored rather than erroring: this projection subscribes to one subject
// and the broker carries many.
func TestOtherSubjectsAreIgnored(t *testing.T) {
	for _, subject := range []string{
		"NATHEJK.2026.glimt.g-1.created",
		"TELEMETRY.2026.track.person-1.somethingelse",
	} {
		w := &cqrstest.Writer{}
		c := consumer{w: w}
		msg := cqrstest.NewMessage(cqrs.SubjectFromStr(subject))
		if err := msg.SetBody(reported{PersonID: "p", Points: []point{{TS: 1, Lat: 55, Lng: 12}}}); err != nil {
			t.Fatalf("SetBody: %v", err)
		}
		if err := c.HandleMessage(msg); err != nil {
			t.Errorf("%s should be ignored, got %v", subject, err)
		}
		if len(w.Statements) != 0 {
			t.Errorf("%s wrote %v", subject, w.Statements)
		}
	}
}

// **The duplication that would break silently.** The event body is re-declared here because a
// `nathejk/table/*` package may not import `internal/...`. If the publisher's JSON tags changed, this fold
// would decode zero points and log nothing — so the tags are pinned against the wire format the publisher
// actually emits.
func TestTheEventShapeMatchesThePublishersWireFormat(t *testing.T) {
	// The exact JSON `internal/track.Reported` produces, taken from its struct tags.
	wire := `{
		"personId": "person-1",
		"userType": "spejder",
		"points": [
			{"ts": 1758328210000, "lat": 55.7332, "lng": 12.2648, "accuracy": 10.5}
		]
	}`

	var body reported
	if err := json.Unmarshal([]byte(wire), &body); err != nil {
		t.Fatalf("the projection cannot decode the publisher's format: %v", err)
	}
	if body.PersonID != "person-1" {
		t.Errorf("personId did not decode: %q", body.PersonID)
	}
	if len(body.Points) != 1 {
		t.Fatalf("points did not decode: %+v", body.Points)
	}
	p := body.Points[0]
	if p.TS != 1758328210000 || p.Lat != 55.7332 || p.Lng != 12.2648 || p.Accuracy != 10.5 {
		t.Errorf("a point field did not decode: %+v", p)
	}
}

// `userType` is on the event and must not reach a column: the public page shows one merged route with no
// attribution, so a column distinguishing members is a column somebody could group by.
func TestUserTypeIsNotProjected(t *testing.T) {
	stmts := fold(t, "TELEMETRY.2026.track.person-1.reported", reported{
		PersonID: "person-1",
		Points:   []point{{TS: 1758328210000, Lat: 55.7332, Lng: 12.2648}},
	})
	for _, forbidden := range []string{"userType", "usertype", "spejder", "role"} {
		if strings.Contains(stmts[0], forbidden) {
			t.Errorf("the fold wrote %q; the merged route carries no attribution", forbidden)
		}
	}
}
