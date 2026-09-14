package main

import (
	"net/http"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/tables/vehicle"
	"github.com/nathejk/shared-go/types"

	"nathejk.dk/internal/plate"
	"nathejk.dk/internal/users"
)

// The vehicle entity's wiring (PRD 010, task 235).
//
// The projection, the command side and the read API all come from
// shared-go/tables/vehicle. Nothing here forks the entity, adds a local vehicle
// table, or publishes a vehicle event by hand: `hq` already consumes these
// subjects, so a car registered in this app appears in the coordinator's view
// with no integration work — which is only true while both sides speak the one
// vocabulary (PRD 010 §8).

// vehicleTable names the three roles one vehicle entity value fills, so main.go
// can hold it in a variable.
//
// It exists because `vehicle.New` returns an *unexported* type — the house style
// across every shared-go entity — so `var vehicles *vehicle.table` does not
// compile here. An interface is the honest way to express the composition
// anyway: `person` and `checkpoint` are declared as concrete `*Table` values only
// because they happen to export theirs.
type vehicleTable interface {
	vehicle.Queries
	vehicle.Commands
	cqrs.Consumer
}

// vehiclesOrNil narrows the entity to its read API for data.Models, mirroring
// raceAreasOrNil and peopleOrNil.
//
// The nil-interface trap is the reason this is a function rather than a plain
// assignment: handing a typed nil to an interface field produces a value that is
// not `== nil`, so a handler's availability check would pass and the call would
// panic. Taking the interface and returning nil for it keeps that untypeable.
func vehiclesOrNil(t vehicleTable) vehicle.Queries {
	if t == nil {
		return nil
	}
	return t
}

// vehicleCommandsOrNil does the same for the write side.
//
// Handlers must check it. A nil command side means "no broker or no database was
// configured", and a registration attempt has to fail loudly: PRD 008 §5 is
// explicit that a write which could not be published has not happened, and a car
// its owner believes is registered is precisely the car nobody dispatches.
func vehicleCommandsOrNil(t vehicleTable) vehicle.Commands {
	if t == nil {
		return nil
	}
	return t
}

// vehicleResponse is one of the caller's own vehicles.
//
// A projection of `vehicle.Vehicle` rather than the row itself, and the fields
// left out are the point: `custodianUserId`, `driverUserId` and `sectionSlug` are
// the coordinator's half of this entity — who was dispatched, which crew group
// owns the car — and none of them is something an owner needs to see their own
// registration. Projecting here rather than trusting the client not to render
// them is the same rule `.rules` states for guardian numbers, applied to a
// smaller case: a response that does not carry a field cannot leak it.
type vehicleResponse struct {
	ID string `json:"id"`
	// LicensePlate carries its country prefix, e.g. "DK+AB12345" — the canonical
	// form from internal/plate, so what the client shows is what the inventory
	// compares.
	LicensePlate string `json:"license_plate"`

	Brand string `json:"brand"`
	Model string `json:"model"`
	Color string `json:"color"`

	// SeatCount excludes the driver, as it does everywhere else in this feature.
	// The client's label has to say so (PRD 010 §7); the name alone does not.
	SeatCount uint `json:"seat_count"`

	Description string `json:"description"`
}

// listOwnVehiclesHandler serves the caller's own vehicles. Runs behind requireAuth.
//
// Session-scoped by construction, like /api/me/profile and /api/me/photo: there is
// no user id in the path, so no caller can ask for somebody else's. The filter is
// the caller's **custodianship**, never their driving — a car lent out for one
// pickup has a different driver and must still be listed for the person who
// answers for it (see vehicle.Filter.CustodianUserIDs in shared-go).
//
// An empty list is a `200`. "You have registered nothing" is a successful answer and
// the profile page renders an empty state from it — whereas an unavailable read model
// is a `503`, because reporting "you have none" when the truth is "we cannot tell"
// invites a member to register a second row for a car that is already in the
// inventory, which is exactly the duplicate PRD 010 §5 is trying to avoid.
//
// @Summary      The caller's own vehicles
// @Description  Vehicles the authenticated member is the custodian of, for the current event year. Scoped to the session: there is no user id in the path, so no caller can read another member's registrations. Filtered by custodianship rather than by who is currently driving, so a car lent out for a pickup still belongs to the person who answers for it. An empty list is a normal 200.
// @Tags         vehicles
// @Produce      json
// @Success      200  {object}  vehiclesResponse
// @Failure      401  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Router       /me/vehicles [get]
func (app *application) listOwnVehiclesHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}

	// Nil when there is no database or the entity failed to build. Deliberately not
	// an empty list — see the doc comment.
	if app.models.Vehicles == nil {
		app.ServiceUnavailableResponse(w, r, "vehicle data is not available")
		return
	}

	vehicles, err := app.models.Vehicles.GetAll(r.Context(), vehicle.Filter{
		YearSlug:         types.YearSlug(app.config.eventYear),
		CustodianUserIDs: []types.UserID{types.UserID(s.UserID)},
	})
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	// Non-nil so the JSON is `[]` rather than `null`: the client iterates it, and a
	// null would make "no vehicles" a special case on every surface that reads this.
	out := make([]vehicleResponse, 0, len(vehicles))
	for _, v := range vehicles {
		out = append(out, vehicleResponse{
			ID:           string(v.VehicleID),
			LicensePlate: v.LicensePlate,
			Brand:        v.Brand,
			Model:        v.Model,
			Color:        v.Color,
			SeatCount:    v.SeatCount,
			Description:  v.Description,
		})
	}

	if err := app.WriteJSON(w, http.StatusOK, vehiclesResponse{Vehicles: out}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// vehiclesResponse wraps the list in an object rather than returning a bare JSON
// array, matching the rest of this API and leaving room to add a sibling field
// later without changing the response's type.
type vehiclesResponse struct {
	Vehicles []vehicleResponse `json:"vehicles"`
}

// registerVehicleRequest is the registration form.
//
// Note what is absent: any notion of who owns this. The custodian is the caller,
// taken from the session — a body field would let one member file a car under
// another's name, and the whole authorisation model downstream (task 239) hangs on
// custodianship being something the server decided.
type registerVehicleRequest struct {
	// LicensePlate is free-form on the way in. Normalised before it is compared or
	// stored, so "ab 12 345" and "AB12345" cannot become two rows.
	LicensePlate string `json:"license_plate"`

	Brand string `json:"brand"`
	Model string `json:"model"`
	Color string `json:"color"`

	// SeatCount **excludes the driver**, matching shared-go's RegisterFields. An
	// off-by-one here has a coordinator dispatching a car with one seat too few at
	// 02:00, which is why the client's label has to say so explicitly (PRD 010 §7)
	// rather than relying on this comment.
	SeatCount uint `json:"seat_count"`

	Description string `json:"description"`
}

// registerVehicleHandler registers a vehicle for the caller. Runs behind requireAuth.
//
// The caller becomes the custodian, and shared-go's projector makes the custodian
// the first driver, so nothing here assigns one.
//
// A duplicate plate answers `409` rather than creating a second row. Two people
// registering one car is the likeliest data problem in a self-registered inventory
// — a crew member and their passenger both filling in the form — and the response
// deliberately does **not** say who registered it: a plate maps to a person, and
// this endpoint is not a lookup surface. "This car is already registered" is all
// the client needs to write a useful sentence.
//
// @Summary      Register a vehicle
// @Description  Registers a car for the authenticated member, who becomes its custodian and first driver. Every role except spejder may register. The licence plate is normalised server-side, and a plate already registered for the current event year is refused with 409 rather than creating a second row for one car; the conflict response does not disclose who registered it. Seat count excludes the driver.
// @Tags         vehicles
// @Accept       json
// @Produce      json
// @Param        request  body      registerVehicleRequest  true  "Vehicle details"
// @Success      201  {object}  vehicleResponse
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Router       /me/vehicles [post]
func (app *application) registerVehicleHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}
	if !users.MayRegisterVehicle(users.Role(s.Role)) {
		app.ForbiddenResponse(w, r)
		return
	}

	var input registerVehicleRequest
	if err := app.ReadJSON(w, r, &input); err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	// Normalised first, before validation, comparison or publishing, so all three
	// see the same string. This is the single place plates enter the inventory from
	// this app.
	normalized, err := plate.Normalize(input.LicensePlate)
	if err != nil {
		app.BadRequestMessageResponse(w, r, "nummerpladen kan ikke genkendes")
		return
	}

	// A registration is a write, so it needs the broker. Failing here is the point:
	// PRD 008 §5 — a write that could not be published has not happened, and a car
	// its owner believes is registered is precisely the car nobody dispatches.
	if app.vehicles == nil {
		app.ServiceUnavailableResponse(w, r, "vehicle registration is not available")
		return
	}

	year := types.YearSlug(app.config.eventYear)

	// Duplicate check before publishing. Skipped rather than fatal when the read
	// model is unavailable: refusing an otherwise valid registration because we
	// cannot check for a duplicate would keep a real car out of the inventory to
	// avoid a duplicate row, which is the worse of the two outcomes.
	if app.models.Vehicles != nil {
		existing, err := app.models.Vehicles.GetAll(r.Context(), vehicle.Filter{
			YearSlug:     year,
			LicensePlate: normalized,
		})
		if err != nil {
			app.ServerErrorResponse(w, r, err)
			return
		}
		if len(existing) > 0 {
			// No identity in the message, deliberately — see the doc comment.
			app.ConflictResponse(w, r, "køretøjet er allerede registreret")
			return
		}
	}

	id, err := app.vehicles.Register(r.Context(), year, vehicle.RegisterFields{
		LicensePlate:    normalized,
		CustodianUserID: types.UserID(s.UserID),
		Brand:           input.Brand,
		Model:           input.Model,
		Color:           input.Color,
		SeatCount:       input.SeatCount,
		Description:     input.Description,
	})
	if err != nil {
		// Includes a publish failure, which must fail the request rather than
		// report a success nothing recorded.
		app.ServerErrorResponse(w, r, err)
		return
	}

	// The normalised plate is echoed back, so the client shows the canonical form
	// rather than what was typed — the same string the inventory will compare next
	// time.
	out := vehicleResponse{
		ID:           string(id),
		LicensePlate: normalized,
		Brand:        input.Brand,
		Model:        input.Model,
		Color:        input.Color,
		SeatCount:    input.SeatCount,
		Description:  input.Description,
	}
	if err := app.WriteJSON(w, http.StatusCreated, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}
