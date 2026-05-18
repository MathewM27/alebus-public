package ports

import (
	"context"
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/types"
)

// BusDeviceRecord represents a provisioned device identity for a bus.
// This is application-layer data; secrets are never returned except at provisioning time.
type BusDeviceRecord struct {
	DeviceID         string
	BusID            types.BusID
	MQTTUsername     string
	MQTTPasswordHash string
	CreatedAt        time.Time
	RevokedAt        *time.Time
	LastSeenAt       *time.Time
}

// BusDeviceStore persists provisioned devices.
// Implementations live in infrastructure (e.g. Postgres).
type BusDeviceStore interface {
	GetActiveByBusID(ctx context.Context, busID types.BusID) (BusDeviceRecord, bool, error)
	GetActiveByUsername(ctx context.Context, username string) (BusDeviceRecord, bool, error)
	Create(ctx context.Context, rec BusDeviceRecord) error
	Revoke(ctx context.Context, deviceID string, revokedAt time.Time) error
	UpdateLastSeen(ctx context.Context, deviceID string, seenAt time.Time) error
}

// BrokerAuthorizer allows the application to enforce broker-side topic permissions.
// In dev this can be a no-op; in production it should configure EMQX authn/authz.
//
// Note: We keep this as a port to avoid importing EMQX SDKs into application code.
type BrokerAuthorizer interface {
	EnsureDeviceACL(ctx context.Context, username string, busID types.BusID) error
	RevokeDeviceACL(ctx context.Context, username string) error
}
