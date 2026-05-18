package usecase

import (
	"context"
	"strings"

	"github.com/MathewM27/busTrack-alebus/application/bus/streaming"
)

type LiveBusStreamFilter struct {
	RouteID string
	BusIDs  map[string]struct{}
}

func NewLiveBusStreamFilter(routeID string, busIDs []string) LiveBusStreamFilter {
	filter := LiveBusStreamFilter{
		RouteID: strings.TrimSpace(routeID),
	}
	if len(busIDs) > 0 {
		filter.BusIDs = make(map[string]struct{}, len(busIDs))
		for _, id := range busIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			filter.BusIDs[id] = struct{}{}
		}
	}
	return filter
}

func (f LiveBusStreamFilter) Matches(update streaming.BusUpdate) bool {
	if f.RouteID != "" && update.Bus.RouteID != f.RouteID {
		return false
	}
	if len(f.BusIDs) > 0 {
		_, ok := f.BusIDs[update.Bus.BusID]
		return ok
	}
	return true
}

// StreamLiveBusUpdatesUseCase is an application-level streaming query.
// It enforces subscription routing semantics (filters) without causing extra Redis reads.
//
// Infrastructure owns:
// - Redis PubSub
// - coalescing-before-read
// - snapshot fetch once
//
// Application owns:
// - who should receive which updates (routeId/busIds filters)
type StreamLiveBusUpdatesUseCase struct {
	source    streaming.UpdatesSource
	outBuffer int
}

func NewStreamLiveBusUpdatesUseCase(source streaming.UpdatesSource) *StreamLiveBusUpdatesUseCase {
	if source == nil {
		panic("StreamLiveBusUpdatesUseCase: source cannot be nil")
	}
	return &StreamLiveBusUpdatesUseCase{source: source, outBuffer: 32}
}

func (u *StreamLiveBusUpdatesUseCase) WithOutBuffer(n int) *StreamLiveBusUpdatesUseCase {
	if n <= 0 {
		n = 1
	}
	u.outBuffer = n
	return u
}

// Stream returns a filtered stream of BusUpdate.
// The returned channel is closed when ctx is canceled or the upstream source closes.
func (u *StreamLiveBusUpdatesUseCase) Stream(ctx context.Context, filter LiveBusStreamFilter) <-chan streaming.BusUpdate {
	if ctx == nil {
		ctx = context.Background()
	}

	out := make(chan streaming.BusUpdate, u.outBuffer)
	in := u.source.Subscribe(ctx)

	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case upd, ok := <-in:
				if !ok {
					return
				}
				if !filter.Matches(upd) {
					continue
				}

				select {
				case <-ctx.Done():
					return
				case out <- upd:
				}
			}
		}
	}()

	return out
}
