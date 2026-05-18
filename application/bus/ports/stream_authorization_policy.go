package ports

import "context"

// StreamAuthorizationPolicy decides whether a caller may subscribe to bus tracking streams.
//
// NOTE: This is a command-side authorization check for streaming endpoints.
// It must be side-effect free.
type StreamAuthorizationPolicy interface {
	CanSubscribeToRoute(ctx context.Context, operatorID string, routeID string) (bool, error)
	CanSubscribeToBus(ctx context.Context, operatorID string, busID string) (bool, error)
}
