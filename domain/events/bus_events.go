package events

import (
	"time"
	// "github.com/MathewM27/busTrack-alebus/domain/errors"
	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/types"
)

type BusPositionUpdated struct {
	EventIDValue     string
	BusID            types.BusID
	PreviousPosition types.PositionSnapshot
	NewPosition      types.PositionSnapshot
	StopIndex        int
	SpeedKmh         float64
	OccurredAtTime   time.Time
}

func (e *BusPositionUpdated) EventID() string {
	return e.EventIDValue
}

// OccurredAt returns the time the event occurred
func (e *BusPositionUpdated) OccurredAt() time.Time {
	return e.OccurredAtTime
}

func (e *BusPositionUpdated) EventType() string {
	return "BusPositionUpdated"
}

// Bus direction event
type BusDirectionChanged struct {
	EventIDValue   string
	BusID          types.BusID
	OldDirection   enums.Direction
	NewDirection   enums.Direction
	OccurredAtTime time.Time
}

func (e *BusDirectionChanged) EventID() string {
	return e.EventIDValue
}

// EventType returns the type of the event
func (e *BusDirectionChanged) EventType() string {
	return "BusDirectionChanged"
}

// OccurredAt returns the time the event occurred
func (e *BusDirectionChanged) OccurredAt() time.Time {
	return e.OccurredAtTime
}

// Event for bus arrival at terminal
type BusArrivedAtTerminal struct {
	EventIDValue   string
	BusID          types.BusID
	ArrivalTime    time.Time
	OccurredAtTime time.Time
}

func (e *BusArrivedAtTerminal) EventID() string {
	return e.EventIDValue
}

// EventType returns the arrival time
func (e *BusArrivedAtTerminal) EventType() string {
	return "BusArrivedAtTerminal"
}

// OccurredAt returns the time the event occurred
func (e *BusArrivedAtTerminal) OccurredAt() time.Time {
	return e.OccurredAtTime
}

// Bus departed from terminal event
type BusDepartedFromTerminal struct {
	EventIDValue   string
	BusID          types.BusID
	DepartureTime  time.Time
	OccurredAtTime time.Time
}

func (e *BusDepartedFromTerminal) EventID() string {
	return e.EventIDValue
}

// EventType returns the type of the event
func (e *BusDepartedFromTerminal) EventType() string {
	return "BusDepartedFromTerminal"
}

// OccurredAt returns the time the event occurred
func (e *BusDepartedFromTerminal) OccurredAt() time.Time {
	return e.OccurredAtTime
}

//Bus change status event

type BusStatusChanged struct {
	EventIDValue   string
	BusID          types.BusID
	OldStatus      types.BusStatus
	NewStatus      types.BusStatus
	Reason         string
	OccurredAtTime time.Time
}

func (e *BusStatusChanged) EventID() string {
	return e.EventIDValue
}

// EventType returns the type of the event
func (e *BusStatusChanged) EventType() string {
	return "BusStatusChanged"
}

// OccurredAt returns the time the event occurred
func (e *BusStatusChanged) OccurredAt() time.Time {
	return e.OccurredAtTime
}
