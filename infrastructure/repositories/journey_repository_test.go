package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
	"github.com/MathewM27/busTrack-alebus/infrastructure/repositories"
)

// Note: testEventRecorder is defined in route_repository_test.go and shared via repositories_test package
// Note: setupTestPool is defined in route_repository_test.go and shared via repositories_test package

func setupJourneyTest(t *testing.T) (*repositories.PostgresJourneyRepository, func()) {
	t.Helper()

	pool := setupTestPool(t)
	repo := repositories.NewPostgresJourneyRepository(pool)

	cleanup := func() {
		// Clean up test data
		_, _ = pool.Exec(context.Background(), "DELETE FROM journeys WHERE journey_id LIKE 'test-%'")
		pool.Close()
	}

	return repo, cleanup
}

// createTestJourney creates a valid Journey for testing
func createTestJourney(t *testing.T, id string) *aggregates.Journey {
	t.Helper()

	origin := types.GeoLocation{Latitude: -26.2041, Longitude: 28.0473}
	originStopID := types.StopID("stop-origin-" + id)
	destinationStopID := types.StopID("stop-dest-" + id)

	// Create sample recommendations
	recommendations := []valueobjects.EnhancedBusRecommendation{
		{
			BusID:               types.BusID("bus-1"),
			OperatorID:          types.OperatorID("op-1"),
			ActualRouteDistance: 5000,
			EstimatedArrival:    types.Duration(10 * time.Minute),
			JourneyInfo: valueobjects.BusJourneyInfo{
				Type:             "direct",
				TotalDistance:    10000,
				EstimatedTime:    types.Duration(25 * time.Minute),
				RequiresTerminal: false,
				TerminalWaitTime: 0,
				Breakdown: valueobjects.JourneyBreakdown{
					ToTerminal:   0,
					AtTerminal:   0,
					FromTerminal: 0,
				},
			},
			Direction:            1,
			RequiredDirection:    1,
			IsWrongDirection:     false,
			DisplayText:          "Bus 1 - 10 min",
			ConfidenceLevel:      0.95,
			RankingScore:         90.5,
			DistanceToOriginStop: 250,
			Confidence:           0.95,
			Rank:                 1,
		},
		{
			BusID:               types.BusID("bus-2"),
			OperatorID:          types.OperatorID("op-1"),
			ActualRouteDistance: 6000,
			EstimatedArrival:    types.Duration(15 * time.Minute),
			JourneyInfo: valueobjects.BusJourneyInfo{
				Type:             "terminal",
				TotalDistance:    12000,
				EstimatedTime:    types.Duration(35 * time.Minute),
				RequiresTerminal: true,
				TerminalWaitTime: types.Duration(5 * time.Minute),
				Breakdown: valueobjects.JourneyBreakdown{
					ToTerminal:   types.Duration(10 * time.Minute),
					AtTerminal:   types.Duration(5 * time.Minute),
					FromTerminal: types.Duration(20 * time.Minute),
				},
			},
			Direction:            -1,
			RequiredDirection:    1,
			IsWrongDirection:     true,
			DisplayText:          "Bus 2 - 15 min (via terminal)",
			ConfidenceLevel:      0.80,
			RankingScore:         75.0,
			DistanceToOriginStop: 500,
			Confidence:           0.80,
			Rank:                 2,
		},
	}

	// Use RehydrateJourney for test flexibility (bypasses time checks)
	journey, err := aggregates.RehydrateJourney(
		types.JourneyID("test-"+id),
		types.UserID("user-"+id),
		origin,
		originStopID,
		destinationStopID,
		recommendations,
		types.BusID("bus-1"), // active bus
		enums.JourneyStatusSearching,
		enums.ProximityLevelNone,
		0,                              // decline count
		types.Direction(1),             // required direction
		types.Duration(25*time.Minute), // estimated duration
		time.Now().Add(30*time.Minute), // expiration
		nil,                            // boarding window not started
		nil,                            // not boarded
		time.Now(),                     // created at
		1,                              // version - initial version for new journey
		&testEventRecorder{},
	)
	if err != nil {
		t.Fatalf("failed to create test journey: %v", err)
	}

	return journey
}

func TestJourneyRepository_SaveAndFindByID(t *testing.T) {
	repo, cleanup := setupJourneyTest(t)
	defer cleanup()

	ctx := context.Background()
	journey := createTestJourney(t, uuid.NewString()[:8])

	// Save
	err := repo.Save(ctx, journey)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// FindByID
	found, err := repo.FindByID(ctx, journey.JourneyID())
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	// Verify core fields
	if found.JourneyID() != journey.JourneyID() {
		t.Errorf("JourneyID mismatch: got %v, want %v", found.JourneyID(), journey.JourneyID())
	}
	if found.UserID() != journey.UserID() {
		t.Errorf("UserID mismatch: got %v, want %v", found.UserID(), journey.UserID())
	}
	if found.Status() != journey.Status() {
		t.Errorf("Status mismatch: got %v, want %v", found.Status(), journey.Status())
	}
	if found.ActiveBusID() != journey.ActiveBusID() {
		t.Errorf("ActiveBusID mismatch: got %v, want %v", found.ActiveBusID(), journey.ActiveBusID())
	}
}

func TestJourneyRepository_RecommendationsJSONBRoundTrip(t *testing.T) {
	repo, cleanup := setupJourneyTest(t)
	defer cleanup()

	ctx := context.Background()
	journey := createTestJourney(t, uuid.NewString()[:8])

	// Save
	err := repo.Save(ctx, journey)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// FindByID
	found, err := repo.FindByID(ctx, journey.JourneyID())
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	// Verify recommendations count
	originalRecs := journey.RecommendedBuses()
	foundRecs := found.RecommendedBuses()
	if len(foundRecs) != len(originalRecs) {
		t.Fatalf("Recommendations count mismatch: got %d, want %d", len(foundRecs), len(originalRecs))
	}

	// Verify first recommendation details
	if len(foundRecs) > 0 {
		orig := originalRecs[0]
		rec := foundRecs[0]

		if rec.BusID != orig.BusID {
			t.Errorf("Rec[0].BusID mismatch: got %v, want %v", rec.BusID, orig.BusID)
		}
		if rec.JourneyInfo.Type != orig.JourneyInfo.Type {
			t.Errorf("Rec[0].JourneyInfo.Type mismatch: got %v, want %v", rec.JourneyInfo.Type, orig.JourneyInfo.Type)
		}
		if rec.JourneyInfo.RequiresTerminal != orig.JourneyInfo.RequiresTerminal {
			t.Errorf("Rec[0].RequiresTerminal mismatch: got %v, want %v", rec.JourneyInfo.RequiresTerminal, orig.JourneyInfo.RequiresTerminal)
		}
		if rec.RankingScore != orig.RankingScore {
			t.Errorf("Rec[0].RankingScore mismatch: got %v, want %v", rec.RankingScore, orig.RankingScore)
		}
	}

	// Verify second recommendation (terminal journey type)
	if len(foundRecs) > 1 {
		orig := originalRecs[1]
		rec := foundRecs[1]

		if rec.JourneyInfo.Type != "terminal" {
			t.Errorf("Rec[1] should be terminal type")
		}
		if rec.JourneyInfo.Breakdown.ToTerminal != orig.JourneyInfo.Breakdown.ToTerminal {
			t.Errorf("Rec[1].Breakdown.ToTerminal mismatch")
		}
		if rec.IsWrongDirection != orig.IsWrongDirection {
			t.Errorf("Rec[1].IsWrongDirection mismatch: got %v, want %v", rec.IsWrongDirection, orig.IsWrongDirection)
		}
	}
}

func TestJourneyRepository_FindByID_NotFound(t *testing.T) {
	repo, cleanup := setupJourneyTest(t)
	defer cleanup()

	ctx := context.Background()

	_, err := repo.FindByID(ctx, "nonexistent-journey-id")
	if err == nil {
		t.Fatal("expected error for nonexistent journey")
	}
	if err != repositories.ErrJourneyNotFound {
		t.Errorf("expected ErrJourneyNotFound, got %v", err)
	}
}

func TestJourneyRepository_FindActiveByUserID(t *testing.T) {
	repo, cleanup := setupJourneyTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user-" + uuid.NewString()[:8]

	// Create an active journey (Searching status)
	activeJourney, err := aggregates.RehydrateJourney(
		types.JourneyID("test-active-"+uuid.NewString()[:8]),
		types.UserID(userID),
		types.GeoLocation{Latitude: -26.2041, Longitude: 28.0473},
		types.StopID("stop-origin"),
		types.StopID("stop-dest"),
		[]valueobjects.EnhancedBusRecommendation{},
		"",
		enums.JourneyStatusSearching, // Active status
		enums.ProximityLevelNone,
		0,
		types.Direction(1),
		types.Duration(25*time.Minute),
		time.Now().Add(30*time.Minute),
		nil,
		nil,
		time.Now(),
		1, // version - initial version
		&testEventRecorder{},
	)
	if err != nil {
		t.Fatalf("failed to create active journey: %v", err)
	}

	err = repo.Save(ctx, activeJourney)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// FindActiveByUserID should return the active journey
	found, err := repo.FindActiveByUserID(ctx, types.UserID(userID))
	if err != nil {
		t.Fatalf("FindActiveByUserID() error = %v", err)
	}

	if found.JourneyID() != activeJourney.JourneyID() {
		t.Errorf("JourneyID mismatch: got %v, want %v", found.JourneyID(), activeJourney.JourneyID())
	}
}

func TestJourneyRepository_FindActiveByUserID_ExcludesCompleted(t *testing.T) {
	repo, cleanup := setupJourneyTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user-completed-" + uuid.NewString()[:8]

	// Create a completed journey
	completedJourney, err := aggregates.RehydrateJourney(
		types.JourneyID("test-completed-"+uuid.NewString()[:8]),
		types.UserID(userID),
		types.GeoLocation{Latitude: -26.2041, Longitude: 28.0473},
		types.StopID("stop-origin"),
		types.StopID("stop-dest"),
		[]valueobjects.EnhancedBusRecommendation{},
		"",
		enums.JourneyStatusCompleted, // Terminal status
		enums.ProximityLevelNone,
		0,
		types.Direction(1),
		types.Duration(25*time.Minute),
		time.Now().Add(30*time.Minute),
		nil,
		nil,
		time.Now(),
		1, // version - initial version
		&testEventRecorder{},
	)
	if err != nil {
		t.Fatalf("failed to create completed journey: %v", err)
	}

	err = repo.Save(ctx, completedJourney)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// FindActiveByUserID should NOT return completed journey
	_, err = repo.FindActiveByUserID(ctx, types.UserID(userID))
	if err == nil {
		t.Fatal("expected error for user with only completed journey")
	}
	if err != repositories.ErrJourneyNotFound {
		t.Errorf("expected ErrJourneyNotFound, got %v", err)
	}
}

func TestJourneyRepository_CountActiveByUserID(t *testing.T) {
	repo, cleanup := setupJourneyTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user-count-" + uuid.NewString()[:8]

	// Initially no active journeys
	count, err := repo.CountActiveByUserID(ctx, types.UserID(userID))
	if err != nil {
		t.Fatalf("CountActiveByUserID() error = %v", err)
	}
	if count != 0 {
		t.Errorf("expected count=0, got %d", count)
	}

	// Create an active journey
	activeJourney, _ := aggregates.RehydrateJourney(
		types.JourneyID("test-count-"+uuid.NewString()[:8]),
		types.UserID(userID),
		types.GeoLocation{Latitude: -26.2041, Longitude: 28.0473},
		types.StopID("stop-origin"),
		types.StopID("stop-dest"),
		[]valueobjects.EnhancedBusRecommendation{},
		"",
		enums.JourneyStatusTracking, // Active status
		enums.ProximityLevelNone,
		0,
		types.Direction(1),
		types.Duration(25*time.Minute),
		time.Now().Add(30*time.Minute),
		nil,
		nil,
		time.Now(),
		1, // version - initial version
		&testEventRecorder{},
	)
	_ = repo.Save(ctx, activeJourney)

	// Now count should be 1
	count, err = repo.CountActiveByUserID(ctx, types.UserID(userID))
	if err != nil {
		t.Fatalf("CountActiveByUserID() error = %v", err)
	}
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}
}

func TestJourneyRepository_OptimisticLocking(t *testing.T) {
	repo, cleanup := setupJourneyTest(t)
	defer cleanup()

	ctx := context.Background()
	journey := createTestJourney(t, uuid.NewString()[:8])

	// First save should succeed (version 1 → 2 in DB)
	err := repo.Save(ctx, journey)
	if err != nil {
		t.Fatalf("First Save() error = %v", err)
	}

	// Load the journey (now properly loads version 2 from DB)
	loaded, err := repo.FindByID(ctx, journey.JourneyID())
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	// Second save with loaded journey should succeed
	// (version check: DB has version 2, loaded has version 2 - they match!)
	// This tests that optimistic locking allows save when versions match
	err = repo.Save(ctx, loaded)
	if err != nil {
		t.Fatalf("Second Save() should succeed, got error = %v", err)
	}

	// Load again to get version 3
	loaded2, err := repo.FindByID(ctx, journey.JourneyID())
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	// Try to save with the stale loaded (version 2), should fail
	// because DB now has version 3
	err = repo.Save(ctx, loaded)
	if err != repositories.ErrJourneyVersionStale {
		t.Errorf("expected ErrJourneyVersionStale for stale version, got %v", err)
	}

	// But saving loaded2 (version 3) should work
	err = repo.Save(ctx, loaded2)
	if err != nil {
		t.Fatalf("Save with current version should succeed, got error = %v", err)
	}
}

func TestJourneyRepository_NullableFields(t *testing.T) {
	repo, cleanup := setupJourneyTest(t)
	defer cleanup()

	ctx := context.Background()

	// Journey without boarding times (nil)
	journey, err := aggregates.RehydrateJourney(
		types.JourneyID("test-nullable-"+uuid.NewString()[:8]),
		types.UserID("user-nullable"),
		types.GeoLocation{Latitude: -26.2041, Longitude: 28.0473},
		types.StopID("stop-origin"),
		types.StopID("stop-dest"),
		[]valueobjects.EnhancedBusRecommendation{},
		"", // No active bus
		enums.JourneyStatusSearching,
		enums.ProximityLevelNone,
		0,
		types.Direction(1),
		types.Duration(25*time.Minute),
		time.Now().Add(30*time.Minute),
		nil, // No boarding window
		nil, // Not boarded
		time.Now(),
		1, // version - initial version
		&testEventRecorder{},
	)
	if err != nil {
		t.Fatalf("failed to create journey: %v", err)
	}

	// Save
	err = repo.Save(ctx, journey)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load and verify nullable fields
	found, err := repo.FindByID(ctx, journey.JourneyID())
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if found.ActiveBusID() != "" {
		t.Errorf("expected empty ActiveBusID, got %v", found.ActiveBusID())
	}
	if found.BoardingWindowStartedAt() != nil {
		t.Errorf("expected nil BoardingWindowStartedAt, got %v", found.BoardingWindowStartedAt())
	}
	if found.BoardedAt() != nil {
		t.Errorf("expected nil BoardedAt, got %v", found.BoardedAt())
	}
}
