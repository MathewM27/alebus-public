package valueobjects

import (
    "github.com/MathewM27/busTrack-alebus/domain/types"
)

type SavedLocation struct {
    Name     string
    Location types.GeoLocation
    StopID   *types.StopID // Use StopID from types, not valueobjects
}