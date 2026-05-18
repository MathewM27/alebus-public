package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/infrastructure/db"
)

// Infrastructure-level errors for Bus repository operations
var (
	ErrBusNotFound     = errors.New("bus not found")
	ErrBusVersionStale = errors.New("bus version conflict: stale version")
)

// PostgresBusRepository implements domain/repositories.BusRepository using Postgres
type PostgresBusRepository struct {
	pool *db.Pool
}

// NewPostgresBusRepository creates a new Postgres-backed BusRepository
func NewPostgresBusRepository(pool *db.Pool) *PostgresBusRepository {
	return &PostgresBusRepository{pool: pool}
}

// Save persists a Bus aggregate to Postgres.
// Uses UPSERT with optimistic locking via version check.
// Following error.md UPSERT template to avoid version conflict issues.
func (r *PostgresBusRepository) Save(ctx context.Context, bus *aggregates.Bus) error {
	// UPSERT with version check for optimistic locking
	// INSERT always works for new buses
	// UPDATE (via ON CONFLICT DO UPDATE) checks version match
	const query = `
		INSERT INTO buses (
			bus_id, operator_id, route_id,
			position_lat, position_lon, position_timestamp, position_accuracy, position_speed_kmh,
			stop_index, direction, current_speed, status,
			is_at_terminal, terminal_arrival_time,
			version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (bus_id) DO UPDATE SET
			operator_id = EXCLUDED.operator_id,
			route_id = EXCLUDED.route_id,
			position_lat = EXCLUDED.position_lat,
			position_lon = EXCLUDED.position_lon,
			position_timestamp = EXCLUDED.position_timestamp,
			position_accuracy = EXCLUDED.position_accuracy,
			position_speed_kmh = EXCLUDED.position_speed_kmh,
			stop_index = EXCLUDED.stop_index,
			direction = EXCLUDED.direction,
			current_speed = EXCLUDED.current_speed,
			status = EXCLUDED.status,
			is_at_terminal = EXCLUDED.is_at_terminal,
			terminal_arrival_time = EXCLUDED.terminal_arrival_time,
			version = EXCLUDED.version,
			updated_at = EXCLUDED.updated_at
		WHERE buses.version = $18
	`

	// Per error.md: Always increment version on save
	currentVersion := int(bus.Version())
	newVersion := currentVersion + 1

	position := bus.Position()

	tag, err := r.pool.Exec(ctx, query,
		string(bus.ID()),            // $1
		string(bus.OperatorID()),    // $2
		string(bus.RouteID()),       // $3
		position.Location.Latitude,  // $4
		position.Location.Longitude, // $5
		position.Timestamp,          // $6
		position.Accuracy,           // $7
		position.SpeedKmh,           // $8
		bus.StopIndex(),             // $9
		int(bus.Direction()),        // $10
		bus.CurrentSpeed(),          // $11
		int(bus.Status()),           // $12
		bus.IsAtTerminal(),          // $13
		bus.TerminalArrivalTime(),   // $14 (nullable)
		newVersion,                  // $15 version to persist
		bus.CreatedAt(),             // $16
		time.Now().UTC(),            // $17 updated_at
		currentVersion,              // $18 WHERE version = (for UPDATE check)
	)
	if err != nil {
		return fmt.Errorf("failed to save bus: %w", err)
	}

	// Per error.md: If no rows affected, version conflict
	if tag.RowsAffected() == 0 {
		return ErrBusVersionStale
	}

	return nil
}

// FindByID loads a Bus aggregate by its ID.
// Uses RehydrateBus() to reconstruct the aggregate without emitting events.
func (r *PostgresBusRepository) FindByID(ctx context.Context, busID types.BusID) (*aggregates.Bus, error) {
	const query = `
		SELECT 
			bus_id, operator_id, route_id,
			position_lat, position_lon, position_timestamp, position_accuracy, position_speed_kmh,
			stop_index, direction, current_speed, status,
			is_at_terminal, terminal_arrival_time,
			version, created_at, updated_at
		FROM buses
		WHERE bus_id = $1
	`

	var (
		id                  string
		operatorID          string
		routeID             string
		positionLat         float64
		positionLon         float64
		positionTimestamp   time.Time
		positionAccuracy    float64
		positionSpeedKmh    float64
		stopIndex           int
		direction           int
		currentSpeed        float64
		status              int
		isAtTerminal        bool
		terminalArrivalTime *time.Time
		version             int
		createdAt           time.Time
		updatedAt           time.Time
	)

	err := r.pool.QueryRow(ctx, query, string(busID)).Scan(
		&id,
		&operatorID,
		&routeID,
		&positionLat,
		&positionLon,
		&positionTimestamp,
		&positionAccuracy,
		&positionSpeedKmh,
		&stopIndex,
		&direction,
		&currentSpeed,
		&status,
		&isAtTerminal,
		&terminalArrivalTime,
		&version,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBusNotFound
		}
		return nil, fmt.Errorf("failed to query bus: %w", err)
	}

	// Reconstruct PositionSnapshot value object
	position := types.PositionSnapshot{
		Location: types.GeoLocation{
			Latitude:  positionLat,
			Longitude: positionLon,
		},
		Timestamp: positionTimestamp,
		Accuracy:  positionAccuracy,
		SpeedKmh:  positionSpeedKmh,
	}

	// Rehydrate aggregate (no events emitted)
	// Note: RehydrateBus requires a non-nil eventRecorder, but it won't emit events
	// since we're rehydrating, not creating new. We pass a no-op recorder.
	bus, err := aggregates.RehydrateBus(
		types.BusID(id),
		types.OperatorID(operatorID),
		types.RouteID(routeID),
		position,
		enums.Direction(direction),
		createdAt,
		updatedAt,
		stopIndex,
		currentSpeed,
		types.BusStatus(status),
		isAtTerminal,
		terminalArrivalTime,
		types.AggregateBusVersion(version),
		&noOpEventRecorder{}, // Required by RehydrateBus but won't emit
	)
	if err != nil {
		return nil, fmt.Errorf("failed to rehydrate bus: %w", err)
	}

	return bus, nil
}

// noOpEventRecorder is a no-op implementation of EventRecorder for rehydration.
// RehydrateBus calls NewBus internally which requires a non-nil eventRecorder,
// but no events should be emitted during rehydration.
type noOpEventRecorder struct{}

func (r *noOpEventRecorder) Record(event types.DomainEvent) error {
	// No-op: events are not recorded during rehydration
	return nil
}

// Compile-time interface satisfaction check
var _ interface {
	Save(ctx context.Context, bus *aggregates.Bus) error
	FindByID(ctx context.Context, busID types.BusID) (*aggregates.Bus, error)
} = (*PostgresBusRepository)(nil)
