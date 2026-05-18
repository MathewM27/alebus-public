package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MathewM27/busTrack-alebus/application/bus/ports"
	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/repositories"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/stretchr/testify/require"
)

type stubBusRepo struct{ exists bool }

func (s stubBusRepo) Save(context.Context, *aggregates.Bus) error { return nil }
func (s stubBusRepo) FindByID(context.Context, types.BusID) (*aggregates.Bus, error) {
	if !s.exists {
		return nil, errors.New("not found")
	}
	return &aggregates.Bus{}, nil
}

var _ repositories.BusRepository = stubBusRepo{}

type memDeviceStore struct {
	activeByBus map[string]ports.BusDeviceRecord
}

func newMemDeviceStore() *memDeviceStore {
	return &memDeviceStore{activeByBus: map[string]ports.BusDeviceRecord{}}
}

func (m *memDeviceStore) GetActiveByBusID(_ context.Context, busID types.BusID) (ports.BusDeviceRecord, bool, error) {
	rec, ok := m.activeByBus[string(busID)]
	return rec, ok, nil
}
func (m *memDeviceStore) GetActiveByUsername(_ context.Context, _ string) (ports.BusDeviceRecord, bool, error) {
	return ports.BusDeviceRecord{}, false, nil
}
func (m *memDeviceStore) Create(_ context.Context, rec ports.BusDeviceRecord) error {
	m.activeByBus[string(rec.BusID)] = rec
	return nil
}
func (m *memDeviceStore) Revoke(_ context.Context, deviceID string, revokedAt time.Time) error {
	for k, v := range m.activeByBus {
		if v.DeviceID == deviceID {
			v.RevokedAt = &revokedAt
			m.activeByBus[k] = v
			delete(m.activeByBus, k)
			break
		}
	}
	return nil
}
func (m *memDeviceStore) UpdateLastSeen(context.Context, string, time.Time) error { return nil }

var _ ports.BusDeviceStore = (*memDeviceStore)(nil)

type noopBroker struct{}

func (noopBroker) EnsureDeviceACL(context.Context, string, types.BusID) error { return nil }
func (noopBroker) RevokeDeviceACL(context.Context, string) error              { return nil }

func TestProvisionBusDeviceUseCase_Provision_Success(t *testing.T) {
	uc := NewProvisionBusDeviceUseCase(stubBusRepo{exists: true}, newMemDeviceStore(), noopBroker{})

	res, err := uc.Provision(context.Background(), ProvisionBusDeviceRequest{BusID: types.BusID("BUS-1")})
	require.NoError(t, err)
	require.NotEmpty(t, res.DeviceID)
	require.NotEmpty(t, res.MQTTUsername)
	require.NotEmpty(t, res.MQTTPassword)
	require.Equal(t, "bus/BUS-1/gps", res.Topic)
}

func TestProvisionBusDeviceUseCase_Provision_ActiveExists(t *testing.T) {
	store := newMemDeviceStore()
	store.activeByBus["BUS-1"] = ports.BusDeviceRecord{DeviceID: "dev-1", BusID: types.BusID("BUS-1"), MQTTUsername: "u", MQTTPasswordHash: "h"}
	uc := NewProvisionBusDeviceUseCase(stubBusRepo{exists: true}, store, noopBroker{})

	_, err := uc.Provision(context.Background(), ProvisionBusDeviceRequest{BusID: types.BusID("BUS-1")})
	require.ErrorIs(t, err, ErrActiveDeviceExists)
}
