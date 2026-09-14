package main

import (
	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/tables/vehicle"
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
