package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/internal/data"
	"nathejk.dk/internal/ratelimit"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/glimt"
	"nathejk.dk/nathejk/table/person"
)

// Create + feed tests (task 304).

// glimtApp builds an app with a person record, a glimt read model and a publisher.
func glimtApp(t *testing.T, rows []glimt.Glimt, p person.Person) (*application, *stubGlimt, *cqrstest.Publisher) {
	t.Helper()
	pub := &cqrstest.Publisher{}
	people := &stubPeople{p: p, found: true}
	app := photoTestApp(t, pub, people)
	app.config.eventYear = "2026"
	store := &stubGlimt{rows: rows}
	app.models = data.NewModels(users.NewMockDirectory(), scans.NewMockSource(), nil, people, nil,
		data.WithGlimt(store))
	return app, store, pub
}

// spejderPerson is a mock-directory spejder with a hold.
func spejderPerson() person.Person {
	return person.Person{
		PersonID:   "mock-spejder-1",
		AppRole:    person.RoleSpejder,
		TeamNumber: "42",
		TeamName:   "Ørnene",
	}
}

// postGlimtJSON posts a typed body with cookies.
//
// Distinct from the package's existing `postJSON` (auth_test.go), which takes a raw string and no
// cookies. Named rather than widened because that helper has a dozen callers and this one wants a
// struct and a session.
func postGlimtJSON(t *testing.T, url string, body any, cookies []*http.Cookie) (*http.Response, []byte) {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	payload, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, payload
}

// readBody runs the package's getWithCookies and returns the payload too.
func readBody(t *testing.T, url string, cookies []*http.Cookie) (*http.Response, []byte) {
	t.Helper()
	resp := getWithCookies(t, url, cookies)
	payload, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, payload
}

// publishedGlimtCreated decodes the one created event a test expects.
//
// Decodes rather than inspecting a captured struct, so the assertions run through the same JSON
// round-trip a real consumer does — the convention `publishedPortrait` established, and the reason
// it can catch a missing or misnamed tag.
func publishedGlimtCreated(t *testing.T, pub *cqrstest.Publisher) glimt.Created {
	t.Helper()
	if len(pub.Messages) != 1 {
		t.Fatalf("published %d events, want 1", len(pub.Messages))
	}
	var body glimt.Created
	if err := pub.Messages[0].Body(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

// oneItem is a valid media list for a create request.
func oneItem() []createGlimtMedia {
	return []createGlimtMedia{{
		Ref:      strings.Repeat("a", 64),
		ThumbRef: strings.Repeat("b", 64),
		Kind:     glimt.MediaKindImage,
		Width:    1600,
		Height:   1200,
	}}
}

func TestCreateGlimt_RequiresAuth(t *testing.T) {
	app, _, _ := glimtApp(t, nil, spejderPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, _ := postGlimtJSON(t, srv.URL+"/api/glimt", createGlimtRequest{Media: oneItem()}, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestCreateGlimt_PublishesWithFrozenAttribution(t *testing.T) {
	app, _, pub := glimtApp(t, nil, spejderPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, payload := postGlimtJSON(t, srv.URL+"/api/glimt", createGlimtRequest{
		Caption: "  ved posten  ",
		Media:   oneItem(),
	}, cookies)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", resp.StatusCode, payload)
	}

	subjects := pub.Subjects()
	if len(subjects) != 1 || !strings.HasPrefix(subjects[0], "NATHEJK.2026.glimt.") ||
		!strings.HasSuffix(subjects[0], ".created") {
		t.Fatalf("subjects = %v", subjects)
	}

	var out glimtResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ID == "" {
		t.Error("no id returned; the client needs it to replace its optimistic card")
	}
	// Attribution frozen from the person record at this moment.
	if out.Hold.Number != "42" || out.Hold.Name != "Ørnene" || out.Hold.Group != "spejder" {
		t.Errorf("attribution = %+v", out.Hold)
	}
	if !out.Own {
		t.Error("the creator's own glimt is not marked Own")
	}
	// Caption trimmed, not stored with the whitespace the composer's textarea left behind.
	if out.Caption != "ved posten" {
		t.Errorf("caption = %q", out.Caption)
	}
	// The response must still not carry an author or a blob ref (task 302).
	if strings.Contains(string(payload), "mock-spejder-1") ||
		strings.Contains(string(payload), strings.Repeat("a", 64)) {
		t.Errorf("create response leaked an author id or a blob ref: %s", payload)
	}
}

// TestCreateGlimt_DefaultsToTheNarrowestAudience is the safety default.
//
// A client that forgets the field must not accidentally publish to the open web. Widening is only
// ever the result of an explicit value.
func TestCreateGlimt_DefaultsToTheNarrowestAudience(t *testing.T) {
	app, _, pub := glimtApp(t, nil, spejderPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, payload := postGlimtJSON(t, srv.URL+"/api/glimt",
		createGlimtRequest{Media: oneItem()}, cookies)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d (body %s)", resp.StatusCode, payload)
	}

	body := publishedGlimtCreated(t, pub)
	if body.Audience != glimt.AudienceGroup {
		t.Errorf("audience = %q, want the narrowest (%q) when unspecified", body.Audience, glimt.AudienceGroup)
	}
}

func TestCreateGlimt_AcceptsEachKnownAudience(t *testing.T) {
	for _, audience := range []string{glimt.AudienceGroup, glimt.AudienceNathejk, glimt.AudiencePublic} {
		t.Run(audience, func(t *testing.T) {
			app, _, pub := glimtApp(t, nil, spejderPerson())
			srv := httptest.NewServer(app.routes())
			defer srv.Close()
			cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

			resp, payload := postGlimtJSON(t, srv.URL+"/api/glimt", createGlimtRequest{
				Audience: audience, Media: oneItem(),
			}, cookies)
			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("status = %d (body %s)", resp.StatusCode, payload)
			}
			body := publishedGlimtCreated(t, pub)
			if body.Audience != audience {
				t.Errorf("audience = %q, want %q", body.Audience, audience)
			}
		})
	}
}

// TestCreateGlimt_RejectsAnUnknownAudience is the other half of the defaulting asymmetry.
//
// An absent audience gets the safe value; an unrecognised one is refused outright. A client sending
// "everyone" by mistake must not be quietly read as having said `public`.
func TestCreateGlimt_RejectsAnUnknownAudience(t *testing.T) {
	app, _, pub := glimtApp(t, nil, spejderPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, _ := postGlimtJSON(t, srv.URL+"/api/glimt", createGlimtRequest{
		Audience: "everyone", Media: oneItem(),
	}, cookies)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	if len(pub.Subjects()) != 0 {
		t.Error("a refused create still published an event")
	}
}

func TestCreateGlimt_ValidatesMedia(t *testing.T) {
	tooMany := make([]createGlimtMedia, 0, maxGlimtItems+1)
	for i := 0; i <= maxGlimtItems; i++ {
		tooMany = append(tooMany, createGlimtMedia{
			Ref:  strings.Repeat(string(rune('a'+i)), 64),
			Kind: glimt.MediaKindImage,
		})
	}
	duplicate := []createGlimtMedia{
		{Ref: strings.Repeat("a", 64), Kind: glimt.MediaKindImage},
		{Ref: strings.Repeat("a", 64), Kind: glimt.MediaKindImage},
	}

	cases := map[string][]createGlimtMedia{
		"none":              nil,
		"too many":          tooMany,
		"path-shaped ref":   {{Ref: "../../etc/passwd"}},
		"non-canonical ref": {{Ref: strings.ToUpper(strings.Repeat("a", 64))}},
		"duplicate":         duplicate,
	}
	for name, media := range cases {
		t.Run(name, func(t *testing.T) {
			app, _, pub := glimtApp(t, nil, spejderPerson())
			srv := httptest.NewServer(app.routes())
			defer srv.Close()
			cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

			resp, _ := postGlimtJSON(t, srv.URL+"/api/glimt",
				createGlimtRequest{Media: media}, cookies)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", resp.StatusCode)
			}
			if len(pub.Subjects()) != 0 {
				t.Error("a refused create still published an event")
			}
		})
	}
}

// TestCreateGlimt_CaptionIsCountedInCharacters is why the limit is runes and not bytes.
//
// The captions are Danish: æ/ø/å cost two bytes each in UTF-8, so a byte limit would silently give a
// member writing "på vej gennem skoven" a shorter caption than one writing in ASCII.
func TestCreateGlimt_CaptionIsCountedInCharacters(t *testing.T) {
	app, _, pub := glimtApp(t, nil, spejderPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	// 280 Danish characters — 560 bytes. Must be accepted.
	atLimit := strings.Repeat("æ", maxGlimtCaption)
	resp, payload := postGlimtJSON(t, srv.URL+"/api/glimt",
		createGlimtRequest{Caption: atLimit, Media: oneItem()}, cookies)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("280 Danish characters rejected: %d (body %s)", resp.StatusCode, payload)
	}
	body := publishedGlimtCreated(t, pub)
	if len([]rune(body.Caption)) != maxGlimtCaption {
		t.Errorf("caption is %d characters, want %d", len([]rune(body.Caption)), maxGlimtCaption)
	}

	// One over is refused.
	over := strings.Repeat("æ", maxGlimtCaption+1)
	resp, _ = postGlimtJSON(t, srv.URL+"/api/glimt",
		createGlimtRequest{Caption: over, Media: oneItem()}, cookies)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("281 characters accepted: %d", resp.StatusCode)
	}
}

// TestCreateGlimt_OrdinalsComeFromTheRequestOrder records why the client's ordinal is ignored.
//
// The order is what the author arranged and it is expressed by the array. A client-supplied ordinal
// would be a second source of truth for the same fact, free to disagree with the array it arrived
// in — and the disagreement would show as a silently reordered glimt.
func TestCreateGlimt_OrdinalsComeFromTheRequestOrder(t *testing.T) {
	app, _, pub := glimtApp(t, nil, spejderPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	media := []createGlimtMedia{
		{Ref: strings.Repeat("a", 64), Kind: glimt.MediaKindImage},
		{Ref: strings.Repeat("b", 64), Kind: glimt.MediaKindVideo, DurationMs: 12000},
		{Ref: strings.Repeat("c", 64), Kind: glimt.MediaKindImage},
	}
	resp, payload := postGlimtJSON(t, srv.URL+"/api/glimt",
		createGlimtRequest{Media: media}, cookies)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d (body %s)", resp.StatusCode, payload)
	}

	body := publishedGlimtCreated(t, pub)
	for i, m := range body.Media {
		if m.Ordinal != i {
			t.Errorf("item %d has ordinal %d", i, m.Ordinal)
		}
	}
	if body.Media[1].Kind != glimt.MediaKindVideo {
		t.Errorf("kind lost: %+v", body.Media[1])
	}
}

// TestCreateGlimt_NoPublisherIs503 is what makes the client's outbox safe.
//
// The composer keeps the glimt queued and retries on the next foreground (task 314) — which is only
// correct because the server told it the truth instead of reporting a success nothing recorded.
func TestCreateGlimt_NoPublisherIs503(t *testing.T) {
	app, _, _ := glimtApp(t, nil, spejderPerson())
	app.commands = commandsWithNoPublisher()
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, _ := postGlimtJSON(t, srv.URL+"/api/glimt",
		createGlimtRequest{Media: oneItem()}, cookies)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}

func TestCreateGlimt_RateLimited(t *testing.T) {
	app, _, _ := glimtApp(t, nil, spejderPerson())
	app.glimtLimiter = ratelimit.New(1, time.Hour)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	first, _ := postGlimtJSON(t, srv.URL+"/api/glimt", createGlimtRequest{Media: oneItem()}, cookies)
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first = %d", first.StatusCode)
	}
	second, _ := postGlimtJSON(t, srv.URL+"/api/glimt", createGlimtRequest{Media: oneItem()}, cookies)
	if second.StatusCode != http.StatusTooManyRequests {
		t.Errorf("second = %d, want 429", second.StatusCode)
	}
}

// feedFixture is a mixed set: one of the caller's own, one from their group, one from another
// group, one all-Nathejk, one public, and one hidden.
func feedFixture() []glimt.Glimt {
	at := func(min int) time.Time {
		return time.Date(2026, 9, 17, 21, min, 0, 0, time.UTC)
	}
	hidden := at(5)
	return []glimt.Glimt{
		{GlimtID: "own", AuthorPersonID: "mock-spejder-1", AuthorGroup: "spejder",
			TeamNumber: "42", TeamName: "Ørnene", Audience: glimt.AudienceGroup, CreatedAt: at(1)},
		{GlimtID: "same-group", AuthorPersonID: "other-spejder", AuthorGroup: "spejder",
			TeamNumber: "43", TeamName: "Ulvene", Audience: glimt.AudienceGroup, CreatedAt: at(2)},
		{GlimtID: "other-group", AuthorPersonID: "a-bandit", AuthorGroup: "bandit",
			TeamNumber: "7", TeamName: "Klan Nord", Audience: glimt.AudienceGroup, CreatedAt: at(3)},
		{GlimtID: "all-nathejk", AuthorPersonID: "a-crew", AuthorGroup: "crew",
			TeamName: "Postmandskab", Audience: glimt.AudienceNathejk, CreatedAt: at(4)},
		{GlimtID: "hidden-same-group", AuthorPersonID: "other-spejder", AuthorGroup: "spejder",
			TeamNumber: "43", TeamName: "Ulvene", Audience: glimt.AudienceGroup,
			CreatedAt: at(5), HiddenAt: &hidden},
		{GlimtID: "public", AuthorPersonID: "a-crew", AuthorGroup: "crew",
			TeamName: "PR", Audience: glimt.AudiencePublic, CreatedAt: at(6)},
	}
}

func feedIDs(t *testing.T, payload []byte) []string {
	t.Helper()
	var out glimtFeedResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("decode feed: %v (body %s)", err, payload)
	}
	ids := make([]string, 0, len(out.Glimt))
	for _, g := range out.Glimt {
		ids = append(ids, g.ID)
	}
	return ids
}

func TestFeed_ShowsOnlyWhatTheCallerMaySee(t *testing.T) {
	app, _, _ := glimtApp(t, feedFixture(), spejderPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, payload := readBody(t, srv.URL+"/api/glimt/feed", cookies)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d (body %s)", resp.StatusCode, payload)
	}
	got := feedIDs(t, payload)

	want := map[string]bool{"own": true, "same-group": true, "all-nathejk": true, "public": true}
	for _, id := range got {
		if !want[id] {
			t.Errorf("feed contains %q, which this spejder may not see", id)
		}
		delete(want, id)
	}
	for id := range want {
		t.Errorf("feed is missing %q", id)
	}
}

// TestFeed_ExcludesAnotherGroupsGlimt is the property named in the task, stated on its own so a
// failure is unambiguous.
func TestFeed_ExcludesAnotherGroupsGlimt(t *testing.T) {
	app, _, _ := glimtApp(t, feedFixture(), spejderPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	_, payload := readBody(t, srv.URL+"/api/glimt/feed", cookies)
	for _, id := range feedIDs(t, payload) {
		if id == "other-group" {
			t.Error("a spejder's feed contains a bandit group-scoped glimt")
		}
		if id == "hidden-same-group" {
			t.Error("a spejder's feed contains a hidden glimt they do not own")
		}
	}
}

// TestFeed_DerivesANarrowingFilter checks the handler asked the right question, not just that the
// stub answered it. A handler that passed an unrestricted filter would pass the test above only
// because the stub happens to filter.
func TestFeed_DerivesANarrowingFilter(t *testing.T) {
	app, store, _ := glimtApp(t, feedFixture(), spejderPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	readBody(t, srv.URL+"/api/glimt/feed", cookies)

	if len(store.filters) == 0 {
		t.Fatal("the feed was never queried")
	}
	f := store.filters[0]
	if f.Unrestricted {
		t.Error("a spejder's feed was queried with an unrestricted filter")
	}
	if f.Denied {
		t.Error("a spejder's feed was denied outright")
	}
	if f.Group != "spejder" {
		t.Errorf("filter group = %q, want spejder", f.Group)
	}
	if f.PersonID == "" {
		t.Error("filter carries no person id, so the caller's own glimt would be invisible to them")
	}
}

func TestFeed_MissingProjectionIsUnavailableNotEmpty(t *testing.T) {
	// An empty feed is a legitimate state the client caches. Reporting one because the
	// database is down would leave a device showing "nothing was shared" all event.
	pub := &cqrstest.Publisher{}
	people := &stubPeople{p: spejderPerson(), found: true}
	app := photoTestApp(t, pub, people)
	app.models = data.NewModels(users.NewMockDirectory(), scans.NewMockSource(), nil, people, nil)

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, _ := readBody(t, srv.URL+"/api/glimt/feed", cookies)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}

func TestFeed_RequiresAuth(t *testing.T) {
	app, _, _ := glimtApp(t, feedFixture(), spejderPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, _ := readBody(t, srv.URL+"/api/glimt/feed", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// TestFeed_RechecksThePredicateOnTheWayOut is the belt-and-braces property.
//
// The stub is made to return a row the predicate refuses, simulating a WHERE clause that has drifted
// from `MaySeeGlimt`. The handler must drop it: the predicate is the authority and the SQL is an
// optimisation, and this is where that ordering is made true rather than merely stated.
func TestFeed_RechecksThePredicateOnTheWayOut(t *testing.T) {
	app, store, _ := glimtApp(t, nil, spejderPerson())
	// A bandit group-scoped glimt, returned regardless of the filter.
	store.rows = []glimt.Glimt{{
		GlimtID: "leaked", AuthorPersonID: "a-bandit", AuthorGroup: "bandit",
		Audience: glimt.AudienceGroup, CreatedAt: time.Now().UTC(),
	}}
	store.ignoreFilter = true

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	_, payload := readBody(t, srv.URL+"/api/glimt/feed", cookies)
	for _, id := range feedIDs(t, payload) {
		if id == "leaked" {
			t.Error("a row the predicate refuses was serialised; the predicate is not the authority")
		}
	}
}

// TestSync_CarriesAGlimtKeyForEveryRole records the deviation from PRD 019 §8.
//
// The PRD asked for `GET /api/glimt/version`. routes.go says outright not to add another per-dataset
// version endpoint — task 292 retired the last one — so freshness is a key here instead.
func TestSync_CarriesAGlimtKeyForEveryRole(t *testing.T) {
	app := syncApp(t)
	_, out := getSync(t, app, "+4530000001", "")
	if out.Versions["glimt"] == "" {
		t.Error("no glimt version; the client has no way to know its feed went stale")
	}
	for _, unwanted := range out.Unavailable {
		if unwanted == "glimt" {
			t.Error("glimt reported unavailable with a projection present")
		}
	}
}

func TestSync_MissingGlimtProjectionIsUnavailableNotAbsent(t *testing.T) {
	// Absence means "you may not hold this", which a client cannot recover from — it stops
	// asking and nothing ever tells it otherwise. A missing projection is a transient fault.
	app := syncApp(t, func(a *application) {
		a.models = data.NewModels(users.NewMockDirectory(), scans.NewMockSource(),
			fakeRaceAreas{area: areaFixture(), ok: true},
			&stubPeople{found: true}, nil,
			data.WithMapReads(fakeMapReads{}))
	})
	_, out := getSync(t, app, "+4530000001", "")

	if _, present := out.Versions["glimt"]; present {
		t.Error("an unavailable dataset must not also carry a version")
	}
	found := false
	for _, name := range out.Unavailable {
		if name == "glimt" {
			found = true
		}
	}
	if !found {
		t.Errorf("glimt is neither versioned nor unavailable: %+v", out)
	}
}
