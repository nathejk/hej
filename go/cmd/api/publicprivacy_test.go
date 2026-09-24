package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"nathejk.dk/nathejk/table/album"
	"nathejk.dk/nathejk/table/glimt"
	"nathejk.dk/nathejk/table/person"
	"nathejk.dk/nathejk/table/publicpatrol"
)

// The public surface names no person (PRD 011 §6, §8; task 337) — with one written-down exception (task 393).
//
// # What this file is for
//
// The entire privacy claim of the public frontpage is that **it names no human being**. A patrol is a
// patrol — a number, a name, a gruppe, a korps — and its route is the patrol's route. That is what makes
// publishing a merged track defensible (PRD 011 §0b.1), and it is why per-member tracks are forbidden
// rather than deferred.
//
// A claim like that is worth exactly as much as the test behind it. So this file is the test.
//
// # The one exception, and why the claim survives it
//
// Since task 393 a photograph may carry a **photographer's credit line** — *"Foto: Anne Sørensen"* — and that
// renders on the public album page. So the claim is no longer "no name appears" but something narrower, and it
// is stated here rather than discovered in a diff:
//
//   - **The only person the public surface may name is somebody who asked to be named**, in a professional
//     capacity, as the author of a photograph. Nobody else: not a participant, not a minor, not a guardian,
//     not a crew member, not a leader.
//   - **A credit is typed, never derived.** It is free text a curator entered. Nothing joins it to the
//     `person` projection, and `TestACreditIsOnlyEverTypedNeverDerived` is what keeps that true.
//
// The second point is the one doing the work. What this file has always really defended is not the absence of
// characters that spell a name — it is that **this service does not take names out of its person records and
// put them on public pages.** A curator typing an attribution does not do that. A lookup would, and is the
// thing to keep failing.
//
// So `isPersonShaped` now flags `credit` and immediately excepts the exact field, which is deliberate: the
// exception is written down in the guard rather than being invisible to it.
//
// # Enumeration, not a hand-written list
//
// The routes are read out of `routes.go` by parsing it, the same way the OpenAPI guard does. That is the
// point: **a public route added later must be covered without anybody remembering to add it here.** A
// list of paths in a test file is a list that goes stale on the first busy afternoon.
//
// # Fixture values, not just field names
//
// Asserting that no response contains the string "phoneParent" would catch a JSON field and miss a name
// rendered into HTML — which is the leak that actually matters on a server-rendered surface. So the
// fixtures carry distinctive personal values and the assertions look for *those* as well.

// publicRoutePaths returns every route registered under the public surface, with its method.
//
// The event-year prefix and `/api/public/*`. Both, because the surface is split across them by content
// type rather than by audience — the pages are under one and the bytes and JSON under the other, and a
// leak is equally bad in either.
func publicRoutePaths(t *testing.T) []registeredRoute {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "routes.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse routes.go: %v", err)
	}

	var out []registeredRoute
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "HandlerFunc" || len(call.Args) < 3 {
			return true
		}
		method := httpMethodName(call.Args[0])
		path, ok := parseRegisteredPath(t, call.Args[1], fset)
		if !ok || method == "" {
			return true
		}
		// Behind the admin credential is not public, whatever the prefix: the curator pages live under the
		// year beside the public ones (task 396). Decided by the wrapper, so a curator page that lost its
		// `requireAdmin` would land in this walk and be tested as public — which is the safe way round.
		if !isPublicSurface(path) || wrapsRequireAdmin(call.Args[2]) {
			return true
		}
		// GET only. The public surface's writes are the anonymous report and the Team-section removals;
		// a 204 has no body to leak, and asserting on one would just be noise.
		if method != http.MethodGet {
			return true
		}
		out = append(out, registeredRoute{
			method:  method,
			path:    path,
			handler: handlerName(call.Args[2]),
			line:    fset.Position(call.Pos()).Line,
		})
		return true
	})
	return out
}

// isPublicSurface reports whether a path is served to the open web.
//
// Deliberately broader than "the routes task 332 added": it catches anything a later task registers
// under either prefix, which is the whole reason this is a predicate rather than a list.
//
// The pages are matched by the **shape** of a year prefix (`/2026`, `/2027`) rather than by this year's
// number, because next year's deployment must not quietly empty this guard.
func isPublicSurface(path string) bool {
	return strings.HasPrefix(path, "/api/public/") || looksLikeYearPrefix(path)
}

// The personal values the fixtures carry. Distinctive enough that a substring match means a real leak
// rather than a coincidence in CSS or Danish prose.
const (
	leakName        = "Astrid Mortensen"
	leakOwnPhone    = "+4530000042"
	leakParentPhone = "+4530000043"
	leakPortraitRef = "portraitrefportraitrefportraitrefportraitrefportraitrefportraitre"
	leakPersonID    = "person-id-that-must-not-appear"
)

// leakyPerson is a directory record stuffed with every personal field the surface must not carry.
func leakyPerson() person.Person {
	parent := leakParentPhone
	return person.Person{
		PersonID:    leakPersonID,
		AppRole:     person.RoleSpejder,
		Name:        leakName,
		Phone:       leakOwnPhone,
		PhoneParent: &parent,
		Address:     "Skovvej 12",
		Email:       "astrid@example.invalid",
		TeamNumber:  "42",
		TeamName:    "Ørnene",
		PortraitRef: leakPortraitRef,
		SectionSlug: person.SectionTeam,
	}
}

// leakTestApp wires every public read model with data carrying personal values.
func leakTestApp(t *testing.T) (*application, *httptest.Server) {
	t.Helper()

	p := leakyPerson()
	app, glimtStore, _ := glimtApp(t, leakyGlimt(), p)
	app.publicGlimtReadLimiter = nil
	app.config.eventYear = "2026"

	store := seedAlbums(t, app)
	// A caption carrying a name, because a caption is participant- or curator-authored free text and is
	// the one field on this surface where a name can legitimately be typed by a human. It must not be
	// scrubbed — that would be censoring a caption — so this asserts the *structured* fields are clean
	// rather than that the word never appears. See the assertion's comment.
	app.models.Albums = store
	app.models.Glimt = glimtStore
	app.models.People = &stubPeople{p: p, found: true}

	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)
	return app, srv
}

func leakyGlimt() []glimt.Glimt {
	at := time.Now().UTC().Add(-time.Hour)
	return []glimt.Glimt{
		{
			GlimtID: "g-public", AuthorPersonID: leakPersonID, AuthorGroup: "spejder",
			TeamNumber: "42", TeamName: "Ørnene", Audience: glimt.AudiencePublic,
			Caption: "ved posten", CreatedAt: at,
			Media: []glimt.Media{
				{Ordinal: 0, Ref: refA, ThumbRef: refB, Kind: "image", Width: 1600, Height: 1200},
			},
		},
	}
}

// **The assertion.** Every public GET route, walked, with every personal value looked for.
func TestNoPublicResponseNamesAPerson(t *testing.T) {
	_, srv := leakTestApp(t)

	routes := publicRoutePaths(t)
	if len(routes) == 0 {
		t.Fatal("no public routes found: the enumeration is broken, which would make this whole file " +
			"pass while asserting nothing")
	}

	// What must never appear, in any representation.
	forbidden := map[string]string{
		leakName:        "a person's name",
		leakOwnPhone:    "a member's own phone number",
		leakParentPhone: "a guardian's phone number (.rules calls this a hard rule)",
		leakPortraitRef: "a portrait reference",
		leakPersonID:    "a person id",
		// Field names too, which catch a JSON payload that grew a column even if the fixture value
		// happened not to be populated.
		"phoneParent":    "the phoneParent field",
		"authorPersonId": "the glimt author field",
		"portraitRef":    "the portrait field",
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			url := srv.URL + concreteURL(route.path)
			resp, body := getPublic(t, url, nil)

			// A 404 or 503 is a fine answer — several of these routes are gated or need data this app
			// does not have. What matters is that whatever comes back is clean.
			if resp.StatusCode >= 500 && resp.StatusCode != http.StatusServiceUnavailable {
				t.Fatalf("%s answered %d; a server error is not a pass for this test", url, resp.StatusCode)
			}

			page := string(body)
			for needle, what := range forbidden {
				if strings.Contains(page, needle) {
					t.Errorf("%s %s (line %d) leaks %s (%q)",
						route.method, route.path, route.line, what, needle)
				}
			}
		})
	}
}

// concreteURL fills httprouter's `:params` with values the fixtures actually have, so the walk exercises
// real responses rather than a parade of 404s.
//
// An unknown parameter gets a plausible value rather than being skipped: a 404 is still a response, and
// a 404 body that leaked a name would be exactly the kind of thing nobody looks at.
func concreteURL(path string) string {
	replacements := map[string]string{
		":slug":    "loerdag-morgen",
		":albumId": "al-1",
		":ordinal": "0",
		":glimtId": "g-public",
		":number":  "42",
	}
	for param, value := range replacements {
		path = strings.ReplaceAll(path, param, value)
	}
	return path
}

// The enumeration must actually cover the routes we know exist. Guards against a parser change that
// silently narrows the walk — which would leave the test above green and blind.
func TestPublicRouteEnumerationCoversTheKnownSurface(t *testing.T) {
	found := map[string]bool{}
	for _, route := range publicRoutePaths(t) {
		found[route.path] = true
	}

	for _, want := range []string{
		// The public pages, under the event-year prefix (task 351). `guardYear` rather than a literal, so
		// this list says "this year's prefix" rather than pinning a year the routes will outlive.
		guardYear,
		guardYear + "/glimt",
		guardYear + "/album/:slug",
		guardYear + "/patrulje/:number",
		"/api/public/glimt",
		"/api/public/albums/:albumId/media/:ordinal",
	} {
		if !found[want] {
			t.Errorf("the enumeration missed %s; every public route must be covered", want)
		}
	}
}

// **The structural half.** The invariant is enforced in the projections, not in the templates: the
// public patrol read model has no name column, so a careless template cannot render one.
//
// This is not theoretical. The row that patrol data comes from — shared-go's `patrulje` — carries
// `contactName`, a personal name sitting directly beside the `groupName` and `korps` the header wants.
// One `SELECT *` is all it would take.
func TestPublicAlbumReadModelHasNowhereToPutAPerson(t *testing.T) {
	// album.Album and album.Item are the types the public album pages render. Asserted by construction:
	// if somebody adds a person-shaped field, this fails and they have to justify it here.
	//
	// A reflective field-name check rather than a comment, because a comment does not fail a build.
	for _, field := range structFieldNames(album.Album{}) {
		if isPersonShaped(field) {
			t.Errorf("album.Album gained a person-shaped field %q: the public surface must name no person",
				field)
		}
	}
	for _, field := range structFieldNames(album.Item{}) {
		if isPersonShaped(field) {
			t.Errorf("album.Item gained a person-shaped field %q: the public surface must name no person",
				field)
		}
	}
	for _, field := range structFieldNames(album.PlottableItem{}) {
		if isPersonShaped(field) {
			t.Errorf("album.PlottableItem gained a person-shaped field %q", field)
		}
	}
}

// And the view types the templates are handed.
func TestPublicViewTypesHaveNowhereToPutAPerson(t *testing.T) {
	for name, v := range map[string]any{
		"publicAlbumSummary": publicAlbumSummary{},
		"publicAlbumItem":    publicAlbumItem{},
		"publicPageData":     publicPageData{},
	} {
		for _, field := range structFieldNames(v) {
			if isPersonShaped(field) {
				t.Errorf("%s gained a person-shaped field %q", name, field)
			}
		}
	}
}

// **The patrol read model, which is where this check earns its keep** (task 338).
//
// The row `publicpatrol` is folded from carries a leader's name, phone, email, role, address and
// postcode — `messages.NathejkTeamUpdated` has all of them, and shared-go's own `patrulje` projection
// stores them because the organizers need them. This projection drops them, and this test is what stops
// somebody adding one back "just for the header".
func TestPublicPatrolTypeHasNowhereToPutAPerson(t *testing.T) {
	fields := structFieldNames(publicpatrol.Patrol{})
	if len(fields) == 0 {
		t.Fatal("no fields found: the reflection is broken, which would make this pass while asserting nothing")
	}

	for _, field := range fields {
		if isPersonShaped(field) {
			t.Errorf("publicpatrol.Patrol gained a person-shaped field %q. The event it is folded from "+
				"carries contactName, contactPhone and contactEmail; this projection exists to drop them", field)
		}
	}

	// The header's fields must all still be there, so "no person" cannot be achieved by removing the
	// feature. A reviewer reads five names and is done.
	want := map[string]bool{"TeamID": true, "Number": true, "Name": true, "GroupName": true, "Korps": true}
	got := map[string]bool{}
	for _, f := range fields {
		got[f] = true
	}
	for f := range want {
		if !got[f] {
			t.Errorf("publicpatrol.Patrol lost %q, which the page header needs", f)
		}
	}
	for f := range got {
		if !want[f] {
			t.Errorf("publicpatrol.Patrol gained %q. Not necessarily wrong — but this type is the public "+
				"surface's boundary, so a new field belongs in this test's list deliberately", f)
		}
	}
}

// **A credit is only ever typed, never derived** (task 393).
//
// This is the property the credit-line exception rests on, and the only one worth a structural guard. A name on
// a public page is a judgement somebody made; a name *looked up from our person records* and put on a public
// page is a different thing entirely, and it is what every other assertion in this file exists to prevent.
//
// So: the write path for a credit must reach it from the request body and nowhere else. If a future edit
// resolves a credit from `person`, from a session, or from a phone number, this fails — and it should, loudly,
// because that is the change that would turn a consented attribution into a directory.
func TestACreditIsOnlyEverTypedNeverDerived(t *testing.T) {
	src, err := os.ReadFile("adminposition.go")
	if err != nil {
		t.Fatalf("reading adminposition.go: %v", err)
	}
	text := string(src)

	setter := text[strings.Index(text, "func (app *application) setAdminPhotoCredits"):]
	setter = setter[:strings.Index(setter, "\n// maxAdminCredit")]
	if setter == "" {
		t.Fatal("could not find setAdminPhotoCredits; this guard needs updating")
	}

	// The value comes from the parameter the handler passed in from the request body. Nothing else.
	if !strings.Contains(setter, "Credit:    &credit,") {
		t.Error("the credit written to the event must be the one the request carried")
	}

	for _, forbidden := range []struct{ needle, why string }{
		{"models.People", "a credit must never be looked up in the person projection"},
		{"PersonID", "a credit must not be resolved from a person id"},
		{"contextGetSession", "a credit must not be taken from whoever is signed in — and on this surface " +
			"there is nobody signed in anyway, which is the point of PRD 022 §8.2"},
		{"Name", "a credit must not be assembled from anybody's name field"},
	} {
		if strings.Contains(setter, forbidden.needle) {
			t.Errorf("setAdminPhotoCredits mentions %q: %s", forbidden.needle, forbidden.why)
		}
	}

	// And the projection's fold writes it from the event, not from a join.
	fold, ferr := os.ReadFile("../../nathejk/table/photo/consumer.go")
	if ferr != nil {
		t.Fatalf("reading photo/consumer.go: %v", ferr)
	}
	if !strings.Contains(string(fold), `sets = append(sets, "credit="+quote(truncateRunes(*body.Credit, maxCreditRunes)))`) {
		t.Error("the fold must write the credit straight from the event body")
	}
	if strings.Contains(string(fold), "JOIN person") {
		t.Error("the photo fold must not join the person projection for any reason")
	}
}

// isPersonShaped flags field names that would carry something about a human being.
//
// A denylist of substrings rather than an allowlist of permitted fields, deliberately: an allowlist has
// to be extended for every legitimate new field, so it gets extended without thought, and the one time
// it matters somebody adds `CuratorName` to the allowlist along with everything else. A denylist fails
// only when a field genuinely looks personal, which is when a human should look.
func isPersonShaped(field string) bool {
	lower := strings.ToLower(field)

	// Known-safe names, listed one by one rather than by loosening a needle.
	//
	// `photoId` / `photoIds` are **content hashes of curated photographs** (PRD 022 §8.3) — an album item names the
	// library photograph it displays, and a bulk action names the selection it applies to. Neither is a picture of
	// a person, and neither identifies one.
	//
	// The `photo` needle below stays blunt on purpose, and this list is why that is affordable. In this codebase a
	// person's picture is consistently a **portrait** (`PortraitRef`, `PortraitThumbRef`, `imaging.Portrait`),
	// while "photo" means a photograph as an object — so dropping the needle would cost little today and would stop
	// catching a genuinely bad `PhotoOfPerson` or `PersonPhotoRef` tomorrow. An allowlist keeps the trap set and
	// makes each exception a deliberate line somebody had to write here, next to the reasoning.
	switch lower {
	case "photoid", "photoids":
		return false
	// A **collection** of photographs, and a flag about one (task 381). Same distinction as above: these
	// name photographs as objects, not people. `Photos` is the library page's list; `PhotoDeleted` is the
	// album view's "the photograph behind this membership is gone", which is precisely the field that keeps
	// a deleted photograph from being rendered.
	case "photos", "photodeleted":
		return false
	// The photograph an album opens with (task 391). A content hash of a curated photograph, like `photoId`
	// above — an album's cover, not a portrait of anybody. Listed separately rather than by loosening the
	// `photoid` case to a prefix match, because `PhotoIdentityOf` would then pass too.
	case "coverphotoid":
		return false
	// A drag-and-drop's landing place (task 396): the album photograph the moved ones go before or after. The same
	// content hash as `photoId`, naming a position in an album rather than anybody in it.
	case "beforephotoid", "afterphotoid":
		return false

	// **The photographer's credit line** (task 393). The one field in this service that is *meant* to name a
	// human being, and the only exception to the claim in this file's header.
	//
	// `credit` was added to the needles below **so that this exception has to be written down.** It matches no
	// other needle and does not end in "By", so without that it would have slipped through unnoticed — which is
	// the worst available outcome: the one field that intentionally carries a name would be the one field this
	// guard says nothing about.
	//
	// What makes it acceptable is not that it is small, it is the bounds:
	//
	//   - it names a **consenting adult volunteer in a professional capacity**, because they asked to be
	//     credited. Not a participant, not a minor, not somebody who never agreed to be in this app.
	//   - it is **free text a curator typed** — never derived, never looked up, never joined to the `person`
	//     projection. `TestACreditIsOnlyEverTypedNeverDerived` holds that, and it is the property that matters:
	//     the hazard was never that a name appears on a page, it is a system that starts deriving names from its
	//     person records and publishing them. A string somebody typed cannot do that.
	//
	// Only the exact name is excepted. `CreditName`, `CreditedBy` and `CreditPersonID` all still fail — the
	// first two because the needle and the "By" suffix catch them, the last because `person` does.
	case "credit":
		return false
	}

	for _, needle := range []string{
		"person", "phone", "portrait", "photo", "author", "curator", "uploader",
		"contactname", "email", "birth", "address",
		// `credit` is here **so the exception above is forced to be explicit** (task 393), not because a credit
		// line is forbidden. Without it the one deliberately person-naming field in the service would pass this
		// walk in silence, and any relative of it — `CreditName`, `CreditLine` — would too.
		"credit",
	} {
		if strings.Contains(lower, needle) {
			// "Photos" as a collection of pictures is fine; a "photo" of somebody is not. See the
			// allowlist above for the names that have had to be excepted.
			return true
		}
	}

	// **Anything ending in "By"** (task 381). A suffix rather than a needle, because the field this whole
	// check exists to stop is called `UploadedBy`, and none of the substrings above matches it — found by
	// adding the field and watching the test pass, which is the only way that gap was ever going to surface.
	//
	// In this codebase `…By` names an actor without exception: `HiddenBy`, `hiddenBy`, `uploadedBy`. That is
	// the shape PRD 022 §8.2 makes impossible to fill honestly anyway — the credential is shared, so there is
	// no actor to name. A false positive here (`standby`, `nearby`) fails loudly and is excepted above with a
	// reason, which is the right direction for this trap to err in.
	if strings.HasSuffix(lower, "by") {
		return true
	}
	// A bare "Name" is ambiguous — an album has a title, a patrol has a name — so it is not flagged.
	// The fixture-value assertions above are what catch a person's name arriving in one.
	return false
}

// structFieldNames returns a struct's exported field names.
//
// Reflection rather than a hand-maintained list, so the check applies to fields added later without
// anybody updating this file — the same reason the route walk parses routes.go.
func structFieldNames(v any) []string {
	t := reflect.TypeOf(v)
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}
	var out []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous {
			// Embedded structs are walked too: publicAlbumPageData embeds publicPageData, and a
			// person-shaped field would be just as public for being one level down.
			out = append(out, structFieldNames(reflect.New(f.Type).Elem().Interface())...)
			continue
		}
		out = append(out, f.Name)
	}
	return out
}
