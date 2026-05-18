package types

import (
	"time"
)

// Define Bus types
type BusID string
type OperatorID string

type Direction int
type BusStatus int
type AggregateBusVersion int

// PositionSnapshot struct to hold bus position data
type PositionSnapshot struct {
	Location  GeoLocation
	Timestamp time.Time
	Accuracy  float64
	SpeedKmh  float64
}
