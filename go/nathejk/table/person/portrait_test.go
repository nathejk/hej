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

func TestPortraitCapturedWritesEveryRendition(t *testing.T) {
	medium := strings.Repeat("b", 64)
	small := strings.Repeat("c", 64)

	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.portrait.member-1.captured",
		PortraitCaptured{
			PersonID: "member-1",
			Ref:      testRef,
			Thumbs: []PortraitThumb{
				{Name: "thumb256", Ref: medium, ContentType: "image/jpeg", Bytes: 20000, Width: 256, Height: 256},
				{Name: "thumb96", Ref: small, ContentType: "image/jpeg", Bytes: 4000, Width: 96, Height: 96},
			},
		}))

	// Every rendition's ref must reach the row, or the purge cannot find it later.
	for _, want := range []string{testRef, medium, small} {
		if !strings.Contains(stmt, want) {
			t.Errorf("statement missing ref %s: %s", want, stmt)
		}
	}
	// The sizes travel with them — that is the point of the list.
	for _, want := range []string{"thumb256", "thumb96", "20000", "4000"} {
		if !strings.Contains(stmt, want) {
			t.Errorf("statement missing %q: %s", want, stmt)
		}
	}
	// The denormalized default is the SMALLEST rendition, not the first: it is served
	// wherever "a thumbnail" is wanted, and the cheapest is the right default.
	if !strings.Contains(stmt, `portraitThumbRef="`+small+`"`) {
		t.Errorf("default thumbnail should be the smallest rendition: %s", stmt)
	}
}

// The stored JSON must be readable back into the same renditions — the projection writes
// it and the querier parses it, and nothing else connects those two.
func TestPortraitThumbsRoundTripThroughTheRow(t *testing.T) {
	thumbs := []PortraitThumb{
		{Name: "thumb256", Ref: strings.Repeat("b", 64), ContentType: "image/jpeg", Bytes: 20000, Width: 256, Height: 192},
	}
	encoded, err := encodeThumbs(thumbs)
	if err != nil {
		t.Fatalf("encodeThumbs: %v", err)
	}

	decoded := decodeThumbs(&encoded)
	if len(decoded) != 1 {
		t.Fatalf("decoded %d renditions, want 1", len(decoded))
	}
	if decoded[0] != thumbs[0] {
		t.Errorf("round trip changed the rendition:\n got %+v\nwant %+v", decoded[0], thumbs[0])
	}
}

// Unparseable JSON must not break a read: the consequence of returning no renditions is
// falling back to the full image, whereas an error here would fail a login.
func TestDecodeThumbsToleratesRubbish(t *testing.T) {
	for _, encoded := range []string{"", "not json", "{}", "["} {
		if got := decodeThumbs(&encoded); got != nil {
			t.Errorf("%q decoded to %+v, want nil", encoded, got)
		}
	}
	if got := decodeThumbs(nil); got != nil {
		t.Errorf("NULL decoded to %+v, want nil", got)
	}
}

// The original's ref reaches the row, and its orientation with it — the row is where a
// future re-render will look.
func TestPortraitCapturedRecordsTheOriginal(t *testing.T) {
	original := strings.Repeat("e", 64)
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.portrait.member-1.captured",
		PortraitCaptured{
			PersonID: "member-1",
			Ref:      testRef,
			Original: &PortraitOriginal{
				Ref: original, ContentType: "image/jpeg",
				Bytes: 2_400_000, Width: 3000, Height: 4000, Orientation: 6,
			},
		}))

	if !strings.Contains(stmt, `portraitOriginalRef="`+original+`"`) {
		t.Errorf("original ref not written: %s", stmt)
	}
	if !strings.Contains(stmt, "portraitOrientation=6") {
		t.Errorf("orientation not written: %s", stmt)
	}
}

// A malformed original ref costs the original, not the portrait: the consequence is that
// this one member cannot be backfilled later, which is much better than losing the photo.
func TestPortraitCapturedDropsABadOriginalRef(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.portrait.member-1.captured",
		PortraitCaptured{
			PersonID: "member-1",
			Ref:      testRef,
			Original: &PortraitOriginal{Ref: "../../etc/passwd", Orientation: 3},
		}))

	if strings.Contains(stmt, "passwd") {
		t.Errorf("the bad ref must not be written: %s", stmt)
	}
	if !strings.Contains(stmt, testRef) {
		t.Errorf("the portrait must survive: %s", stmt)
	}
	if !strings.Contains(stmt, `portraitOriginalRef=""`) {
		t.Errorf("want an empty original ref: %s", stmt)
	}
}

// An event with no original at all — keepOriginal off, an unstrippable format, or an event
// predating the field — must apply cleanly.
func TestPortraitCapturedWithoutAnOriginal(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.portrait.member-1.captured",
		PortraitCaptured{PersonID: "member-1", Ref: testRef}))
	if !strings.Contains(stmt, `portraitOriginalRef=""`) {
		t.Errorf("want an empty original ref: %s", stmt)
	}
	if !strings.Contains(stmt, "portraitOrientation=0") {
		t.Errorf("want an unknown orientation: %s", stmt)
	}
}

// PortraitRefs is what the purge deletes, so it must cover every object exactly once.
func TestPortraitRefsCoversEveryObjectOnce(t *testing.T) {
	medium := strings.Repeat("b", 64)
	small := strings.Repeat("c", 64)
	original := strings.Repeat("e", 64)
	p := Person{
		PortraitRef: testRef,
		// A copy of one of the renditions, as the projection denormalizes it.
		PortraitThumbRef: small,
		PortraitThumbs: []PortraitThumb{
			{Name: "thumb256", Ref: medium},
			{Name: "thumb96", Ref: small},
		},
		// The original must be deleted too: leaving it behind means a full-resolution
		// face still on disk after the record says the portrait is gone.
		PortraitOriginalRef: original,
	}

	got := p.PortraitRefs()
	if len(got) != 4 {
		t.Fatalf("refs = %v, want 4 distinct objects", got)
	}
	seen := map[string]int{}
	for _, ref := range got {
		seen[ref]++
	}
	for _, want := range []string{testRef, medium, small, original} {
		if seen[want] != 1 {
			t.Errorf("%s appears %d times, want exactly once", want, seen[want])
		}
	}

	// No portrait, nothing to delete.
	if refs := (Person{}).PortraitRefs(); refs != nil {
		t.Errorf("refs = %v, want none", refs)
	}
}

func TestThumbLooksUpByName(t *testing.T) {
	p := Person{PortraitThumbs: []PortraitThumb{{Name: "thumb256", Ref: testRef, Width: 256}}}
	if got, ok := p.Thumb("thumb256"); !ok || got.Width != 256 {
		t.Errorf("Thumb(thumb256) = %+v, %v", got, ok)
	}
	if _, ok := p.Thumb("thumb96"); ok {
		t.Error("a rendition this portrait does not have must not be found")
	}
}

// Events published before the list existed are on the log permanently. Their single
// thumbRef must still be found, or a replay silently loses the thumbnail.
func TestPortraitCapturedPromotesTheDeprecatedThumbRef(t *testing.T) {
	legacy := strings.Repeat("d", 64)
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.portrait.member-1.captured",
		PortraitCaptured{PersonID: "member-1", Ref: testRef, ThumbRef: legacy}))

	if !strings.Contains(stmt, legacy) {
		t.Errorf("the deprecated thumbRef must still be recorded: %s", stmt)
	}
	if !strings.Contains(stmt, `portraitThumbRef="`+legacy+`"`) {
		t.Errorf("it should also become the default thumbnail: %s", stmt)
	}
}

// The list wins when both shapes are present — it is the one with dimensions.
func TestPortraitCapturedPrefersTheListOverTheDeprecatedRef(t *testing.T) {
	listed := strings.Repeat("b", 64)
	legacy := strings.Repeat("d", 64)
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.portrait.member-1.captured",
		PortraitCaptured{
			PersonID: "member-1",
			Ref:      testRef,
			ThumbRef: legacy,
			Thumbs:   []PortraitThumb{{Name: "thumb256", Ref: listed, Width: 256, Height: 256}},
		}))

	if strings.Contains(stmt, legacy) {
		t.Errorf("the deprecated ref should be ignored when a list is present: %s", stmt)
	}
	if !strings.Contains(stmt, listed) {
		t.Errorf("statement should carry the listed rendition: %s", stmt)
	}
}

// A malformed rendition ref costs that rendition, not the portrait.
func TestPortraitCapturedDropsABadRenditionButKeepsTheRest(t *testing.T) {
	good := strings.Repeat("b", 64)
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.portrait.member-1.captured",
		PortraitCaptured{
			PersonID: "member-1",
			Ref:      testRef,
			Thumbs: []PortraitThumb{
				{Name: "thumb256", Ref: "../../etc/passwd", Width: 256},
				{Name: "thumb96", Ref: good, Width: 96},
			},
		}))

	if strings.Contains(stmt, "passwd") {
		t.Errorf("the bad ref must not be written: %s", stmt)
	}
	if !strings.Contains(stmt, testRef) || !strings.Contains(stmt, good) {
		t.Errorf("the portrait and the good rendition must survive: %s", stmt)
	}
}

// An event with no renditions at all must still apply cleanly on replay.
func TestPortraitCapturedWithoutAThumbnailIsAccepted(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.portrait.member-1.captured",
		PortraitCaptured{PersonID: "member-1", Ref: testRef}))
	if !strings.Contains(stmt, `portraitThumbRef=""`) {
		t.Errorf("want an empty default thumbnail: %s", stmt)
	}
	if !strings.Contains(stmt, `portraitThumbs=""`) {
		t.Errorf("want an empty rendition list: %s", stmt)
	}
}
