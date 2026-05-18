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
// Mock Repository
// ============================================================================

type MockJourneyRepository struct {
	mock.Mock
}

func (m *MockJourneyRepository) Save(ctx context.Context, journey *aggregates.Journey) error {
	args := m.Called(ctx, journey)
	return args.Error(0)
}

func (m *MockJourneyRepository) FindByID(ctx context.Context, id types.JourneyID) (*aggregates.Journey, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*aggregates.Journey), args.Error(1)
}

func (m *MockJourneyRepository) FindActiveByUserID(ctx context.Context, userID types.UserID) (*aggregates.Journey, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*aggregates.Journey), args.Error(1)
}

func (m *MockJourneyRepository) CountActiveByUserID(ctx context.Context, userID types.UserID) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

// ============================================================================
// Test Helpers
// ============================================================================

func validOriginLocation() types.GeoLocation {
	return types.GeoLocation{
		Latitude:  -20.2936,
		Longitude: 57.3641,
	}
}

func validCreateJourneyCommand() CreateJourneyCommand {
	return CreateJourneyCommand{
		JourneyID:         "journey-1",
		UserID:            "user-1",
		OriginLocation:    validOriginLocation(),
		OriginStopID:      "stop-origin",
		DestinationStopID: "stop-dest",
		CreatedAt:         time.Now(),
		EventRecorder:     &testhelpers.MockEventRecorder{},
	}
}

func validRecommendations() []valueobjects.EnhancedBusRecommendation {
	return []valueobjects.EnhancedBusRecommendation{
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
	}
}

func createValidJourney() *aggregates.Journey {
	journey, _ := aggregates.NewJourney(
		"journey-1",
		"user-1",
		validOriginLocation(),
		"stop-origin",
		"stop-dest",
		time.Now(),
		&testhelpers.MockEventRecorder{},
	)
	return journey
}

func createTrackingJourney() *aggregates.Journey {
	journey := createValidJourney()
	_ = journey.InitializeTracking(
		validRecommendations(),
		types.Duration(30*time.Minute),
		0, // Outbound
	)
	return journey
}

func createBoardingPromptJourney() *aggregates.Journey {
	journey := createTrackingJourney()
	// Progress through proximity levels to reach BoardingPrompt
	_ = journey.UpdateProximity("bus-1", enums.ProximityLevelApproaching, 400)
	_ = journey.UpdateProximity("bus-1", enums.ProximityLevelNearby, 80)
	_ = journey.UpdateProximity("bus-1", enums.ProximityLevelArrived, 30)
	return journey
}

func createBoardedJourney() *aggregates.Journey {
	journey := createBoardingPromptJourney()
	_ = journey.ConfirmBoarded()
	return journey
}

// ============================================================================
// CreateJourney Tests
// ============================================================================

func TestCreateJourney_Success(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateJourneyCommand()

	mockRepo.On("Save", ctx, mock.AnythingOfType("*aggregates.Journey")).Return(nil)

	err := handler.CreateJourney(ctx, cmd)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestCreateJourney_InvalidLocation(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateJourneyCommand()
	cmd.OriginLocation = types.GeoLocation{Latitude: 0, Longitude: 0} // Invalid

	err := handler.CreateJourney(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestCreateJourney_SameOriginAndDestination(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateJourneyCommand()
	cmd.DestinationStopID = cmd.OriginStopID // Same as origin

	err := handler.CreateJourney(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestCreateJourney_NilEventRecorder(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateJourneyCommand()
	cmd.EventRecorder = nil // Required by domain

	err := handler.CreateJourney(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestCreateJourney_RepositorySaveFailure(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateJourneyCommand()

	mockRepo.On("Save", ctx, mock.AnythingOfType("*aggregates.Journey")).
		Return(errors.New("database error"))

	err := handler.CreateJourney(ctx, cmd)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
	mockRepo.AssertExpectations(t)
}

// ============================================================================
// InitializeTracking Tests
// ============================================================================

func TestInitializeTracking_Success(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createValidJourney()
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)
	mockRepo.On("Save", ctx, journey).Return(nil)

	cmd := InitializeTrackingCommand{
		JourneyID:         "journey-1",
		Recommendations:   validRecommendations(),
		Duration:          types.Duration(30 * time.Minute),
		RequiredDirection: 0,
	}

	err := handler.InitializeTracking(ctx, cmd)

	assert.NoError(t, err)
	assert.Equal(t, enums.JourneyStatusTracking, journey.Status())
	mockRepo.AssertExpectations(t)
}

func TestInitializeTracking_EmptyRecommendations(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createValidJourney()
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)

	cmd := InitializeTrackingCommand{
		JourneyID:         "journey-1",
		Recommendations:   []valueobjects.EnhancedBusRecommendation{}, // Empty
		Duration:          types.Duration(30 * time.Minute),
		RequiredDirection: 0,
	}

	err := handler.InitializeTracking(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestInitializeTracking_InvalidDuration(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createValidJourney()
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)

	cmd := InitializeTrackingCommand{
		JourneyID:         "journey-1",
		Recommendations:   validRecommendations(),
		Duration:          types.Duration(1 * time.Minute), // Too short (< 5 min)
		RequiredDirection: 0,
	}

	err := handler.InitializeTracking(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestInitializeTracking_JourneyNotFound(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).
		Return(nil, errors.New("journey not found"))

	cmd := InitializeTrackingCommand{
		JourneyID:         "journey-1",
		Recommendations:   validRecommendations(),
		Duration:          types.Duration(30 * time.Minute),
		RequiredDirection: 0,
	}

	err := handler.InitializeTracking(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

// ============================================================================
// SwitchBus Tests
// ============================================================================

func TestSwitchBus_Success(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createTrackingJourney()
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)
	mockRepo.On("Save", ctx, journey).Return(nil)

	cmd := SwitchBusCommand{
		JourneyID: "journey-1",
		NewBusID:  "bus-2", // Switch to second recommended bus
		Reason:    enums.JourneySwitchReasonUser,
	}

	err := handler.SwitchBus(ctx, cmd)

	assert.NoError(t, err)
	assert.Equal(t, types.BusID("bus-2"), journey.ActiveBusID())
	mockRepo.AssertExpectations(t)
}

func TestSwitchBus_BusNotInRecommendations(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createTrackingJourney()
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)

	cmd := SwitchBusCommand{
		JourneyID: "journey-1",
		NewBusID:  "bus-unknown", // Not in recommendations
		Reason:    enums.JourneySwitchReasonUser,
	}

	err := handler.SwitchBus(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestSwitchBus_JourneyNotFound(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).
		Return(nil, errors.New("journey not found"))

	cmd := SwitchBusCommand{
		JourneyID: "journey-1",
		NewBusID:  "bus-2",
		Reason:    enums.JourneySwitchReasonUser,
	}

	err := handler.SwitchBus(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

// ============================================================================
// UpdateProximity Tests
// ============================================================================

func TestUpdateProximity_Success(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createTrackingJourney()
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)
	mockRepo.On("Save", ctx, journey).Return(nil)

	cmd := UpdateProximityCommand{
		JourneyID: "journey-1",
		BusID:     "bus-1",
		Level:     enums.ProximityLevelApproaching,
		Distance:  400,
	}

	err := handler.UpdateProximity(ctx, cmd)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestUpdateProximity_TriggersBoarding(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	// Create journey already at Nearby level
	journey := createTrackingJourney()
	_ = journey.UpdateProximity("bus-1", enums.ProximityLevelApproaching, 400)
	_ = journey.UpdateProximity("bus-1", enums.ProximityLevelNearby, 80)

	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)
	mockRepo.On("Save", ctx, journey).Return(nil)

	cmd := UpdateProximityCommand{
		JourneyID: "journey-1",
		BusID:     "bus-1",
		Level:     enums.ProximityLevelArrived,
		Distance:  30,
	}

	err := handler.UpdateProximity(ctx, cmd)

	assert.NoError(t, err)
	assert.Equal(t, enums.JourneyStatusBoardingPrompt, journey.Status())
	mockRepo.AssertExpectations(t)
}

func TestUpdateProximity_WrongBus(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createTrackingJourney()
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)

	cmd := UpdateProximityCommand{
		JourneyID: "journey-1",
		BusID:     "bus-wrong", // Not the active bus
		Level:     enums.ProximityLevelApproaching,
		Distance:  400,
	}

	err := handler.UpdateProximity(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestUpdateProximity_InvalidProgression(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createTrackingJourney()
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)

	// Try to skip from None to Arrived (invalid)
	cmd := UpdateProximityCommand{
		JourneyID: "journey-1",
		BusID:     "bus-1",
		Level:     enums.ProximityLevelArrived, // Skipping levels
		Distance:  30,
	}

	err := handler.UpdateProximity(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

// ============================================================================
// ConfirmBoarding Tests
// ============================================================================

func TestConfirmBoarding_Success(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createBoardingPromptJourney()
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)
	mockRepo.On("Save", ctx, journey).Return(nil)

	cmd := ConfirmBoardingCommand{
		Journey: "journey-1",
	}

	err := handler.ConfirmBoarding(ctx, cmd)

	assert.NoError(t, err)
	assert.Equal(t, enums.JourneyStatusBoarded, journey.Status())
	mockRepo.AssertExpectations(t)
}

func TestConfirmBoarding_NotInBoardingState(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createTrackingJourney() // Still in Tracking, not BoardingPrompt
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)

	cmd := ConfirmBoardingCommand{
		Journey: "journey-1",
	}

	err := handler.ConfirmBoarding(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestConfirmBoarding_JourneyNotFound(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).
		Return(nil, errors.New("journey not found"))

	cmd := ConfirmBoardingCommand{
		Journey: "journey-1",
	}

	err := handler.ConfirmBoarding(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

// ============================================================================
// DeclineBoarding Tests
// ============================================================================

func TestDeclineBoarding_Success(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createBoardingPromptJourney()
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)
	mockRepo.On("Save", ctx, journey).Return(nil)

	cmd := DeclineBoardingCommand{
		Journey: "journey-1",
	}

	err := handler.DeclineBoarding(ctx, cmd)

	assert.NoError(t, err)
	assert.Equal(t, enums.JourneyStatusTracking, journey.Status()) // Back to Tracking
	assert.Equal(t, 1, journey.DeclineCount())
	mockRepo.AssertExpectations(t)
}

func TestDeclineBoarding_NotInBoardingState(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createTrackingJourney() // Not in BoardingPrompt
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)

	cmd := DeclineBoardingCommand{
		Journey: "journey-1",
	}

	err := handler.DeclineBoarding(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

// ============================================================================
// CompleteJourney Tests
// ============================================================================

func TestCompleteJourney_Success(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createBoardedJourney()
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)
	mockRepo.On("Save", ctx, journey).Return(nil)

	cmd := CompleteJourneyCommand{
		Journey: "journey-1",
	}

	err := handler.CompleteJourney(ctx, cmd)

	assert.NoError(t, err)
	assert.Equal(t, enums.JourneyStatusCompleted, journey.Status())
	mockRepo.AssertExpectations(t)
}

func TestCompleteJourney_NotBoarded(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createTrackingJourney() // Not boarded yet
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)

	cmd := CompleteJourneyCommand{
		Journey: "journey-1",
	}

	err := handler.CompleteJourney(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestCompleteJourney_JourneyNotFound(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).
		Return(nil, errors.New("journey not found"))

	cmd := CompleteJourneyCommand{
		Journey: "journey-1",
	}

	err := handler.CompleteJourney(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

// ============================================================================
// CancelJourney Tests
// ============================================================================

func TestCancelJourney_Success(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createTrackingJourney()
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)
	mockRepo.On("Save", ctx, journey).Return(nil)

	cmd := CancelJourneyCommand{
		Journey: "journey-1",
	}

	err := handler.CancelJourney(ctx, cmd)

	assert.NoError(t, err)
	assert.Equal(t, enums.JourneyStatusCancelled, journey.Status())
	mockRepo.AssertExpectations(t)
}

func TestCancelJourney_AlreadyCompleted(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createBoardedJourney()
	_ = journey.CompleteJourney() // Complete it first
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)

	cmd := CancelJourneyCommand{
		Journey: "journey-1",
	}

	err := handler.CancelJourney(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestCancelJourney_JourneyNotFound(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).
		Return(nil, errors.New("journey not found"))

	cmd := CancelJourneyCommand{
		Journey: "journey-1",
	}

	err := handler.CancelJourney(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

// ============================================================================
// ExpireJourney Tests
// ============================================================================

func TestExpireJourney_Success(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createTrackingJourney()
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)
	mockRepo.On("Save", ctx, journey).Return(nil)

	cmd := ExpireJourneyCommand{
		Journey:   "journey-1",
		ExpiredAt: time.Now(),
	}

	err := handler.ExpireJourney(ctx, cmd)

	assert.NoError(t, err)
	assert.Equal(t, enums.JourneyStatusExpired, journey.Status())
	mockRepo.AssertExpectations(t)
}

func TestExpireJourney_AlreadyCompleted(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createBoardedJourney()
	_ = journey.CompleteJourney() // Complete it first
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)

	cmd := ExpireJourneyCommand{
		Journey:   "journey-1",
		ExpiredAt: time.Now(),
	}

	err := handler.ExpireJourney(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestExpireJourney_JourneyNotFound(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).
		Return(nil, errors.New("journey not found"))

	cmd := ExpireJourneyCommand{
		Journey:   "journey-1",
		ExpiredAt: time.Now(),
	}

	err := handler.ExpireJourney(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

// ============================================================================
// UpdateRecommendations Tests
// ============================================================================

func TestUpdateRecommendations_Success(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	journey := createTrackingJourney()
	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).Return(journey, nil)
	mockRepo.On("Save", ctx, journey).Return(nil)

	// Create new recommendations with different ranking
	newRecs := []valueobjects.EnhancedBusRecommendation{
		{
			BusID:                "bus-2", // Now first
			OperatorID:           "operator-1",
			EstimatedArrival:     types.Duration(3 * time.Minute),
			Direction:            0,
			RequiredDirection:    0,
			IsWrongDirection:     false,
			ConfidenceLevel:      0.95,
			DisplayText:          "Bus 2 arriving in 3 min",
			DistanceToOriginStop: 300,
		},
		{
			BusID:                "bus-1",
			OperatorID:           "operator-1",
			EstimatedArrival:     types.Duration(8 * time.Minute),
			Direction:            0,
			RequiredDirection:    0,
			IsWrongDirection:     false,
			ConfidenceLevel:      0.85,
			DisplayText:          "Bus 1 arriving in 8 min",
			DistanceToOriginStop: 700,
		},
	}

	cmd := UpdateRecommendationsCommand{
		JourneyID:       "journey-1",
		Recommendations: newRecs,
	}

	err := handler.UpdateRecommendations(ctx, cmd)

	assert.NoError(t, err)
	assert.Equal(t, types.BusID("bus-2"), journey.ActiveBusID()) // Active bus changed
	mockRepo.AssertExpectations(t)
}

func TestUpdateRecommendations_JourneyNotFound(t *testing.T) {
	mockRepo := new(MockJourneyRepository)
	handler := NewJourneyCommandHandler(mockRepo)
	ctx := context.Background()

	mockRepo.On("FindByID", ctx, types.JourneyID("journey-1")).
		Return(nil, errors.New("journey not found"))

	cmd := UpdateRecommendationsCommand{
		JourneyID:       "journey-1",
		Recommendations: validRecommendations(),
	}

	err := handler.UpdateRecommendations(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}
