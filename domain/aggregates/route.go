package aggregates

import (
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/errors"
	"github.com/MathewM27/busTrack-alebus/domain/events"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
)

// Route aggregate represents a bus route in the system.
// Business rules and behaviors related to routes will be encapsulated here.

type Route struct {
	routeID       types.RouteID
	operatorID    []types.OperatorID
	stops         []valueobjects.Stop
	name          string
	direction     enums.RouteDirection
	routeType     enums.RouteType
	avgDetourRate float64
	status        enums.RouteStatus
	activeFrom    time.Time
	activeUntil   time.Time

	version types.AggregateRouteVersion

	createdAt     time.Time
	updatedAt     time.Time
	eventRecorder types.EventRecorder
}

func NewRoute(
	routeID types.RouteID,
	operatorID []types.OperatorID,
	stops []valueobjects.Stop,
	name string,
	direction enums.RouteDirection,
	routeType enums.RouteType,
	activeFrom time.Time,
	activeUntil time.Time,
	createdAt time.Time,
	eventRecorder types.EventRecorder,
) (*Route, error) {
	if routeID == "" {
		return nil, errors.ErrRouteIdRequired
	}
	if len(operatorID) == 0 {
		return nil, errors.ErrOperatorIdRequired
	}
	if name == "" {
		return nil, errors.ErrRouteNameRequired
	}
	if len(stops) < 2 {
		return nil, errors.ErrInsufficientStops
	}
	if direction != enums.RouteDirectionBidirectional && direction != enums.RouteDirectionUnidirectional {
		return nil, errors.ErrInvalidRouteDirection
	}
	if routeType != enums.RouteTypeUrban && routeType != enums.RouteTypeHighway && routeType != enums.RouteTypeMixed {
		return nil, errors.ErrInvalidRouteType
	}
	if activeFrom.IsZero() || activeUntil.IsZero() || !activeUntil.After(activeFrom) {
		return nil, errors.ErrInvalidActivePeriod
	}
	r := &Route{
		routeID:       routeID,
		operatorID:    operatorID,
		stops:         stops,
		name:          name,
		direction:     direction,
		routeType:     routeType,
		status:        enums.RouteStatusActive,
		activeFrom:    activeFrom,
		activeUntil:   activeUntil,
		createdAt:     createdAt,
		updatedAt:     createdAt,
		eventRecorder: eventRecorder,
		version:       1,
	}
	r.setRouteCharacteristics(routeType)

	// Calculate cumulative distances for all stops (algorithm.md compliance)
	// This enables segment-based distance calculations for bus tracking
	r.CalculateCumulativeDistances()

	// Record RouteCreated event
	if eventRecorder != nil {
		stopIDs := make([]string, len(stops))
		for i, s := range stops {
			stopIDs[i] = string(s.ID)
		}
		_ = eventRecorder.Record(events.RouteCreated{
			RouteID:        routeID,
			OperatorIDs:    operatorID,
			Name:           name,
			Stops:          stopIDs,
			Direction:      direction,
			RouteType:      routeType,
			ActiveFrom:     activeFrom,
			ActiveUntil:    activeUntil,
			OccurredAtTime: createdAt,
		})
	}

	return r, nil
}

//Getters

func (r *Route) ID() types.RouteID {
	return r.routeID
}

func (r *Route) OperatorIDs() []types.OperatorID {
	return r.operatorID
}
func (r *Route) Stops() []valueobjects.Stop {
	return r.stops
}

func (r *Route) Name() string {
	return r.name
}
func (r *Route) Direction() enums.RouteDirection {
	return r.direction
}
func (r *Route) RouteType() enums.RouteType {
	return r.routeType
}

func (r *Route) Status() enums.RouteStatus {
	return r.status
}

func (r *Route) ActiveFrom() time.Time {
	return r.activeFrom
}

func (r *Route) ActiveUntil() time.Time {
	return r.activeUntil
}

func (r *Route) CreatedAt() time.Time {
	return r.createdAt
}

func (r *Route) UpdatedAt() time.Time {
	return r.updatedAt
}

func (r *Route) Version() types.AggregateRouteVersion {
	return r.version
}

// Add this getter method
func (r *Route) ActivePeriod() valueobjects.TimePeriod {
	return valueobjects.TimePeriod{
		StartTime: r.activeFrom,  // Use existing field names
		EndTime:   r.activeUntil, // Use existing field names
	}
}
