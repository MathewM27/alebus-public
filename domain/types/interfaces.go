// Package types contains domain types and port interfaces.
package types

import (
	"context"
)

// RouteDistanceCalculator is a domain port for calculating distance between stops.
// Infrastructure implements the fallback policy (e.g., external API → cache → approximation).
// This keeps orchestration logic OUT of the aggregate.
//
// NOTE: As of Dec 2024, no external distance service is wired in the application.
// Route.CalculateDistanceBetweenStops will fall back to ApproximateDistanceBetweenStops
// if no calculator is provided or if the calculator returns an error.
type RouteDistanceCalculator interface {
	CalculateDistance(ctx context.Context, from, to GeoLocation) (Distance, error)
}

// BusLocationInfo represents a bus's identity and location for geo-spatial queries.
type BusLocationInfo struct {
	BusID    BusID
	Location GeoLocation
}

func (b BusLocationInfo) ID() BusID {
	return b.BusID
}
