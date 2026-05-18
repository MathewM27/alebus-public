package types

import (
	"fmt"
	"time"
)

type StopID string

type Duration time.Duration

func (dur Duration) Minutes() float64 {
	return time.Duration(dur).Minutes()
}
func (dur Duration) String() string {
	minutes := int(dur.Minutes())
	if minutes < 60 {
		return fmt.Sprintf("%d min", minutes)
	}
	hours := minutes / 60
	remainingMinutes := minutes % 60
	if remainingMinutes == 0 {
		return fmt.Sprintf("%d hr", hours)
	}
	return fmt.Sprintf("%d hr %d min", hours, remainingMinutes)
}

type DomainEvent interface {
	EventID() string
	EventType() string
	OccurredAt() time.Time
}

// EventRecorder interface for event sourcing
type EventRecorder interface {
	Record(event DomainEvent) error
}
