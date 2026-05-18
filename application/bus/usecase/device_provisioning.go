package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MathewM27/busTrack-alebus/application/bus/ports"
	"github.com/MathewM27/busTrack-alebus/domain/repositories"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrBusNotFound           = errors.New("bus not found")
	ErrActiveDeviceExists    = errors.New("active device already exists for bus")
	ErrInvalidBusID          = errors.New("invalid bus id")
	ErrInvalidProvisionInput = errors.New("invalid provisioning input")
)

type ProvisionBusDeviceRequest struct {
	BusID          types.BusID
	RotateIfExists bool
}

type ProvisionBusDeviceResult struct {
	DeviceID     string `json:"deviceId"`
	BusID        string `json:"busId"`
	MQTTUsername string `json:"mqttUsername"`
	MQTTPassword string `json:"mqttPassword"` // one-time secret

	Topic string `json:"topic"` // bus/{busId}/gps
}

// ProvisionBusDeviceUseCase provisions per-bus MQTT credentials for a physical tracker device.
//
// Responsibilities:
// - Validate bus exists
// - Ensure at most one active device per bus (unless RotateIfExists)
// - Generate username + random password
// - Hash password and store
// - Optionally configure broker ACL via port
//
// Non-responsibilities:
// - HTTP request parsing
// - Storing plaintext secrets beyond the response
// - EMQX implementation details (infrastructure)
type ProvisionBusDeviceUseCase struct {
	buses  repositories.BusRepository
	store  ports.BusDeviceStore
	broker ports.BrokerAuthorizer
}

func NewProvisionBusDeviceUseCase(
	buses repositories.BusRepository,
	store ports.BusDeviceStore,
	broker ports.BrokerAuthorizer,
) *ProvisionBusDeviceUseCase {
	if buses == nil {
		panic("ProvisionBusDeviceUseCase: buses repo cannot be nil")
	}
	if store == nil {
		panic("ProvisionBusDeviceUseCase: store cannot be nil")
	}
	return &ProvisionBusDeviceUseCase{buses: buses, store: store, broker: broker}
}

func (u *ProvisionBusDeviceUseCase) Provision(ctx context.Context, req ProvisionBusDeviceRequest) (ProvisionBusDeviceResult, error) {
	busID := types.BusID(strings.TrimSpace(string(req.BusID)))
	if busID == "" {
		return ProvisionBusDeviceResult{}, ErrInvalidBusID
	}

	// Confirm bus exists (domain aggregate lookup)
	if _, err := u.buses.FindByID(ctx, busID); err != nil {
		// Repo contract returns error for not-found as well; treat as not found.
		return ProvisionBusDeviceResult{}, ErrBusNotFound
	}

	// Enforce single active device per bus
	active, found, err := u.store.GetActiveByBusID(ctx, busID)
	if err != nil {
		return ProvisionBusDeviceResult{}, err
	}
	if found {
		if !req.RotateIfExists {
			return ProvisionBusDeviceResult{}, ErrActiveDeviceExists
		}
		// Rotate: revoke old device first (best-effort)
		now := time.Now().UTC()
		_ = u.store.Revoke(ctx, active.DeviceID, now)
		if u.broker != nil {
			_ = u.broker.RevokeDeviceACL(ctx, active.MQTTUsername)
		}
	}

	deviceID := uuid.NewString()
	username := fmt.Sprintf("bus:%s:%s", busID, deviceID[:8])
	password, err := generatePassword(32)
	if err != nil {
		return ProvisionBusDeviceResult{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return ProvisionBusDeviceResult{}, err
	}

	now := time.Now().UTC()
	rec := ports.BusDeviceRecord{
		DeviceID:         deviceID,
		BusID:            busID,
		MQTTUsername:     username,
		MQTTPasswordHash: string(hash),
		CreatedAt:        now,
	}
	if err := u.store.Create(ctx, rec); err != nil {
		return ProvisionBusDeviceResult{}, err
	}

	if u.broker != nil {
		if err := u.broker.EnsureDeviceACL(ctx, username, busID); err != nil {
			// If broker provisioning fails, revoke the record to avoid orphaned credentials.
			_ = u.store.Revoke(ctx, deviceID, time.Now().UTC())
			return ProvisionBusDeviceResult{}, err
		}
	}

	return ProvisionBusDeviceResult{
		DeviceID:     deviceID,
		BusID:        string(busID),
		MQTTUsername: username,
		MQTTPassword: password,
		Topic:        fmt.Sprintf("bus/%s/gps", busID),
	}, nil
}

func generatePassword(n int) (string, error) {
	if n < 16 {
		return "", ErrInvalidProvisionInput
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// URL-safe, no padding, easy to paste.
	return base64.RawURLEncoding.EncodeToString(b), nil
}
