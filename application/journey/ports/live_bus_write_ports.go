// Package ports defines infrastructure interfaces for journey-related operations.
// This file contains WRITE-SIDE ports for live bus state ingestion.
//
// ═══════════════════════════════════════════════════════════════════════════════
// Phase 8A: Live Bus State Write-Side — Contract & Rules Definition
// ═══════════════════════════════════════════════════════════════════════════════
//
// PURPOSE:
//   Define transport-agnostic interfaces for ingesting live bus state into Redis.
//   These ports enable the simulator, EMQX/Kafka consumers, or real bus telemetry
//   to publish state updates without coupling to Redis internals.
//
// RELATIONSHIP TO READ PORTS:
//   - Read ports (live_bus_ports.go): LiveBusStateReader, LiveBusFinder, BusProximityFinder
//   - Write ports (this file): LiveBusStateWriter, RouteBusIndexer, BusGeoUpdater
//   - Write operations update the same Redis keys that read operations query.
//
// ═══════════════════════════════════════════════════════════════════════════════

package ports

import (
	"context"
	"fmt"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Write-Side DTOs
// ─────────────────────────────────────────────────────────────────────────────

// LiveBusUpdate represents a single bus state update from an external source.
// This DTO is transport-agnostic — it can come from HTTP, Kafka, EMQX, or simulator.
//
// VALIDATION RULES (enforced by callers via Validate(now)):
//   - BusID: Required, non-empty string
//   - RouteID: Required, non-empty string
//   - Direction: Must be 0 (outbound) or 1 (inbound)
//   - StopIndex: Must be >= 0
//   - Lat: Must be in range [-90, 90]
//   - Lon: Must be in range [-180, 180]
//   - SpeedKmh: Must be >= 0
//   - Status: Must be "active", "inactive", or "delayed"
//   - Timestamp: Required, must not be in the future (with tolerance)
//
// IDEMPOTENCY:
//   - Updates with the same BusID and Timestamp are idempotent
//   - Updates with Timestamp older than the current state are rejected
//   - This ensures eventual consistency across distributed sources
type LiveBusUpdate struct {
	// BusID is the unique identifier of the bus (required).
	BusID string

	// RouteID is the route this bus is currently serving (required).
	RouteID string

	// Direction indicates travel direction: 0 = outbound, 1 = inbound.
	Direction int

	// StopIndex is the last confirmed stop index on the route (0-based).
	StopIndex int

	// StopID is the unique identifier of the stop at StopIndex.
	// This is resolved from the route geometry by enrichment (authoritative source).
	// Prevents index→ID mapping drift when routes change.
	StopID string

	// IsAtTerminal indicates whether the bus is CONFIRMED at terminal.
	//
	// Contract (DEV CUTOVER): this must map to Redis `is_terminal`.
	// Meaning: bus is at a terminal index AND has satisfied terminal confirmation
	// conditions (dwell + low-speed) as determined by resolver.DetectTerminal.
	IsAtTerminal bool

	// IsTerminalIndex indicates whether the bus is currently at a terminal stop INDEX
	// (boundary-only).
	//
	// Contract (DEV CUTOVER): this must map to Redis `is_terminal_index`.
	// Meaning: stop_index == 0 || stop_index == lastStopIndex (no dwell/speed).
	IsTerminalIndex bool

	// Lat is the latitude of the bus's current position.
	Lat float64

	// Lon is the longitude of the bus's current position.
	Lon float64

	// SpeedKmh is the current speed in kilometers per hour.
	SpeedKmh float64

	// Status is the operational status: "active", "inactive", or "delayed".
	Status string

	// Timestamp is the authoritative ordering timestamp used for idempotency and
	// staleness detection.
	Timestamp time.Time

	// DeviceTimestamp is the raw timestamp claimed by the GPS device (untrusted).
	// This is metadata only and is NOT used for ordering.
	DeviceTimestamp time.Time

	// ReceivedAt is when Alebus received the message at the edge (server time).
	// This is the trusted anchor for validation and freshness semantics.
	ReceivedAt time.Time
}

const defaultFutureTimestampTolerance = 30 * time.Second

// Validate performs validation on the LiveBusUpdate using the supplied reference time.
// Returns an error describing the first validation failure, or nil if valid.
func (u *LiveBusUpdate) Validate(now time.Time) error {
	if u.BusID == "" {
		return &ValidationError{Field: "BusID", Reason: "required"}
	}
	if u.RouteID == "" {
		return &ValidationError{Field: "RouteID", Reason: "required"}
	}
	if u.Direction != 0 && u.Direction != 1 {
		return &ValidationError{Field: "Direction", Reason: "must be 0 or 1"}
	}
	if u.StopIndex < 0 {
		return &ValidationError{Field: "StopIndex", Reason: "must be >= 0"}
	}
	if u.StopID == "" {
		return &ValidationError{Field: "StopID", Reason: "required"}
	}
	if u.Lat < -90 || u.Lat > 90 {
		return &ValidationError{Field: "Lat", Reason: "must be in range [-90, 90]"}
	}
	if u.Lon < -180 || u.Lon > 180 {
		return &ValidationError{Field: "Lon", Reason: "must be in range [-180, 180]"}
	}
	if u.SpeedKmh < 0 {
		return &ValidationError{Field: "SpeedKmh", Reason: "must be >= 0"}
	}
	if u.Status != "active" && u.Status != "inactive" && u.Status != "delayed" {
		return &ValidationError{Field: "Status", Reason: "must be active, inactive, or delayed"}
	}
	if u.Timestamp.IsZero() {
		return &ValidationError{Field: "Timestamp", Reason: "required"}
	}
	ref := now
	if !u.ReceivedAt.IsZero() {
		ref = u.ReceivedAt
	}
	// Allow tolerance for future timestamps relative to trusted server receive time.
	if u.Timestamp.After(ref.Add(defaultFutureTimestampTolerance)) {
		return &ValidationError{Field: "Timestamp", Reason: "must not be too far in the future"}
	}
	return nil
}

// ValidationError represents a validation failure for a specific field.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Reason)
}

// UpdateResult contains the outcome of a bus state update operation.
type UpdateResult struct {
	// Applied indicates whether the update was applied to Redis.
	// False if rejected due to staleness or validation failure.
	Applied bool

	// Reason explains why the update was not applied (empty if Applied is true).
	Reason string

	// PreviousTimestamp is the timestamp that was in Redis before this update.
	// Zero if no previous state existed.
	PreviousTimestamp time.Time
}

// ─────────────────────────────────────────────────────────────────────────────
// Write-Side Port Interfaces
// ─────────────────────────────────────────────────────────────────────────────

// LiveBusStateWriter accepts bus state updates and persists them to Redis.
// This is the primary write port for all bus telemetry ingestion.
//
// RESPONSIBILITIES:
//   - Validate incoming updates
//   - Check staleness (reject updates older than current state)
//   - Persist state to Redis hash with TTL
//   - Return result indicating if update was applied
//
// IDEMPOTENCY CONTRACT:
//   - Same BusID + Timestamp → same Redis state (safe to retry)
//   - Older Timestamp → rejected, no state change
//   - Newer Timestamp → applied, state updated
//
// TTL STRATEGY:
//   - Each bus state key has 60-second TTL
//   - TTL refreshed on every successful update
//   - Expired key = bus offline (app layer handles gracefully)
type LiveBusStateWriter interface {
	// UpdateBusState validates and persists a bus state update to Redis.
	//
	// Returns UpdateResult with:
	//   - Applied=true if update was persisted
	//   - Applied=false with Reason if rejected (validation or staleness)
	//
	// Returns error only on infrastructure failure (Redis unavailable).
	// Validation failures are NOT errors — they return Applied=false.
	UpdateBusState(ctx context.Context, update LiveBusUpdate) (UpdateResult, error)
}

// RouteBusIndexer maintains the route → bus index sets.
// This enables efficient lookup of all buses on a given route/direction.
//
// RESPONSIBILITIES:
//   - Add bus to route index set on state update
//   - Remove bus from old route index on route change
//   - Maintain TTL on index sets
//
// KEY PATTERN:
//
//	route:{routeID}:dir:{direction}:buses → Redis SET of busIDs
//
// TTL STRATEGY:
//   - Index sets have 90-second TTL (slightly longer than bus state)
//   - Refreshed when any bus on route is updated
//   - If a bus changes route, it's removed from old index and added to new
type RouteBusIndexer interface {
	// AddToRouteIndex adds a bus to the route's direction-specific index.
	// Refreshes the index TTL.
	AddToRouteIndex(ctx context.Context, routeID string, direction int, busID string) error

	// RemoveFromRouteIndex removes a bus from a route's index.
	// Used when a bus changes routes.
	RemoveFromRouteIndex(ctx context.Context, routeID string, direction int, busID string) error

	// TransferBusRoute atomically moves a bus from one route/direction to another.
	// This prevents the bus from briefly appearing in both or neither index.
	TransferBusRoute(ctx context.Context, busID string, fromRouteID string, fromDirection int, toRouteID string, toDirection int) error
}

// BusGeoUpdater maintains the geo-spatial index for bus proximity queries.
// This enables efficient "find buses near me" queries.
//
// RESPONSIBILITIES:
//   - Update bus position in geo index on state update
//   - Remove bus from geo index on deactivation
//
// KEY PATTERN:
//
//	buses:geo → Redis GEO (GEOADD/GEOPOS/GEOSEARCH)
//
// TTL STRATEGY:
//   - Geo index entries do NOT have individual TTL
//   - Stale entries are cleaned up by a separate process (or on next update)
//   - Read operations filter against bus state existence
type BusGeoUpdater interface {
	// UpdateBusGeo updates the bus's position in the geo index.
	UpdateBusGeo(ctx context.Context, busID string, lat, lon float64) error

	// RemoveBusGeo removes the bus from the geo index.
	// Used when a bus goes offline or is deactivated.
	RemoveBusGeo(ctx context.Context, busID string) error
}

// ─────────────────────────────────────────────────────────────────────────────
// Composite Write Port
// ─────────────────────────────────────────────────────────────────────────────

// LiveBusPublisher is a composite port that handles all aspects of bus state
// publishing in a single atomic operation.
//
// This is the PREFERRED interface for most callers (simulator, EMQX consumer).
// It internally coordinates:
//  1. State validation
//  2. Staleness check
//  3. State hash update (with TTL)
//  4. Route index update (with old route cleanup if changed)
//  5. Geo index update
//
// ARCHITECTURE NOTE:
// Implementations MUST execute this as a single atomic Redis Lua script (single EVAL).
// Pipelines are not sufficient because partial failures can desynchronize state, geo,
// and route indexes.
type LiveBusPublisher interface {
	// PublishBusState handles all aspects of a bus state update.
	//
	// This is the main entry point for bus telemetry ingestion.
	// It coordinates state, index, and geo updates atomically.
	//
	// Returns UpdateResult indicating if the update was applied.
	// Returns error only on infrastructure failure.
	PublishBusState(ctx context.Context, update LiveBusUpdate) (UpdateResult, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Staleness Detection
// ─────────────────────────────────────────────────────────────────────────────
//
// STALENESS RULES:
//
// 1. Timestamp Comparison (Primary Rule)
//    - If incoming Timestamp <= existing Timestamp → REJECT
//    - If incoming Timestamp > existing Timestamp → ACCEPT
//    - If no existing state → ACCEPT
//
// 2. Clock Skew Tolerance
//    - Updates with Timestamp in the future (> now + 5s) are rejected
//    - This prevents badly-configured sources from poisoning state
//
// 3. Idempotency
//    - Retrying the same update (same BusID + Timestamp) is safe
//    - It will be rejected as "not newer" but won't corrupt state
//
// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// TTL Strategy
// ─────────────────────────────────────────────────────────────────────────────
//
// DESIGN RATIONALE:
//
// 1. Bus State TTL = 90 seconds
//    - Buses should publish at least every 30 seconds
//    - 90s TTL gives 3x buffer for network delays
//    - Expired key = bus is offline, not returned by readers
//
// 2. Route Index TTL = 90 seconds
//    - Slightly longer than bus state to avoid race conditions
//    - If bus key expires but index entry remains, read operation
//      will filter out the stale entry (no bus state to fetch)
//    - Refreshed on any update to a bus on that route
//
// 3. Geo Index = No TTL
//    - Geo entries persist until explicitly removed
//    - Read operations always validate against bus state existence
//    - Background cleanup can remove orphaned entries periodically
//
// WHAT HAPPENS WHEN BUSES STOP PUBLISHING:
//
// 1. After 90s: Bus state key expires
// 2. After 90s: Route index entry expires (or is cleaned on next access)
// 3. Geo entry remains until cleanup or next update from same bus
// 4. App layer: LiveBusFinder returns (zero, false, nil) for expired bus
// 5. App layer: LiveBusStateReader filters out buses with expired state
// 6. Result: Bus is no longer available (Redis-only architecture)
//
// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// Design Decisions & Open Questions
// ─────────────────────────────────────────────────────────────────────────────
//
// DECIDED:
//
// 1. Source-of-Truth Ordering: TIMESTAMP COMPARISON
//    - Last-write-wins is NOT used; timestamp comparison is authoritative
//    - This allows out-of-order message delivery to be handled correctly
//    - Trade-off: Requires synchronized clocks across bus telemetry sources
//
// 2. Multi-Direction Buses: NO CONFLICTS
//    - A bus can only be in ONE route/direction index at a time
//    - TransferBusRoute handles atomic movement between indexes
//    - If a bus changes direction, it's removed from old and added to new
//
// 3. Validation Failures: LOGGED, NOT PANICKED
//    - Invalid updates return Applied=false with Reason
//    - Infrastructure should log warnings for monitoring
//    - App layer can track rejection rates for alerting
//
// OPEN QUESTIONS FOR PHASE 8B:
//
// 1. Redis Downtime Handling:
//    Q: What should happen when Redis is unavailable?
//    Options:
//    a) Buffer updates in memory and retry (risk: memory pressure)
//    b) Drop updates and log (risk: temporary data loss)
//    c) Return error to caller (risk: backpressure on telemetry pipeline)
//    RECOMMENDATION: Option (c) - let caller decide retry policy
//
// 2. Geo Index Cleanup:
//    Q: How to clean up orphaned geo entries from buses that stopped publishing?
//    Options:
//    a) Background goroutine scans geo index periodically
//    b) Read operations remove entries for non-existent buses
//    c) Accept orphans (they don't affect correctness, just memory)
//    RECOMMENDATION: Option (c) for Phase 8B; address in Phase 9 if needed
//
// 3. Route Change Detection:
//    Q: How does the writer know if a bus changed routes?
//    Options:
//    a) Caller provides previous route in update (requires caller state)
//    b) Writer reads existing state inside the Lua script (no Go-side pre-read)
//    c) Route indexes are append-only; cleanup happens on read
//    RECOMMENDATION: Option (b) - compute old membership in Lua
//
// 4. Batch Updates:
//    Q: Should we support bulk updates for efficiency?
//    Currently defined: Single-update interface only
//    RECOMMENDATION: Start with single updates; add batch in Phase 9 if needed
//
// ─────────────────────────────────────────────────────────────────────────────
