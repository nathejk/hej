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
	return f.all, f.err
}

var _ vehicle.Queries = fakeVehicles{}

func vehicleApp(t *testing.T, vehicles vehicle.Queries) *application {
	t.Helper()
	app := newTestApp(t)
	app.models = data.NewModels(users.NewMockDirectory(), scans.NewMockSource(), nil, nil, vehicles)
	return app
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

// A car whose current driver is somebody else still belongs to its custodian. The
// endpoint returns whatever the custodian filter matched, so this is really an
// assertion that the handler does no second-guessing of its own on the way out.
func TestOwnVehiclesIncludesACarLentToSomebodyElse(t *testing.T) {
	lent := vehicle.Vehicle{
		VehicleID:       "v1",
		LicensePlate:    "DK+AB12345",
		CustodianUserID: "30000001",
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
		CustodianUserID: "30000001",
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
