package repositories

import (
	"context"
	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/types"
)

type BusRepository interface {
	Save(ctx context.Context, bus *aggregates.Bus) error
	FindByID(ctx context.Context, busID types.BusID) (*aggregates.Bus, error)
	// GetBusesByOperatorID(ctx context.Context, operatorID types.OperatorID) ([]*aggregates.Bus, error)
	// GetBusesByRouteID(ctx context.Context, routeID types.RouteID) ([]*aggregates.Bus, error)
	// GetActiveBusesAtTime(ctx context.Context, t time.Time) ([]*aggregates.Bus, error)
	// GetNearestBuses(ctx context.Context, position types.PositionSnapshot, radiusMeters float64) ([]*aggregates.Bus, error)
	// FindNearLocation(ctx context.Context, location types.GeoLocation, radiusKm float64) ([]*aggregates.Bus, error)
	// Delete(ctx context.Context, busID types.BusID) error
}