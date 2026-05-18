package testhelpers

import "github.com/MathewM27/busTrack-alebus/domain/types"

type MockEventRecorder struct {
	Events []types.DomainEvent
}

func (m *MockEventRecorder) Record(event types.DomainEvent) error {
	m.Events = append(m.Events, event)
	return nil
}
