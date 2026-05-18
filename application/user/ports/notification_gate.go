package userports

import (
	"context"
	"time"
)

// NotificationGate is an application port used to prevent duplicate/spammy notifications
// across worker ticks and restarts.
//
// Implementations should be atomic under concurrency.
//
// AllowMonotonicMax returns allow=true only when nextValue is greater than the
// previously stored value for the same key. The stored value should expire after ttl.
//
// This is used for monotonic journey proximity upgrades:
// none -> approaching -> nearby -> arrived.
//
// NOTE: This is not a durable event log; it's a lightweight gate.
type NotificationGate interface {
	AllowMonotonicMax(ctx context.Context, key string, nextValue int, ttl time.Duration) (allow bool, err error)
}
