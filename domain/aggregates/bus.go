package aggregates

import (
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/errors"
	"github.com/MathewM27/busTrack-alebus/domain/types"
)

type Bus struct {
	//identity of the bus
	busID      types.BusID
	operatorID types.OperatorID
	routeID    types.RouteID

	//Bus position
	currentPosition types.PositionSnapshot
	stopIndex       int
	direction       enums.Direction
	currentSpeed    float64

	//Operational status
	status              types.BusStatus
	isAtTerminal        bool
	terminalArrivalTime *time.Time

	//Aggregate metadata
	version       types.AggregateBusVersion
	createdAt     time.Time
	updatedAt     time.Time
	eventRecorder types.EventRecorder
}

func NewBus(
	busID types.BusID,
	operatorID types.OperatorID,
	routeID types.RouteID,
	initialPosition types.PositionSnapshot,
	initialStopIndex int,
	direction enums.Direction,
	createdAt time.Time,
	eventRecorder types.EventRecorder,
) (*Bus, error) {
	if busID == "" {
		return nil, errors.ErrBusIdRequired
	}
	if operatorID == "" {
		return nil, errors.ErrOperatorIdRequired
	}
	if routeID == "" {
		return nil, errors.ErrRouteIdRequired
	}
	if !initialPosition.Location.IsValid() {
		return nil, errors.ErrInvalidGeoLocation
	}
	if initialStopIndex < 0 {
		return nil, errors.ErrInvalidStopIndex
	}
	if direction != enums.DirectionOutbound && direction != enums.DirectionInbound {
		return nil, errors.ErrInvalidDirection
	}
	if createdAt.IsZero() {
		return nil, errors.ErrCreatedAtRequired
	}
	if eventRecorder == nil {
		return nil, errors.ErrEventRecorderRequired
	}

	return &Bus{
		busID:           busID,
		operatorID:      operatorID,
		routeID:         routeID,
		currentPosition: initialPosition,
		stopIndex:       initialStopIndex,
		direction:       direction,
		currentSpeed:    initialPosition.SpeedKmh,
		status:          enums.BusStatusActive,
		isAtTerminal:    false,
		version:         1,
		createdAt:       createdAt,
		updatedAt:       createdAt,
		eventRecorder:   eventRecorder,
	}, nil
}

// ============================================================================
// GETTER METHODS - All Public Accessors
// ============================================================================

func (b *Bus) ID() types.BusID {
	return b.busID
}

func (b *Bus) OperatorID() types.OperatorID {
	return b.operatorID
}

func (b *Bus) RouteID() types.RouteID {
	return b.routeID
}

func (b *Bus) Position() types.PositionSnapshot {
	return b.currentPosition
}

func (b *Bus) Direction() enums.Direction {
	return b.direction
}

func (b *Bus) Status() types.BusStatus {
	return b.status
}

func (b *Bus) StopIndex() int {
	return b.stopIndex
}
func (b *Bus) IsAtTerminal() bool {
	return b.isAtTerminal
}

func (b *Bus) TerminalArrivalTime() *time.Time {
	return b.terminalArrivalTime
}

// NEW GETTERS - Missing in original
func (b *Bus) CurrentSpeed() float64 {
	return b.currentSpeed
}

func (b *Bus) Version() types.AggregateBusVersion {
	return b.version
}

func (b *Bus) CreatedAt() time.Time {
	return b.createdAt
}

func (b *Bus) UpdatedAt() time.Time {
	return b.updatedAt
}

// ============================================================================
// PRIVATE HELPER
// ============================================================================

func (b *Bus) recordEvent(event types.DomainEvent) error {
	if b.eventRecorder == nil {
		return errors.ErrEventRecorderRequired
	}
	return b.eventRecorder.Record(event)
}
