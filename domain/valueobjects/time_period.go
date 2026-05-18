package valueobjects

import "time"

// TimePeriod represents a period of time with start and end
type TimePeriod struct {
	StartTime time.Time
	EndTime   time.Time
}

// IsActive checks if current time is within this period
func (tp TimePeriod) IsActive() bool {
	now := time.Now()
	return now.After(tp.StartTime) && now.Before(tp.EndTime)
}

// Duration returns the duration of this period
func (tp TimePeriod) Duration() time.Duration {
	return tp.EndTime.Sub(tp.StartTime)
}
