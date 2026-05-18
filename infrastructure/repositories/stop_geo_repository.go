package repositories

import (
	"context"
	"fmt"

	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
	"github.com/MathewM27/busTrack-alebus/infrastructure/db"
)

// PostgresStopGeoRepository implements ports.StopGeoRepository using PostGIS.
// This is a read-optimized query adapter for geospatial stop lookups.
// Uses PostGIS ST_DWithin with GIST spatial index for efficient queries.
type PostgresStopGeoRepository struct {
	pool *db.Pool
}

// NewPostgresStopGeoRepository creates a new PostGIS-backed StopGeoRepository.
func NewPostgresStopGeoRepository(pool *db.Pool) *PostgresStopGeoRepository {
	return &PostgresStopGeoRepository{pool: pool}
}

// FindNearbyStops returns stops within the given radius of a location.
// Uses PostGIS ST_DWithin with GIST index for O(log n) performance.
// Returns stops ordered by distance (closest first).
func (r *PostgresStopGeoRepository) FindNearbyStops(
	ctx context.Context,
	lat float64,
	lon float64,
	radiusMeters float64,
) ([]valueobjects.Stop, error) {
	// PostGIS query using GIST spatial index
	// ST_DWithin filters by distance, then we calculate exact distance for sorting
	// Note: PostGIS uses (longitude, latitude) order for ST_MakePoint
	const query = `
		SELECT 
			stop_id,
			name,
			ST_Y(location::geometry) as latitude,
			ST_X(location::geometry) as longitude,
			cumulative_distance_meters,
			ST_Distance(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) as distance_meters
		FROM stops
		WHERE ST_DWithin(
			location,
			ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
			$3
		)
		ORDER BY distance_meters ASC
	`

	rows, err := r.pool.Query(ctx, query, lon, lat, radiusMeters)
	if err != nil {
		return nil, fmt.Errorf("failed to find nearby stops: %w", err)
	}
	defer rows.Close()

	var result []valueobjects.Stop
	for rows.Next() {
		var stopID, name string
		var latitude, longitude float64
		var cumulativeDistance *float64
		var distanceMeters float64

		if err := rows.Scan(&stopID, &name, &latitude, &longitude, &cumulativeDistance, &distanceMeters); err != nil {
			continue
		}

		cumDist := float64(0)
		if cumulativeDistance != nil {
			cumDist = *cumulativeDistance
		}

		result = append(result, valueobjects.Stop{
			ID:   types.StopID(stopID),
			Name: name,
			Location: types.GeoLocation{
				Latitude:  latitude,
				Longitude: longitude,
			},
			CumulativeDistanceMeters: cumDist,
		})
	}

	return result, nil
}

// FindStopByID retrieves a stop by its ID.
func (r *PostgresStopGeoRepository) FindStopByID(
	ctx context.Context,
	stopID string,
) (valueobjects.Stop, bool, error) {
	const query = `
		SELECT 
			stop_id,
			name,
			ST_Y(location::geometry) as latitude,
			ST_X(location::geometry) as longitude,
			cumulative_distance_meters
		FROM stops
		WHERE stop_id = $1
	`

	var name string
	var latitude, longitude float64
	var cumulativeDistance *float64

	err := r.pool.QueryRow(ctx, query, stopID).Scan(&stopID, &name, &latitude, &longitude, &cumulativeDistance)
	if err != nil {
		// Check if no rows - that means stop not found
		if err.Error() == "no rows in result set" {
			return valueobjects.Stop{}, false, nil
		}
		return valueobjects.Stop{}, false, fmt.Errorf("failed to find stop: %w", err)
	}

	cumDist := float64(0)
	if cumulativeDistance != nil {
		cumDist = *cumulativeDistance
	}

	return valueobjects.Stop{
		ID:   types.StopID(stopID),
		Name: name,
		Location: types.GeoLocation{
			Latitude:  latitude,
			Longitude: longitude,
		},
		CumulativeDistanceMeters: cumDist,
	}, true, nil
}

// FindAllStops returns all stops in the system.
// Used for autocomplete/search functionality.
func (r *PostgresStopGeoRepository) FindAllStops(ctx context.Context) ([]valueobjects.Stop, error) {
	const query = `
		SELECT 
			stop_id,
			name,
			ST_Y(location::geometry) as latitude,
			ST_X(location::geometry) as longitude,
			cumulative_distance_meters
		FROM stops
		ORDER BY name ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to find all stops: %w", err)
	}
	defer rows.Close()

	var result []valueobjects.Stop
	for rows.Next() {
		var stopID, name string
		var latitude, longitude float64
		var cumulativeDistance *float64

		if err := rows.Scan(&stopID, &name, &latitude, &longitude, &cumulativeDistance); err != nil {
			continue
		}

		cumDist := float64(0)
		if cumulativeDistance != nil {
			cumDist = *cumulativeDistance
		}

		result = append(result, valueobjects.Stop{
			ID:   types.StopID(stopID),
			Name: name,
			Location: types.GeoLocation{
				Latitude:  latitude,
				Longitude: longitude,
			},
			CumulativeDistanceMeters: cumDist,
		})
	}

	return result, nil
}

// FindNearestStops returns the K nearest stops to a location.
// Uses PostGIS KNN (K-Nearest Neighbors) with <-> operator for efficient lookup.
func (r *PostgresStopGeoRepository) FindNearestStops(
	ctx context.Context,
	lat float64,
	lon float64,
	limit int,
) ([]valueobjects.Stop, error) {
	// PostGIS KNN query using <-> operator with GIST index
	const query = `
		SELECT 
			stop_id,
			name,
			ST_Y(location::geometry) as latitude,
			ST_X(location::geometry) as longitude,
			cumulative_distance_meters,
			ST_Distance(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) as distance_meters
		FROM stops
		ORDER BY location <-> ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
		LIMIT $3
	`

	rows, err := r.pool.Query(ctx, query, lon, lat, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to find nearest stops: %w", err)
	}
	defer rows.Close()

	var result []valueobjects.Stop
	for rows.Next() {
		var stopID, name string
		var latitude, longitude float64
		var cumulativeDistance *float64
		var distanceMeters float64

		if err := rows.Scan(&stopID, &name, &latitude, &longitude, &cumulativeDistance, &distanceMeters); err != nil {
			continue
		}

		cumDist := float64(0)
		if cumulativeDistance != nil {
			cumDist = *cumulativeDistance
		}

		result = append(result, valueobjects.Stop{
			ID:   types.StopID(stopID),
			Name: name,
			Location: types.GeoLocation{
				Latitude:  latitude,
				Longitude: longitude,
			},
			CumulativeDistanceMeters: cumDist,
		})
	}

	return result, nil
}
