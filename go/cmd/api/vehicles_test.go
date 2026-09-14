package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nathejk/shared-go/tables"
	"github.com/nathejk/shared-go/tables/vehicle"
	"github.com/nathejk/shared-go/types"

	"nathejk.dk/internal/data"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
)

// fakeVehicles stands in for shared-go's vehicle projection.
//
// It records the filters it was asked for, which is most of the point: the thing
// worth asserting about this endpoint is not the JSON but *what it asked the
// projection for* — the difference between "vehicles I answer for" and "vehicles I
// am driving" is invisible in a response and decides whether a borrower can delete
// somebody else's car.
type fakeVehicles struct {
	all     []vehicle.Vehicle
	err     error
	filters *[]vehicle.Filter
}

func (f fakeVehicles) GetByID(_ context.Context, id types.VehicleID) (*vehicle.Vehicle, error) {
	for i := range f.all {
		if f.all[i].VehicleID == id {
			return &f.all[i], nil
		}
	}
	return nil, tables.ErrRecordNotFound
}

func (f fakeVehicles) GetAll(_ context.Context, filter vehicle.Filter) ([]vehicle.Vehicle, error) {
	if f.filters != nil {
		*f.filters = append(*f.filters, filter)
	}
	if f.err != nil {
		return nil, f.err
	}

	// The filter is honoured rather than ignored, deliberately. A fake that returns
	// everything makes a duplicate-detection test pass whether or not the handler
	// actually narrowed by plate — the same "passes for the wrong reason" failure
	// that the whereOf helper in shared-go's tests had.
	out := []vehicle.Vehicle{}
	for _, v := range f.all {
		if filter.LicensePlate != "" && v.LicensePlate != filter.LicensePlate {
			continue
		}
		if len(filter.CustodianUserIDs) > 0 && !containsUserID(filter.CustodianUserIDs, v.CustodianUserID) {
			continue
		}
		if len(filter.DriverUserIDs) > 0 && !containsUserID(filter.DriverUserIDs, v.DriverUserID) {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

func containsUserID(ids []types.UserID, want types.UserID) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

var _ vehicle.Queries = fakeVehicles{}

// fakeVehicleCommands records what the write side was asked to publish.
type fakeVehicleCommands struct {
	registered *[]vehicle.RegisterFields
	updated    *[]vehicle.UpdateFields
	deleted    *[]types.VehicleID
	id         types.VehicleID
	err        error
}

func (f fakeVehicleCommands) Register(_ context.Context, _ types.YearSlug, fields vehicle.RegisterFields) (types.VehicleID, error) {
	if f.registered != nil {
		*f.registered = append(*f.registered, fields)
	}
	if f.err != nil {
		return "", f.err
	}
	id := f.id
	if id == "" {
		id = "v-new"
	}
	return id, nil
}

func (f fakeVehicleCommands) Update(_ context.Context, _ types.YearSlug, _ types.VehicleID, fields vehicle.UpdateFields) error {
	if f.updated != nil {
		*f.updated = append(*f.updated, fields)
	}
	return f.err
}

func (f fakeVehicleCommands) AssignDriver(context.Context, types.YearSlug, types.VehicleID, types.UserID) error {
	return nil
}

func (f fakeVehicleCommands) AssignSection(context.Context, types.YearSlug, types.VehicleID, types.Slug) error {
	return nil
}

func (f fakeVehicleCommands) Delete(_ context.Context, _ types.YearSlug, id types.VehicleID) error {
	if f.deleted != nil {
		*f.deleted = append(*f.deleted, id)
	}
	return f.err
}

var _ vehicle.Commands = fakeVehicleCommands{}

func vehicleApp(t *testing.T, vehicles vehicle.Queries) *application {
	t.Helper()
	app := newTestApp(t)
	app.models = data.NewModels(users.NewMockDirectory(), scans.NewMockSource(), nil, nil, vehicles)
	return app
}

// The mock directory's phone numbers, so a session resolves to a real role. The
// distinction only matters from the POST tests down, and it matters a lot there:
// +4530000001 is a **spejder**, who may not register a vehicle.
const (
	spejderPhone = "30000001"
	banditPhone  = "30000002"
)

// postVehicle registers as the given member and returns the response.
func postVehicle(t *testing.T, app *application, localPhone, body string) (*http.Response, string) {
	t.Helper()
	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)

	cookies := authedCookies(t, app, srv, localPhone, "+45"+localPhone)
	resp := postJSONWithCookies(t, srv.URL+"/api/me/vehicles", body, cookies)
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, string(out)
}

func getOwnVehicles(t *testing.T, app *application, authed bool) (*http.Response, string) {
	t.Helper()
	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)

	var cookies []*http.Cookie
	if authed {
		cookies = authedCookies(t, app, srv, "30000001", "+4530000001")
	}
	resp := getWithCookies(t, srv.URL+"/api/me/vehicles", cookies)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, string(body)
}

func TestOwnVehiclesRequiresASession(t *testing.T) {
	resp, _ := getOwnVehicles(t, vehicleApp(t, fakeVehicles{}), false)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// The filter is the security boundary, so it is asserted directly.
func TestOwnVehiclesAsksByCustodianNotByDriver(t *testing.T) {
	var filters []vehicle.Filter
	app := vehicleApp(t, fakeVehicles{filters: &filters})
	app.config.eventYear = "2026"

	resp, body := getOwnVehicles(t, app, true)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", resp.StatusCode, body)
	}
	if len(filters) != 1 {
		t.Fatalf("expected one query, got %d", len(filters))
	}
	f := filters[0]
	if len(f.CustodianUserIDs) != 1 {
		t.Fatalf("expected exactly one custodian in the filter, got %v", f.CustodianUserIDs)
	}
	// A driver filter here would silently hand a borrowed car to the borrower's
	// "my vehicles" — and with it the edit and delete rights of task 239.
	if len(f.DriverUserIDs) != 0 {
		t.Errorf("the filter must not narrow by driver, got %v", f.DriverUserIDs)
	}
	if f.YearSlug != types.YearSlug("2026") {
		t.Errorf("year = %q, want 2026", f.YearSlug)
	}
}

// A car whose current driver is somebody else still belongs to its custodian. With
// the fake honouring the custodian filter, this exercises the real path: the
// fixture is returned *because* the handler asked by custodianship.
func TestOwnVehiclesIncludesACarLentToSomebodyElse(t *testing.T) {
	lent := vehicle.Vehicle{
		VehicleID:       "v1",
		LicensePlate:    "DK+AB12345",
		CustodianUserID: "mock-spejder-1",
		DriverUserID:    "39999999",
		SeatCount:       4,
	}
	resp, body := getOwnVehicles(t, vehicleApp(t, fakeVehicles{all: []vehicle.Vehicle{lent}}), true)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", resp.StatusCode, body)
	}

	var got vehiclesResponse
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, body)
	}
	if len(got.Vehicles) != 1 || got.Vehicles[0].LicensePlate != "DK+AB12345" {
		t.Fatalf("expected the lent car in the list, got %+v", got.Vehicles)
	}
	if got.Vehicles[0].SeatCount != 4 {
		t.Errorf("seat_count = %d, want 4", got.Vehicles[0].SeatCount)
	}
}

// The coordinator's half of the entity must not travel to the owner's client. A
// response that does not carry a field cannot leak it, however the client changes.
func TestOwnVehiclesOmitsDispatchFields(t *testing.T) {
	v := vehicle.Vehicle{
		VehicleID:       "v1",
		LicensePlate:    "DK+AB12345",
		CustodianUserID: "mock-spejder-1",
		DriverUserID:    "39999999",
		SectionSlug:     "post-nord",
	}
	_, body := getOwnVehicles(t, vehicleApp(t, fakeVehicles{all: []vehicle.Vehicle{v}}), true)
	for _, unwanted := range []string{"custodian", "driver", "section", "39999999", "post-nord"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("response carries %q, which is the coordinator's half of this entity:\n%s", unwanted, body)
		}
	}
}

// "You have registered nothing" is a successful answer, and an array rather than
// null so no client has to special-case it.
func TestOwnVehiclesEmptyListIs200AndAnArray(t *testing.T) {
	resp, body := getOwnVehicles(t, vehicleApp(t, fakeVehicles{}), true)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", resp.StatusCode, body)
	}
	if !strings.Contains(body, `"vehicles":[]`) {
		t.Errorf("expected an empty array, got:\n%s", body)
	}
}

// The distinction this endpoint has to keep: "you have none" versus "we cannot
// tell". Answering the first when the second is true invites a member to register a
// duplicate of a car that is already in the inventory.
func TestOwnVehiclesUnavailableIsNotAnEmptyList(t *testing.T) {
	resp, body := getOwnVehicles(t, vehicleApp(t, nil), true)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (%s)", resp.StatusCode, body)
	}
	if strings.Contains(body, `"vehicles"`) {
		t.Errorf("an unavailable read model must not answer with a list:\n%s", body)
	}
}

func TestOwnVehiclesReportsAQueryFailure(t *testing.T) {
	app := vehicleApp(t, fakeVehicles{err: errors.New("boom")})
	resp, _ := getOwnVehicles(t, app, true)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

// --- POST /api/me/vehicles (task 238) ---

// registerApp wires both halves: a read model for the duplicate check and a
// recording command side.
func registerApp(t *testing.T, existing []vehicle.Vehicle, registered *[]vehicle.RegisterFields) *application {
	t.Helper()
	app := vehicleApp(t, fakeVehicles{all: existing})
	app.config.eventYear = "2026"
	app.vehicles = fakeVehicleCommands{registered: registered}
	return app
}

func TestRegisterVehicleRequiresASession(t *testing.T) {
	app := registerApp(t, nil, nil)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp := postJSONWithCookies(t, srv.URL+"/api/me/vehicles", `{"license_plate":"ab12345"}`, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// The role gate. A spejder is a minor who does not drive to the event, so there is
// nothing here for them — and the refusal is the server's, not the client's
// decision to hide a form.
func TestRegisterVehicleRefusesASpejder(t *testing.T) {
	var registered []vehicle.RegisterFields
	app := registerApp(t, nil, &registered)

	resp, body := postVehicle(t, app, spejderPhone, `{"license_plate":"ab12345"}`)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (%s)", resp.StatusCode, body)
	}
	if len(registered) != 0 {
		t.Errorf("a refused registration must publish nothing, got %+v", registered)
	}
}

func TestRegisterVehicleStoresTheNormalisedPlate(t *testing.T) {
	var registered []vehicle.RegisterFields
	app := registerApp(t, nil, &registered)

	resp, body := postVehicle(t, app, banditPhone,
		`{"license_plate":"ab 12 345","brand":"VW","model":"Transporter","color":"rød","seat_count":4}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (%s)", resp.StatusCode, body)
	}
	if len(registered) != 1 {
		t.Fatalf("expected one registration, got %d", len(registered))
	}
	got := registered[0]
	// The whole point of normalising before publishing: what lands on the stream is
	// the canonical form, so the next duplicate check can find it.
	if got.LicensePlate != "DK+AB12345" {
		t.Errorf("published plate = %q, want DK+AB12345", got.LicensePlate)
	}
	if got.SeatCount != 4 {
		t.Errorf("seat count = %d, want 4", got.SeatCount)
	}
	// The custodian is the session's user, so a registration cannot be filed under
	// anyone else — which is what task 239's authorisation rests on.
	if got.CustodianUserID == "" {
		t.Error("custodian should be taken from the session")
	}
	if !strings.Contains(body, "DK+AB12345") {
		t.Errorf("the response should echo the canonical plate:\n%s", body)
	}
}

// The custodian must not be settable from the body. A member who could name
// somebody else as custodian could file a car they do not answer for — and, worse,
// take one out of its real custodian's hands.
//
// The guarantee turns out to be stronger than "the field is ignored": `ReadJSON`
// disallows unknown fields, so an attempt to name a custodian is refused outright
// rather than silently dropped. Asserted as a 400 because that is the actual
// behaviour — and it is the better one, since a caller who thinks they set the
// custodian is told they did not.
func TestRegisterVehicleRefusesACustodianInTheBody(t *testing.T) {
	var registered []vehicle.RegisterFields
	app := registerApp(t, nil, &registered)

	resp, body := postVehicle(t, app, banditPhone,
		`{"license_plate":"ab12345","custodian_user_id":"somebody-else"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", resp.StatusCode, body)
	}
	if len(registered) != 0 {
		t.Errorf("nothing should be published, got %+v", registered)
	}
}

// And with no custodian field in play at all, the one that is published is the
// session's user.
func TestRegisterVehicleTakesTheCustodianFromTheSession(t *testing.T) {
	var registered []vehicle.RegisterFields
	app := registerApp(t, nil, &registered)

	resp, body := postVehicle(t, app, banditPhone, `{"license_plate":"ab12345"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (%s)", resp.StatusCode, body)
	}
	if registered[0].CustodianUserID != "mock-bandit-1" {
		t.Errorf("custodian = %q, want the session's user", registered[0].CustodianUserID)
	}
}

func TestRegisterVehicleRejectsAnUnusablePlate(t *testing.T) {
	for _, plate := range []string{"", "   ", "!", "A"} {
		var registered []vehicle.RegisterFields
		app := registerApp(t, nil, &registered)

		resp, body := postVehicle(t, app, banditPhone, `{"license_plate":"`+plate+`"}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("plate %q: status = %d, want 400 (%s)", plate, resp.StatusCode, body)
		}
		if len(registered) != 0 {
			t.Errorf("plate %q: nothing should be published", plate)
		}
	}
}

// Two people registering one car is the likeliest data problem in a
// self-registered inventory, and the cost is a coordinator reconciling two rows for
// one car by hand.
func TestRegisterVehicleRefusesADuplicatePlate(t *testing.T) {
	existing := []vehicle.Vehicle{{
		VehicleID:       "v1",
		LicensePlate:    "DK+AB12345",
		CustodianUserID: "somebody-else",
		YearSlug:        "2026",
	}}
	var registered []vehicle.RegisterFields
	app := registerApp(t, existing, &registered)

	// Typed differently from the stored form on purpose: normalisation is what makes
	// the duplicate detectable at all.
	resp, body := postVehicle(t, app, banditPhone, `{"license_plate":"ab 12 345"}`)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (%s)", resp.StatusCode, body)
	}
	if len(registered) != 0 {
		t.Errorf("a duplicate must publish nothing, got %+v", registered)
	}
	// The conflict says the car is taken, never by whom: a plate maps to a person,
	// and this endpoint is not a lookup surface.
	if strings.Contains(body, "somebody-else") {
		t.Errorf("the conflict must not disclose the other registrant:\n%s", body)
	}
}

// The duplicate check asks for the year and the plate, so last year's cars cannot
// block this year's registration.
func TestRegisterVehicleChecksTheDuplicateWithinTheYear(t *testing.T) {
	var filters []vehicle.Filter
	app := vehicleApp(t, fakeVehicles{filters: &filters})
	app.config.eventYear = "2026"
	app.vehicles = fakeVehicleCommands{}

	resp, body := postVehicle(t, app, banditPhone, `{"license_plate":"ab12345"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (%s)", resp.StatusCode, body)
	}
	if len(filters) != 1 {
		t.Fatalf("expected one duplicate check, got %d", len(filters))
	}
	if filters[0].YearSlug != types.YearSlug("2026") {
		t.Errorf("duplicate check year = %q, want 2026", filters[0].YearSlug)
	}
	if filters[0].LicensePlate != "DK+AB12345" {
		t.Errorf("duplicate check plate = %q, want the normalised form", filters[0].LicensePlate)
	}
}

// A write that could not be published has not happened (PRD 008 §5): the request
// must fail rather than tell a member their car is registered.
func TestRegisterVehicleFailsWhenThePublishFails(t *testing.T) {
	app := vehicleApp(t, fakeVehicles{})
	app.config.eventYear = "2026"
	app.vehicles = fakeVehicleCommands{err: errors.New("broker down")}

	resp, _ := postVehicle(t, app, banditPhone, `{"license_plate":"ab12345"}`)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestRegisterVehicleUnavailableWithNoWritePath(t *testing.T) {
	app := vehicleApp(t, fakeVehicles{})
	app.vehicles = nil

	resp, _ := postVehicle(t, app, banditPhone, `{"license_plate":"ab12345"}`)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}

// An unavailable read model must not block a registration. Refusing a real car
// because we cannot check for a duplicate keeps that car out of the inventory to
// avoid a duplicate row, which is the worse of the two outcomes.
func TestRegisterVehicleProceedsWithoutADuplicateCheck(t *testing.T) {
	var registered []vehicle.RegisterFields
	app := vehicleApp(t, nil)
	app.config.eventYear = "2026"
	app.vehicles = fakeVehicleCommands{registered: &registered}

	resp, body := postVehicle(t, app, banditPhone, `{"license_plate":"ab12345"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (%s)", resp.StatusCode, body)
	}
	if len(registered) != 1 {
		t.Errorf("expected the registration to go through, got %+v", registered)
	}
}

// --- PATCH / DELETE /api/me/vehicles/{id} (task 239) ---

// The bandit's own car, as the projection would return it. "mock-bandit-1" is the
// id the mock directory resolves +4530000002 to, so a session for that phone is
// this vehicle's custodian.
func ownedCar() vehicle.Vehicle {
	return vehicle.Vehicle{
		VehicleID:       "v-mine",
		YearSlug:        "2026",
		LicensePlate:    "DK+AB12345",
		CustodianUserID: "mock-bandit-1",
		DriverUserID:    "mock-bandit-1",
		SeatCount:       4,
	}
}

func requestWithCookies(t *testing.T, method, url, body string, cookies []*http.Cookie) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

// mutateVehicle sends a PATCH or DELETE as the given member.
func mutateVehicle(t *testing.T, app *application, method, localPhone, id, body string) (*http.Response, string) {
	t.Helper()
	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)

	cookies := authedCookies(t, app, srv, localPhone, "+45"+localPhone)
	resp := requestWithCookies(t, method, srv.URL+"/api/me/vehicles/"+id, body, cookies)
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, string(out)
}

func mutateApp(t *testing.T, existing []vehicle.Vehicle, cmds *fakeVehicleCommands) *application {
	t.Helper()
	app := vehicleApp(t, fakeVehicles{all: existing})
	app.config.eventYear = "2026"
	app.vehicles = *cmds
	return app
}

// The delta's whole reason for being pointers: absent leaves a field alone, so a
// one-field edit publishes one field.
func TestUpdateVehiclePublishesOnlyTheFieldsSent(t *testing.T) {
	var updated []vehicle.UpdateFields
	app := mutateApp(t, []vehicle.Vehicle{ownedCar()}, &fakeVehicleCommands{updated: &updated})

	resp, body := mutateVehicle(t, app, http.MethodPatch, banditPhone, "v-mine", `{"color":"blå"}`)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (%s)", resp.StatusCode, body)
	}
	if len(updated) != 1 {
		t.Fatalf("expected one update, got %d", len(updated))
	}
	got := updated[0]
	if got.Color == nil || *got.Color != "blå" {
		t.Errorf("colour should be set, got %v", got.Color)
	}
	for name, field := range map[string]any{
		"brand": got.Brand, "model": got.Model, "description": got.Description,
		"licensePlate": got.LicensePlate, "seatCount": got.SeatCount,
	} {
		if !isNilPointer(field) {
			t.Errorf("%s was absent from the request and must stay nil, got %v", name, field)
		}
	}
}

func isNilPointer(v any) bool {
	switch p := v.(type) {
	case *string:
		return p == nil
	case *uint:
		return p == nil
	default:
		return false
	}
}

// The other half of the pointer decision: present-but-empty clears. Without this,
// a user could never remove a description they no longer want.
func TestUpdateVehicleClearsAFieldSentEmpty(t *testing.T) {
	var updated []vehicle.UpdateFields
	app := mutateApp(t, []vehicle.Vehicle{ownedCar()}, &fakeVehicleCommands{updated: &updated})

	resp, body := mutateVehicle(t, app, http.MethodPatch, banditPhone, "v-mine", `{"description":""}`)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (%s)", resp.StatusCode, body)
	}
	if updated[0].Description == nil {
		t.Fatal("an empty description must be sent as a pointer to \"\", not dropped")
	}
	if *updated[0].Description != "" {
		t.Errorf("description = %q, want empty", *updated[0].Description)
	}
}

// Somebody else's car must be indistinguishable from one that does not exist. A
// 403 would confirm the id names a real registration, turning this into a lookup.
func TestUpdateVehicleAnswers404ForAnotherMembersCar(t *testing.T) {
	someoneElses := ownedCar()
	someoneElses.CustodianUserID = "mock-crew-1"

	var updated []vehicle.UpdateFields
	app := mutateApp(t, []vehicle.Vehicle{someoneElses}, &fakeVehicleCommands{updated: &updated})

	resp, body := mutateVehicle(t, app, http.MethodPatch, banditPhone, "v-mine", `{"color":"blå"}`)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (%s)", resp.StatusCode, body)
	}
	if len(updated) != 0 {
		t.Errorf("nothing should be published, got %+v", updated)
	}
}

// The same answer for an id that names nothing, so the two cannot be told apart.
func TestUpdateVehicleAnswers404ForAnUnknownID(t *testing.T) {
	app := mutateApp(t, nil, &fakeVehicleCommands{})
	resp, _ := mutateVehicle(t, app, http.MethodPatch, banditPhone, "v-nope", `{"color":"blå"}`)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// A borrower must not be able to edit the car they were lent, even while driving it.
func TestUpdateVehicleRefusesTheDriverWhoIsNotCustodian(t *testing.T) {
	lent := ownedCar()
	lent.CustodianUserID = "mock-crew-1"
	lent.DriverUserID = "mock-bandit-1"

	app := mutateApp(t, []vehicle.Vehicle{lent}, &fakeVehicleCommands{})
	resp, _ := mutateVehicle(t, app, http.MethodPatch, banditPhone, "v-mine", `{"color":"blå"}`)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 — driving a car is not custodianship", resp.StatusCode)
	}
}

// A plate change is duplicate-checked with the same helper registration uses.
func TestUpdateVehicleRefusesAPlateAlreadyRegistered(t *testing.T) {
	other := vehicle.Vehicle{VehicleID: "v-other", LicensePlate: "DK+XY98765", CustodianUserID: "mock-crew-1"}
	app := mutateApp(t, []vehicle.Vehicle{ownedCar(), other}, &fakeVehicleCommands{})

	resp, body := mutateVehicle(t, app, http.MethodPatch, banditPhone, "v-mine", `{"license_plate":"xy 98 765"}`)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (%s)", resp.StatusCode, body)
	}
	if strings.Contains(body, "mock-crew-1") {
		t.Errorf("the conflict must not disclose the other registrant:\n%s", body)
	}
}

// Re-submitting a form with the plate unchanged must not report the vehicle as a
// duplicate of itself — the check runs only when the value actually differs.
func TestUpdateVehicleAcceptsItsOwnPlateUnchanged(t *testing.T) {
	var updated []vehicle.UpdateFields
	app := mutateApp(t, []vehicle.Vehicle{ownedCar()}, &fakeVehicleCommands{updated: &updated})

	resp, body := mutateVehicle(t, app, http.MethodPatch, banditPhone, "v-mine", `{"license_plate":"ab 12 345"}`)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (%s)", resp.StatusCode, body)
	}
	if updated[0].LicensePlate == nil || *updated[0].LicensePlate != "DK+AB12345" {
		t.Errorf("the normalised plate should be published, got %v", updated[0].LicensePlate)
	}
}

func TestUpdateVehicleRejectsAnUnusablePlate(t *testing.T) {
	app := mutateApp(t, []vehicle.Vehicle{ownedCar()}, &fakeVehicleCommands{})
	resp, _ := mutateVehicle(t, app, http.MethodPatch, banditPhone, "v-mine", `{"license_plate":"!"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDeleteVehicleWithdrawsOwnCar(t *testing.T) {
	var deleted []types.VehicleID
	app := mutateApp(t, []vehicle.Vehicle{ownedCar()}, &fakeVehicleCommands{deleted: &deleted})

	resp, body := mutateVehicle(t, app, http.MethodDelete, banditPhone, "v-mine", "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (%s)", resp.StatusCode, body)
	}
	if len(deleted) != 1 || deleted[0] != "v-mine" {
		t.Errorf("expected v-mine to be withdrawn, got %v", deleted)
	}
}

// The borrower cannot delete the car they were lent. This is the criterion worth
// the most in this task: an accidental or malicious deletion takes a car out of the
// dispatch pool, and its custodian has no way to know why.
func TestDeleteVehicleRefusesAnotherMembersCar(t *testing.T) {
	someoneElses := ownedCar()
	someoneElses.CustodianUserID = "mock-crew-1"
	someoneElses.DriverUserID = "mock-bandit-1"

	var deleted []types.VehicleID
	app := mutateApp(t, []vehicle.Vehicle{someoneElses}, &fakeVehicleCommands{deleted: &deleted})

	resp, _ := mutateVehicle(t, app, http.MethodDelete, banditPhone, "v-mine", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	if len(deleted) != 0 {
		t.Errorf("nothing should be withdrawn, got %v", deleted)
	}
}

// A second delete answers 404, because a deleted row is invisible to the read API
// and therefore indistinguishable from a car that never existed or belongs to
// somebody else — telling those apart is exactly what must not be possible. So the
// endpoint is idempotent in effect (no second event, the car stays gone) rather
// than in status code.
func TestDeleteVehicleRepeatedIsNotASecondEvent(t *testing.T) {
	var deleted []types.VehicleID
	// The projection no longer returns it, which is what a soft-deleted row looks
	// like to this handler.
	app := mutateApp(t, nil, &fakeVehicleCommands{deleted: &deleted})

	resp, _ := mutateVehicle(t, app, http.MethodDelete, banditPhone, "v-mine", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	if len(deleted) != 0 {
		t.Errorf("a repeat must not publish a second deletion, got %v", deleted)
	}
}

func TestDeleteVehicleFailsWhenThePublishFails(t *testing.T) {
	app := mutateApp(t, []vehicle.Vehicle{ownedCar()}, &fakeVehicleCommands{err: errors.New("broker down")})
	resp, _ := mutateVehicle(t, app, http.MethodDelete, banditPhone, "v-mine", "")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestMutateVehicleRequiresASession(t *testing.T) {
	app := mutateApp(t, []vehicle.Vehicle{ownedCar()}, &fakeVehicleCommands{})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, method := range []string{http.MethodPatch, http.MethodDelete} {
		resp := requestWithCookies(t, method, srv.URL+"/api/me/vehicles/v-mine", `{"color":"blå"}`, nil)
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s status = %d, want 401", method, resp.StatusCode)
		}
	}
}
