package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/MathewM27/busTrack-alebus/application/bus/ports"
	"github.com/MathewM27/busTrack-alebus/domain/types"
)

var (
	ErrNoActiveDevice = errors.New("no active device for bus")
)

type RevokeBusDeviceRequest struct {
	BusID types.BusID
}

type RevokeBusDeviceResult struct {
	BusID    string `json:"busId"`
	DeviceID string `json:"deviceId"`
	Revoked  bool   `json:"revoked"`
}

type RevokeBusDeviceUseCase struct {
	store  ports.BusDeviceStore
	broker ports.BrokerAuthorizer
}

func NewRevokeBusDeviceUseCase(store ports.BusDeviceStore, broker ports.BrokerAuthorizer) *RevokeBusDeviceUseCase {
	if store == nil {
		panic("RevokeBusDeviceUseCase: store cannot be nil")
	}
	return &RevokeBusDeviceUseCase{store: store, broker: broker}
}

func (u *RevokeBusDeviceUseCase) Revoke(ctx context.Context, req RevokeBusDeviceRequest) (RevokeBusDeviceResult, error) {
	busID := types.BusID(strings.TrimSpace(string(req.BusID)))
	if busID == "" {
		return RevokeBusDeviceResult{}, ErrInvalidBusID
	}

	active, found, err := u.store.GetActiveByBusID(ctx, busID)
	if err != nil {
		return RevokeBusDeviceResult{}, err
	}
	if !found {
		return RevokeBusDeviceResult{}, ErrNoActiveDevice
	}

	now := time.Now().UTC()
	if err := u.store.Revoke(ctx, active.DeviceID, now); err != nil {
		return RevokeBusDeviceResult{}, err
	}
	if u.broker != nil {
		_ = u.broker.RevokeDeviceACL(ctx, active.MQTTUsername)
	}

	return RevokeBusDeviceResult{BusID: string(busID), DeviceID: active.DeviceID, Revoked: true}, nil
}
