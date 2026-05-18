// Package ports defines infrastructure interfaces for journey-related operations.
// These are application-layer ports that define WHAT capabilities are needed,
// not HOW they are implemented. Infrastructure layer provides implementations.
package ports

import (
	"context"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// DTOs (Data Transfer Objects)
// ─────────────────────────────────────────────────────────────────────────────

// GeoPosition holds lat/lon for a bus.
type GeoPosition struct {
	Lat float64
	Lon float64
}

// LiveBusSnapshot represents a bus's current state as seen by the application.
// This is a DTO, not a domain aggregate.
// All fields are required; if any field is missing from Redis, the snapshot is invalid.
type LiveBusSnapshot struct {
	BusID         string
	RouteID       string
	Direction     int    // 0 = outbound, 1 = inbound
	LastStopIndex int    // Last confirmed stop index on route
	LastStopID    string // Last confirmed stop ID (resolved from geometry by enrichment)
	// IsAtTerminal indicates whether the bus is CONFIRMED at terminal.
	// Contract (DEV CUTOVER): maps to Redis `is_terminal`.
	IsAtTerminal bool
	// IsTerminalIndex indicates whether the bus is at a terminal stop index (boundary-only).
	// Contract (DEV CUTOVER): maps to Redis `is_terminal_index`.
	IsTerminalIndex bool
	SpeedKmh        float64   // Current speed in km/h
	LastUpdatedAt   time.Time // Timestamp of this state snapshot (REQUIRED for staleness/confidence). Maps to ordering_ts_ms.
	DeviceTimestamp time.Time // Raw device timestamp (untrusted metadata). May be zero for older snapshots.
	ReceivedAt      time.Time // Server receive time (trusted metadata). May be zero for older snapshots.
	Position        GeoPosition
	Status          string // "active", "inactive", or "delayed" — used for liveness detection
}

// BusWithDistance represents a bus and its distance from a query point.
// Used for proximity-based results.
type BusWithDistance struct {
	BusID          string
	DistanceMeters float64
	Position       GeoPosition
}

// ─────────────────────────────────────────────────────────────────────────────
// Port Interfaces
// ─────────────────────────────────────────────────────────────────────────────

// LiveBusStateReader reads live bus state by route.
// This is a Redis-backed port; Postgres is NOT authoritative for live state.
// Application code uses this for bus recommendations and tracking refresh.
type LiveBusStateReader interface {
	// ListByRoute returns all buses currently assigned to the given route.
	// If direction is nil, returns buses in both directions.
	// Returns an empty slice (not an error) if no buses are found.
	// Returns an error only on infrastructure failure (e.g., Redis unavailable).
	//
	// Expected freshness: Data should reflect bus state within the last 60 seconds.
	// Application computes confidence degradation for older data using LastUpdatedAt.
	ListByRoute(ctx context.Context, routeID string, direction *int) ([]LiveBusSnapshot, error)
}

// LiveBusFinder retrieves a single bus's live state.
// This is a Redis-backed port; Postgres is NOT authoritative for live state.
type LiveBusFinder interface {
	// FindByID returns the live state of a specific bus.
	// Returns (snapshot, true, nil) if found.
	// Returns (zero, false, nil) if bus is not in live state (missing or expired).
	// Returns (zero, false, error) only on infrastructure failure.
	//
	// Expected freshness: Data should reflect state within the last 60 seconds.
	// Application uses LastUpdatedAt to assess data quality.
	FindByID(ctx context.Context, busID string) (LiveBusSnapshot, bool, error)
}

// BusProximityFinder finds buses near a location from a candidate set.
// This is a Redis-backed port using geo indexing.
type BusProximityFinder interface {
	// FindNearbyFromSet returns buses from candidateBusIDs that are within radiusMeters
	// of the given location, ordered by distance (closest first).
	//
	// Only buses present in the geo index AND in candidateBusIDs are returned.
	// If a candidate bus has no geo position in Redis, it is excluded from results.
	// Returns an empty slice (not an error) if no matching buses are found.
	// Returns an error only on infrastructure failure.
	FindNearbyFromSet(
		ctx context.Context,
		lat float64,
		lon float64,
		radiusMeters float64,
		candidateBusIDs []string,
	) ([]BusWithDistance, error)
}
