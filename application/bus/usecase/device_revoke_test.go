package usecase

import (
	"context"
	"testing"

	"github.com/MathewM27/busTrack-alebus/application/bus/ports"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/stretchr/testify/require"
)

func TestRevokeBusDeviceUseCase_Revoke_NoActive(t *testing.T) {
	uc := NewRevokeBusDeviceUseCase(newMemDeviceStore(), noopBroker{})
	_, err := uc.Revoke(context.Background(), RevokeBusDeviceRequest{BusID: types.BusID("BUS-1")})
	require.ErrorIs(t, err, ErrNoActiveDevice)
}

func TestRevokeBusDeviceUseCase_Revoke_Success(t *testing.T) {
	store := newMemDeviceStore()
	store.activeByBus["BUS-1"] = ports.BusDeviceRecord{DeviceID: "dev-1", BusID: types.BusID("BUS-1"), MQTTUsername: "u", MQTTPasswordHash: "h"}
	uc := NewRevokeBusDeviceUseCase(store, noopBroker{})
	res, err := uc.Revoke(context.Background(), RevokeBusDeviceRequest{BusID: types.BusID("BUS-1")})
	require.NoError(t, err)
	require.True(t, res.Revoked)
	require.Equal(t, "dev-1", res.DeviceID)
}
