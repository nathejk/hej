package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/nathejk/table/photo"
	"nathejk.dk/nathejk/table/publicpatrol"
)

// The curator's patrol tags (PRD 022 §8.6, §11 Q1, task 377).

// stubPublicPatrols resolves numbers to patrols, and records what it was asked.
type stubPublicPatrols struct {
	// byNumber is keyed on the **normalised** number, as the handler will look it up.
	byNumber map[string]publicpatrol.Patrol
	err      error

	asked []string
}

func (s *stubPublicPatrols) ByNumber(_, number string) (publicpatrol.Patrol, bool, error) {
	s.asked = append(s.asked, number)
	if s.err != nil {
		return publicpatrol.Patrol{}, false, s.err
	}
	p, ok := s.byNumber[number]
	return p, ok, nil
}

// tagApp returns an admin app able to resolve patrols and publish tags.
func tagApp(t *testing.T, patrols *stubPublicPatrols) (*application, *httptest.Server, *cqrstest.Publisher) {
	t.Helper()

	app, srv := adminApp(t)
	app.models.PhotoCurator = &libraryCurator{}
	app.models.PublicPatrols = patrols
	pub := &cqrstest.Publisher{}
	app.commands = commandsWithPublisher(t, pub)
	return app, srv, pub
}

// oernene is a patrol as the public projection returns one: a patrol name, a group, a korps slug — and no person.
func oernene() publicpatrol.Patrol {
	return publicpatrol.Patrol{
		TeamID:    "team-9",
		Number:    "42",
		Name:      "Ørnene",
		GroupName: "1. Søllerød Gruppe",
		Korps:     "dds",
	}
}

func deleteAdmin(t *testing.T, srv *httptest.Server, path string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	req.Header.Set("X-Forwarded-Proto", "https")
	req.SetBasicAuth(testAdminUser, testAdminPass)

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// The confirmation step: typing a number shows the patrol back, and nothing is written.
func TestAdminResolvesAPatrolNumberWithoutWriting(t *testing.T) {
	patrols := &stubPublicPatrols{byNumber: map[string]publicpatrol.Patrol{"42": oernene()}}
	_, srv, pub := tagApp(t, patrols)

	resp := getAdmin(t, srv, "/api/admin/patrols/42", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var out adminPatrolResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if out.Name != "Ørnene" || out.Number != "42" || out.TeamID != "team-9" {
		t.Errorf("want the patrol back for confirmation, got %+v", out)
	}
	// The korps **label**, not the slug: a slug is an internal token and "DDS" is what a human recognises.
	if out.Korps == "dds" {
		t.Errorf("want the korps label rather than the slug, got %q", out.Korps)
	}
	if len(pub.Subjects()) != 0 {
		t.Error("resolving must write nothing: the curator has not confirmed yet")
	}
}

// The same normalisation the public patrol page applies, reused rather than reimplemented — so "042" and "42" are
// one patrol, which is what the number on the sign means.
func TestAdminNormalisesThePatrolNumber(t *testing.T) {
	patrols := &stubPublicPatrols{byNumber: map[string]publicpatrol.Patrol{"42": oernene()}}
	_, srv, _ := tagApp(t, patrols)

	for _, raw := range []string{"42", "042", "0042"} {
		resp := getAdmin(t, srv, "/api/admin/patrols/"+raw, testAdminUser, testAdminPass)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%q should resolve to the same patrol, got %d", raw, resp.StatusCode)
		}
	}
	for _, asked := range patrols.asked {
		if asked != "42" {
			t.Errorf("the projection should be asked for the normalised number, got %q", asked)
		}
	}
}

// An unknown number is refused with a plain reason and nothing is written.
func TestAdminRefusesAnUnknownPatrolNumber(t *testing.T) {
	patrols := &stubPublicPatrols{byNumber: map[string]publicpatrol.Patrol{"42": oernene()}}
	_, srv, pub := tagApp(t, patrols)

	if got := getAdmin(t, srv, "/api/admin/patrols/99", testAdminUser, testAdminPass).StatusCode; got != http.StatusNotFound {
		t.Errorf("want 404 for an unknown number, got %d", got)
	}
	for _, bad := range []string{"abc", "4-2", "999999999"} {
		got := getAdmin(t, srv, "/api/admin/patrols/"+bad, testAdminUser, testAdminPass).StatusCode
		if got != http.StatusBadRequest && got != http.StatusNotFound {
			t.Errorf("%q: want a refusal, got %d", bad, got)
		}
	}
	if len(pub.Subjects()) != 0 {
		t.Errorf("a refused resolve must publish nothing, got %d", len(pub.Subjects()))
	}
}

// Tagging works across a selection in one request, and the tag carries **both** the resolved id and the number.
func TestAdminTagsASelectionWithOnePatrol(t *testing.T) {
	patrols := &stubPublicPatrols{byNumber: map[string]publicpatrol.Patrol{"42": oernene()}}
	_, srv, pub := tagApp(t, patrols)

	body := `{"photoIds":[` + selection(3) + `],"number":"42"}`
	resp := postAdmin(t, srv, "/api/admin/photos/tags", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}

	var out tagAdminPhotosResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if out.Tagged != 3 {
		t.Errorf("want 3 tagged, got %d", out.Tagged)
	}
	if !strings.Contains(out.Message, "patrulje 42") {
		t.Errorf("the message should name the patrol number, got %q", out.Message)
	}

	if got := len(pub.Subjects()); got != 3 {
		t.Fatalf("want one event per photograph, got %d", got)
	}
	var tag photo.PatrolTagged
	if err := pub.Messages[0].Body(&tag); err != nil {
		t.Fatalf("decoding the event: %v", err)
	}
	if tag.TeamID != "team-9" {
		t.Errorf("the tag must carry the resolved team id, got %q", tag.TeamID)
	}
	if tag.Number != "42" {
		t.Errorf("the tag must also carry the number, got %q", tag.Number)
	}
}

// **The hazard the tag's shape exists for.** `teamNumber` is not unique per year — `ByNumber` does
// `ORDER BY teamId LIMIT 1` over a deliberately non-unique index — and numbers are handed out late.
//
// So the tag is keyed on the resolved `teamId`. This walks the scenario: a patrol is tagged as number 42, the
// numbers are then reassigned so 42 belongs to somebody else, and the original tag must still name the original
// patrol.
func TestATagSurvivesARenumberingEndToEnd(t *testing.T) {
	patrols := &stubPublicPatrols{byNumber: map[string]publicpatrol.Patrol{"42": oernene()}}
	_, srv, pub := tagApp(t, patrols)

	postAdmin(t, srv, "/api/admin/photos/tags", `{"photoIds":[`+selection(1)+`],"number":"42"}`)

	var first photo.PatrolTagged
	if err := pub.Messages[0].Body(&first); err != nil {
		t.Fatalf("decoding: %v", err)
	}

	// The numbers are reassigned: 42 is now a different patrol.
	patrols.byNumber["42"] = publicpatrol.Patrol{
		TeamID: "team-77", Number: "42", Name: "Ulvene", GroupName: "3. Lyngby", Korps: "dds",
	}

	// The original tag still names the original patrol, because it stored the id rather than the number.
	if first.TeamID != "team-9" {
		t.Fatalf("the tag should hold team-9, got %q", first.TeamID)
	}

	// And untagging that tag is addressed by the id, so it still works after the renumbering — the number would
	// now resolve to the wrong patrol entirely.
	pub.Reset()
	if got := deleteAdmin(t, srv, "/api/admin/photos/"+strings.Repeat("a", 64)+"/tags/team-9").StatusCode; got != http.StatusNoContent {
		t.Fatalf("want 204, got %d", got)
	}
	var untag photo.PatrolUntagged
	if err := pub.Messages[0].Body(&untag); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if untag.TeamID != "team-9" {
		t.Errorf("the untag must name the original patrol, got %q", untag.TeamID)
	}
}

// A photograph may carry several tags: two patrols in one frame is ordinary, not a corner case.
func TestAPhotographMayCarrySeveralTags(t *testing.T) {
	patrols := &stubPublicPatrols{byNumber: map[string]publicpatrol.Patrol{
		"42": oernene(),
		"7":  {TeamID: "team-7", Number: "7", Name: "Ulvene", GroupName: "3. Lyngby", Korps: "kfum"},
	}}
	_, srv, pub := tagApp(t, patrols)

	one := selection(1)
	postAdmin(t, srv, "/api/admin/photos/tags", `{"photoIds":[`+one+`],"number":"42"}`)
	postAdmin(t, srv, "/api/admin/photos/tags", `{"photoIds":[`+one+`],"number":"7"}`)

	if got := len(pub.Subjects()); got != 2 {
		t.Fatalf("want two tags on one photograph, got %d", got)
	}
	teams := map[string]bool{}
	for _, msg := range pub.Messages {
		var tag photo.PatrolTagged
		if err := msg.Body(&tag); err != nil {
			t.Fatalf("decoding: %v", err)
		}
		teams[tag.TeamID] = true
	}
	if !teams["team-9"] || !teams["team-7"] {
		t.Errorf("want both patrols tagged, got %v", teams)
	}
}

// The tag request sends the **number**, and the server re-resolves it. The confirmation showed the curator a name;
// the request that follows must not be able to name a different patrol than the one confirmed, and the only way
// to guarantee that is for the server to do the lookup both times.
func TestTheTagRequestCannotNameATeamDirectly(t *testing.T) {
	for _, f := range structFieldNames(tagAdminPhotosRequest{}) {
		if strings.EqualFold(f, "teamId") {
			t.Error("the tag request must not accept a teamId: the server resolves the number, so a client " +
				"cannot tag a patrol other than the one the curator confirmed")
		}
	}
}

// Untagging removes exactly one tag, scoped by all three key parts.
func TestAdminUntagsExactlyOneTag(t *testing.T) {
	patrols := &stubPublicPatrols{byNumber: map[string]publicpatrol.Patrol{"42": oernene()}}
	_, srv, pub := tagApp(t, patrols)

	if got := deleteAdmin(t, srv, "/api/admin/photos/"+strings.Repeat("a", 64)+"/tags/team-9").StatusCode; got != http.StatusNoContent {
		t.Fatalf("want 204, got %d", got)
	}
	if got := len(pub.Subjects()); got != 1 {
		t.Fatalf("want one event, got %d", got)
	}
	var untag photo.PatrolUntagged
	if err := pub.Messages[0].Body(&untag); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if untag.PhotoID != strings.Repeat("a", 64) || untag.TeamID != "team-9" || untag.Year != "2026" {
		t.Errorf("the untag must be scoped by year, photograph and team, got %+v", untag)
	}
}

// **No response from these endpoints names a person, and no type has a field that could.**
//
// PRD 022 §4 repeats this rule specifically because a tagging UI is where somebody reaches for a name. The patrol
// projection this reads through has no person column either — the upstream event carries a leader's name, phone
// and email, and they reach none of it.
func TestThePatrolTagSurfaceNamesNoPerson(t *testing.T) {
	for name, v := range map[string]any{
		"adminPatrolResponse":    adminPatrolResponse{},
		"tagAdminPhotosRequest":  tagAdminPhotosRequest{},
		"tagAdminPhotosResponse": tagAdminPhotosResponse{},
	} {
		for _, field := range structFieldNames(v) {
			if isPersonShaped(field) {
				t.Errorf("%s gained a person-shaped field %q: a photograph is attributed to a patrulje, "+
					"never to a person", name, field)
			}
		}
	}

	// And the values that do come back are a patrol's, not a person's — asserted on a real response so a field
	// renamed to something innocuous would still be caught by what it carries.
	patrols := &stubPublicPatrols{byNumber: map[string]publicpatrol.Patrol{"42": oernene()}}
	_, srv, _ := tagApp(t, patrols)

	body := adminBody(t, getAdmin(t, srv, "/api/admin/patrols/42", testAdminUser, testAdminPass))
	for _, forbidden := range []string{"phone", "telefon", "email", "contact", "kontakt"} {
		if strings.Contains(strings.ToLower(body), forbidden) {
			t.Errorf("the resolved patrol carries %q\ngot: %s", forbidden, body)
		}
	}
}

// **`publicpatrol.Queries` gains no list read.**
//
// Its doc comment records why: *"a list read is what a scraper would ask for"*, and there is no patrol list
// anywhere in this service at any layer. This task needed one less than it looked — the curator types the number
// from the sign and the confirmation covers what a list would have added.
//
// Asserted on the interface declaration, because this is exactly the kind of thing a later "small convenience"
// would add.
func TestThePublicPatrolInterfaceGainsNoListRead(t *testing.T) {
	src := adminSource(t, "../../nathejk/table/publicpatrol/querier.go")

	start := strings.Index(src, "type Queries interface {")
	if start < 0 {
		t.Fatal("publicpatrol.Queries no longer exists; this guard needs updating")
	}
	end := strings.Index(src[start:], "\n}")
	decl := src[start : start+end]

	for _, smell := range []string{"All(", "List(", "Search(", "Numbers(", "Every("} {
		if strings.Contains(decl, smell) {
			t.Errorf("publicpatrol.Queries has gained %s. There is deliberately no patrol list in this "+
				"service: a list read is what a scraper would ask for. If a picker is wanted it needs a "+
				"separate authenticated interface (PRD 022 §8.6).", smell)
		}
	}

	// It had exactly one method. A second is not forbidden, but it is worth a human looking.
	if got := strings.Count(decl, "ByNumber("); got != 1 {
		t.Errorf("want ByNumber on the interface, found %d", got)
	}
}

// And no admin route lists patrols either. The absence is the feature, so it is asserted rather than assumed from
// there being no handler today.
func TestThereIsNoRouteThatListsPatrols(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "routes.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse routes.go: %v", err)
	}

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "HandlerFunc" || len(call.Args) < 3 {
			return true
		}
		path, ok := parseRegisteredPath(t, call.Args[1], fset)
		if !ok {
			return true
		}
		// A collection route is one ending in `patrols` or `patruljer` with no id segment after it.
		if strings.HasSuffix(path, "/patrols") || strings.HasSuffix(path, "/patruljer") {
			t.Errorf("routes.go registers %s, which enumerates patrols. The curator types the number from "+
				"the sign; a list read is what a scraper would ask for (PRD 022 §8.6).", path)
		}
		return true
	})
}

// A broken stream must not report the photographs as tagged.
func TestAdminTagWithNoStreamDoesNotClaimToHaveTagged(t *testing.T) {
	patrols := &stubPublicPatrols{byNumber: map[string]publicpatrol.Patrol{"42": oernene()}}
	app, srv, _ := tagApp(t, patrols)
	app.commands = commandsWithNoPublisher()

	body := `{"photoIds":[` + selection(1) + `],"number":"42"}`
	if got := postAdmin(t, srv, "/api/admin/photos/tags", body).StatusCode; got != http.StatusServiceUnavailable {
		t.Errorf("want 503 with no event stream, got %d", got)
	}
}

// Every write logs what happened and from where — and the log carries the id and the number, not the patrol's
// name: a log line is a durable record, and there is no reason for one to hold more than identifies the tag.
func TestAdminTagWritesAreLogged(t *testing.T) {
	src := adminSource(t, "adminpatrol.go")

	for _, want := range []string{
		`"admin tagged photographs with a patrol"`,
		`"admin removed a patrol tag"`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("missing the audit log line %s", want)
		}
	}
	if strings.Contains(src, `"name", p.Name`) {
		t.Error("the log should carry the team id and number, not the patrol's name")
	}
}
