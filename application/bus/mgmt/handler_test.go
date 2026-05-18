package busmgmt

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/testhelpers"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock Repository ---

type MockBusRepository struct {
	mock.Mock
}

func (m *MockBusRepository) Save(ctx context.Context, bus *aggregates.Bus) error {
	args := m.Called(ctx, bus)
	return args.Error(0)
}

func (m *MockBusRepository) FindByID(ctx context.Context, id types.BusID) (*aggregates.Bus, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*aggregates.Bus), args.Error(1)
}

// --- Test Helpers ---

func validPosition() types.PositionSnapshot {
	return types.PositionSnapshot{
		Location: types.GeoLocation{
			Latitude:  10.0,
			Longitude: 20.0,
		},
		Timestamp: time.Now(),
		Accuracy:  5.0,
		SpeedKmh:  30.0,
	}
}

func validCreateBusCommand() CreateBusCommand {
	now := time.Now()
	return CreateBusCommand{
		BusID:            "bus-1",
		OperatorID:       "operator-1",
		RouteID:          "route-1",
		InitalPosition:   validPosition(),
		InitialStopIndex: 0,
		Direction:        enums.DirectionOutbound,
		CreatedAt:        now,
		EventRecorder:    &testhelpers.MockEventRecorder{},
	}
}

func createValidBus() *aggregates.Bus {
	now := time.Now()
	bus, _ := aggregates.NewBus(
		"bus-1",
		"operator-1",
		"route-1",
		validPosition(),
		0, // initialStopIndex
		enums.DirectionOutbound,
		now,
		&testhelpers.MockEventRecorder{},
	)
	return bus
}

// --- CreateBus Tests ---

func TestCreateBus_Success(t *testing.T) {
	mockRepo := new(MockBusRepository)
	handler := NewBusCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateBusCommand()

	// Expect Save to be called with any bus (aggregate is created internally)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*aggregates.Bus")).Return(nil)

	err := handler.CreateBus(ctx, cmd)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestCreateBus_DomainValidationFailure_EmptyBusID(t *testing.T) {
	mockRepo := new(MockBusRepository)
	handler := NewBusCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateBusCommand()
	cmd.BusID = "" // Invalid: empty bus ID

	err := handler.CreateBus(ctx, cmd)

	assert.Error(t, err)
	// Save should NOT be called when domain validation fails
	mockRepo.AssertNotCalled(t, "Save")
}

func TestCreateBus_DomainValidationFailure_EmptyOperatorID(t *testing.T) {
	mockRepo := new(MockBusRepository)
	handler := NewBusCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateBusCommand()
	cmd.OperatorID = "" // Invalid: empty operator ID

	err := handler.CreateBus(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestCreateBus_DomainValidationFailure_EmptyRouteID(t *testing.T) {
	mockRepo := new(MockBusRepository)
	handler := NewBusCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateBusCommand()
	cmd.RouteID = "" // Invalid: empty route ID

	err := handler.CreateBus(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestCreateBus_DomainValidationFailure_InvalidDirection(t *testing.T) {
	mockRepo := new(MockBusRepository)
	handler := NewBusCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateBusCommand()
	cmd.Direction = 99 // Invalid direction

	err := handler.CreateBus(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestCreateBus_DomainValidationFailure_NilEventRecorder(t *testing.T) {
	mockRepo := new(MockBusRepository)
	handler := NewBusCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateBusCommand()
	cmd.EventRecorder = nil // Invalid: nil event recorder

	err := handler.CreateBus(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestCreateBus_RepositorySaveFailure(t *testing.T) {
	mockRepo := new(MockBusRepository)
	handler := NewBusCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateBusCommand()

	mockRepo.On("Save", ctx, mock.AnythingOfType("*aggregates.Bus")).
		Return(errors.New("database error"))

	err := handler.CreateBus(ctx, cmd)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
	mockRepo.AssertExpectations(t)
}

// --- UpdateBusPosition Tests ---

func TestUpdateBusPosition_Success(t *testing.T) {
	mockRepo := new(MockBusRepository)
	handler := NewBusCommandHandler(mockRepo)
	ctx := context.Background()

	existingBus := createValidBus()

	cmd := UpdateBusPositionCommand{
		BusID: "bus-1",
		NewPosition: types.PositionSnapshot{
			Location: types.GeoLocation{
				Latitude:  11.0,
				Longitude: 21.0,
			},
			Timestamp: time.Now(),
			Accuracy:  3.0,
			SpeedKmh:  40.0,
		},
		StopIndex: 1,
		SpeedKmh:  40.0,
	}

	mockRepo.On("FindByID", ctx, types.BusID("bus-1")).Return(existingBus, nil)
	mockRepo.On("Save", ctx, existingBus).Return(nil)

	err := handler.UpdateBusPosition(ctx, cmd)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestUpdateBusPosition_BusNotFound(t *testing.T) {
	mockRepo := new(MockBusRepository)
	handler := NewBusCommandHandler(mockRepo)
	ctx := context.Background()

	cmd := UpdateBusPositionCommand{
		BusID:       "bus-nonexistent",
		NewPosition: validPosition(),
		StopIndex:   1,
		SpeedKmh:    40.0,
	}

	mockRepo.On("FindByID", ctx, types.BusID("bus-nonexistent")).
		Return(nil, errors.New("bus not found"))

	err := handler.UpdateBusPosition(ctx, cmd)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bus not found")
	mockRepo.AssertNotCalled(t, "Save")
}

// --- ChangeBusDirection Tests ---

func TestChangeBusDirection_Success(t *testing.T) {
	mockRepo := new(MockBusRepository)
	handler := NewBusCommandHandler(mockRepo)
	ctx := context.Background()

	existingBus := createValidBus()

	cmd := ChangeBusDirectionCommand{
		BusID:        "bus-1",
		NewDirection: enums.DirectionInbound,
	}

	mockRepo.On("FindByID", ctx, types.BusID("bus-1")).Return(existingBus, nil)
	mockRepo.On("Save", ctx, existingBus).Return(nil)

	err := handler.ChangeBusDirection(ctx, cmd)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestChangeBusDirection_BusNotFound(t *testing.T) {
	mockRepo := new(MockBusRepository)
	handler := NewBusCommandHandler(mockRepo)
	ctx := context.Background()

	cmd := ChangeBusDirectionCommand{
		BusID:        "bus-nonexistent",
		NewDirection: enums.DirectionInbound,
	}

	mockRepo.On("FindByID", ctx, types.BusID("bus-nonexistent")).
		Return(nil, errors.New("bus not found"))

	err := handler.ChangeBusDirection(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

// --- ArriveAtTerminal Tests ---

func TestArriveAtTerminal_Success(t *testing.T) {
	mockRepo := new(MockBusRepository)
	handler := NewBusCommandHandler(mockRepo)
	ctx := context.Background()

	existingBus := createValidBus()

	cmd := ArriveAtTerminalCommand{
		BusID:       "bus-1",
		ArrivalTime: time.Now(),
	}

	mockRepo.On("FindByID", ctx, types.BusID("bus-1")).Return(existingBus, nil)
	mockRepo.On("Save", ctx, existingBus).Return(nil)

	err := handler.ArriveAtTerminal(ctx, cmd)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestArriveAtTerminal_BusNotFound(t *testing.T) {
	mockRepo := new(MockBusRepository)
	handler := NewBusCommandHandler(mockRepo)
	ctx := context.Background()

	cmd := ArriveAtTerminalCommand{
		BusID:       "bus-nonexistent",
		ArrivalTime: time.Now(),
	}

	mockRepo.On("FindByID", ctx, types.BusID("bus-nonexistent")).
		Return(nil, errors.New("bus not found"))

	err := handler.ArriveAtTerminal(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

// --- DepartFromTerminal Tests ---

func TestDepartFromTerminal_Success(t *testing.T) {
	mockRepo := new(MockBusRepository)
	handler := NewBusCommandHandler(mockRepo)
	ctx := context.Background()

	existingBus := createValidBus()
	// First arrive at terminal to set up state
	_ = existingBus.ArriveAtTerminal(time.Now())

	cmd := DepartFromTerminalCommand{
		BusID:         "bus-1",
		DepartureTime: time.Now().Add(5 * time.Minute), // departure time
	}

	mockRepo.On("FindByID", ctx, types.BusID("bus-1")).Return(existingBus, nil)
	mockRepo.On("Save", ctx, existingBus).Return(nil)

	err := handler.DepartFromTerminal(ctx, cmd)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestDepartFromTerminal_BusNotFound(t *testing.T) {
	mockRepo := new(MockBusRepository)
	handler := NewBusCommandHandler(mockRepo)
	ctx := context.Background()

	cmd := DepartFromTerminalCommand{
		BusID:         "bus-nonexistent",
		DepartureTime: time.Now(),
	}

	mockRepo.On("FindByID", ctx, types.BusID("bus-nonexistent")).
		Return(nil, errors.New("bus not found"))

	err := handler.DepartFromTerminal(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

// --- ChangeBusStatus Tests ---

func TestChangeBusStatus_Success(t *testing.T) {
	mockRepo := new(MockBusRepository)
	handler := NewBusCommandHandler(mockRepo)
	ctx := context.Background()

	existingBus := createValidBus()

	cmd := ChangeBusStatusCommand{
		BusID:  "bus-1",
		Status: enums.BusStatusMaintenance,
		Reason: "Scheduled maintenance",
	}

	mockRepo.On("FindByID", ctx, types.BusID("bus-1")).Return(existingBus, nil)
	mockRepo.On("Save", ctx, existingBus).Return(nil)

	err := handler.ChangeBusStatus(ctx, cmd)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestChangeBusStatus_BusNotFound(t *testing.T) {
	mockRepo := new(MockBusRepository)
	handler := NewBusCommandHandler(mockRepo)
	ctx := context.Background()

	cmd := ChangeBusStatusCommand{
		BusID:  "bus-nonexistent",
		Status: enums.BusStatusOffline,
		Reason: "Network issue",
	}

	mockRepo.On("FindByID", ctx, types.BusID("bus-nonexistent")).
		Return(nil, errors.New("bus not found"))

	err := handler.ChangeBusStatus(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestChangeBusStatus_RepositorySaveFailure(t *testing.T) {
	mockRepo := new(MockBusRepository)
	handler := NewBusCommandHandler(mockRepo)
	ctx := context.Background()

	existingBus := createValidBus()

	cmd := ChangeBusStatusCommand{
		BusID:  "bus-1",
		Status: enums.BusStatusOffline,
		Reason: "Going offline",
	}

	mockRepo.On("FindByID", ctx, types.BusID("bus-1")).Return(existingBus, nil)
	mockRepo.On("Save", ctx, existingBus).Return(errors.New("database error"))

	err := handler.ChangeBusStatus(ctx, cmd)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
	mockRepo.AssertExpectations(t)
}
