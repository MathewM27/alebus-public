package aggregates

import (
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/types"
)

// RehydrateBus rebuilds a Bus aggregate from a persisted snapshot.
// It enforces NewBus invariants but does not emit domain events.
func RehydrateBus(
	busID types.BusID,
	operatorID types.OperatorID,
	routeID types.RouteID,
	position types.PositionSnapshot,
	direction enums.Direction,
	createdAt time.Time,
	updatedAt time.Time,
	stopIndex int,
	currentSpeed float64,
	status types.BusStatus,
	isAtTerminal bool,
	terminalArrivalTime *time.Time,
	version types.AggregateBusVersion,
	eventRecorder types.EventRecorder,
) (*Bus, error) {
	b, err := NewBus(
		busID,
		operatorID,
		routeID,
		position,
		stopIndex,
		direction,
		createdAt,
		eventRecorder,
	)
	if err != nil {
		return nil, err
	}

	b.updatedAt = updatedAt
	b.currentSpeed = currentSpeed
	b.status = status
	b.isAtTerminal = isAtTerminal
	b.terminalArrivalTime = terminalArrivalTime
	if version > 0 {
		b.version = version
	}

	return b, nil
}
