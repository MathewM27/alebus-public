package events

import (
	"time"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/enums"
)


type RouteCreated struct {
    EventIDValue     string // <-- Add this field
    RouteID     types.RouteID
    OperatorIDs []types.OperatorID
    Name        string
    Stops       []string
    Direction   enums.RouteDirection
    RouteType   enums.RouteType
    ActiveFrom  time.Time
    ActiveUntil time.Time
    OccurredAtTime  time.Time
}

func (e RouteCreated) EventID() string      { return e.EventIDValue }
func (e RouteCreated) EventType() string    { return "RouteCreated" }
func (e RouteCreated) OccurredAt() time.Time { return e.OccurredAtTime }



type RouteStatusChanged struct {
	EventIDValue string
	RouteID 	types.RouteID
	OldStatus  enums.RouteStatus
	NewStatus enums.RouteStatus
	Reason string
	OccurredAtTime   time.Time
}

func (e RouteStatusChanged) EventID() string      { return e.EventIDValue }
func (e RouteStatusChanged) EventType() string    { return "RouteStatusChanged" }
func (e RouteStatusChanged) OccurredAt() time.Time { return e.OccurredAtTime }


type RouteStopsUpdated struct {
	EventIDValue string
	RouteID   	types.RouteID
	OldStops  	[]types.StopID
	NewStops  	[]types.StopID
	Reason 		string
	OccurredAtTime   time.Time
}
func (e RouteStopsUpdated) EventID() string      { return e.EventIDValue }
func (e RouteStopsUpdated) EventType() string    { return "RouteStopsUpdated" }
func (e RouteStopsUpdated) OccurredAt() time.Time { return e.OccurredAtTime }



type RouteTypeChanged struct {
	EventIDValue string
	RouteID   	types.RouteID
	OldType  	enums.RouteType
	NewType  	enums.RouteType
	Reason 		string
	OccurredAtTime   time.Time
}
func (e RouteTypeChanged) EventID() string      { return e.EventIDValue }
func (e RouteTypeChanged) EventType() string    { return "RouteTypeChanged" }
func (e RouteTypeChanged) OccurredAt() time.Time { return e.OccurredAtTime }


type RouteActivePeriodChanged struct {
	EventIDValue string
	RouteID   	types.RouteID
	OldActiveFrom  	time.Time
	OldActiveUntil  time.Time
	NewActiveFrom  	time.Time
	NewActiveUntil  time.Time
	Reason 			string
	OccurredAtTime   time.Time
}
func (e RouteActivePeriodChanged) EventID() string      { return e.EventIDValue }
func (e RouteActivePeriodChanged) EventType() string    { return "RouteActivePeriodChanged" }
func (e RouteActivePeriodChanged) OccurredAt() time.Time { return e.OccurredAtTime }



type RouteNameChanged struct {
    EventIDValue   string
    RouteID        types.RouteID
    OldName        string
    NewName        string
    Reason         string
    OccurredAtTime time.Time
}

func (e RouteNameChanged) EventID() string      { return e.EventIDValue }
func (e RouteNameChanged) EventType() string    { return "RouteNameChanged" }
func (e RouteNameChanged) OccurredAt() time.Time { return e.OccurredAtTime }