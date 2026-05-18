package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
	"github.com/MathewM27/busTrack-alebus/infrastructure/repositories"
)

// Note: testEventRecorder is defined in route_repository_test.go and shared via repositories_test package
// Note: setupTestPool is defined in route_repository_test.go and shared via repositories_test package

func setupUserTest(t *testing.T) (*repositories.PostgresUserRepository, func()) {
	t.Helper()

	pool := setupTestPool(t)
	repo := repositories.NewPostgresUserRepository(pool)

	cleanup := func() {
		// Clean up test data
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE user_id LIKE 'test-%'")
		pool.Close()
	}

	return repo, cleanup
}

// createTestUser creates a valid User for testing
func createTestUser(t *testing.T, id string) *aggregates.User {
	t.Helper()

	subscription := valueobjects.Subscription{
		Status:     valueobjects.SubscriptionStatusActive,
		Plan:       valueobjects.SubscriptionPlanBasic,
		StartDate:  time.Now().Add(-30 * 24 * time.Hour), // 30 days ago
		ExpiryDate: time.Now().Add(335 * 24 * time.Hour), // 335 days from now
	}

	user, err := aggregates.NewUser(
		types.UserID("test-"+id),
		"test-"+id+"@example.com",
		subscription,
		time.Now().UTC().Truncate(time.Microsecond),
		&testEventRecorder{},
	)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	return user
}

func TestUserRepository_SaveAndFindByID(t *testing.T) {
	repo, cleanup := setupUserTest(t)
	defer cleanup()

	ctx := context.Background()
	user := createTestUser(t, uuid.NewString()[:8])

	// Save
	err := repo.Save(ctx, user)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// FindByID
	found, err := repo.FindByID(ctx, user.ID())
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	// Verify core fields
	if found.ID() != user.ID() {
		t.Errorf("ID mismatch: got %v, want %v", found.ID(), user.ID())
	}
	if found.Email() != user.Email() {
		t.Errorf("Email mismatch: got %v, want %v", found.Email(), user.Email())
	}
}

func TestUserRepository_FindByID_NotFound(t *testing.T) {
	repo, cleanup := setupUserTest(t)
	defer cleanup()

	ctx := context.Background()

	_, err := repo.FindByID(ctx, "nonexistent-user-id")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
	if err != repositories.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserRepository_SubscriptionRoundTrip(t *testing.T) {
	repo, cleanup := setupUserTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create user with Premium subscription
	// Use future dates to ensure subscription is considered active
	// Truncate to microseconds since PostgreSQL TIMESTAMP has microsecond precision
	now := time.Now().UTC().Truncate(time.Microsecond)
	subscription := valueobjects.Subscription{
		Status:     valueobjects.SubscriptionStatusActive,
		Plan:       valueobjects.SubscriptionPlanPremium,
		StartDate:  now.AddDate(0, -1, 0), // 1 month ago
		ExpiryDate: now.AddDate(1, 0, 0),  // 1 year from now
	}

	user, err := aggregates.NewUser(
		types.UserID("test-sub-"+uuid.NewString()[:8]),
		"premium@example.com",
		subscription,
		time.Now().UTC().Truncate(time.Microsecond),
		&testEventRecorder{},
	)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Save
	err = repo.Save(ctx, user)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load and verify subscription
	found, err := repo.FindByID(ctx, user.ID())
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	foundSub := found.Subscription()
	origSub := user.Subscription()

	if foundSub.Status != origSub.Status {
		t.Errorf("Subscription.Status mismatch: got %v, want %v", foundSub.Status, origSub.Status)
	}
	if foundSub.Plan != origSub.Plan {
		t.Errorf("Subscription.Plan mismatch: got %v, want %v", foundSub.Plan, origSub.Plan)
	}
	if !foundSub.StartDate.Equal(origSub.StartDate) {
		t.Errorf("Subscription.StartDate mismatch: got %v, want %v", foundSub.StartDate, origSub.StartDate)
	}
	if !foundSub.ExpiryDate.Equal(origSub.ExpiryDate) {
		t.Errorf("Subscription.ExpiryDate mismatch: got %v, want %v", foundSub.ExpiryDate, origSub.ExpiryDate)
	}
}

func TestUserRepository_SavedLocationsJSONBRoundTrip(t *testing.T) {
	repo, cleanup := setupUserTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create saved locations
	stopID := types.StopID("stop-123")
	savedLocations := []valueobjects.SavedLocation{
		{
			Name: "Home",
			Location: types.GeoLocation{
				Latitude:  -26.2041,
				Longitude: 28.0473,
			},
			StopID: &stopID,
		},
		{
			Name: "Work",
			Location: types.GeoLocation{
				Latitude:  -26.1076,
				Longitude: 28.0567,
			},
			StopID: nil, // No associated stop
		},
	}

	subscription := valueobjects.Subscription{
		Status:     valueobjects.SubscriptionStatusActive,
		Plan:       valueobjects.SubscriptionPlanBasic,
		StartDate:  time.Now().Add(-30 * 24 * time.Hour),
		ExpiryDate: time.Now().Add(335 * 24 * time.Hour),
	}

	// Use RehydrateUser to set saved locations
	user, err := aggregates.RehydrateUser(
		types.UserID("test-locs-"+uuid.NewString()[:8]),
		"locations@example.com",
		subscription,
		savedLocations,
		time.Now().UTC().Truncate(time.Microsecond),
		1,
		&testEventRecorder{},
	)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Save
	err = repo.Save(ctx, user)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load and verify saved locations
	found, err := repo.FindByID(ctx, user.ID())
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	foundLocs := found.SavedLocations()
	if len(foundLocs) != len(savedLocations) {
		t.Fatalf("SavedLocations count mismatch: got %d, want %d", len(foundLocs), len(savedLocations))
	}

	// Verify first location (with StopID)
	if foundLocs[0].Name != savedLocations[0].Name {
		t.Errorf("Loc[0].Name mismatch: got %v, want %v", foundLocs[0].Name, savedLocations[0].Name)
	}
	if foundLocs[0].Location.Latitude != savedLocations[0].Location.Latitude {
		t.Errorf("Loc[0].Latitude mismatch")
	}
	if foundLocs[0].StopID == nil || *foundLocs[0].StopID != stopID {
		t.Errorf("Loc[0].StopID mismatch: got %v, want %v", foundLocs[0].StopID, stopID)
	}

	// Verify second location (without StopID)
	if foundLocs[1].Name != savedLocations[1].Name {
		t.Errorf("Loc[1].Name mismatch")
	}
	if foundLocs[1].StopID != nil {
		t.Errorf("Loc[1].StopID should be nil, got %v", foundLocs[1].StopID)
	}
}

func TestUserRepository_EmptySavedLocations(t *testing.T) {
	repo, cleanup := setupUserTest(t)
	defer cleanup()

	ctx := context.Background()
	user := createTestUser(t, uuid.NewString()[:8])

	// User created via NewUser has empty saved locations
	if len(user.SavedLocations()) != 0 {
		t.Fatalf("new user should have empty saved locations")
	}

	// Save
	err := repo.Save(ctx, user)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load and verify
	found, err := repo.FindByID(ctx, user.ID())
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if len(found.SavedLocations()) != 0 {
		t.Errorf("expected empty saved locations, got %d", len(found.SavedLocations()))
	}
}

func TestUserRepository_OptimisticLocking(t *testing.T) {
	repo, cleanup := setupUserTest(t)
	defer cleanup()

	ctx := context.Background()
	user := createTestUser(t, uuid.NewString()[:8])

	// First save should succeed
	err := repo.Save(ctx, user)
	if err != nil {
		t.Fatalf("First Save() error = %v", err)
	}

	// To test optimistic locking, attempt to save an entity with a stale version.
	// Simplest: save the original user twice without reloading.
	err = repo.Save(ctx, user) // original user still has version 1
	if err != repositories.ErrUserVersionStale {
		t.Errorf("expected ErrUserVersionStale for stale version, got %v", err)
	}
}

func TestUserRepository_VersionIncrement(t *testing.T) {
	repo, cleanup := setupUserTest(t)
	defer cleanup()

	ctx := context.Background()
	user := createTestUser(t, uuid.NewString()[:8])

	// Initial version should be 1
	if user.Version() != 1 {
		t.Fatalf("initial version should be 1, got %d", user.Version())
	}

	// Save - version in DB should become 2
	err := repo.Save(ctx, user)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load and check version
	loaded, err := repo.FindByID(ctx, user.ID())
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if loaded.Version() != 2 {
		t.Errorf("version after first save should be 2, got %d", loaded.Version())
	}
}
