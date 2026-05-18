package valueobjects

import (
	"github.com/MathewM27/busTrack-alebus/domain/types"
)

// Stop represents a bus stop on a route.
// CumulativeDistanceMeters is the distance from the route's origin (stop index 0)
// to this stop, following the route path. This enables segment-based distance
// calculations that are more accurate than pure GPS Haversine.
type Stop struct {
	ID                       types.StopID
	Name                     string
	Location                 types.GeoLocation
	CumulativeDistanceMeters float64 // Distance from route origin (stop 0) to this stop
}
