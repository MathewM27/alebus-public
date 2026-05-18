package usermgmt

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	domainerrors "github.com/MathewM27/busTrack-alebus/domain/errors"
	"github.com/MathewM27/busTrack-alebus/domain/testhelpers"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock Repository ---

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Save(ctx context.Context, user *aggregates.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) FindByID(ctx context.Context, id types.UserID) (*aggregates.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*aggregates.User), args.Error(1)
}

// --- Test Helpers ---

func activeSubscription(plan valueobjects.SubscriptionPlan) valueobjects.Subscription {
	now := time.Now()
	return valueobjects.Subscription{
		Status:     valueobjects.SubscriptionStatusActive,
		Plan:       plan,
		StartDate:  now.Add(-24 * time.Hour),
		ExpiryDate: now.Add(30 * 24 * time.Hour),
	}
}

func validCreateUserCommand() CreateUserCommand {
	now := time.Now()
	return CreateUserCommand{
		UserID:        "user-1",
		Email:         "user1@example.com",
		Subscription:  activeSubscription(valueobjects.SubscriptionPlanBasic),
		CreatedAt:     now,
		EventRecorder: &testhelpers.MockEventRecorder{},
	}
}

func createValidUser() *aggregates.User {
	now := time.Now()
	user, _ := aggregates.NewUser(
		"user-1",
		"user1@example.com",
		activeSubscription(valueobjects.SubscriptionPlanBasic),
		now,
		&testhelpers.MockEventRecorder{},
	)
	return user
}

// --- CreateUser Tests ---

func TestCreateUser_Success(t *testing.T) {
	mockRepo := new(MockUserRepository)
	handler := NewUserCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateUserCommand()

	mockRepo.On("Save", ctx, mock.AnythingOfType("*aggregates.User")).Return(nil)

	err := handler.CreateUser(ctx, cmd)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestCreateUser_DomainValidationFailure_EmptyUserID(t *testing.T) {
	mockRepo := new(MockUserRepository)
	handler := NewUserCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateUserCommand()
	cmd.UserID = ""

	err := handler.CreateUser(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestCreateUser_RepositorySaveFailure(t *testing.T) {
	mockRepo := new(MockUserRepository)
	handler := NewUserCommandHandler(mockRepo)
	ctx := context.Background()
	cmd := validCreateUserCommand()

	mockRepo.On("Save", ctx, mock.AnythingOfType("*aggregates.User")).Return(errors.New("db error"))

	err := handler.CreateUser(ctx, cmd)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
	mockRepo.AssertExpectations(t)
}

// --- ChangeEmail Tests ---

func TestChangeEmail_Success(t *testing.T) {
	mockRepo := new(MockUserRepository)
	handler := NewUserCommandHandler(mockRepo)
	ctx := context.Background()

	existingUser := createValidUser()
	cmd := ChangeEmailCommand{UserID: "user-1", NewEmail: "new@example.com"}

	mockRepo.On("FindByID", ctx, types.UserID("user-1")).Return(existingUser, nil)
	mockRepo.On("Save", ctx, existingUser).Return(nil)

	err := handler.ChangeEmail(ctx, cmd)

	assert.NoError(t, err)
	assert.Equal(t, "new@example.com", existingUser.Email())
	mockRepo.AssertExpectations(t)
}

func TestChangeEmail_InvalidEmailFormat(t *testing.T) {
	mockRepo := new(MockUserRepository)
	handler := NewUserCommandHandler(mockRepo)
	ctx := context.Background()

	existingUser := createValidUser()
	cmd := ChangeEmailCommand{UserID: "user-1", NewEmail: "not-an-email"}

	mockRepo.On("FindByID", ctx, types.UserID("user-1")).Return(existingUser, nil)

	err := handler.ChangeEmail(ctx, cmd)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "Save")
}

// --- Subscription Tests ---

func TestUpdateSubscription_Success(t *testing.T) {
	mockRepo := new(MockUserRepository)
	handler := NewUserCommandHandler(mockRepo)
	ctx := context.Background()

	existingUser := createValidUser()
	cmd := UpdateSubscriptionCommand{UserID: "user-1", NewSubscription: activeSubscription(valueobjects.SubscriptionPlanPremium)}

	mockRepo.On("FindByID", ctx, types.UserID("user-1")).Return(existingUser, nil)
	mockRepo.On("Save", ctx, existingUser).Return(nil)

	err := handler.UpdateSubscription(ctx, cmd)

	assert.NoError(t, err)
	assert.Equal(t, valueobjects.SubscriptionPlanPremium, existingUser.Subscription().Plan)
	mockRepo.AssertExpectations(t)
}

func TestUpdateSubscription_DomainValidationFailure_InactiveSubscription(t *testing.T) {
	mockRepo := new(MockUserRepository)
	handler := NewUserCommandHandler(mockRepo)
	ctx := context.Background()

	existingUser := createValidUser()
	inactive := activeSubscription(valueobjects.SubscriptionPlanBasic)
	inactive.Status = valueobjects.SubscriptionStatusInactive
	cmd := UpdateSubscriptionCommand{UserID: "user-1", NewSubscription: inactive}

	mockRepo.On("FindByID", ctx, types.UserID("user-1")).Return(existingUser, nil)

	err := handler.UpdateSubscription(ctx, cmd)

	assert.ErrorIs(t, err, domainerrors.ErrInvalidSubscription)
	mockRepo.AssertNotCalled(t, "Save")
}

// --- Saved Location Tests ---

func TestAddSavedLocation_Success(t *testing.T) {
	mockRepo := new(MockUserRepository)
	handler := NewUserCommandHandler(mockRepo)
	ctx := context.Background()

	existingUser := createValidUser()
	cmd := AddSavedLocationCommand{UserID: "user-1", Name: "Home", Lat: 10.0, Lon: 20.0, StopID: nil}

	mockRepo.On("FindByID", ctx, types.UserID("user-1")).Return(existingUser, nil)
	mockRepo.On("Save", ctx, existingUser).Return(nil)

	err := handler.AddSavedLocation(ctx, cmd)

	assert.NoError(t, err)
	assert.True(t, existingUser.HasSavedLocation("Home"))
	mockRepo.AssertExpectations(t)
}

func TestAddSavedLocation_DomainValidationFailure_MaxLocationsReached(t *testing.T) {
	mockRepo := new(MockUserRepository)
	handler := NewUserCommandHandler(mockRepo)
	ctx := context.Background()

	existingUser := createValidUser()
	// Fill up to the domain max (10)
	for i := 0; i < 10; i++ {
		_ = existingUser.AddSavedLocation(valueobjects.SavedLocation{
			Name:     fmt.Sprintf("Loc-%d", i),
			Location: types.GeoLocation{Latitude: float64(i), Longitude: float64(i)},
		})
	}

	cmd := AddSavedLocationCommand{UserID: "user-1", Name: "Overflow", Lat: 0, Lon: 0}

	mockRepo.On("FindByID", ctx, types.UserID("user-1")).Return(existingUser, nil)

	err := handler.AddSavedLocation(ctx, cmd)

	assert.ErrorIs(t, err, domainerrors.ErrMaxSavedLocationsReached)
	mockRepo.AssertNotCalled(t, "Save")
}

func TestClearSavedLocations_Success(t *testing.T) {
	mockRepo := new(MockUserRepository)
	handler := NewUserCommandHandler(mockRepo)
	ctx := context.Background()

	existingUser := createValidUser()
	_ = existingUser.AddSavedLocation(valueobjects.SavedLocation{Name: "Home", Location: types.GeoLocation{Latitude: 1, Longitude: 2}})

	cmd := ClearSavedLocationsCommand{UserID: "user-1"}

	mockRepo.On("FindByID", ctx, types.UserID("user-1")).Return(existingUser, nil)
	mockRepo.On("Save", ctx, existingUser).Return(nil)

	err := handler.ClearSavedLocations(ctx, cmd)

	assert.NoError(t, err)
	assert.Len(t, existingUser.SavedLocations(), 0)
	mockRepo.AssertExpectations(t)
}
