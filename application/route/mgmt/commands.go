package routemgmt

import (
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
)

// CreateRouteCommand is an input DTO for creating a new route.
// Contains raw input - no behavior, no validation.
// Validation happens in the domain aggregate.
type CreateRouteCommand struct {
	RouteID       types.RouteID
	Name          string
	OperatorIDs   []types.OperatorID
	Stops         []valueobjects.Stop
	Direction     enums.RouteDirection
	RouteType     enums.RouteType
	ActiveFrom    time.Time
	ActiveUntil   time.Time
	CreatedAt     time.Time            // When the route is being created
	EventRecorder types.EventRecorder  // For domain events
}

// UpdateRouteStopsCommand is an input DTO for updating route stops.
type UpdateRouteStopsCommand struct {
	RouteID types.RouteID
	Stops   []valueobjects.Stop
	Reason  string
}

// ChangeRouteStatusCommand is an input DTO for changing route status.
type ChangeRouteStatusCommand struct {
	RouteID types.RouteID
	Status  enums.RouteStatus
	Reason  string
}

// ChangeRouteNameCommand is an input DTO for changing route name.
type ChangeRouteNameCommand struct {
	RouteID types.RouteID
	Name    string
	Reason  string
}

// ChangeRouteTypeCommand is an input DTO for changing route type.
type ChangeRouteTypeCommand struct {
	RouteID   types.RouteID
	RouteType enums.RouteType
	Reason    string
}

// ChangeActivePeriodCommand is an input DTO for changing route active period.
type ChangeActivePeriodCommand struct {
	RouteID     types.RouteID
	ActiveFrom  time.Time
	ActiveUntil time.Time
	Reason      string
}

// AddStopCommand is an input DTO for adding a stop to a route.
type AddStopCommand struct {
	RouteID  types.RouteID
	Stop     valueobjects.Stop
	Position int
	Reason   string
}

// RemoveStopCommand is an input DTO for removing a stop from a route.
type RemoveStopCommand struct {
	RouteID types.RouteID
	StopID  types.StopID
	Reason  string
}
