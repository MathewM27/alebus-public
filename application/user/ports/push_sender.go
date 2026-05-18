package userports

import (
	"context"
)

// PushMessage is an application DTO for sending a push notification.
// It is intentionally minimal and provider-agnostic.
type PushMessage struct {
	To    string
	Title string
	Body  string
	Data  map[string]any
}

// PushSendReport is provider-agnostic feedback from a send attempt.
//
// InvalidTokens are recipients that should be revoked/removed because the
// provider indicates they are no longer valid (e.g. app uninstalled).
//
// NOTE: This is best-effort feedback. Providers may return partial results.
type PushSendReport struct {
	InvalidTokens []string
}

// PushSender is an application port implemented by infrastructure (e.g. Expo).
// It should be best-effort and return an error only when the send attempt could not be made.
// Provider-specific delivery/receipt handling can be added later.
type PushSender interface {
	Send(ctx context.Context, messages []PushMessage) error
}

// PushSenderWithReport is an optional extension for providers that can return
// per-recipient feedback. Use cases can type-assert to this interface.
type PushSenderWithReport interface {
	SendWithReport(ctx context.Context, messages []PushMessage) (PushSendReport, error)
}
