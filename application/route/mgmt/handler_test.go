package routemgmt

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock Repository ---

type MockRouteRepository struct {
	mock.Mock
}

func (m *MockRouteRepository) Save(ctx context.Context, route *aggregates.Route) error {
	args := m.Called(ctx, route)
	return args.Error(0)
}

func (m *MockRouteRepository) FindByID(ctx context.Context, id types.RouteID) (*aggregates.Route, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*aggregates.Route), args.Error(1)
}

func (m *MockRouteRepository) FindActiveRoutes(ctx context.Context, at time.Time) ([]*aggregates.Route, error) {
	args := m.Called(ctx, at)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*aggregates.Route), args.Error(1)
}

// --- Test Helpers ---

func validStops() []valueobjects.Stop {
	return []valueobjects.Stop{
		{ID: "stop-1", Name: "First Stop", Location: types.GeoLocation{Latitude: 10, Longitude: 20}},
		{ID: "stop-2", Name: "Second Stop", Location: types.GeoLocation{Latitude: 11, Longitude: 21}},
	}
}

func validCreateCommand() CreateRouteCommand {
	now := time.Now()
	return CreateRouteCommand{
		RouteID:       "route-1",
		Name:          "Test Route",
		OperatorIDs:   []types.OperatorID{"operator-1"},
		Stops:         validStops(),
		Direction:     enums.RouteDirectionBidirectional,
		RouteType:     enums.RouteTypeMixed,
		ActiveFrom:    now,
		ActiveUntil:   now.Add(365 * 24 * time.Hour),
		CreatedAt:     now,
		EventRecorder: nil,
	}
}

// --- CreateRoute Tests ---

func TestCreateRoute_Success(t *testing.T) {
	mockRepo := new(MockRouteRepository)
	handler := NewRouteCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateCommand()

	// Expect Save to be called with any route (aggregate is created internally)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*aggregates.Route")).Return(nil)

	err := handler.CreateRoute(ctx, cmd)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestCreateRoute_DomainValidationFailure_EmptyName(t *testing.T) {
	mockRepo := new(MockRouteRepository)
	handler := NewRouteCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateCommand()
	cmd.Name = "" // Invalid: empty name

	err := handler.CreateRoute(ctx, cmd)

	assert.Error(t, err)
	// Save should NOT be called when domain validation fails
	mockRepo.AssertNotCalled(t, "Save")
}

func TestCreateRoute_DomainValidationFailure_InsufficientStops(t *testing.T) {
	mockRepo := new(MockRouteRepository)
	handler := NewRouteCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateCommand()
	cmd.Stops = []valueobjects.Stop{{ID: "stop-1", Name: "Only Stop"}} // Invalid: < 2 stops

	err := handler.CreateRoute(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestCreateRoute_RepositorySaveFailure(t *testing.T) {
	mockRepo := new(MockRouteRepository)
	handler := NewRouteCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateCommand()

	mockRepo.On("Save", ctx, mock.AnythingOfType("*aggregates.Route")).
		Return(errors.New("database error"))

	err := handler.CreateRoute(ctx, cmd)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
	mockRepo.AssertExpectations(t)
}

// --- UpdateRouteStops Tests ---

func TestUpdateRouteStops_Success(t *testing.T) {
	mockRepo := new(MockRouteRepository)
	handler := NewRouteCommandHandler(mockRepo)
	ctx := context.Background()

	// Create a route to be returned by FindByID
	now := time.Now()
	existingRoute, _ := aggregates.NewRoute(
		"route-1",
		[]types.OperatorID{"operator-1"},
		validStops(),
		"Test Route",
		enums.RouteDirectionBidirectional,
		enums.RouteTypeMixed,
		now,
		now.Add(365*24*time.Hour),
		now,
		nil,
	)

	newStops := []valueobjects.Stop{
		{ID: "stop-a", Name: "New First Stop", Location: types.GeoLocation{Latitude: 12, Longitude: 22}},
		{ID: "stop-b", Name: "New Second Stop", Location: types.GeoLocation{Latitude: 13, Longitude: 23}},
	}

	cmd := UpdateRouteStopsCommand{
		RouteID: "route-1",
		Stops:   newStops,
		Reason:  "Route optimization",
	}

	mockRepo.On("FindByID", ctx, types.RouteID("route-1")).Return(existingRoute, nil)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*aggregates.Route")).Return(nil)

	err := handler.UpdateRouteStops(ctx, cmd)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestUpdateRouteStops_RouteNotFound(t *testing.T) {
	mockRepo := new(MockRouteRepository)
	handler := NewRouteCommandHandler(mockRepo)
	ctx := context.Background()

	cmd := UpdateRouteStopsCommand{
		RouteID: "non-existent",
		Stops:   validStops(),
		Reason:  "Test",
	}

	mockRepo.On("FindByID", ctx, types.RouteID("non-existent")).
		Return(nil, errors.New("route not found"))

	err := handler.UpdateRouteStops(ctx, cmd)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "route not found")
	mockRepo.AssertNotCalled(t, "Save")
}

func TestUpdateRouteStops_DomainValidationFailure(t *testing.T) {
	mockRepo := new(MockRouteRepository)
	handler := NewRouteCommandHandler(mockRepo)
	ctx := context.Background()

	now := time.Now()
	existingRoute, _ := aggregates.NewRoute(
		"route-1",
		[]types.OperatorID{"operator-1"},
		validStops(),
		"Test Route",
		enums.RouteDirectionBidirectional,
		enums.RouteTypeMixed,
		now,
		now.Add(365*24*time.Hour),
		now,
		nil,
	)

	// Invalid: only 1 stop (domain requires >= 2)
	cmd := UpdateRouteStopsCommand{
		RouteID: "route-1",
		Stops:   []valueobjects.Stop{{ID: "stop-1", Name: "Only Stop"}},
		Reason:  "Test",
	}

	mockRepo.On("FindByID", ctx, types.RouteID("route-1")).Return(existingRoute, nil)

	err := handler.UpdateRouteStops(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

// --- ChangeName Tests ---

func TestChangeName_Success(t *testing.T) {
	mockRepo := new(MockRouteRepository)
	handler := NewRouteCommandHandler(mockRepo)
	ctx := context.Background()

	now := time.Now()
	existingRoute, _ := aggregates.NewRoute(
		"route-1",
		[]types.OperatorID{"operator-1"},
		validStops(),
		"Old Name",
		enums.RouteDirectionBidirectional,
		enums.RouteTypeMixed,
		now,
		now.Add(365*24*time.Hour),
		now,
		nil,
	)

	cmd := ChangeRouteNameCommand{
		RouteID: "route-1",
		Name:    "New Name",
		Reason:  "Rebranding",
	}

	mockRepo.On("FindByID", ctx, types.RouteID("route-1")).Return(existingRoute, nil)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*aggregates.Route")).Return(nil)

	err := handler.ChangeRouteName(ctx, cmd)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestChangeName_EmptyName_Fails(t *testing.T) {
	mockRepo := new(MockRouteRepository)
	handler := NewRouteCommandHandler(mockRepo)
	ctx := context.Background()

	now := time.Now()
	existingRoute, _ := aggregates.NewRoute(
		"route-1",
		[]types.OperatorID{"operator-1"},
		validStops(),
		"Old Name",
		enums.RouteDirectionBidirectional,
		enums.RouteTypeMixed,
		now,
		now.Add(365*24*time.Hour),
		now,
		nil,
	)

	cmd := ChangeRouteNameCommand{
		RouteID: "route-1",
		Name:    "", // Invalid: empty
		Reason:  "Test",
	}

	mockRepo.On("FindByID", ctx, types.RouteID("route-1")).Return(existingRoute, nil)

	err := handler.ChangeRouteName(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

// --- ChangeRouteType Tests ---

func TestChangeRouteType_Success(t *testing.T) {
	mockRepo := new(MockRouteRepository)
	handler := NewRouteCommandHandler(mockRepo)
	ctx := context.Background()

	now := time.Now()
	existingRoute, _ := aggregates.NewRoute(
		"route-1",
		[]types.OperatorID{"operator-1"},
		validStops(),
		"Test Route",
		enums.RouteDirectionBidirectional,
		enums.RouteTypeMixed,
		now,
		now.Add(365*24*time.Hour),
		now,
		nil,
	)

	cmd := ChangeRouteTypeCommand{
		RouteID:   "route-1",
		RouteType: enums.RouteTypeUrban,
		Reason:    "Route now operates in urban area",
	}

	mockRepo.On("FindByID", ctx, types.RouteID("route-1")).Return(existingRoute, nil)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*aggregates.Route")).Return(nil)

	err := handler.ChangeRouteType(ctx, cmd)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// --- ChangeActivePeriod Tests ---

func TestChangeActivePeriod_Success(t *testing.T) {
	mockRepo := new(MockRouteRepository)
	handler := NewRouteCommandHandler(mockRepo)
	ctx := context.Background()

	now := time.Now()
	existingRoute, _ := aggregates.NewRoute(
		"route-1",
		[]types.OperatorID{"operator-1"},
		validStops(),
		"Test Route",
		enums.RouteDirectionBidirectional,
		enums.RouteTypeMixed,
		now,
		now.Add(365*24*time.Hour),
		now,
		nil,
	)

	newFrom := now.Add(30 * 24 * time.Hour)
	newUntil := now.Add(730 * 24 * time.Hour)

	cmd := ChangeActivePeriodCommand{
		RouteID:     "route-1",
		ActiveFrom:  newFrom,
		ActiveUntil: newUntil,
		Reason:      "Extended operation period",
	}

	mockRepo.On("FindByID", ctx, types.RouteID("route-1")).Return(existingRoute, nil)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*aggregates.Route")).Return(nil)

	err := handler.ChangeActivePeriod(ctx, cmd)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestChangeActivePeriod_InvalidPeriod_Fails(t *testing.T) {
	mockRepo := new(MockRouteRepository)
	handler := NewRouteCommandHandler(mockRepo)
	ctx := context.Background()

	now := time.Now()
	existingRoute, _ := aggregates.NewRoute(
		"route-1",
		[]types.OperatorID{"operator-1"},
		validStops(),
		"Test Route",
		enums.RouteDirectionBidirectional,
		enums.RouteTypeMixed,
		now,
		now.Add(365*24*time.Hour),
		now,
		nil,
	)

	// Invalid: activeUntil is before activeFrom
	cmd := ChangeActivePeriodCommand{
		RouteID:     "route-1",
		ActiveFrom:  now.Add(100 * 24 * time.Hour),
		ActiveUntil: now.Add(50 * 24 * time.Hour), // Before activeFrom!
		Reason:      "Test",
	}

	mockRepo.On("FindByID", ctx, types.RouteID("route-1")).Return(existingRoute, nil)

	err := handler.ChangeActivePeriod(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}
