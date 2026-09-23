package photo

import (
	"os"
	"strings"
	"testing"
)

// The patrol tag's two properties that are not about SQL syntax (PRD 022 §8.6, task 367).

// **The hazard the tag's shape exists for.** `public_patrol.teamNumber` is *not unique per year* — the index
// is deliberately non-unique and `publicpatrol.ByNumber` does `ORDER BY teamId LIMIT 1`. Numbers are also
// handed out late and can be reassigned.
//
// So a tag keyed on the number would silently start pointing at a different patrol. This asserts the fold
// keys on `teamId` and treats the number as carried data:
//
//   - re-tagging the same patrol after its number changed updates the row rather than creating a second one,
//     because the key did not change;
//   - an untag finds the row by id, so it still works after a renumbering.
//
// Both matter more once tags become public (PRD 022 §11 Q1): the failure would be a photograph appearing on
// the wrong family's page.
func TestATagSurvivesARenumbering(t *testing.T) {
	const teamID = "team-9"
	photoID := hash("a")

	// Tagged when the patrol was number 42.
	first := fold(t, "NATHEJK.2026.photo."+photoID+".patroltagged", PatrolTagged{
		PhotoID: photoID, Year: "2026", TeamID: teamID, Number: "42", TaggedAt: at,
	})
	// Re-tagged after the patrol became number 7 — the same patrol, a different number.
	second := fold(t, "NATHEJK.2026.photo."+photoID+".patroltagged", PatrolTagged{
		PhotoID: photoID, Year: "2026", TeamID: teamID, Number: "7", TaggedAt: at,
	})

	// The key is what makes this an update rather than a second row. Both statements must therefore write
	// the same teamId, and the number must be in the update clause so the new one takes effect.
	for _, stmts := range [][]string{first, second} {
		if !strings.Contains(stmts[0], `teamId="`+teamID+`"`) {
			t.Fatalf("the tag must be keyed on the team id\ngot: %s", stmts[0])
		}
	}
	clause := second[0][strings.Index(second[0], "ON DUPLICATE KEY UPDATE"):]
	if !strings.Contains(clause, "teamNumber=VALUES(teamNumber)") {
		t.Errorf("a renumbering must update the stored number\n%s", clause)
	}
	// And the id must not be in the update clause: it is the key, so writing it would be either a no-op or
	// a sign somebody had started keying on something else.
	if strings.Contains(clause, "teamId=VALUES(teamId)") {
		t.Errorf("teamId is the key and must not be updated\n%s", clause)
	}

	// The untag is by id, not by number, so it still matches after the number changed hands.
	untag := fold(t, "NATHEJK.2026.photo."+photoID+".patroluntagged", PatrolUntagged{
		PhotoID: photoID, Year: "2026", TeamID: teamID, UntaggedAt: at,
	})
	if !strings.Contains(untag[0], `teamId="`+teamID+`"`) {
		t.Errorf("an untag must find the row by id\ngot: %s", untag[0])
	}
	if strings.Contains(untag[0], "teamNumber") {
		t.Errorf("an untag must not match on the number, which may have moved\ngot: %s", untag[0])
	}
}

// Untagging one patrol must leave the photograph and its other attributions intact. A group shot with two
// patrols in it is ordinary, and correcting one of them must not clear both.
func TestUntaggingIsNarrowedToOnePatrol(t *testing.T) {
	photoID := hash("a")
	stmts := fold(t, "NATHEJK.2026.photo."+photoID+".patroluntagged", PatrolUntagged{
		PhotoID: photoID, Year: "2026", TeamID: "team-9", UntaggedAt: at,
	})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	s := stmts[0]

	// Scoped to all three parts of the key. Dropping any one of them widens the untag: without the photo it
	// clears the patrol's tag on every photograph, and without the team it clears every tag on this one.
	for _, want := range []string{`year="2026"`, `photoId="` + photoID + `"`, `teamId="team-9"`} {
		if !strings.Contains(s, want) {
			t.Errorf("the untag must be scoped by %s\ngot: %s", want, s)
		}
	}
	// And it must not touch the photograph itself.
	if strings.Contains(s, "UPDATE photo SET") || strings.Contains(s, "photo_patrol, photo") {
		t.Errorf("untagging must not modify the photograph\ngot: %s", s)
	}
}

// No public read may touch `photo_patrol`.
//
// PRD 022 §11 Q1 resolved that a tag *will* eventually surface a photograph on that patrol's public page —
// and deliberately **not in v1**, so that the tagging can be used in anger and corrected before a mistag can
// put a photograph on the wrong family's page.
//
// "Not yet" is the kind of constraint that decays into "why not", so it is asserted rather than trusted. The
// tag is reachable only through `CuratorQueries.Tags`, which lives in curator.go and which
// `cmd/api/curatorboundary_test.go` keeps out of public handlers. What this adds is the other half: the file
// holding the *public* interface must not learn the table's name at all.
func TestNoPublicReadTouchesTheTags(t *testing.T) {
	src, err := os.ReadFile("querier.go")
	if err != nil {
		t.Fatalf("reading querier.go: %v", err)
	}
	if strings.Contains(string(src), "photo_patrol") {
		t.Error("querier.go holds the public read API and must not query photo_patrol: in v1 a patrol tag has " +
			"no public effect (PRD 022 §11 Q1). Surfacing it is its own task and its own review.")
	}

	// And the public `Photo` type must have no tag field — even a count would leak "this patrol appears in some
	// photograph" onto a public surface, which is a smaller version of the same disclosure.
	for _, field := range []string{"Tags ", "TagCount ", "TeamID ", "Patrol "} {
		if strings.Contains(string(src), "\t"+field) {
			t.Errorf("the public photo type has gained a %q field; patrol attribution is curator-only in v1",
				strings.TrimSpace(field))
		}
	}
}

// The tag tables and event types have no place to put a person.
//
// Task 381 walks this structurally across the package; this is the narrower, local statement, here because a
// tagging feature is precisely where somebody reaches for a name. The fold's generated SQL is checked rather
// than the struct, so a field that existed but was written into the statement would also be caught.
func TestATagNamesAPatrolAndNeverAPerson(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".patroltagged", PatrolTagged{
		PhotoID: hash("a"), Year: "2026", TeamID: "team-9", Number: "42", TaggedAt: at,
	})

	lower := strings.ToLower(stmts[0])
	for _, forbidden := range []string{
		"person", "phone", "email", "contactname", "memberid", "portrait", "birth", "address",
	} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("a tag must not carry %q: a photograph is attributed to a patrulje, never to a person"+
				"\ngot: %s", forbidden, stmts[0])
		}
	}
}
