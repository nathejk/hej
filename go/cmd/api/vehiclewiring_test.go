package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jrgensen/cqrs/cqrstest"
	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/tables/vehicle"

	"nathejk.dk/internal/commands"
	"nathejk.dk/internal/data"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
)

// Wiring tests for the vehicle entity (task 247).
//
// Everything in vehicles_test.go injects a fake `app.vehicles`, which is right for
// testing handler behaviour and is precisely why a registration could panic in the dev
// stack while the whole suite was green: **no test used the real entity**, so nothing
// exercised what main.go actually hands it.
//
// So these build the genuine `vehicle.New(...)` over the same lazyPublisher main.go
// uses, and drive it through the HTTP handler. They are the tests that would have
// caught the nil publisher.

// migrationReader is a database that answers only what vehicle.New's startup migration
// asks.
//
// Needed because `vehicle.New` runs `cqrs.EnsureColumn` for the `kind` column, which
// queries INFORMATION_SCHEMA through the reader — so the entity cannot be constructed
// with a nil one. My first version of this file passed nil and panicked inside the
// migration, which is a fair warning about how much the constructor does; production is
// unaffected, since `openEventing` refuses a nil database outright and so `ev != nil`
// always implies a reader.
func migrationReader(t *testing.T) *sql.DB {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// The column already exists, which is the state every run after the first is in.
	mock.MatchExpectationsInOrder(false)
	mock.ExpectQuery("INFORMATION_SCHEMA").WillReturnRows(
		sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1),
	)
	return db
}

// vehicleWiredApp wires the real entity the way main.go does, over the given holder.
func vehicleWiredApp(t *testing.T, holder *commands.PublisherHolder) *application {
	t.Helper()
	app := newTestApp(t)
	app.config.eventYear = "2026"

	// A cqrstest.Writer stands in for the projection writer, and the reader answers only
	// the startup migration: the command side is what is under test here.
	table := vehicle.New(lazyPublisher{holder: holder}, &cqrstest.Writer{}, migrationReader(t))

	app.vehicles = vehicleCommandsOrNil(table)
	// The read model stays a fake: the duplicate check is not what is being tested,
	// and a real querier would need a database.
	app.models = data.NewModels(users.NewMockDirectory(), scans.NewMockSource(), nil, nil, fakeVehicles{})
	return app
}

// The regression. Before this fix, main.go passed `ev.publisherOrNil()` — nil at wiring
// time, and captured forever — so the first registration dereferenced a nil publisher
// inside shared-go's commander and the request died with a panic and a stack trace.
//
// The assertion is deliberately about the *response*: a 500 with no body is what a
// panicking handler produces, so asserting a clean 503 with a message is what
// distinguishes "failed properly" from "crashed".
func TestRegisterVehicleWithNoBrokerFailsCleanly(t *testing.T) {
	app := vehicleWiredApp(t, commands.NewPublisherHolder())

	resp, body := postVehicle(t, app, banditPhone, `{"license_plate":"ca63640","brand":"Citroen","seat_count":6}`)

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (%s)", resp.StatusCode, body)
	}
	// A panic recovered by net/http sends an empty body; a real answer carries JSON.
	if !strings.Contains(body, `"error"`) {
		t.Errorf("expected a JSON error, got %q", body)
	}
}

// The half that a "check before publishing" fix would have missed entirely.
//
// The publisher arrives *after* wiring, always — the broker is connected in the
// background. So the entity must pick it up without being rebuilt, or registration stays
// broken for the life of the process even though the log says "jetstream connected".
func TestRegisterVehicleWorksOnceTheBrokerArrives(t *testing.T) {
	holder := commands.NewPublisherHolder()
	app := vehicleWiredApp(t, holder)

	// Exactly the ordering production has: wire first, connect later.
	pub := &cqrstest.Publisher{}
	holder.Set(pub)

	resp, body := postVehicle(t, app, banditPhone,
		`{"license_plate":"ca 63 640","brand":"Citroen","model":"C4","color":"Grå","seat_count":6}`)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (%s)", resp.StatusCode, body)
	}
	if len(pub.Messages) != 1 {
		t.Fatalf("want one published event, got %d", len(pub.Messages))
	}
	if !pub.Messages[0].Subject().Match("NATHEJK.2026.vehicle.*.registered") {
		t.Errorf("unexpected subject %q", pub.Messages[0].Subject().Subject())
	}

	// The event carries what the form said, with the plate normalised — end to end
	// through the real commander rather than a fake that records whatever it is given.
	var event messages.NathejkVehicleRegistered
	if err := pub.Messages[0].Body(&event); err != nil {
		t.Fatalf("decoding the event: %v", err)
	}
	if event.LicensePlate != "DK+CA63640" {
		t.Errorf("plate = %q, want the canonical form", event.LicensePlate)
	}
	if event.SeatCount != 6 {
		t.Errorf("seatCount = %d, want 6", event.SeatCount)
	}
	if event.CustodianUserId == "" {
		t.Error("the custodian should be the session's user")
	}

	// And the response names the vehicle the event created.
	var created vehicleResponse
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("decoding the response: %v (%s)", err, body)
	}
	if created.ID != string(event.VehicleID) {
		t.Errorf("response id %q does not match the published vehicle %q", created.ID, event.VehicleID)
	}
}

// A broker that goes away between the availability check and the publish is a real race,
// not a theoretical one, and it must not panic either — which is why lazyPublisher's
// fallback MessageFunc exists rather than returning nil.
func TestVehicleCommandsSurviveAPublisherDisappearing(t *testing.T) {
	holder := commands.NewPublisherHolder()
	holder.Set(&cqrstest.Publisher{})
	table := vehicle.New(lazyPublisher{holder: holder}, &cqrstest.Writer{}, migrationReader(t))

	holder.Set(nil)

	_, err := table.Register(context.Background(), "2026", vehicle.RegisterFields{
		LicensePlate:    "DK+CA63640",
		CustodianUserID: "user-1",
	})
	if err == nil {
		t.Fatal("expected an error when the publisher has gone away")
	}
	if !strings.Contains(err.Error(), "publisher") {
		t.Errorf("unexpected error %v", err)
	}
}

// Reads must keep working with no broker: a member has to be able to see the car they
// registered during an outage, since that needs only the database (PRD 008 §5).
func TestOwnVehiclesStillReadableWithNoBroker(t *testing.T) {
	app := vehicleWiredApp(t, commands.NewPublisherHolder())

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, banditPhone, "+45"+banditPhone)
	resp := getWithCookies(t, srv.URL+"/api/me/vehicles", cookies)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (%s)", resp.StatusCode, string(body))
	}
}
