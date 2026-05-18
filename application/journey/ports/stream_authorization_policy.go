package ports

import "context"

// StreamAuthorizationPolicy decides whether a caller may subscribe to journey tracking streams.
//
// NOTE: This is a command-side authorization check for streaming endpoints.
// It must be side-effect free.
type StreamAuthorizationPolicy interface {
	CanSubscribeToJourney(ctx context.Context, userID string, journeyID string) (bool, error)
}
