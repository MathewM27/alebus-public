package ports

import "context"

// BusRosterRepository provides roster lookup capabilities for bus eligibility filtering.
// This is an application-layer port implemented by infrastructure (Postgres).
//
// PURPOSE:
// Journey recommendations should only include buses that are registered in the system roster.
// This port enables roster membership checks without leaking Postgres details into application logic.
//
// USAGE:
// Used by application/journey/adapters/eligible_bus_reader.go to filter live (Redis) buses
// to only those registered in Postgres.
type BusRosterRepository interface {
	// ListBusIDsByRoute returns bus IDs registered on a route.
	// Supports optional filtering by operator and status.
	//
	// Returns empty slice if no buses match the criteria.
	// Returns error only on infrastructure failure.
	ListBusIDsByRoute(ctx context.Context, req RosterQuery) ([]string, error)

	// Exists checks if a bus ID exists in the roster.
	//
	// Returns (true, nil) if bus exists.
	// Returns (false, nil) if bus does not exist (not an error).
	// Returns (false, error) only on infrastructure failure.
	Exists(ctx context.Context, busID string) (bool, error)
}

// RosterQuery specifies criteria for roster lookups.
type RosterQuery struct {
	// RouteID is required
	RouteID string

	// OperatorID is optional. If provided, only buses from this operator are returned.
	OperatorID string

	// Status is optional. If provided, only buses with this status are returned.
	// Maps to enums.BusStatus values (0=active, 1=offline, etc.)
	Status *int
}
