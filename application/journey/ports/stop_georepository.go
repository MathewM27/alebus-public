package ports

import (
	"context"

	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
)

// StopGeoRepository is an infrastructure port for geospatial lookups.
// This is NOT a domain repository - it's a read-optimized query port.
// Implementation will use PostGIS, Redis, or similar technology.
type StopGeoRepository interface {
	// FindNearbyStops returns stops within the given radius of a location.
	// Returns value objects only, not aggregates.
	FindNearbyStops(
		ctx context.Context,
		lat float64,
		lon float64,
		radiusMeters float64,
	) ([]valueobjects.Stop, error)

	// FindStopByID retrieves a stop by its ID.
	// This supports destination selection by stop (e.g., autocomplete) without requiring destination GPS.
	FindStopByID(
		ctx context.Context,
		stopID string,
	) (valueobjects.Stop, bool, error)
}
