package journeymgmt

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/testhelpers"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ============================================================================
// Mock BusLocationFinder
// ============================================================================

type mockBusLocationFinder struct {
	mock.Mock
}

func (m *mockBusLocationFinder) FindBusesNearLocation(
	ctx context.Context,
	location types.GeoLocation,
	radiusKm float64,
	busIDs []types.BusID,
) ([]types.BusLocationInfo, error) {
	args := m.Called(ctx, location, radiusKm, busIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.BusLocationInfo), args.Error(1)
}

// ============================================================================
// Test Helpers (specific to SwitchBusByLocation)
// ============================================================================

func createSwitchableJourney() *aggregates.Journey {
	journey, _ := aggregates.NewJourney(
		"journey-switch",
		"user-1",
		types.GeoLocation{Latitude: 1.3521, Longitude: 103.8198},
		"stop-origin",
		"stop-dest",
		time.Now(),
		&testhelpers.MockEventRecorder{},
	)

	// Initialize tracking with multiple bus recommendations
	_ = journey.InitializeTracking(
		[]valueobjects.EnhancedBusRecommendation{
			{
				BusID:                "bus-1",
				OperatorID:           "operator-1",
				EstimatedArrival:     types.Duration(5 * time.Minute),
				Direction:            0,
				RequiredDirection:    0,
				IsWrongDirection:     false,
				ConfidenceLevel:      0.9,
				DisplayText:          "Bus 1 arriving in 5 min",
				DistanceToOriginStop: 500,
			},
			{
				BusID:                "bus-2",
				OperatorID:           "operator-1",
				EstimatedArrival:     types.Duration(10 * time.Minute),
				Direction:            0,
				RequiredDirection:    0,
				IsWrongDirection:     false,
				ConfidenceLevel:      0.8,
				DisplayText:          "Bus 2 arriving in 10 min",
				DistanceToOriginStop: 1000,
			},
			{
				BusID:                "bus-3",
				OperatorID:           "operator-2",
				EstimatedArrival:     types.Duration(15 * time.Minute),
				Direction:            0,
				RequiredDirection:    0,
				IsWrongDirection:     false,
				ConfidenceLevel:      0.7,
				DisplayText:          "Bus 3 arriving in 15 min",
				DistanceToOriginStop: 1500,
			},
		},
		types.Duration(30*time.Minute),
		0,
	)

	return journey
}

// ============================================================================
// SwitchBusByLocation Handler Tests
// ============================================================================

func TestSwitchBusByLocation_NilFinder_ReturnsError(t *testing.T) {
	// CRITICAL: When Redis is disabled, handler should fail explicitly
	// This test verifies the "explicit Redis requirement" contract
	mockRepo := new(MockJourneyRepository)
	handler := NewSwitchBusByLocationHandler(mockRepo, nil) // nil finder = no Redis

	cmd := SwitchBusByLocationCommand{
		JourneyID:    "journey-1",
		UserLocation: types.GeoLocation{Latitude: 1.35, Longitude: 103.82},
		Reason:       enums.JourneySwitchReasonLocation,
	}

	_, err := handler.Handle(context.Background(), cmd)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis")
	mockRepo.AssertNotCalled(t, "FindByID")
}

func TestSwitchBusByLocation_Success_SwitchesToNearestBus(t *testing.T) {
	// When a different bus is closer, switch to it
	mockRepo := new(MockJourneyRepository)
	mockFinder := new(mockBusLocationFinder)
	handler := NewSwitchBusByLocationHandler(mockRepo, mockFinder)
	ctx := context.Background()

	journey := createSwitchableJourney()
	assert.Equal(t, types.BusID("bus-1"), journey.ActiveBusID()) // Initial active bus

	mockRepo.On("FindByID", ctx, types.JourneyID("journey-switch")).Return(journey, nil)
	mockRepo.On("Save", ctx, journey).Return(nil)

	// Redis returns bus-2 as the nearest
	userLocation := types.GeoLocation{Latitude: 1.355, Longitude: 103.825}
	mockFinder.On("FindBusesNearLocation", ctx, userLocation, 2.0, mock.Anything).Return(
		[]types.BusLocationInfo{
			{BusID: "bus-2", Location: types.GeoLocation{Latitude: 1.356, Longitude: 103.826}},
			{BusID: "bus-1", Location: types.GeoLocation{Latitude: 1.360, Longitude: 103.830}},
		},
		nil,
	)

	cmd := SwitchBusByLocationCommand{
		JourneyID:    "journey-switch",
		UserLocation: userLocation,
		Reason:       enums.JourneySwitchReasonLocation,
	}

	response, err := handler.Handle(ctx, cmd)

	assert.NoError(t, err)
	assert.True(t, response.WasSwitched)
	assert.Equal(t, "bus-1", response.OldBusID)
	assert.Equal(t, "bus-2", response.NewBusID)
	mockRepo.AssertExpectations(t)
	mockFinder.AssertExpectations(t)
}

func TestSwitchBusByLocation_SameBus_NoSwitch(t *testing.T) {
	// When the current bus is nearest, don't switch
	mockRepo := new(MockJourneyRepository)
	mockFinder := new(mockBusLocationFinder)
	handler := NewSwitchBusByLocationHandler(mockRepo, mockFinder)
	ctx := context.Background()

	journey := createSwitchableJourney()
	assert.Equal(t, types.BusID("bus-1"), journey.ActiveBusID())

	mockRepo.On("FindByID", ctx, types.JourneyID("journey-switch")).Return(journey, nil)

	// Redis returns bus-1 as the nearest (same as active)
	userLocation := types.GeoLocation{Latitude: 1.352, Longitude: 103.820}
	mockFinder.On("FindBusesNearLocation", ctx, userLocation, 2.0, mock.Anything).Return(
		[]types.BusLocationInfo{
			{BusID: "bus-1", Location: types.GeoLocation{Latitude: 1.353, Longitude: 103.821}},
		},
		nil,
	)

	cmd := SwitchBusByLocationCommand{
		JourneyID:    "journey-switch",
		UserLocation: userLocation,
		Reason:       enums.JourneySwitchReasonLocation,
	}

	response, err := handler.Handle(ctx, cmd)

	assert.NoError(t, err)
	assert.False(t, response.WasSwitched)
	assert.Equal(t, "bus-1", response.OldBusID)
	assert.Equal(t, "bus-1", response.NewBusID)
	mockRepo.AssertNotCalled(t, "Save") // No save needed if no change
}

func TestSwitchBusByLocation_NoNearbyBuses_NoSwitch(t *testing.T) {
	// When Redis returns empty results, keep current bus
	mockRepo := new(MockJourneyRepository)
	mockFinder := new(mockBusLocationFinder)
	handler := NewSwitchBusByLocationHandler(mockRepo, mockFinder)
	ctx := context.Background()

	journey := createSwitchableJourney()
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-switch")).Return(journey, nil)

	// Redis returns no nearby buses
	mockFinder.On("FindBusesNearLocation", ctx, mock.Anything, 2.0, mock.Anything).Return(
		[]types.BusLocationInfo{},
		nil,
	)

	cmd := SwitchBusByLocationCommand{
		JourneyID:    "journey-switch",
		UserLocation: types.GeoLocation{Latitude: 1.0, Longitude: 100.0},
		Reason:       enums.JourneySwitchReasonLocation,
	}

	response, err := handler.Handle(ctx, cmd)

	assert.NoError(t, err)
	assert.False(t, response.WasSwitched)
	assert.Equal(t, "bus-1", response.OldBusID)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestSwitchBusByLocation_RedisError_NoSwitch(t *testing.T) {
	// When Redis fails, keep current bus (graceful degradation)
	mockRepo := new(MockJourneyRepository)
	mockFinder := new(mockBusLocationFinder)
	handler := NewSwitchBusByLocationHandler(mockRepo, mockFinder)
	ctx := context.Background()

	journey := createSwitchableJourney()
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-switch")).Return(journey, nil)

	// Redis returns an error
	mockFinder.On("FindBusesNearLocation", ctx, mock.Anything, 2.0, mock.Anything).Return(
		nil,
		errors.New("redis connection refused"),
	)

	cmd := SwitchBusByLocationCommand{
		JourneyID:    "journey-switch",
		UserLocation: types.GeoLocation{Latitude: 1.35, Longitude: 103.82},
		Reason:       enums.JourneySwitchReasonLocation,
	}

	response, err := handler.Handle(ctx, cmd)

	// Note: This is graceful degradation - no error propagated, but no switch
	assert.NoError(t, err)
	assert.False(t, response.WasSwitched)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestSwitchBusByLocation_JourneyNotFound_ReturnsError(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	mockFinder := new(mockBusLocationFinder)
	handler := NewSwitchBusByLocationHandler(mockRepo, mockFinder)
	ctx := context.Background()

	mockRepo.On("FindByID", ctx, types.JourneyID("nonexistent")).
		Return(nil, nil)

	cmd := SwitchBusByLocationCommand{
		JourneyID:    "nonexistent",
		UserLocation: types.GeoLocation{Latitude: 1.35, Longitude: 103.82},
		Reason:       enums.JourneySwitchReasonLocation,
	}

	_, err := handler.Handle(ctx, cmd)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	mockFinder.AssertNotCalled(t, "FindBusesNearLocation")
}

func TestSwitchBusByLocation_EmptyJourneyID_ReturnsError(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	mockFinder := new(mockBusLocationFinder)
	handler := NewSwitchBusByLocationHandler(mockRepo, mockFinder)

	cmd := SwitchBusByLocationCommand{
		JourneyID:    "", // Empty
		UserLocation: types.GeoLocation{Latitude: 1.35, Longitude: 103.82},
		Reason:       enums.JourneySwitchReasonLocation,
	}

	_, err := handler.Handle(context.Background(), cmd)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "journeyId")
	mockRepo.AssertNotCalled(t, "FindByID")
}

func TestSwitchBusByLocation_NoRecommendedBuses_NoSwitch(t *testing.T) {
	// When journey has no recommended buses, don't attempt geo search
	mockRepo := new(MockJourneyRepository)
	mockFinder := new(mockBusLocationFinder)
	handler := NewSwitchBusByLocationHandler(mockRepo, mockFinder)
	ctx := context.Background()

	// Create a journey without recommendations (just created, not tracking)
	journey, _ := aggregates.NewJourney(
		"journey-no-recs",
		"user-1",
		types.GeoLocation{Latitude: 1.35, Longitude: 103.82},
		"stop-origin",
		"stop-dest",
		time.Now(),
		&testhelpers.MockEventRecorder{},
	)

	mockRepo.On("FindByID", ctx, types.JourneyID("journey-no-recs")).Return(journey, nil)

	cmd := SwitchBusByLocationCommand{
		JourneyID:    "journey-no-recs",
		UserLocation: types.GeoLocation{Latitude: 1.35, Longitude: 103.82},
		Reason:       enums.JourneySwitchReasonLocation,
	}

	response, err := handler.Handle(ctx, cmd)

	assert.NoError(t, err)
	assert.False(t, response.WasSwitched)
	mockFinder.AssertNotCalled(t, "FindBusesNearLocation") // No geo search if no recommendations
}

func TestSwitchBusByLocation_VerifiesRadiusKm(t *testing.T) {
	// Verify the handler uses the correct radius (2.0 km)
	mockRepo := new(MockJourneyRepository)
	mockFinder := new(mockBusLocationFinder)
	handler := NewSwitchBusByLocationHandler(mockRepo, mockFinder)
	ctx := context.Background()

	journey := createSwitchableJourney()
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-switch")).Return(journey, nil)

	// Capture the radius parameter
	mockFinder.On("FindBusesNearLocation", ctx, mock.Anything, 2.0, mock.Anything).Return(
		[]types.BusLocationInfo{},
		nil,
	)

	cmd := SwitchBusByLocationCommand{
		JourneyID:    "journey-switch",
		UserLocation: types.GeoLocation{Latitude: 1.35, Longitude: 103.82},
		Reason:       enums.JourneySwitchReasonLocation,
	}

	_, _ = handler.Handle(ctx, cmd)

	// Verify radius was 2.0 km
	mockFinder.AssertCalled(t, "FindBusesNearLocation", ctx, mock.Anything, 2.0, mock.Anything)
}

func TestSwitchBusByLocation_PassesRecommendedBusIDsToFinder(t *testing.T) {
	// Verify the handler only queries for buses in the recommendations
	mockRepo := new(MockJourneyRepository)
	mockFinder := new(mockBusLocationFinder)
	handler := NewSwitchBusByLocationHandler(mockRepo, mockFinder)
	ctx := context.Background()

	journey := createSwitchableJourney()
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-switch")).Return(journey, nil)

	// Capture and verify busIDs parameter
	var capturedBusIDs []types.BusID
	mockFinder.On("FindBusesNearLocation", ctx, mock.Anything, 2.0, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedBusIDs = args.Get(3).([]types.BusID)
		}).
		Return([]types.BusLocationInfo{}, nil)

	cmd := SwitchBusByLocationCommand{
		JourneyID:    "journey-switch",
		UserLocation: types.GeoLocation{Latitude: 1.35, Longitude: 103.82},
		Reason:       enums.JourneySwitchReasonLocation,
	}

	_, _ = handler.Handle(ctx, cmd)

	// Verify we passed the recommended bus IDs
	assert.Len(t, capturedBusIDs, 3)
	assert.Contains(t, capturedBusIDs, types.BusID("bus-1"))
	assert.Contains(t, capturedBusIDs, types.BusID("bus-2"))
	assert.Contains(t, capturedBusIDs, types.BusID("bus-3"))
}
