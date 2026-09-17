package glimt

import (
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/cqrstest"
)

// ref builds a valid-looking content hash: 64 lowercase hex characters.
func ref(seed byte) string {
	return strings.Repeat(string([]byte{seed}), 64)
}

var (
	refA = ref('a')
	refB = ref('b')
	refC = ref('c')
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

func foldExpectingError(t *testing.T, subject string, body any) error {
	t.Helper()

	w := &cqrstest.Writer{}
	c := consumer{w: w}

	msg := cqrstest.NewMessage(cqrs.SubjectFromStr(subject))
	if err := msg.SetBody(body); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	err := c.HandleMessage(msg)
	if err == nil {
		t.Fatalf("HandleMessage(%s) = nil, want an error (statements: %v)", subject, w.Statements)
	}
	return err
}

func mustContain(t *testing.T, stmt string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(stmt, want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmt)
		}
	}
}

func createdBody() Created {
	return Created{
		GlimtID:        "g-1",
		Year:           "2026",
		AuthorPersonID: "p-1",
		AuthorGroup:    "spejder",
		TeamNumber:     "42",
		TeamName:       "Ørnene",
		Audience:       AudienceGroup,
		Caption:        "ved posten",
		Media: []Media{
			{Ordinal: 0, Ref: refA, ThumbRef: refB, Kind: MediaKindImage, Width: 1024, Height: 768},
		},
		CreatedAt: time.Date(2026, 9, 17, 21, 30, 0, 0, time.UTC),
	}
}

func TestConsumesEverySubject(t *testing.T) {
	// Six verbs, and the projection must listen for all of them. A verb published but not
	// consumed fails silently — the message is simply never delivered — so this is asserted
	// rather than assumed.
	want := []string{"created", "deleted", "reported", "hidden", "unhidden", "purged"}
	got := consumer{}.Consumes()
	if len(got) != len(want) {
		t.Fatalf("Consumes() has %d subjects, want %d: %v", len(got), len(want), got)
	}
	for i, verb := range want {
		s := got[i].Subject()
		if !strings.HasSuffix(strings.ToLower(s), "."+verb) {
			t.Errorf("subject %d = %q, want it to end in .%s", i, s, verb)
		}
		if !strings.HasPrefix(s, "NATHEJK.") {
			// Dot form, not the colon some upstream producers use — see Consumes.
			t.Errorf("subject %d = %q, want the NATHEJK. dot prefix", i, s)
		}
	}
}

func TestCreatedWritesTheGlimtAndItsMedia(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.glimt.g-1.created", createdBody())
	if len(stmts) != 3 {
		t.Fatalf("want 3 statements (glimt, media delete, media insert), got %d: %v", len(stmts), stmts)
	}

	mustContain(t, stmts[0], "INSERT INTO glimt SET",
		`glimtId="g-1"`, `year="2026"`, `authorPersonId="p-1"`, `authorGroup="spejder"`,
		`teamNumber="42"`, `audience="group"`, `createdAt="2026-09-17 21:30:00"`,
		"mediaCount=1", "deleted=0")

	// A create must not reset a takedown. On a re-delivery after a hide, writing zeros here
	// would silently restore a reported glimt to the public feed.
	for _, forbidden := range []string{"hiddenAt", "hiddenBy", "reportCount"} {
		if strings.Contains(stmts[0], forbidden) {
			t.Errorf("create touches %s, which later events own\ngot: %s", forbidden, stmts[0])
		}
	}

	mustContain(t, stmts[1], "DELETE FROM glimt_media", `glimtId="g-1"`)
	mustContain(t, stmts[2], "INSERT INTO glimt_media SET", "ordinal=0",
		`blobRef="`+refA+`"`, `thumbRef="`+refB+`"`, `kind="image"`)
}

func TestCreatedIsIdempotent(t *testing.T) {
	// The tables are rebuilt from sequence zero on every boot, so folding twice must produce
	// the same rows. Asserted through the statements: an upsert on the parent, and a
	// replace on the media list.
	first := fold(t, "NATHEJK.2026.glimt.g-1.created", createdBody())
	second := fold(t, "NATHEJK.2026.glimt.g-1.created", createdBody())

	if len(first) != len(second) {
		t.Fatalf("statement counts differ between folds: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("statement %d differs between folds:\n1: %s\n2: %s", i, first[i], second[i])
		}
	}
	mustContain(t, first[0], "ON DUPLICATE KEY UPDATE")
}

func TestCreatedTakesTheIdFromTheSubjectWhenTheBodyOmitsIt(t *testing.T) {
	body := createdBody()
	body.GlimtID = ""
	stmts := fold(t, "NATHEJK.2026.glimt.g-99.created", body)
	mustContain(t, stmts[0], `glimtId="g-99"`)
}

func TestCreatedOrdinalsComeFromTheEventNotTheIndex(t *testing.T) {
	// The author's arrangement is the display order. Renumbering by loop index would
	// silently reorder a set whose ordinals have a gap.
	body := createdBody()
	body.Media = []Media{
		{Ordinal: 5, Ref: refA, Kind: MediaKindImage},
		{Ordinal: 2, Ref: refB, Kind: MediaKindVideo, DurationMs: 12000},
	}
	stmts := fold(t, "NATHEJK.2026.glimt.g-1.created", body)
	if len(stmts) != 4 {
		t.Fatalf("want 4 statements for 2 media, got %d: %v", len(stmts), stmts)
	}
	mustContain(t, stmts[2], "ordinal=5", `blobRef="`+refA+`"`)
	mustContain(t, stmts[3], "ordinal=2", `kind="video"`, "durationMs=12000")
}

func TestCreatedDropsUnusableMediaButKeepsTheRest(t *testing.T) {
	// A malformed ref costs that item, not the whole glimt — the same rule the portrait
	// fold applies to a rendition. Losing one photo from a set of three is recoverable;
	// refusing the event loses the other two as well.
	body := createdBody()
	body.Media = []Media{
		{Ordinal: 0, Ref: refA, Kind: MediaKindImage},
		{Ordinal: 1, Ref: "../../etc/passwd", Kind: MediaKindImage},
		{Ordinal: 2, Ref: refC, ThumbRef: "not-a-hash", Kind: MediaKindImage},
	}
	stmts := fold(t, "NATHEJK.2026.glimt.g-1.created", body)

	mustContain(t, stmts[0], "mediaCount=2")
	joined := strings.Join(stmts, "\n")
	if strings.Contains(joined, "etc/passwd") {
		t.Error("a path-shaped ref reached a SQL statement")
	}
	// The bad thumbnail blanks the thumbnail, not the item: the client falls back to the
	// full media rather than showing a gap.
	mustContain(t, stmts[3], `blobRef="`+refC+`"`, `thumbRef=""`)
}

func TestCreatedRefusesWhatItCannotStoreHonestly(t *testing.T) {
	cases := map[string]func(*Created){
		// Ownership is the only thing that authorises a delete, and an unowned row would
		// also match the "own glimt" visibility branch for every caller with an empty id.
		"no author": func(c *Created) { c.AuthorPersonID = "" },
		// No audience means no answer to "who may see this", and defaulting it here would
		// be this file inventing an access decision.
		"no audience": func(c *Created) { c.Audience = "" },
		// Retention measures from createdAt; time.Now() would differ on every replay and
		// make one row's purge window unpredictable.
		"no createdAt": func(c *Created) { c.CreatedAt = time.Time{} },
		// Nothing to show.
		"no media":      func(c *Created) { c.Media = nil },
		"unusable refs": func(c *Created) { c.Media = []Media{{Ordinal: 0, Ref: "nope"}} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			body := createdBody()
			mutate(&body)
			foldExpectingError(t, "NATHEJK.2026.glimt.g-1.created", body)
		})
	}
}

func TestDeletedTombstonesAndDropsMedia(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.glimt.g-1.deleted", Deleted{
		GlimtID:   "g-1",
		Year:      "2026",
		Refs:      []string{refA},
		DeletedAt: time.Date(2026, 9, 18, 8, 0, 0, 0, time.UTC),
	})
	if len(stmts) != 2 {
		t.Fatalf("want 2 statements, got %d: %v", len(stmts), stmts)
	}
	// The row stays so a replay cannot resurrect the glimt and the feed can tell "gone"
	// from "never existed".
	mustContain(t, stmts[0], "UPDATE glimt SET deleted=1", "mediaCount=0", `glimtId="g-1"`)
	mustContain(t, stmts[1], "DELETE FROM glimt_media", `glimtId="g-1"`)
}

func TestReportedRecordsAndHidesInTheSameFold(t *testing.T) {
	// The property that matters most in this file. The public scope publishes with no
	// approval queue, so a gap between "reported" and "hidden" is a gap in which the thing
	// somebody objected to is still on the open web.
	stmts := fold(t, "NATHEJK.2026.glimt.g-1.reported", Reported{
		GlimtID:          "g-1",
		Year:             "2026",
		ReporterPersonID: "p-9",
		Reason:           "ikke ok",
		ReportedAt:       time.Date(2026, 9, 17, 22, 0, 0, 0, time.UTC),
	})
	if len(stmts) != 2 {
		t.Fatalf("want 2 statements, got %d: %v", len(stmts), stmts)
	}
	mustContain(t, stmts[0], "INSERT IGNORE INTO glimt_report SET", `reporterPersonId="p-9"`)
	mustContain(t, stmts[1], "UPDATE glimt SET", "hiddenAt=IF(hiddenAt IS NULL")
}

func TestReportedCountIsDerivedNotIncremented(t *testing.T) {
	// `reportCount = reportCount + 1` would climb on every rebuild. Deriving it from the
	// report table is what makes the moderation queue's ordering survive a replay.
	stmts := fold(t, "NATHEJK.2026.glimt.g-1.reported", Reported{
		GlimtID: "g-1", Year: "2026", ReporterPersonID: "p-9",
		ReportedAt: time.Now().UTC(),
	})
	if strings.Contains(stmts[1], "reportCount=reportCount") ||
		strings.Contains(stmts[1], "reportCount = reportCount") {
		t.Errorf("report count is incremented, so it will climb on replay\ngot: %s", stmts[1])
	}
	mustContain(t, stmts[1], "SELECT COUNT(*) FROM glimt_report")
}

func TestReportedRequiresAReporter(t *testing.T) {
	// Without the reporter as a key, one person tapping twice looks like two people
	// objecting — and that count is the signal the moderation queue sorts on.
	foldExpectingError(t, "NATHEJK.2026.glimt.g-1.reported", Reported{
		GlimtID: "g-1", Year: "2026", ReportedAt: time.Now().UTC(),
	})
}

func TestHiddenAndUnhiddenSetAbsoluteState(t *testing.T) {
	hidden := fold(t, "NATHEJK.2026.glimt.g-1.hidden", Hidden{
		GlimtID: "g-1", Year: "2026", HiddenBy: "p-team",
		HiddenAt: time.Date(2026, 9, 17, 23, 0, 0, 0, time.UTC),
	})
	stmt := hidden[0]
	mustContain(t, stmt, `hiddenAt="2026-09-17 23:00:00"`, `hiddenBy="p-team"`)
	// No media touched: that is the entire difference from a delete, and what makes
	// hiding cheap enough to be the default response to a report.
	if strings.Contains(strings.Join(hidden, "\n"), "glimt_media") {
		t.Error("hiding touched the media rows — it must be reversible")
	}

	unhidden := fold(t, "NATHEJK.2026.glimt.g-1.unhidden", Unhidden{
		GlimtID: "g-1", Year: "2026", UnhiddenBy: "p-team",
	})
	// hiddenBy is cleared too: leaving it on a visible row would read to the next
	// moderator as though it were still hidden by that person.
	mustContain(t, unhidden[0], "hiddenAt=NULL", "hiddenBy=''")
}

func TestPurgedLooksLikeADeleteInTheRowAndDiffersOnTheStream(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.glimt.g-1.purged", Purged{
		GlimtID: "g-1", Year: "2026", Refs: []string{refA, refB}, Reason: "retention",
		PurgedAt: time.Now().UTC(),
	})
	if len(stmts) != 2 {
		t.Fatalf("want 2 statements, got %d: %v", len(stmts), stmts)
	}
	mustContain(t, stmts[0], "UPDATE glimt SET deleted=1")
	mustContain(t, stmts[1], "DELETE FROM glimt_media")
}

func TestUnknownSubjectIsIgnored(t *testing.T) {
	// A verb this projection does not know is not an error: another consumer may own it,
	// and failing would dead-letter somebody else's message.
	w := &cqrstest.Writer{}
	c := consumer{w: w}
	msg := cqrstest.NewMessage(cqrs.SubjectFromStr("NATHEJK.2026.glimt.g-1.liked"))
	if err := c.HandleMessage(msg); err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	if len(w.Statements) != 0 {
		t.Errorf("unknown verb wrote %v", w.Statements)
	}
}

func TestSubjectMissingYearFails(t *testing.T) {
	w := &cqrstest.Writer{}
	c := consumer{w: w}
	msg := cqrstest.NewMessage(cqrs.SubjectFromStr("NATHEJK"))
	if err := c.HandleMessage(msg); err == nil {
		t.Error("a subject with no year folded without error")
	}
}

func TestCaptionsAreEscaped(t *testing.T) {
	// Captions are typed by participants, which makes them the least trusted input in the
	// service, and cqrs.Writer takes a finished statement rather than args.
	body := createdBody()
	body.Caption = `"; DROP TABLE glimt; --`
	stmts := fold(t, "NATHEJK.2026.glimt.g-1.created", body)
	if strings.Contains(stmts[0], "DROP TABLE glimt;") &&
		!strings.Contains(stmts[0], `\"; DROP TABLE glimt; --`) {
		t.Errorf("caption was not escaped\ngot: %s", stmts[0])
	}
}

func TestSubjectBuildsAndRejectsBadTokens(t *testing.T) {
	s, err := Subject("2026", "g-1", VerbCreated)
	if err != nil {
		t.Fatalf("Subject: %v", err)
	}
	if got := s.Subject(); got != "NATHEJK.2026.glimt.g-1.created" {
		t.Errorf("Subject() = %q", got)
	}

	// An id containing a dot would still publish and still match NATHEJK.> while quietly
	// no longer matching the per-glimt patterns — making that glimt impossible to hide.
	for _, bad := range []string{"g.1", "g 1", "g*", "g>", ""} {
		if _, err := Subject("2026", bad, VerbCreated); err == nil {
			t.Errorf("Subject accepted glimt id %q", bad)
		}
	}
	if _, err := Subject("", "g-1", VerbCreated); err == nil {
		t.Error("Subject accepted an empty year")
	}
	if _, err := Subject("2026", "g-1", ""); err == nil {
		t.Error("Subject accepted an empty verb")
	}
}

func TestValidRef(t *testing.T) {
	if !validRef(refA) {
		t.Error("a 64-char lowercase hex string is not valid")
	}
	for _, bad := range []string{
		"",
		strings.Repeat("a", 63),
		strings.Repeat("a", 65),
		strings.Repeat("A", 64), // uppercase hex is not what the blob store emits
		strings.Repeat("g", 64), // not hex
		"../../etc/passwd",
	} {
		if validRef(bad) {
			t.Errorf("validRef(%q) = true", bad)
		}
	}
}

func TestKindOrDefaultIsLenientOnlyTowardsImages(t *testing.T) {
	// Calling an unknown thing an image means a possibly-broken <img>: visible and
	// harmless. The reverse would hand arbitrary bytes to a media element to play.
	if got := (Media{Kind: "gif89a"}).kindOrDefault(); got != MediaKindImage {
		t.Errorf("unknown kind = %q, want image", got)
	}
	if got := (Media{Kind: MediaKindVideo}).kindOrDefault(); got != MediaKindVideo {
		t.Errorf("video kind = %q", got)
	}
}
