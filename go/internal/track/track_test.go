package track

import (
	"testing"
	"time"
)

var now = time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)

func ms(t time.Time) int64 { return t.UnixMilli() }

// A good point must survive Clean untouched. Without this the drop tests below could all
// pass against a Clean that discards everything.
func TestCleanKeepsPlausiblePoints(t *testing.T) {
	in := []Point{
		{TS: ms(now.Add(-time.Minute)), Lat: 55.7, Lng: 12.2, Accuracy: 7.2},
		{TS: ms(now.Add(-30 * time.Second)), Lat: 55.71, Lng: 12.21, Accuracy: 11.5},
		// Accuracy 0 is legal (some devices report it) and a poor cell fix is real data.
		{TS: ms(now), Lat: 55.72, Lng: 12.22, Accuracy: 0},
		{TS: ms(now), Lat: 55.72, Lng: 12.22, Accuracy: 4800},
	}
	kept, dropped := Clean(in, now)
	if dropped != 0 {
		t.Fatalf("dropped = %d, want 0", dropped)
	}
	if len(kept) != len(in) {
		t.Fatalf("kept %d points, want %d", len(kept), len(in))
	}
	if kept[0] != in[0] {
		t.Fatalf("point altered: got %+v, want %+v", kept[0], in[0])
	}
}

func TestCleanDropsImplausiblePoints(t *testing.T) {
	tests := []struct {
		name  string
		point Point
	}{
		{"zero timestamp", Point{TS: 0, Lat: 55.7, Lng: 12.2}},
		{"timestamp before 2020", Point{TS: ms(time.Date(2019, 12, 31, 23, 59, 0, 0, time.UTC)), Lat: 55.7, Lng: 12.2}},
		{"timestamp far in the future", Point{TS: ms(now.Add(48 * time.Hour)), Lat: 55.7, Lng: 12.2}},
		{"latitude out of range", Point{TS: ms(now), Lat: 91, Lng: 12.2}},
		{"latitude out of range negative", Point{TS: ms(now), Lat: -91, Lng: 12.2}},
		{"longitude out of range", Point{TS: ms(now), Lat: 55.7, Lng: 181}},
		{"null island", Point{TS: ms(now), Lat: 0, Lng: 0}},
		{"negative accuracy", Point{TS: ms(now), Lat: 55.7, Lng: 12.2, Accuracy: -1}},
		{"absurd accuracy", Point{TS: ms(now), Lat: 55.7, Lng: 12.2, Accuracy: 100_001}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kept, dropped := Clean([]Point{tc.point}, now)
			if dropped != 1 || len(kept) != 0 {
				t.Fatalf("kept %d / dropped %d, want 0 / 1 for %+v", len(kept), dropped, tc.point)
			}
		})
	}
}

// The behaviour that keeps one bad point from stalling a member's whole track: a mixed
// batch is partially accepted, not rejected. The client retries a rejected batch forever,
// so a poison pill would block every later point behind it.
func TestCleanKeepsGoodPointsFromAMixedBatch(t *testing.T) {
	in := []Point{
		{TS: ms(now.Add(-2 * time.Minute)), Lat: 55.70, Lng: 12.20, Accuracy: 8},
		{TS: 0, Lat: 55.71, Lng: 12.21},                    // junk
		{TS: ms(now.Add(-time.Minute)), Lat: 0, Lng: 0},    // junk
		{TS: ms(now), Lat: 55.72, Lng: 12.22, Accuracy: 9}, //
	}
	kept, dropped := Clean(in, now)
	if dropped != 2 {
		t.Fatalf("dropped = %d, want 2", dropped)
	}
	if len(kept) != 2 {
		t.Fatalf("kept = %d, want 2", len(kept))
	}
	// Order must be preserved: a track is a sequence, and reordering it would invent a
	// route nobody walked.
	if kept[0].Lat != 55.70 || kept[1].Lat != 55.72 {
		t.Fatalf("order not preserved: %+v", kept)
	}
}

// The subject is the erasure address (PRD 002 §11.1), so its exact shape is a contract with
// the operator running `nats stream purge --subject`, not an internal detail.
func TestSubjectShape(t *testing.T) {
	subj, err := Subject("2026", "b8825d84-c7d6-4e67-8836-73d6658a6c09")
	if err != nil {
		t.Fatalf("Subject: %v", err)
	}
	want := "TELEMETRY.2026.track.b8825d84-c7d6-4e67-8836-73d6658a6c09.reported"
	if got := subj.Subject(); got != want {
		t.Fatalf("subject = %q, want %q", got, want)
	}
	// The domain is what routes the message to the stream declared in the nathejk repo.
	if got := subj.Domain(); got != "TELEMETRY" {
		t.Fatalf("domain = %q, want TELEMETRY", got)
	}
}

// A person id that is not a single subject token must be refused rather than published.
// A dot would split into extra tokens: the publish would still succeed (it matches
// TELEMETRY.>), but the per-person purge pattern would no longer match, silently making
// that person's track unerasable.
func TestSubjectRejectsIdsThatAreNotOneToken(t *testing.T) {
	for _, bad := range []string{"", "a.b", "a b", "a>b", "a*b", "a\tb", "a\nb"} {
		if _, err := Subject("2026", bad); err == nil {
			t.Errorf("Subject(year, %q) = nil error, want rejection", bad)
		}
	}
	for _, bad := range []string{"", "20.26", "20 26", "*"} {
		if _, err := Subject(bad, "person-1"); err == nil {
			t.Errorf("Subject(%q, person) = nil error, want rejection", bad)
		}
	}
}
