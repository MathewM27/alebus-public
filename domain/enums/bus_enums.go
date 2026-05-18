package enums

import (
	"github.com/MathewM27/busTrack-alebus/domain/types"
)

// Distance type to represent distance in kilometers
type Distance float64

// Enum definition of direction and bus status


// Enum definition of bus status
const (
	BusStatusActive types.BusStatus = iota
	BusStatusOffline
	BusStatusMaintenance
)
