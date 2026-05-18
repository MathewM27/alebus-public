package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
	"github.com/MathewM27/busTrack-alebus/infrastructure/db"
)

// Infrastructure-level errors for repository operations
var (
	ErrRouteNotFound     = errors.New("route not found")
	ErrRouteVersionStale = errors.New("route version conflict: stale version")
)

// stopJSON is the JSON structure for serializing stops to JSONB
type stopJSON struct {
	ID                 string       `json:"id"`
	Name               string       `json:"name"`
	Location           locationJSON `json:"location"`
	CumulativeDistance float64      `json:"cumulative_distance"` // Distance from route origin (stop 0) in meters
}

type locationJSON struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// PostgresRouteRepository implements domain/repositories.RouteRepository using Postgres
type PostgresRouteRepository struct {
	pool *db.Pool
}

// NewPostgresRouteRepository creates a new Postgres-backed RouteRepository
func NewPostgresRouteRepository(pool *db.Pool) *PostgresRouteRepository {
	return &PostgresRouteRepository{pool: pool}
}

// Save persists a Route aggregate to Postgres.
// Uses UPSERT with optimistic locking via version check.
func (r *PostgresRouteRepository) Save(ctx context.Context, route *aggregates.Route) error {
	// Marshal operator IDs to JSONB
	operatorIDsJSON, err := r.marshalOperatorIDs(route.OperatorIDs())
	if err != nil {
		return fmt.Errorf("failed to marshal operator IDs: %w", err)
	}

	// Marshal stops to JSONB
	stopsJSON, err := r.marshalStops(route.Stops())
	if err != nil {
		return fmt.Errorf("failed to marshal stops: %w", err)
	}

	// Use UPSERT with version check for optimistic locking
	// For INSERT: always succeeds for new routes
	// For UPDATE: version must match the current DB version
	const query = `
		INSERT INTO routes (
			route_id, name, operator_ids, stops, direction, route_type,
			avg_detour_rate, status, active_from, active_until,
			version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (route_id) DO UPDATE SET
			name = EXCLUDED.name,
			operator_ids = EXCLUDED.operator_ids,
			stops = EXCLUDED.stops,
			direction = EXCLUDED.direction,
			route_type = EXCLUDED.route_type,
			avg_detour_rate = EXCLUDED.avg_detour_rate,
			status = EXCLUDED.status,
			active_from = EXCLUDED.active_from,
			active_until = EXCLUDED.active_until,
			version = EXCLUDED.version,
			updated_at = EXCLUDED.updated_at
		WHERE routes.version = $14
	`

	// Optimistic locking:
	// - For new routes (never saved before): expectedVersion = route.Version() (will be 1)
	//   But INSERT doesn't check version, so this is fine
	// - For loaded routes: expectedVersion = route.Version() (the version we loaded)
	//   UPDATE will only succeed if DB still has this version
	//
	// On save, we increment the version
	currentVersion := int(route.Version())
	newVersion := currentVersion + 1

	tag, err := r.pool.Exec(ctx, query,
		string(route.ID()),
		route.Name(),
		operatorIDsJSON,
		stopsJSON,
		int(route.Direction()),
		int(route.RouteType()),
		r.getAvgDetourRate(route.RouteType()),
		int(route.Status()),
		route.ActiveFrom(),
		route.ActiveUntil(),
		newVersion,
		route.CreatedAt(),
		time.Now().UTC(),
		currentVersion, // WHERE routes.version = current version
	)
	if err != nil {
		return fmt.Errorf("failed to save route: %w", err)
	}

	// If no rows affected and it's an update (conflict path), version mismatch
	// For INSERT path, rows affected will be 1
	if tag.RowsAffected() == 0 {
		return ErrRouteVersionStale
	}

	// Sync stops to the PostGIS-indexed stops table for efficient geo queries
	if err := r.syncStopsToGeoTable(ctx, route); err != nil {
		// Log but don't fail the save - stops table is a read model optimization
		// In production, this could be async or use eventual consistency
		fmt.Printf("warning: failed to sync stops to geo table: %v\n", err)
	}

	return nil
}

// FindByID loads a Route aggregate by its ID.
// Uses RehydrateRoute() to reconstruct the aggregate without emitting events.
func (r *PostgresRouteRepository) FindByID(ctx context.Context, id types.RouteID) (*aggregates.Route, error) {
	const query = `
		SELECT 
			route_id, name, operator_ids, stops, direction, route_type,
			avg_detour_rate, status, active_from, active_until,
			version, created_at, updated_at
		FROM routes
		WHERE route_id = $1
	`

	var (
		routeID       string
		name          string
		operatorIDs   []byte
		stops         []byte
		direction     int
		routeType     int
		avgDetourRate float64
		status        int
		activeFrom    time.Time
		activeUntil   time.Time
		version       int
		createdAt     time.Time
		updatedAt     time.Time
	)

	err := r.pool.QueryRow(ctx, query, string(id)).Scan(
		&routeID,
		&name,
		&operatorIDs,
		&stops,
		&direction,
		&routeType,
		&avgDetourRate,
		&status,
		&activeFrom,
		&activeUntil,
		&version,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRouteNotFound
		}
		return nil, fmt.Errorf("failed to query route: %w", err)
	}

	// Unmarshal JSONB fields
	opIDs, err := r.unmarshalOperatorIDs(operatorIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal operator IDs: %w", err)
	}

	stopsList, err := r.unmarshalStops(stops)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal stops: %w", err)
	}

	// Rehydrate aggregate (no events emitted)
	route, err := aggregates.RehydrateRoute(
		types.RouteID(routeID),
		opIDs,
		stopsList,
		name,
		enums.RouteDirection(direction),
		enums.RouteType(routeType),
		enums.RouteStatus(status),
		activeFrom,
		activeUntil,
		createdAt,
		updatedAt,
		types.AggregateRouteVersion(version),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to rehydrate route: %w", err)
	}

	return route, nil
}

// FindActiveRoutes returns all routes that are active at the given time.
// Active means: status = Active AND activeFrom <= at AND activeUntil > at
func (r *PostgresRouteRepository) FindActiveRoutes(ctx context.Context, at time.Time) ([]*aggregates.Route, error) {
	const query = `
		SELECT 
			route_id, name, operator_ids, stops, direction, route_type,
			avg_detour_rate, status, active_from, active_until,
			version, created_at, updated_at
		FROM routes
		WHERE status = $1 AND active_from <= $2 AND active_until > $2
		ORDER BY name
	`

	rows, err := r.pool.Query(ctx, query, int(enums.RouteStatusActive), at)
	if err != nil {
		return nil, fmt.Errorf("failed to query active routes: %w", err)
	}
	defer rows.Close()

	var routes []*aggregates.Route
	for rows.Next() {
		var (
			routeID       string
			name          string
			operatorIDs   []byte
			stops         []byte
			direction     int
			routeType     int
			avgDetourRate float64
			status        int
			activeFrom    time.Time
			activeUntil   time.Time
			version       int
			createdAt     time.Time
			updatedAt     time.Time
		)

		err := rows.Scan(
			&routeID,
			&name,
			&operatorIDs,
			&stops,
			&direction,
			&routeType,
			&avgDetourRate,
			&status,
			&activeFrom,
			&activeUntil,
			&version,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan route row: %w", err)
		}

		opIDs, err := r.unmarshalOperatorIDs(operatorIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal operator IDs: %w", err)
		}

		stopsList, err := r.unmarshalStops(stops)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal stops: %w", err)
		}

		route, err := aggregates.RehydrateRoute(
			types.RouteID(routeID),
			opIDs,
			stopsList,
			name,
			enums.RouteDirection(direction),
			enums.RouteType(routeType),
			enums.RouteStatus(status),
			activeFrom,
			activeUntil,
			createdAt,
			updatedAt,
			types.AggregateRouteVersion(version),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to rehydrate route %s: %w", routeID, err)
		}

		routes = append(routes, route)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating routes: %w", err)
	}

	return routes, nil
}

// marshalOperatorIDs converts []types.OperatorID to JSON bytes
func (r *PostgresRouteRepository) marshalOperatorIDs(ids []types.OperatorID) ([]byte, error) {
	strIDs := make([]string, len(ids))
	for i, id := range ids {
		strIDs[i] = string(id)
	}
	return json.Marshal(strIDs)
}

// unmarshalOperatorIDs converts JSON bytes to []types.OperatorID
func (r *PostgresRouteRepository) unmarshalOperatorIDs(data []byte) ([]types.OperatorID, error) {
	var strIDs []string
	if err := json.Unmarshal(data, &strIDs); err != nil {
		return nil, err
	}
	ids := make([]types.OperatorID, len(strIDs))
	for i, s := range strIDs {
		ids[i] = types.OperatorID(s)
	}
	return ids, nil
}

// marshalStops converts []valueobjects.Stop to JSON bytes
func (r *PostgresRouteRepository) marshalStops(stops []valueobjects.Stop) ([]byte, error) {
	jsonStops := make([]stopJSON, len(stops))
	for i, s := range stops {
		jsonStops[i] = stopJSON{
			ID:   string(s.ID),
			Name: s.Name,
			Location: locationJSON{
				Latitude:  s.Location.Latitude,
				Longitude: s.Location.Longitude,
			},
			CumulativeDistance: s.CumulativeDistanceMeters,
		}
	}
	return json.Marshal(jsonStops)
}

// unmarshalStops converts JSON bytes to []valueobjects.Stop
func (r *PostgresRouteRepository) unmarshalStops(data []byte) ([]valueobjects.Stop, error) {
	var jsonStops []stopJSON
	if err := json.Unmarshal(data, &jsonStops); err != nil {
		return nil, err
	}
	stops := make([]valueobjects.Stop, len(jsonStops))
	for i, js := range jsonStops {
		stops[i] = valueobjects.Stop{
			ID:   types.StopID(js.ID),
			Name: js.Name,
			Location: types.GeoLocation{
				Latitude:  js.Location.Latitude,
				Longitude: js.Location.Longitude,
			},
			CumulativeDistanceMeters: js.CumulativeDistance,
		}
	}
	return stops, nil
}

// getAvgDetourRate returns the detour rate for a route type
// This mirrors the domain logic in Route.setRouteCharacteristics
func (r *PostgresRouteRepository) getAvgDetourRate(rt enums.RouteType) float64 {
	switch rt {
	case enums.RouteTypeUrban:
		return 1.3
	case enums.RouteTypeHighway:
		return 1.1
	case enums.RouteTypeMixed:
		return 1.2
	default:
		return 1.3
	}
}

// syncStopsToGeoTable syncs stops from the route to the PostGIS-indexed stops table.
// This maintains a read model optimized for geospatial queries (FindNearbyStops).
// Uses UPSERT to handle both new and existing stops, and appends route_id to the array.
func (r *PostgresRouteRepository) syncStopsToGeoTable(ctx context.Context, route *aggregates.Route) error {
	const query = `
		INSERT INTO stops (stop_id, name, location, route_ids, cumulative_distance_meters)
		VALUES ($1, $2, ST_SetSRID(ST_MakePoint($3, $4), 4326)::geography, ARRAY[$5::text], $6)
		ON CONFLICT (stop_id) DO UPDATE SET
			name = EXCLUDED.name,
			location = EXCLUDED.location,
			route_ids = (
				SELECT ARRAY(SELECT DISTINCT unnest(array_cat(stops.route_ids, EXCLUDED.route_ids)) ORDER BY 1)
			),
			cumulative_distance_meters = COALESCE(EXCLUDED.cumulative_distance_meters, stops.cumulative_distance_meters)
	`

	for _, stop := range route.Stops() {
		_, err := r.pool.Exec(ctx, query,
			string(stop.ID),
			stop.Name,
			stop.Location.Longitude, // PostGIS uses (lon, lat) order
			stop.Location.Latitude,
			string(route.ID()),
			stop.CumulativeDistanceMeters,
		)
		if err != nil {
			return fmt.Errorf("failed to sync stop %s: %w", stop.ID, err)
		}
	}

	return nil
}

// Compile-time interface satisfaction check
var _ interface {
	Save(ctx context.Context, route *aggregates.Route) error
	FindByID(ctx context.Context, id types.RouteID) (*aggregates.Route, error)
	FindActiveRoutes(ctx context.Context, at time.Time) ([]*aggregates.Route, error)
} = (*PostgresRouteRepository)(nil)
