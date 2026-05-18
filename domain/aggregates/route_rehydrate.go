package aggregates

import (
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
)

// RehydrateRoute rebuilds a Route aggregate from a persisted snapshot.
// It enforces the same invariants as NewRoute but does NOT emit domain events.
func RehydrateRoute(
	routeID types.RouteID,
	operatorIDs []types.OperatorID,
	stops []valueobjects.Stop,
	name string,
	direction enums.RouteDirection,
	routeType enums.RouteType,
	status enums.RouteStatus,
	activeFrom time.Time,
	activeUntil time.Time,
	createdAt time.Time,
	updatedAt time.Time,
	version types.AggregateRouteVersion,
) (*Route, error) {
	r, err := NewRoute(
		routeID,
		operatorIDs,
		stops,
		name,
		direction,
		routeType,
		activeFrom,
		activeUntil,
		createdAt,
		nil, // no events on rehydration
	)
	if err != nil {
		return nil, err
	}

	r.status = status
	r.updatedAt = updatedAt
	if version > 0 {
		r.version = version
	}

	return r, nil
}
