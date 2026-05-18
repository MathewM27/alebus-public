package streaming

import (
	"context"
	"time"

	busreadmodel "github.com/MathewM27/busTrack-alebus/application/bus/readmodel"
)

// BusUpdate is the canonical application DTO emitted by the live-bus update stream.
// Infrastructure is responsible for producing it (Redis PubSub + Redis snapshot read).
// Presentation (SSE) may wrap it with additional framing fields like seq.
//
// This is NOT a domain event.
type BusUpdate struct {
	Bus      busreadmodel.BusDTO
	ServerTs time.Time
}

// UpdatesSource is an application port: a read-only stream of BusUpdate.
//
// Contract:
// - Subscribe MUST return immediately.
// - The returned channel MUST be closed when ctx is canceled or the source stops.
// - Ordering is best-effort; consumers must tolerate duplicates and gaps.
type UpdatesSource interface {
	Subscribe(ctx context.Context) <-chan BusUpdate
}
