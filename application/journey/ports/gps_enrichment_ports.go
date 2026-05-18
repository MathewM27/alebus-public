// Package ports defines infrastructure interfaces for journey-related operations.
// This file contains ports for GPS Enrichment (Option B+ - Phase 1).
//
// ═══════════════════════════════════════════════════════════════════════════════
// GPS Enrichment Ports — Contract Definition
// ═══════════════════════════════════════════════════════════════════════════════
//
// PURPOSE:
//   Define interfaces for converting raw GPS telemetry into enriched LiveBusUpdate.
//   These ports enable the GPS enrichment use case to:
//   - Read bus-to-route assignments (cached)
//   - Read route geometry (stops with cumulative distances)
//   - Store/retrieve resolver state (Redis-backed)
//
// RELATIONSHIP TO EXISTING PORTS:
//   - Enrichment produces LiveBusUpdate (defined in live_bus_write_ports.go)
//   - Enriched updates flow through existing LiveBusPublisher
//   - No changes to existing read/write ports
//
// ERROR CLASSIFICATION:
//   - Invalid payload → caller should ACK + drop
//   - No assignment / route not found → caller should ACK + metric (business non-applicable)
//   - Redis/Postgres unavailable → return error (caller retries with backoff)
//
// ═══════════════════════════════════════════════════════════════════════════════

package ports

import (
	"context"
	"fmt"
	"time"
)

// RawGPSPublisher publishes raw GPS data (typically to MQTT) for the ingestion/enrichment pipeline.
//
// This is an application-layer port: implementations live in infrastructure.
// It is primarily used by dev tooling/HTTP endpoints to inject telemetry into the system.
type RawGPSPublisher interface {
	PublishRawGPS(ctx context.Context, update RawGPSUpdate) error
}

// ─────────────────────────────────────────────────────────────────────────────
// Raw GPS Input DTO
// ─────────────────────────────────────────────────────────────────────────────

// RawGPSUpdate represents raw GPS data from a device (no enrichment).
// This is the input to the GPS enrichment use case.
//
// VALIDATION RULES (enforced by Validate()):
//   - BusID: Required, non-empty string
//   - Lat: Must be in range [-90, 90]
//   - Lon: Must be in range [-180, 180]
//   - Timestamp: Required, must not be zero
//   - SpeedKmh: Must be >= 0 if provided
//   - Heading: Must be in range [0, 360] if provided
//   - AccuracyM: Must be >= 0 if provided
type RawGPSUpdate struct {
	// BusID is the unique identifier of the bus (required).
	BusID string

	// Lat is the latitude in decimal degrees (required, -90 to 90).
	Lat float64

	// Lon is the longitude in decimal degrees (required, -180 to 180).
	Lon float64

	// Timestamp is the GPS fix time (required).
	Timestamp time.Time

	// ReceivedAt is when Alebus received the message at the edge (server time).
	// This enables ADR-00X timestamp semantics (freshness/physics vs ordering).
	ReceivedAt time.Time

	// SpeedKmh is the ground speed in km/h (optional, 0 if not provided).
	SpeedKmh float64

	// Heading is the compass heading in degrees (optional, 0 if not provided).
	// Range: 0-360 where 0=North, 90=East, 180=South, 270=West.
	Heading float64

	// AccuracyM is the GPS accuracy in meters (optional, 0 if not provided).
	AccuracyM float64
}

// Validate performs validation on the RawGPSUpdate.
// Returns an error describing the first validation failure, or nil if valid.
func (r *RawGPSUpdate) Validate() error {
	if r.BusID == "" {
		return &GPSValidationError{Field: "BusID", Reason: "required"}
	}
	if r.Lat < -90 || r.Lat > 90 {
		return &GPSValidationError{Field: "Lat", Reason: "must be in range [-90, 90]"}
	}
	if r.Lon < -180 || r.Lon > 180 {
		return &GPSValidationError{Field: "Lon", Reason: "must be in range [-180, 180]"}
	}
	if r.Timestamp.IsZero() {
		return &GPSValidationError{Field: "Timestamp", Reason: "required"}
	}
	if r.SpeedKmh < 0 {
		return &GPSValidationError{Field: "SpeedKmh", Reason: "must be >= 0"}
	}
	if r.Heading < 0 || r.Heading > 360 {
		return &GPSValidationError{Field: "Heading", Reason: "must be in range [0, 360]"}
	}
	if r.AccuracyM < 0 {
		return &GPSValidationError{Field: "AccuracyM", Reason: "must be >= 0"}
	}
	return nil
}

// GPSValidationError represents a validation failure for raw GPS data.
type GPSValidationError struct {
	Field  string
	Reason string
}

func (e *GPSValidationError) Error() string {
	return fmt.Sprintf("invalid GPS: %s %s", e.Field, e.Reason)
}

// IsGPSValidationError returns true if err is a GPSValidationError.
func IsGPSValidationError(err error) bool {
	_, ok := err.(*GPSValidationError)
	return ok
}

// ─────────────────────────────────────────────────────────────────────────────
// Assignment Reader Port
// ─────────────────────────────────────────────────────────────────────────────

// BusAssignment represents a bus's current route assignment.
type BusAssignment struct {
	// BusID is the unique identifier of the bus.
	BusID string

	// RouteID is the assigned route ID.
	RouteID string

	// Direction is the initial direction from assignment (0=outbound, 1=inbound).
	Direction int

	// UpdatedAt is when this assignment was last updated.
	UpdatedAt time.Time
}

// AssignmentReader reads bus-to-route assignments.
// Implementations should use caching (Redis) with Postgres fallback.
//
// ERROR SEMANTICS:
//   - (assignment, true, nil): Bus has an assignment
//   - (zero, false, nil): Bus has no assignment (business non-applicable, ACK)
//   - (zero, false, error): Infrastructure failure (retry)
type AssignmentReader interface {
	// GetAssignment returns the active route assignment for a bus.
	//
	// Returns (assignment, true, nil) if found.
	// Returns (zero, false, nil) if bus has no assignment.
	// Returns (zero, false, error) on infrastructure failure.
	GetAssignment(ctx context.Context, busID string) (BusAssignment, bool, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Route Geometry Reader Port
// ─────────────────────────────────────────────────────────────────────────────

// StopGeometry represents a stop with position and cumulative distance.
// Used by the resolver to project GPS onto route segments.
type StopGeometry struct {
	// StopID is the unique identifier of the stop.
	StopID string

	// Name is the human-readable stop name.
	Name string

	// Lat is the latitude of the stop.
	Lat float64

	// Lon is the longitude of the stop.
	Lon float64

	// CumulativeDistanceMeters is the distance from route start to this stop.
	// First stop has 0, subsequent stops have cumulative distance.
	CumulativeDistanceMeters float64
}

// RouteGeometry contains stops with cumulative distances for GPS enrichment.
// This is used by the resolver to determine stop index and direction.
type RouteGeometry struct {
	// RouteID is the unique identifier of the route.
	RouteID string

	// DirectionType indicates route direction capability.
	// 0 = unidirectional (outbound only)
	// 1 = bidirectional (outbound and inbound)
	DirectionType int

	// Stops is the ordered list of stops with geometry.
	// Stops are ordered from first to last in the outbound direction.
	Stops []StopGeometry

	// TotalDistanceMeters is the total route length.
	TotalDistanceMeters float64
}

// RouteGeometryReader reads route geometry (stops + cumulative distances).
// Implementations should use caching (Redis) with Postgres fallback.
//
// ERROR SEMANTICS:
//   - (geometry, true, nil): Route found
//   - (zero, false, nil): Route not found (business non-applicable, ACK)
//   - (zero, false, error): Infrastructure failure (retry)
type RouteGeometryReader interface {
	// GetGeometry returns the route's stop geometry.
	//
	// Returns (geometry, true, nil) if found.
	// Returns (zero, false, nil) if route not found.
	// Returns (zero, false, error) on infrastructure failure.
	GetGeometry(ctx context.Context, routeID string) (RouteGeometry, bool, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Resolver State Store Port
// ─────────────────────────────────────────────────────────────────────────────

// ResolverMode represents the resolver's operating mode.
type ResolverMode string

const (
	// ResolverModeBootstrap is used when no prior state exists.
	// Performs global search across all stops.
	ResolverModeBootstrap ResolverMode = "BOOTSTRAP"

	// ResolverModeTracking is used when prior state is reliable.
	// Performs windowed search around last known position.
	ResolverModeTracking ResolverMode = "TRACKING"

	// ResolverModeReacquire is used when state becomes unreliable.
	// Widens search window to recover from anomalies.
	ResolverModeReacquire ResolverMode = "REACQUIRE"
)

// ResolverState represents the resolver's memory for a bus.
// This is persisted in Redis to maintain state across GPS updates.
type ResolverState struct {
	// BusID is the unique identifier of the bus.
	BusID string

	// RouteID is the route this state applies to.
	// If bus is reassigned, state should be cleared.
	RouteID string

	// Mode is the current resolver operating mode.
	Mode ResolverMode

	// LastStopIndex is the last computed stop index (0-based).
	LastStopIndex int

	// LastFractionalIndex is the fractional position between stops (0.0-1.0).
	// 0.0 = at stop N, 0.5 = halfway to stop N+1, 1.0 = at stop N+1.
	LastFractionalIndex float64

	// LastDirection is the travel direction (0=outbound, 1=inbound).
	LastDirection int

	// DirectionConfidence is the confidence in the current direction (0.0-1.0).
	DirectionConfidence float64

	// LastDistanceAlongRoute is the cumulative distance from route start (meters).
	LastDistanceAlongRoute float64

	// LastLat is the last accepted latitude (for speed validation).
	LastLat float64

	// LastLon is the last accepted longitude (for speed validation).
	LastLon float64

	// LastUpdateMs is the timestamp of the last update (Unix milliseconds).
	LastUpdateMs int64

	// DwellStartMs is when dwelling started at a terminal (0 if not dwelling).
	DwellStartMs int64

	// Confidence is the overall enrichment confidence (0.0-1.0).
	Confidence float64

	// LastOrderingTsMs is the last per-bus monotonic ordering timestamp (Unix ms).
	// This is the authoritative ordering key used for Redis staleness prevention.
	LastOrderingTsMs int64

	// LastDeviceTsMs is the last raw device timestamp seen (Unix ms).
	// This is metadata for debugging and ordering algorithm input.
	LastDeviceTsMs int64

	// LastReceivedAtMs is the last server receive time seen (Unix ms).
	// This anchors physics/plausibility checks to server time.
	LastReceivedAtMs int64
}

// ResolverStateStore reads/writes resolver state.
// This is a Redis-backed port with TTL (90s aligned with bus state).
//
// ERROR SEMANTICS:
//   - GetState: (state, true, nil) if found, (zero, false, nil) if not found (cold start)
//   - GetState: (zero, false, error) on infrastructure failure (retry)
//   - SaveState: nil on success, error on infrastructure failure (retry)
type ResolverStateStore interface {
	// GetState returns the resolver state for a bus.
	//
	// Returns (state, true, nil) if found.
	// Returns (zero, false, nil) if no state exists (triggers BOOTSTRAP).
	// Returns (zero, false, error) on infrastructure failure.
	GetState(ctx context.Context, busID string) (ResolverState, bool, error)

	// SaveState persists resolver state with TTL.
	// Returns error only on infrastructure failure.
	SaveState(ctx context.Context, state ResolverState) error
}

// ─────────────────────────────────────────────────────────────────────────────
// Enrichment Result DTO
// ─────────────────────────────────────────────────────────────────────────────

// EnrichmentResult is the output of GPS enrichment.
// Used to communicate success/failure and metadata back to the caller.
type EnrichmentResult struct {
	// Applied indicates whether the GPS was successfully enriched.
	Applied bool

	// Reason explains why enrichment failed (empty if Applied=true).
	// Values: "invalid_gps", "no_assignment", "route_not_found", etc.
	Reason string

	// Update is the enriched LiveBusUpdate (valid only if Applied=true).
	Update LiveBusUpdate

	// Confidence is the enrichment confidence (0.0-1.0).
	Confidence float64

	// Mode is the resolver mode used for this enrichment.
	Mode ResolverMode

	// Debug contains optional diagnostics to validate stop-index projection.
	// Populated only when GPS debug is enabled at the edge.
	Debug *EnrichmentDebugInfo
}

// EnrichmentDebugInfo provides detailed diagnostics about stop-index projection.
// This is intended for validation and troubleshooting, not for domain logic.
type EnrichmentDebugInfo struct {
	RouteID string `json:"routeId"`

	RawLat float64 `json:"rawLat"`
	RawLon float64 `json:"rawLon"`

	Mode ResolverMode `json:"mode"`

	CandidateIndex      int     `json:"candidateIndex"`
	ProjectionDirection int     `json:"projectionDirection"`
	ProposedStopIndex   int     `json:"proposedStopIndex"`
	AcceptedStopIndex   int     `json:"acceptedStopIndex"`
	FractionalIndex     float64 `json:"fractionalIndex"`
	IsInterpolated      bool    `json:"isInterpolated"`

	FromStopIndex int `json:"fromStopIndex"`
	ToStopIndex   int `json:"toStopIndex"`

	ProjectionT           float64 `json:"projectionT"`
	DistanceToSegmentM    float64 `json:"distanceToSegmentM"`
	DistanceToFromStopM   float64 `json:"distanceToFromStopM"`
	DistanceToToStopM     float64 `json:"distanceToToStopM"`
	MaxDistanceToSegmentM float64 `json:"maxDistanceToSegmentM"`
	FallbackReason        string  `json:"fallbackReason"`
	ProjectionConfidence  float64 `json:"projectionConfidence"`

	// GpsDistanceFromExpectedM is the distance (in meters) from the route segment
	// implied by the persisted resolver state ("expected" segment) to the current
	// raw GPS point. This is the signal used for REACQUIRE and severe escalation.
	GpsDistanceFromExpectedM float64 `json:"gpsDistanceFromExpectedM"`

	// SevereEscalationUsed indicates whether REACQUIRE escalated to global search
	// because the bus was very far from the expected segment.
	SevereEscalationUsed bool `json:"severeEscalationUsed"`

	HysteresisThreshold    float64 `json:"hysteresisThreshold"`
	HysteresisAcceptedSame bool    `json:"hysteresisAcceptedSame"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Enrichment Reasons (for consistent error classification)
// ─────────────────────────────────────────────────────────────────────────────

const (
	// EnrichmentReasonSuccess indicates successful enrichment.
	EnrichmentReasonSuccess = ""

	// EnrichmentReasonInvalidGPS indicates the GPS payload was invalid.
	// Caller should ACK + drop (retrying won't help).
	EnrichmentReasonInvalidGPS = "invalid_gps"

	// EnrichmentReasonNoAssignment indicates the bus has no route assignment.
	// Caller should ACK + metric (business non-applicable).
	EnrichmentReasonNoAssignment = "no_assignment"

	// EnrichmentReasonRouteNotFound indicates the assigned route doesn't exist.
	// Caller should ACK + metric (business non-applicable).
	EnrichmentReasonRouteNotFound = "route_not_found"

	// EnrichmentReasonDisabled indicates GPS enrichment is disabled.
	// Caller should ACK + metric.
	EnrichmentReasonDisabled = "gps_enrichment_disabled"
)

// IsRetryableEnrichmentError returns true if the error indicates an infrastructure
// failure that should trigger a retry (Redis/Postgres unavailable).
// Returns false for validation errors and business non-applicable cases.
func IsRetryableEnrichmentError(err error) bool {
	if err == nil {
		return false
	}
	// GPS validation errors are not retryable
	if IsGPSValidationError(err) {
		return false
	}
	// All other errors (Redis, Postgres, network) are retryable
	return true
}
