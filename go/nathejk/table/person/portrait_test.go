package person

import (
	"strings"
	"testing"
	"time"
)

const testRef = "3b1f8c2d4e5a6b7c8d9e0f1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e"

func TestPortraitSubjectShape(t *testing.T) {
	s, err := PortraitSubject("2026", "member-1")
	if err != nil {
		t.Fatalf("PortraitSubject: %v", err)
	}
	if got := s.Subject(); got != "NATHEJK.2026.portrait.member-1.captured" {
		t.Errorf("subject = %q", got)
	}
	// The subject the projection subscribes to must actually match the one it
	// publishes. These are two strings in two files, so nothing but a test connects
	// them — and a subject that does not match is completely silent: the projection
	// just stays empty.
	if !s.Match("nathejk.*.portrait.*.captured") {
		t.Errorf("published subject %q does not match the consumed pattern", s.Subject())
	}
}

// An id or year that would split into extra subject tokens must be refused, not
// published: it would still match NATHEJK.> (so the publish succeeds) while no longer
// matching the per-person purge pattern, quietly making that portrait unerasable.
func TestPortraitSubjectRejectsBadTokens(t *testing.T) {
	for _, tc := range []struct{ year, person string }{
		{"", "member-1"},
		{"2026", ""},
		{"2026", "member.1"},
		{"2026", "member *"},
		{"2026", "member>1"},
		{"20 26", "member-1"},
	} {
		if _, err := PortraitSubject(tc.year, tc.person); err == nil {
			t.Errorf("PortraitSubject(%q, %q) = nil error, want a refusal", tc.year, tc.person)
		}
	}
}

func TestPortraitCapturedSetsRefAndTimestamp(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.portrait.member-1.captured",
		PortraitCaptured{
			PersonID:    "member-1",
			Year:        "2026",
			Ref:         testRef,
			ContentType: "image/jpeg",
			Bytes:       12345,
			Width:       1024,
			Height:      1024,
			CapturedAt:  time.Date(2026, 8, 28, 21, 30, 0, 0, time.UTC),
		}))

	// An UPDATE, not an upsert: a portrait event must not invent a person.
	if !strings.HasPrefix(stmt, "UPDATE person SET portraitRef=") {
		t.Fatalf("statement = %q", stmt)
	}
	for _, want := range []string{testRef, "2026-08-28 21:30:00", "member-1"} {
		if !strings.Contains(stmt, want) {
			t.Errorf("statement missing %q: %s", want, stmt)
		}
	}
}

// A timestamp in another zone must be stored as UTC, or the retention job compares
// local wall-clock times against UTC ones and purges up to two hours early or late.
func TestPortraitCapturedStoresUTC(t *testing.T) {
	loc := time.FixedZone("CEST", 2*60*60)
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.portrait.member-1.captured",
		PortraitCaptured{
			PersonID:   "member-1",
			Ref:        testRef,
			CapturedAt: time.Date(2026, 8, 28, 23, 30, 0, 0, loc),
		}))
	if !strings.Contains(stmt, "2026-08-28 21:30:00") {
		t.Errorf("timestamp not converted to UTC: %s", stmt)
	}
}

// Replay safety: the same event applied twice must produce identical SQL, which for an
// absolute UPDATE means the second application is a no-op on the row.
func TestPortraitCapturedIsReplaySafe(t *testing.T) {
	body := PortraitCaptured{
		PersonID:   "member-1",
		Ref:        testRef,
		CapturedAt: time.Date(2026, 8, 28, 21, 30, 0, 0, time.UTC),
	}
	first := onlyStatement(t, mustHandle(t, "NATHEJK.2026.portrait.member-1.captured", body))
	second := onlyStatement(t, mustHandle(t, "NATHEJK.2026.portrait.member-1.captured", body))
	if first != second {
		t.Errorf("replay produced different SQL:\n%s\n%s", first, second)
	}
}

// Replacing a portrait is another captured event with a different ref; the latest wins.
func TestPortraitCapturedReplacesTheRef(t *testing.T) {
	newer := strings.Repeat("a", 64)
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.portrait.member-1.captured",
		PortraitCaptured{PersonID: "member-1", Ref: newer}))
	if !strings.Contains(stmt, newer) {
		t.Errorf("statement should write the new ref: %s", stmt)
	}
}

// A ref that is not a content hash must fail loudly. Writing it would leave the row
// claiming a portrait that no blob can satisfy, and every read would then degrade to
// "no photo" while the row insisted otherwise — a disagreement nobody would notice.
func TestPortraitCapturedRejectsNonHashRefs(t *testing.T) {
	for _, ref := range []string{
		"",
		"../../etc/passwd",
		strings.Repeat("a", 63),
		strings.Repeat("a", 65),
		strings.ToUpper(testRef),
		strings.Repeat("g", 64),
	} {
		if _, err := handle(t, "NATHEJK.2026.portrait.member-1.captured",
			PortraitCaptured{PersonID: "member-1", Ref: ref}); err == nil {
			t.Errorf("ref %q was accepted, want a refusal", ref)
		}
	}
}

// The person id may come from the subject when the body omits it, matching how the
// delete and team handlers already behave.
func TestPortraitCapturedFallsBackToTheSubjectId(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.portrait.member-7.captured",
		PortraitCaptured{Ref: testRef}))
	if !strings.Contains(stmt, "member-7") {
		t.Errorf("statement should key off the subject id: %s", stmt)
	}
}

// A missing timestamp writes NULL rather than time.Now(): "now" differs on every replay,
// which is the one thing the retention job must not depend on.
func TestPortraitCapturedWithoutTimestampWritesNull(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.portrait.member-1.captured",
		PortraitCaptured{PersonID: "member-1", Ref: testRef}))
	if !strings.Contains(stmt, "portraitCapturedAt=NULL") {
		t.Errorf("want a NULL timestamp, got: %s", stmt)
	}
}

func TestPortraitCapturedWritesTheThumbnailRef(t *testing.T) {
	thumb := strings.Repeat("b", 64)
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.portrait.member-1.captured",
		PortraitCaptured{PersonID: "member-1", Ref: testRef, ThumbRef: thumb}))
	if !strings.Contains(stmt, thumb) {
		t.Errorf("statement should carry the thumbnail ref: %s", stmt)
	}
}

// A malformed thumbnail ref costs the thumbnail, not the portrait: readers fall back to
// the full image, whereas failing the event would lose the photo entirely over a
// secondary artefact.
func TestPortraitCapturedDropsABadThumbnailRefButKeepsThePortrait(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.portrait.member-1.captured",
		PortraitCaptured{PersonID: "member-1", Ref: testRef, ThumbRef: "../../etc/passwd"}))
	if !strings.Contains(stmt, testRef) {
		t.Errorf("the portrait ref must still be written: %s", stmt)
	}
	if strings.Contains(stmt, "passwd") {
		t.Errorf("the bad thumbnail ref must not be written: %s", stmt)
	}
	if !strings.Contains(stmt, `portraitThumbRef=""`) {
		t.Errorf("want an empty thumbnail ref: %s", stmt)
	}
}

// An event from before thumbnails existed must still apply cleanly on replay.
func TestPortraitCapturedWithoutAThumbnailIsAccepted(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.portrait.member-1.captured",
		PortraitCaptured{PersonID: "member-1", Ref: testRef}))
	if !strings.Contains(stmt, `portraitThumbRef=""`) {
		t.Errorf("want an empty thumbnail ref: %s", stmt)
	}
}
