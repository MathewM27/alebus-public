package aggregates

import (
	"time"

	"github.com/google/uuid"

	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/errors"
	"github.com/MathewM27/busTrack-alebus/domain/events"
	"github.com/MathewM27/busTrack-alebus/domain/types"
)

// UpdatePosition updates the bus's current position, stop index, and speed every time the bus reports a new position.
func (b *Bus) UpdatePosition(newPosition types.PositionSnapshot, stopIndex int, speedKmh float64) error {
	//Ensure the new position timestamp is after the current one
	if newPosition.Timestamp.Before(b.currentPosition.Timestamp) {
		return errors.ErrPositionTimestampRegression
	}
	// Ensure valid position
	if !newPosition.Location.IsValid() {
		return errors.ErrInvalidGeoLocation
	}
	previousPosition := b.currentPosition
	b.currentPosition = newPosition
	b.stopIndex = stopIndex
	b.currentSpeed = speedKmh
	b.updatedAt = time.Now()

	if b.eventRecorder != nil {
		event := &events.BusPositionUpdated{
			EventIDValue:     uuid.New().String(),
			BusID:            b.busID,
			PreviousPosition: previousPosition,
			NewPosition:      newPosition,
			StopIndex:        stopIndex,
			SpeedKmh:         speedKmh,
			OccurredAtTime:   b.updatedAt,
		}
		_ = b.recordEvent(event)
	}

	return nil
}

// ChangeDirection changes the bus's direction.

func (b *Bus) ChangeDirection(newDirection enums.Direction) error {
	//check if direction is inbound or outbound
	if newDirection != enums.DirectionInbound && newDirection != enums.DirectionOutbound {
		return errors.ErrInvalidDirection
	}
	//check if direction is different from current direction
	if b.direction == newDirection {
		return nil
	}
	oldDirection := b.direction
	b.direction = newDirection
	b.updatedAt = time.Now()

	if b.eventRecorder != nil {
		// Create and record a BusDirectionChanged event (not implemented here)
		event := &events.BusDirectionChanged{
			EventIDValue:   uuid.New().String(),
			BusID:          b.busID,
			OldDirection:   oldDirection,
			NewDirection:   newDirection,
			OccurredAtTime: b.updatedAt,
		}
		_ = b.recordEvent(event)
	}
	return nil
}

// Arrive at terminal marks the bus as having arrived at the terminal.
func (b *Bus) ArriveAtTerminal(arrivalTime time.Time) error {
	if b.isAtTerminal {
		return errors.ErrAlreadyAtTerminal
	}
	if arrivalTime.IsZero() {
		return errors.ErrInvalidArrivalTime
	}
	b.isAtTerminal = true
	b.terminalArrivalTime = &arrivalTime
	b.updatedAt = arrivalTime

	if b.eventRecorder != nil {
		event := &events.BusArrivedAtTerminal{
			EventIDValue:   uuid.New().String(),
			BusID:          b.busID,
			ArrivalTime:    arrivalTime,
			OccurredAtTime: arrivalTime,
		}
		_ = b.recordEvent(event)
	}
	return nil
}

// DepartFromTerminal marks the bus as having departed from the terminal.
func (b *Bus) DepartFromTerminal(departureTime time.Time) error {
	if !b.isAtTerminal {
		return errors.ErrNotAtTerminal
	}
	if departureTime.IsZero() {
		return errors.ErrInvalidDepartureTime
	}
	b.isAtTerminal = false
	b.terminalArrivalTime = nil
	b.updatedAt = departureTime

	if b.eventRecorder != nil {
		event := &events.BusDepartedFromTerminal{
			EventIDValue:   uuid.New().String(),
			BusID:          b.busID,
			DepartureTime:  departureTime,
			OccurredAtTime: departureTime,
		}
		_ = b.recordEvent(event)
	}
	return nil
}

// Change Status changes the bus's operational status.
func (b *Bus) ChangeStatus(newStatus types.BusStatus, reason string) error {
	if newStatus == enums.BusStatusOffline && b.status == enums.BusStatusOffline {
		return errors.ErrAlreadyOffline
	}
	if b.status == newStatus {
		return nil
	}
	oldStatus := b.status
	b.status = newStatus
	b.updatedAt = time.Now()

	if b.eventRecorder != nil {
		event := &events.BusStatusChanged{
			EventIDValue:   uuid.New().String(),
			BusID:          b.busID,
			OldStatus:      oldStatus,
			NewStatus:      newStatus,
			Reason:         reason,
			OccurredAtTime: b.updatedAt,
		}
		_ = b.recordEvent(event)
	}
	return nil
}

// CalculateTerminalDwellTime calculates bus dwell time at terminal.
// Returns zero if bus has no terminal arrival time.
// Used to determine which bus is best to serve when at terminal.
func (b *Bus) CalculateTerminalDwellTime(departureTime time.Time) types.Duration {
	if b.terminalArrivalTime == nil {
		return types.Duration(0)
	}
	if departureTime.Before(*b.terminalArrivalTime) {
		return types.Duration(0)
	}
	return types.Duration(departureTime.Sub(*b.terminalArrivalTime))
}
