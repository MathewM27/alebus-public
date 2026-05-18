package streaming

import (
	"context"

	journeytrackingreadmodel "github.com/MathewM27/busTrack-alebus/application/journey/trackingreadmodel"
)

// JourneyUpdate is a lightweight event used for streaming journey tracking UI updates.
//
// Producers (e.g. background worker) compute the latest UI-safe journey snapshot
// and publish it (typically via Redis PubSub). Consumers (e.g. SSE handlers)
// fan-out updates to connected clients.
type JourneyUpdate struct {
	ServerTs string                                      `json:"serverTs"`
	Journey  journeytrackingreadmodel.JourneyTrackingDTO `json:"journey"`
}

// JourneyUpdatesSubscriber provides a shared (single-subscription) stream of updates.
//
// Implementations must:
// - be safe for concurrent Subscribe calls
// - close the returned channel when ctx is canceled
// - not block producers on slow consumers (disconnect or drop as needed)
type JourneyUpdatesSubscriber interface {
	// Subscribe returns a stream of updates for a single journeyId.
	// The returned channel is closed when ctx is canceled or the subscriber is disconnected.
	Subscribe(ctx context.Context, journeyID string) <-chan JourneyUpdate
}
