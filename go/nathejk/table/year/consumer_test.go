package year

import (
	"strings"
	"testing"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/cqrstest"
	"github.com/nathejk/shared-go/messages"
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

func str(s string) *string { return &s }

func TestUpdatedWritesBothCities(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.updated", messages.NathejkYearUpdated{
		Slug:            "2026",
		CityDeparture:   str("Lundby"),
		CityDestination: str("Glumsø"),
	})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d: %v", len(stmts), stmts)
	}
	for _, want := range []string{
		"INSERT INTO event_year", "ON DUPLICATE KEY UPDATE",
		`"2026"`, `cityDeparture="Lundby"`, `cityDestination="Glumsø"`,
	} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %q:\n%s", want, stmts[0])
		}
	}
}

// **A nil is not an empty string**, and this is the trap the pointers in the message exist to avoid.
//
// `NathejkYearUpdated` carries pointers so an event can say "the description changed" without saying anything
// about the cities. If the fold wrote "" for a nil, an organizer fixing a typo in hq's description field would
// silently erase the route from every diploma — a failure nobody would connect to the edit that caused it.
func TestAnUpdateThatNamesNoCityWritesNothing(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.updated", messages.NathejkYearUpdated{
		Slug:        "2026",
		Headline:    str("Troldtøjets Alliance"),
		Description: str("Vi ses i mørket!"),
	})

	if len(stmts) != 0 {
		t.Errorf("want no write at all, got %d: %v", len(stmts), stmts)
	}
}

// One city named and the other not: write the one, leave the other alone. The read is what refuses to render
// half a route — see querier.Route.
func TestOneCityIsWrittenWithoutTouchingTheOther(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.updated", messages.NathejkYearUpdated{
		Slug:          "2026",
		CityDeparture: str("Lundby"),
	})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d: %v", len(stmts), stmts)
	}
	if !strings.Contains(stmts[0], `cityDeparture="Lundby"`) {
		t.Errorf("want the named city written:\n%s", stmts[0])
	}
	if strings.Contains(stmts[0], "cityDestination") {
		t.Errorf("must not touch the city the event said nothing about:\n%s", stmts[0])
	}
}

// The editorial fields are read past. Asserted because "we do not select it" is a habit and "it is not here" is
// a property — the same reasoning as public_patrol's contact fields, for much less sensitive data.
func TestTheEditorialFieldsReachNoColumn(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.updated", messages.NathejkYearUpdated{
		Slug:            "2026",
		Headline:        str("Troldtøjets Alliance"),
		Description:     str("Vi ses i mørket!"),
		CityDeparture:   str("Lundby"),
		CityDestination: str("Glumsø"),
	})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d: %v", len(stmts), stmts)
	}
	for _, forbidden := range []string{"headline", "description", "Troldtøjets", "mørket", "dateStart", "dateEnd"} {
		if strings.Contains(stmts[0], forbidden) {
			t.Errorf("the fold wrote %q, which this table does not have:\n%s", forbidden, stmts[0])
		}
	}
}

func TestCreatedMakesTheRowAndDeletedRemovesIt(t *testing.T) {
	created := fold(t, "NATHEJK.2026.created", messages.NathejkYearUpdated{Slug: "2026"})
	if len(created) != 1 || !strings.Contains(created[0], "INSERT INTO event_year") {
		t.Errorf("want the row created, got %v", created)
	}

	deleted := fold(t, "NATHEJK.2026.deleted", messages.NathejkYearDeleted{Slug: "2026"})
	if len(deleted) != 1 || !strings.Contains(deleted[0], `DELETE FROM event_year WHERE slug="2026"`) {
		t.Errorf("want the row deleted, got %v", deleted)
	}
}

// **The subscription is three tokens wide, and that is the whole filter.**
//
// `NATHEJK.*.updated` looks alarmingly broad next to the other projections' patterns, and it is not: a `*`
// matches exactly one token, so a patrol's `NATHEJK.2026.patrulje.<id>.updated` cannot reach this consumer.
// Asserted rather than trusted, because the entire safety of this projection rests on that library behaviour —
// if it ever changed, this fold would start writing a row per patrol id.
func TestTheSubscriptionDoesNotMatchEntityEvents(t *testing.T) {
	for _, subject := range []string{
		"NATHEJK.2026.patrulje.team-42.updated",
		"NATHEJK.2026.glimt.g-1.created",
		"NATHEJK.2026.patrulje.team-42.numberassigned",
	} {
		for _, pattern := range []string{"NATHEJK:*.created", "NATHEJK:*.updated", "NATHEJK:*.deleted"} {
			if cqrs.SubjectFromStr(subject).Match(pattern) {
				t.Errorf("%q must not match %q", subject, pattern)
			}
		}
	}
}

// And the year's own subject must match, or the projection silently never folds anything. The colon upstream
// spells these with is normalised to a dot by the library; this pins that too.
func TestTheYearsOwnSubjectMatches(t *testing.T) {
	if got := cqrs.SubjectFromStr("NATHEJK:2026.updated").Subject(); got != "NATHEJK.2026.updated" {
		t.Errorf("the colon should normalise to a dot, got %q", got)
	}
	if !cqrs.SubjectFromStr("NATHEJK:2026.updated").Match("NATHEJK.*.updated") {
		t.Error("a year update must match the subscription")
	}
}
