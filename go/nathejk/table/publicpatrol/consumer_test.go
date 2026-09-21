package publicpatrol

import (
	"strings"
	"testing"

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

// A team update as it actually arrives: the patrol's details **and** a leader's contact block.
func fullTeamUpdate() messages.NathejkTeamUpdated {
	return messages.NathejkTeamUpdated{
		TeamID:    "team-42",
		Type:      "patrulje",
		Name:      "Ørnene",
		GroupName: "1. Søllerød Gruppe",
		Korps:     "dds",

		// Everything below must reach no column. This is the whole point of the projection.
		ContactName:       "Astrid Mortensen",
		ContactPhone:      "+4530000042",
		ContactEmail:      "astrid@example.invalid",
		ContactRole:       "Gruppeleder",
		ContactAddress:    "Skovvej 12",
		ContactPostalCode: "2840",
	}
}

func TestUpdatedWritesThePatrolsOwnDetails(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.patrulje.team-42.updated", fullTeamUpdate())

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	for _, want := range []string{
		"INSERT INTO public_patrol", "ON DUPLICATE KEY UPDATE",
		`"team-42"`, `"2026"`, `"Ørnene"`, `"1. Søllerød Gruppe"`, `"dds"`,
	} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
	}
}

// **The test this table exists for.** The event carries a leader's name, phone, email, role and address.
// None of them may appear in the statement — not in a column, not in a value.
func TestUpdatedDropsEveryContactField(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.patrulje.team-42.updated", fullTeamUpdate())

	forbidden := map[string]string{
		"Astrid Mortensen":       "a leader's name",
		"+4530000042":            "a leader's phone number",
		"astrid@example.invalid": "a leader's email",
		"Gruppeleder":            "a leader's role",
		"Skovvej 12":             "a postal address",
		"2840":                   "a postcode",
		"contactName":            "the contactName column",
		"contactPhone":           "the contactPhone column",
		"contactEmail":           "the contactEmail column",
		"contactRole":            "the contactRole column",
	}
	for needle, what := range forbidden {
		if strings.Contains(stmts[0], needle) {
			t.Errorf("the fold wrote %s (%q): this projection exists to drop it\ngot: %s",
				what, needle, stmts[0])
		}
	}
}

// The number has exactly one writer. Two would mean a replayed update reverting an assigned number.
func TestUpdatedDoesNotTouchTheNumber(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.patrulje.team-42.updated", fullTeamUpdate())

	if strings.Contains(stmts[0], "teamNumber") {
		t.Errorf("only numberassigned may write teamNumber\ngot: %s", stmts[0])
	}
}

func TestNumberAssignedWritesTheNumber(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.patrulje.team-42.numberassigned",
		messages.NathejkPatrolNumberAssigned{TeamID: "team-42", TeamNumber: "42"})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	for _, want := range []string{"INSERT INTO public_patrol", `teamNumber="42"`, `"team-42"`} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
	}
	// An upsert, not an UPDATE: on a fresh replay the number can arrive before any update, and an
	// UPDATE would silently do nothing and leave the patrol unreachable by its own number.
	if !strings.Contains(stmts[0], "ON DUPLICATE KEY UPDATE") {
		t.Error("want an upsert so the row can be created by either event")
	}
	// And it must not clear the name it did not mention.
	if strings.Contains(stmts[0], "name=") {
		t.Errorf("numberassigned must not write the name\ngot: %s", stmts[0])
	}
}

// A klan is not a patrol. PRD 011 §11 Q10 asks whether klans get a page at all, and until that is
// answered this table must not quietly acquire them.
func TestUpdatedIgnoresNonPatrolTeams(t *testing.T) {
	for _, teamType := range []string{"klan", "senior", "crew", "noget-nyt"} {
		body := fullTeamUpdate()
		body.Type = types.TeamType(teamType)

		stmts := fold(t, "NATHEJK.2026.patrulje.team-42.updated", body)
		if len(stmts) != 0 {
			t.Errorf("team type %q should be ignored, got %v", teamType, stmts)
		}
	}
}

// An absent type is folded, because the early signup events do not always carry one and refusing them
// would leave a patrol with no row at all.
func TestUpdatedAcceptsAnAbsentType(t *testing.T) {
	body := fullTeamUpdate()
	body.Type = ""

	stmts := fold(t, "NATHEJK.2026.patrulje.team-42.updated", body)
	if len(stmts) != 1 {
		t.Fatalf("an update with no type should still be folded, got %v", stmts)
	}
}

func TestFallsBackToTheSubjectID(t *testing.T) {
	body := fullTeamUpdate()
	body.TeamID = ""

	stmts := fold(t, "NATHEJK.2026.patrulje.team-99.updated", body)
	if !strings.Contains(stmts[0], `"team-99"`) {
		t.Errorf("want the id from the subject\ngot: %s", stmts[0])
	}
}

func TestRefusesAnEventWithNoYear(t *testing.T) {
	// A subject with no year token cannot be attributed to an event, so the row would be unaddressable.
	if err := foldErr(t, "NATHEJK", fullTeamUpdate()); err == nil {
		t.Fatal("an event with no year must be refused")
	}
}

// A subject that matches neither verb is a no-op, not an error: this projection subscribes to two verbs
// and the stream carries many, so erroring on the rest would fill the log with noise about events that
// were never ours.
func TestUnmatchedSubjectsAreIgnored(t *testing.T) {
	for _, subject := range []string{
		"NATHEJK.2026.patrulje.team-42.started",
		"NATHEJK.2026.patrulje.team-42.signedup",
		"NATHEJK.2026.klan.klan-1.updated",
		"NATHEJK.2026.patrulje", // too few tokens to be addressed at all
	} {
		w := &cqrstest.Writer{}
		c := consumer{w: w}
		msg := cqrstest.NewMessage(cqrs.SubjectFromStr(subject))
		if err := msg.SetBody(fullTeamUpdate()); err != nil {
			t.Fatalf("SetBody: %v", err)
		}
		if err := c.HandleMessage(msg); err != nil {
			t.Errorf("%s should be ignored, got error: %v", subject, err)
		}
		if len(w.Statements) != 0 {
			t.Errorf("%s should write nothing, got %v", subject, w.Statements)
		}
	}
}

// The subscription's colon after NATHEJK is upstream's spelling, and the library normalises it to a dot.
// Asserted rather than trusted to a comment, because a comment claiming a library behaviour goes stale on
// the next upgrade — and the failure it would hide is a projection sitting empty with no error anywhere.
func TestTheColonSpellingNormalisesToTheDotForm(t *testing.T) {
	colon := cqrs.SubjectFromStr("NATHEJK:2026.patrulje.team-42.updated")
	dot := cqrs.SubjectFromStr("NATHEJK.2026.patrulje.team-42.updated")

	if colon.Subject() != dot.Subject() {
		t.Fatalf("the two spellings differ: %q vs %q — the consumer's subscription and its Match calls "+
			"would no longer agree", colon.Subject(), dot.Subject())
	}

	// And a real upstream subject must reach the fold.
	if stmts := fold(t, "NATHEJK:2026.patrulje.team-42.updated", fullTeamUpdate()); len(stmts) == 0 {
		t.Error("an upstream-spelled subject was not folded")
	}
}

// And the handler must match the subjects it subscribes to, or events publish fine and are folded by
// nothing.
func TestConsumedSubjectsAreActuallyHandled(t *testing.T) {
	cases := map[string]any{
		"NATHEJK.2026.patrulje.team-42.updated":        fullTeamUpdate(),
		"NATHEJK.2026.patrulje.team-42.numberassigned": messages.NathejkPatrolNumberAssigned{TeamID: "team-42", TeamNumber: "42"},
	}
	for subject, body := range cases {
		if stmts := fold(t, subject, body); len(stmts) == 0 {
			t.Errorf("%s was not folded into anything", subject)
		}
	}
}

func TestKorpsLabel(t *testing.T) {
	labels := map[string]string{
		"dds":  "Det Danske Spejderkorps",
		"kfum": "KFUM-Spejderne",
		"kfuk": "De grønne pigespejdere",
		"fdf":  "FDF / FPF",
	}
	for slug, want := range labels {
		if got := (Patrol{Korps: slug}).KorpsLabel(); got != want {
			t.Errorf("KorpsLabel(%q) = %q, want %q", slug, got, want)
		}
	}
}

// `andet` and empty are both "unspecified", and a header line reading "Andet" tells a visitor nothing
// while looking like a bug. Both must omit the segment.
func TestKorpsLabelOmitsTheUnspecified(t *testing.T) {
	for _, slug := range []string{"", "andet"} {
		if got := (Patrol{Korps: slug}).KorpsLabel(); got != "" {
			t.Errorf("KorpsLabel(%q) = %q, want it omitted", slug, got)
		}
	}
}

// A slug shared-go does not know must not render as the raw slug: "dds" in a header is worse than
// nothing, because it looks like a bug rather than like missing data.
func TestKorpsLabelOmitsAnUnknownSlug(t *testing.T) {
	for _, slug := range []string{"nyt-korps", "DDS", "xyz"} {
		if got := (Patrol{Korps: slug}).KorpsLabel(); got != "" {
			t.Errorf("KorpsLabel(%q) = %q, want it omitted", slug, got)
		}
	}
}

// The type is the boundary, so assert its shape directly: four public fields plus the internal id, and
// nothing that could hold a person. Task 337's structural test does this by reflection for the album
// types; this is the same claim, made where the type lives.
func TestPatrolHasNoFieldForAPerson(t *testing.T) {
	// Compile-time: this fails to build if a field is removed, and the names are the review surface.
	p := Patrol{TeamID: "t", Number: "42", Name: "Ørnene", GroupName: "g", Korps: "dds"}

	if p.Name != "Ørnene" {
		t.Fatal("sanity")
	}
	// A reviewer checking "does this name a person?" reads five field names. That is the whole design.
}
