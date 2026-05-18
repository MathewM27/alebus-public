package busmgmt

import (
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/types"
)

type CreateBusCommand struct {
	BusID            types.BusID
	OperatorID       types.OperatorID
	RouteID          types.RouteID
	InitalPosition   types.PositionSnapshot
	InitialStopIndex int
	Direction        enums.Direction
	CreatedAt        time.Time
	EventRecorder    types.EventRecorder
}

type UpdateBusPositionCommand struct {
	BusID       types.BusID
	NewPosition types.PositionSnapshot
	StopIndex   int
	SpeedKmh    float64
}

type ChangeBusDirectionCommand struct {
	BusID        types.BusID
	NewDirection enums.Direction
}

type ArriveAtTerminalCommand struct {
	BusID       types.BusID
	ArrivalTime time.Time
}

type DepartFromTerminalCommand struct {
	BusID         types.BusID
	DepartureTime time.Time
}

type ChangeBusStatusCommand struct {
	BusID  types.BusID
	Status types.BusStatus
	Reason string
}
