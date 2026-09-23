package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jrgensen/cqrs/cqrstest"
	"github.com/nathejk/shared-go/types"

	"nathejk.dk/nathejk/table/checkpoint"
	"nathejk.dk/nathejk/table/photo"
)

// The curator's bulk position (PRD 022 §6, task 376).

// stubCheckpointCurator is the picker's list.
type stubCheckpointCurator struct {
	points []checkpoint.PickerCheckpoint
	err    error
}

func (s stubCheckpointCurator) Positioned(string) ([]checkpoint.PickerCheckpoint, error) {
	return s.points, s.err
}

// positionApp returns an admin app able to set positions, with a race area and a checkpoint list.
func positionApp(t *testing.T) (*application, *httptest.Server, *cqrstest.Publisher) {
	t.Helper()

	app, srv := adminApp(t)
	app.models.PhotoCurator = &libraryCurator{}
	app.models.RaceAreas = stubRaceAreas{area: testRaceArea(), ok: true}
	app.models.CheckpointCurator = stubCheckpointCurator{points: []checkpoint.PickerCheckpoint{
		// Inside the test race area.
		{ID: types.CheckpointID("cp-3"), Name: "Post 3", SortOrder: 3, Lat: 55.7332, Lng: 12.2648},
		{ID: types.CheckpointID("cp-4"), Name: "Post 4", SortOrder: 4, Lat: 55.74, Lng: 12.27},
	}}
	pub := &cqrstest.Publisher{}
	app.commands = commandsWithPublisher(t, pub)
	return app, srv, pub
}

func patchAdmin(t *testing.T, srv *httptest.Server, body string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/api/admin/photos", strings.NewReader(body))
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.SetBasicAuth(testAdminUser, testAdminPass)

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("PATCH /api/admin/photos: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func decodePatch(t *testing.T, resp *http.Response) patchAdminPhotosResponse {
	t.Helper()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 before decoding, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}
	var out patchAdminPhotosResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	return out
}

// selection builds a JSON id list of n distinct photo ids.
func selection(n int) string {
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		ids = append(ids, `"`+strings.Repeat(string(rune('a'+i)), 64)+`"`)
	}
	return strings.Join(ids, ",")
}

// One request, one coordinate, a whole selection. This is the requirement rather than a convenience: "these
// forty are from Post 3" is the true shape of the work (PRD 022 §3).
func TestAdminSetsAPositionOnAWholeSelection(t *testing.T) {
	_, srv, pub := positionApp(t)

	body := `{"photoIds":[` + selection(5) + `],"location":{"lat":55.7332,"lng":12.2648}}`
	out := decodePatch(t, patchAdmin(t, srv, body))

	if out.Updated != 5 {
		t.Errorf("want 5 updated, got %d", out.Updated)
	}
	if out.BoundsVerdict != photo.BoundsInside {
		t.Errorf("a coordinate at the event should be inside, got %q", out.BoundsVerdict)
	}
	if !out.Plottable {
		t.Error("an inside verdict must be plottable")
	}
	if got := len(pub.Subjects()); got != 5 {
		t.Fatalf("want one event per photograph, got %d", got)
	}
	for _, s := range pub.Subjects() {
		if !strings.HasSuffix(s, ".updated") {
			t.Errorf("a set must publish `updated`, got %q", s)
		}
	}
}

// **A bulk position must not blank forty captions.** That is what `photo.Updated`'s pointer fields are for
// (task 363), and it is the single most likely way this action could do damage: a curator spends an evening
// writing captions, then sets a position on the selection.
func TestAdminBulkPositionMentionsOnlyTheLocation(t *testing.T) {
	_, srv, pub := positionApp(t)

	body := `{"photoIds":[` + selection(1) + `],"location":{"lat":55.7332,"lng":12.2648}}`
	patchAdmin(t, srv, body)

	var upd photo.Updated
	if err := pub.Messages[0].Body(&upd); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if upd.Caption != nil {
		t.Error("a bulk position must not carry a caption; it would blank every caption in the selection")
	}
	if upd.Location == nil {
		t.Fatal("want the location")
	}
}

// **The bounds check is re-run, and a curator-placed point is not exempt.** Nothing reaches the public map
// unverified.
func TestAdminReRunsTheBoundsCheckOnACuratorPlacedPoint(t *testing.T) {
	_, srv, pub := positionApp(t)

	// Manhattan: a real coordinate, not at the event.
	body := `{"photoIds":[` + selection(3) + `],"location":{"lat":40.7128,"lng":-74.0060}}`
	out := decodePatch(t, patchAdmin(t, srv, body))

	if out.BoundsVerdict != photo.BoundsOutside {
		t.Fatalf("want the outside verdict for a point away from the event, got %q", out.BoundsVerdict)
	}
	if out.Plottable {
		t.Error("an out-of-bounds coordinate must never be plottable")
	}

	// The coordinate is **kept**, not discarded: a curator has to be able to see what was refused rather than
	// wonder why a pin is missing.
	var upd photo.Updated
	if err := pub.Messages[0].Body(&upd); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if upd.Location == nil {
		t.Fatal("a rejected coordinate must still be stored, so the curator can see it was rejected")
	}
	if upd.Location.BoundsVerdict != photo.BoundsOutside {
		t.Errorf("the event must carry the verdict, got %q", upd.Location.BoundsVerdict)
	}
	// And the message says which, in a way that does not read as the curator's fault.
	if !strings.Contains(out.Message, "uden for løbsområdet") {
		t.Errorf("the message must say the point is outside the race area, got %q", out.Message)
	}
}

// **With no checkpoint sited there is no area to judge against, and the verdict is `unknown`, not `outside`.**
//
// `unknown` is a statement about us. Calling it `outside` would blame the photograph for our missing data and
// would permanently condemn everything positioned before the course was laid out.
func TestAdminPositionIsUnknownBeforeTheCourseIsSited(t *testing.T) {
	app, srv, _ := positionApp(t)
	app.models.RaceAreas = stubRaceAreas{ok: false} // no positioned checkpoints yet

	body := `{"photoIds":[` + selection(2) + `],"location":{"lat":55.7332,"lng":12.2648}}`
	out := decodePatch(t, patchAdmin(t, srv, body))

	if out.BoundsVerdict != photo.BoundsUnknown {
		t.Fatalf("want unknown when there is no race area, got %q", out.BoundsVerdict)
	}
	if out.BoundsVerdict == photo.BoundsOutside {
		t.Fatal("never outside: that would blame the photograph for our own missing data")
	}
	if out.Plottable {
		t.Error("an unchecked coordinate must not be plottable")
	}
	// The copy PRD 022 §7 asks to be written carefully: it must say why, and the why is about us.
	if !strings.Contains(out.Message, "kunne ikke vurderes") {
		t.Errorf("the message must say the position could not be judged, got %q", out.Message)
	}
	if !strings.Contains(out.Message, "ingen poster") {
		t.Errorf("the message must say why — no post has a placement yet — got %q", out.Message)
	}
}

// A checkpoint id is resolved **server-side** to that checkpoint's coordinate. The client sends the post, not
// the coordinate, so a stale coordinate in a browser cannot become a pin on a public map.
func TestAdminSetsAPositionFromACheckpoint(t *testing.T) {
	_, srv, pub := positionApp(t)

	body := `{"photoIds":[` + selection(2) + `],"checkpointId":"cp-3"}`
	out := decodePatch(t, patchAdmin(t, srv, body))

	if out.Updated != 2 || out.BoundsVerdict != photo.BoundsInside {
		t.Fatalf("want 2 updated and inside, got %+v", out)
	}

	var upd photo.Updated
	if err := pub.Messages[0].Body(&upd); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if upd.Location.Lat != 55.7332 || upd.Location.Lng != 12.2648 {
		t.Errorf("want Post 3's own coordinate, got %+v", upd.Location)
	}
}

// An unknown or unsited post is a 404 rather than a silent fallback, because the alternative is a selection that
// quietly did not move.
func TestAdminRefusesAnUnknownCheckpoint(t *testing.T) {
	_, srv, pub := positionApp(t)

	body := `{"photoIds":[` + selection(1) + `],"checkpointId":"cp-nope"}`
	resp := patchAdmin(t, srv, body)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404 for an unknown checkpoint, got %d", resp.StatusCode)
	}
	if got := len(pub.Subjects()); got != 0 {
		t.Errorf("nothing should publish, got %d events", got)
	}
}

// **Clearing publishes its own event**, not an `updated` carrying nulls. PRD 022 §8.7: a coordinate somebody
// deliberately removed is not the same fact as one that was changed, and the log should record which.
func TestAdminClearingPublishesItsOwnEvent(t *testing.T) {
	_, srv, pub := positionApp(t)

	body := `{"photoIds":[` + selection(3) + `],"clearLocation":true,"reason":"forkert fix"}`
	out := decodePatch(t, patchAdmin(t, srv, body))

	if out.Updated != 3 {
		t.Errorf("want 3 cleared, got %d", out.Updated)
	}
	if out.Plottable {
		t.Error("a cleared position is not plottable")
	}
	if got := len(pub.Subjects()); got != 3 {
		t.Fatalf("want one event per photograph, got %d", got)
	}
	for _, s := range pub.Subjects() {
		if !strings.HasSuffix(s, ".locationcleared") {
			t.Errorf("clearing must publish `locationcleared`, got %q", s)
		}
		if strings.HasSuffix(s, ".updated") {
			t.Error("clearing must not publish `updated` with nulls; the log would lose the intent")
		}
	}

	// The reason reaches the event, so the log records why — which is the question anyone auditing a correction
	// will ask.
	var cleared photo.LocationCleared
	if err := pub.Messages[0].Body(&cleared); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if cleared.Reason != "forkert fix" {
		t.Errorf("want the curator's reason on the event, got %q", cleared.Reason)
	}
}

// `photo.Updated` must have no way to express a clear. If `Location: nil` meant "remove", the caption-blanking
// bug would be possible for positions too — which is the reason the two are separate events at all.
func TestUpdatedCannotExpressAClear(t *testing.T) {
	src := adminSource(t, "adminposition.go")
	if !strings.Contains(src, "photo.LocationCleared{") {
		t.Error("clearing must publish photo.LocationCleared")
	}

	// The clear path must not reach for Updated at all.
	//
	// Sliced to the function's own body rather than to the next comment heading: the first version ran to
	// `adminVerdictMessage` and started failing the moment `setAdminPhotoCaptions` was inserted between them — that
	// function legitimately publishes `Updated`, so the test was reading the wrong code.
	start := strings.Index(src, "func (app *application) clearAdminPhotoLocations")
	if start < 0 {
		t.Fatal("clearAdminPhotoLocations no longer exists; this guard needs updating")
	}
	body := src[start:]
	if end := strings.Index(body[1:], "\nfunc "); end >= 0 {
		body = body[:end+1]
	}
	if strings.Contains(body, "photo.Updated{") {
		t.Error("the clear path must not publish photo.Updated; the log would lose the intent")
	}
}

// Exactly one action. Two would be contradictory and zero is a broken client — both refused rather than resolved
// by a precedence rule, because a precedence rule here is a silent decision about forty photographs.
func TestAdminPositionRefusesAmbiguousRequests(t *testing.T) {
	_, srv, pub := positionApp(t)

	for name, body := range map[string]string{
		"nothing at all": `{"photoIds":[` + selection(1) + `]}`,
		"point and post": `{"photoIds":[` + selection(1) + `],"location":{"lat":55.7,"lng":12.2},` +
			`"checkpointId":"cp-3"}`,
		"point and clear": `{"photoIds":[` + selection(1) + `],"location":{"lat":55.7,"lng":12.2},` +
			`"clearLocation":true}`,
		"post and clear": `{"photoIds":[` + selection(1) + `],"checkpointId":"cp-3","clearLocation":true}`,
		"no selection":   `{"photoIds":[],"clearLocation":true}`,
	} {
		resp := patchAdmin(t, srv, body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", name, resp.StatusCode)
		}
	}
	if got := len(pub.Subjects()); got != 0 {
		t.Errorf("a refused request must publish nothing, got %d", got)
	}
}

// 0,0 is refused for the reason `imaging.ReadGPS` refuses it: a real place in the Atlantic, and also what an
// empty form submits. A curator meaning "no position" uses the clear, which is a different fact.
func TestAdminPositionRefusesNullIslandAndNonsense(t *testing.T) {
	_, srv, pub := positionApp(t)

	for name, body := range map[string]string{
		"null island":   `{"photoIds":[` + selection(1) + `],"location":{"lat":0,"lng":0}}`,
		"latitude 91":   `{"photoIds":[` + selection(1) + `],"location":{"lat":91,"lng":12}}`,
		"longitude 181": `{"photoIds":[` + selection(1) + `],"location":{"lat":55,"lng":181}}`,
	} {
		resp := patchAdmin(t, srv, body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", name, resp.StatusCode)
		}
	}
	if got := len(pub.Subjects()); got != 0 {
		t.Errorf("nothing should publish, got %d", got)
	}
}

// A broken stream must not report the positions as set.
func TestAdminPositionWithNoStreamDoesNotClaimToHaveSet(t *testing.T) {
	app, srv, _ := positionApp(t)
	app.commands = commandsWithNoPublisher()

	body := `{"photoIds":[` + selection(1) + `],"location":{"lat":55.7332,"lng":12.2648}}`
	if got := patchAdmin(t, srv, body).StatusCode; got != http.StatusServiceUnavailable {
		t.Errorf("want 503 with no event stream, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// The checkpoint picker.
// ---------------------------------------------------------------------------

func TestAdminCheckpointPickerListsSitedPosts(t *testing.T) {
	_, srv, _ := positionApp(t)

	resp := getAdmin(t, srv, "/api/admin/checkpoints", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var out listAdminCheckpointsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(out.Checkpoints) != 2 {
		t.Fatalf("want 2 posts, got %d", len(out.Checkpoints))
	}
	if out.Checkpoints[0].Name != "Post 3" {
		t.Errorf("want route order, got %q first", out.Checkpoints[0].Name)
	}
}

// An empty list is a normal early-season answer, not a fault: no checkpoint has a position yet. It is also
// exactly when a curator-placed point gets the `unknown` verdict, so the two states are consistent.
func TestAdminCheckpointPickerIsEmptyBeforeTheCourseIsSited(t *testing.T) {
	app, srv, _ := positionApp(t)
	app.models.CheckpointCurator = stubCheckpointCurator{}

	resp := getAdmin(t, srv, "/api/admin/checkpoints", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("an empty course is not an error, got %d", resp.StatusCode)
	}

	// Read once. `adminBody` drains the reader, so decoding first and then reading would give an empty string —
	// which the first version of this test reported as "want an empty array, got ".
	body := adminBody(t, resp)

	var out listAdminCheckpointsResponse
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(out.Checkpoints) != 0 {
		t.Errorf("want an empty list, got %d", len(out.Checkpoints))
	}
	// And a JSON array rather than null, so the client can iterate without a guard.
	if !strings.Contains(body, `"checkpoints":[]`) {
		t.Errorf("want an empty array rather than null, got %s", body)
	}
}

// **The boundary this task had to respect.** `checkpoint.Queries` deliberately has no way to ask for all
// checkpoints, because the event area is not fully known to participants (PRD 002) — both its reads are bounded
// by ids the caller already named.
//
// Adding an `All` there would have been three lines and would have handed every patrol-scoped handler in the app
// a way to enumerate every position in the event. So the enumerating read is on its own interface, and this
// asserts the public one did not grow.
func TestThePatrolScopedCheckpointInterfaceCannotEnumerate(t *testing.T) {
	src := adminSource(t, "../../nathejk/table/checkpoint/querier.go")

	start := strings.Index(src, "type Queries interface {")
	if start < 0 {
		t.Fatal("checkpoint.Queries no longer exists; this guard needs updating")
	}
	end := strings.Index(src[start:], "\n}")
	decl := src[start : start+end]

	for _, smell := range []string{"All(", "Positioned(", "Every(", "List("} {
		if strings.Contains(decl, smell) {
			t.Errorf("checkpoint.Queries has gained %s. Every read there must be bounded by ids the caller "+
				"already named, or a patrol-scoped handler can enumerate the event's geography (PRD 002). The "+
				"curator's list belongs on checkpoint.CuratorQueries.", smell)
		}
	}
}

// Every write logs what happened and from where. With a shared credential the log is the only audit trail.
func TestAdminPositionWritesAreLogged(t *testing.T) {
	src := adminSource(t, "adminposition.go")

	for _, want := range []string{
		`"admin set a position on a selection"`,
		`"admin cleared a position on a selection"`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("missing the audit log line %s", want)
		}
	}
	// The verdict is logged, because "why is nothing on the map" is the question an operator asks, and the
	// answer is usually in it.
	if !strings.Contains(src, `"verdict", verdict`) {
		t.Error("the set must log the verdict it reached")
	}
}
