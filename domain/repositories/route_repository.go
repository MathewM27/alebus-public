package repositories

import (
	"context"
	"time"
	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/types"
)

type RouteRepository interface {
	Save(ctx context.Context, route *aggregates.Route) error
	FindByID(ctx context.Context, id types.RouteID) (*aggregates.Route, error)
	FindActiveRoutes(ctx context.Context, at time.Time) ([]*aggregates.Route, error)
}