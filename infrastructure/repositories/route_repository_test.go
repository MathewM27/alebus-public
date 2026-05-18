package repositories_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
	"github.com/MathewM27/busTrack-alebus/infrastructure/db"
	"github.com/MathewM27/busTrack-alebus/infrastructure/repositories"
)

// testEventRecorder is a simple event recorder for testing
type testEventRecorder struct {
	events []types.DomainEvent
}

func (r *testEventRecorder) Record(event types.DomainEvent) error {
	r.events = append(r.events, event)
	return nil
}

// getTestDatabaseURL returns the database URL for testing
func getTestDatabaseURL() string {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		// Default for local Docker setup
		url = "postgres://alebus:alebus@localhost:5432/alebus?sslmode=disable"
	}
	return url
}

// setupTestPool creates a database pool for testing
func setupTestPool(t *testing.T) *db.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := db.NewPoolFromURL(ctx, getTestDatabaseURL())
	if err != nil {
		t.Skipf("Skipping test: unable to connect to database: %v", err)
	}

	return pool
}

// cleanupRoutes removes test routes from the database
func cleanupRoutes(t *testing.T, pool *db.Pool, routeIDs ...string) {
	t.Helper()
	ctx := context.Background()

	for _, id := range routeIDs {
		_, _ = pool.Exec(ctx, "DELETE FROM routes WHERE route_id = $1", id)
	}
}

// createTestRoute creates a Route aggregate for testing
func createTestRoute(t *testing.T, routeID string) *aggregates.Route {
	t.Helper()

	stops := []valueobjects.Stop{
		{
			ID:   types.StopID("stop-1"),
			Name: "Central Station",
			Location: types.GeoLocation{
				Latitude:  -1.2921,
				Longitude: 36.8219,
			},
		},
		{
			ID:   types.StopID("stop-2"),
			Name: "Market Square",
			Location: types.GeoLocation{
				Latitude:  -1.2850,
				Longitude: 36.8250,
			},
		},
		{
			ID:   types.StopID("stop-3"),
			Name: "University",
			Location: types.GeoLocation{
				Latitude:  -1.2780,
				Longitude: 36.8180,
			},
		},
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	activeFrom := now.Add(-24 * time.Hour)
	activeUntil := now.Add(365 * 24 * time.Hour)

	recorder := &testEventRecorder{}

	route, err := aggregates.NewRoute(
		types.RouteID(routeID),
		[]types.OperatorID{"operator-1", "operator-2"},
		stops,
		"Test Route "+routeID,
		enums.RouteDirectionBidirectional,
		enums.RouteTypeUrban,
		activeFrom,
		activeUntil,
		now,
		recorder,
	)
	if err != nil {
		t.Fatalf("Failed to create test route: %v", err)
	}

	return route
}

func TestPostgresRouteRepository_Save_NewRoute(t *testing.T) {
	pool := setupTestPool(t)
	defer pool.Close()

	repo := repositories.NewPostgresRouteRepository(pool)
	routeID := "test-route-save-new-" + time.Now().Format("20060102150405")

	// Cleanup before and after
	cleanupRoutes(t, pool, routeID)
	defer cleanupRoutes(t, pool, routeID)

	route := createTestRoute(t, routeID)

	// Save the route
	err := repo.Save(context.Background(), route)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify by loading
	loaded, err := repo.FindByID(context.Background(), types.RouteID(routeID))
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	if loaded.ID() != route.ID() {
		t.Errorf("ID mismatch: got %v, want %v", loaded.ID(), route.ID())
	}
	if loaded.Name() != route.Name() {
		t.Errorf("Name mismatch: got %v, want %v", loaded.Name(), route.Name())
	}
}

func TestPostgresRouteRepository_FindByID_NotFound(t *testing.T) {
	pool := setupTestPool(t)
	defer pool.Close()

	repo := repositories.NewPostgresRouteRepository(pool)

	_, err := repo.FindByID(context.Background(), types.RouteID("non-existent-route"))
	if err == nil {
		t.Error("Expected error for non-existent route, got nil")
	}
	if err != repositories.ErrRouteNotFound {
		t.Errorf("Expected ErrRouteNotFound, got: %v", err)
	}
}

func TestPostgresRouteRepository_RoundTrip_AllFields(t *testing.T) {
	pool := setupTestPool(t)
	defer pool.Close()

	repo := repositories.NewPostgresRouteRepository(pool)
	routeID := "test-route-roundtrip-" + time.Now().Format("20060102150405")

	cleanupRoutes(t, pool, routeID)
	defer cleanupRoutes(t, pool, routeID)

	original := createTestRoute(t, routeID)
	originalVersion := original.Version()

	// Save (version will be incremented to original+1)
	if err := repo.Save(context.Background(), original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load
	loaded, err := repo.FindByID(context.Background(), types.RouteID(routeID))
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	// Verify all fields
	if loaded.ID() != original.ID() {
		t.Errorf("ID mismatch: got %v, want %v", loaded.ID(), original.ID())
	}
	if loaded.Name() != original.Name() {
		t.Errorf("Name mismatch: got %v, want %v", loaded.Name(), original.Name())
	}
	if loaded.Direction() != original.Direction() {
		t.Errorf("Direction mismatch: got %v, want %v", loaded.Direction(), original.Direction())
	}
	if loaded.RouteType() != original.RouteType() {
		t.Errorf("RouteType mismatch: got %v, want %v", loaded.RouteType(), original.RouteType())
	}
	if loaded.Status() != original.Status() {
		t.Errorf("Status mismatch: got %v, want %v", loaded.Status(), original.Status())
	}
	// Version should be incremented after save
	expectedVersion := originalVersion + 1
	if loaded.Version() != expectedVersion {
		t.Errorf("Version mismatch: got %v, want %v", loaded.Version(), expectedVersion)
	}

	// Verify operator IDs
	if len(loaded.OperatorIDs()) != len(original.OperatorIDs()) {
		t.Errorf("OperatorIDs length mismatch: got %v, want %v", len(loaded.OperatorIDs()), len(original.OperatorIDs()))
	}
	for i, opID := range loaded.OperatorIDs() {
		if opID != original.OperatorIDs()[i] {
			t.Errorf("OperatorID[%d] mismatch: got %v, want %v", i, opID, original.OperatorIDs()[i])
		}
	}

	// Verify stops
	if len(loaded.Stops()) != len(original.Stops()) {
		t.Errorf("Stops length mismatch: got %v, want %v", len(loaded.Stops()), len(original.Stops()))
	}
	for i, stop := range loaded.Stops() {
		origStop := original.Stops()[i]
		if stop.ID != origStop.ID {
			t.Errorf("Stop[%d].ID mismatch: got %v, want %v", i, stop.ID, origStop.ID)
		}
		if stop.Name != origStop.Name {
			t.Errorf("Stop[%d].Name mismatch: got %v, want %v", i, stop.Name, origStop.Name)
		}
		if stop.Location.Latitude != origStop.Location.Latitude {
			t.Errorf("Stop[%d].Location.Lat mismatch: got %v, want %v", i, stop.Location.Latitude, origStop.Location.Latitude)
		}
		if stop.Location.Longitude != origStop.Location.Longitude {
			t.Errorf("Stop[%d].Location.Lon mismatch: got %v, want %v", i, stop.Location.Longitude, origStop.Location.Longitude)
		}
	}

	// Verify time fields (with tolerance for DB precision)
	if !loaded.ActiveFrom().Round(time.Second).Equal(original.ActiveFrom().Round(time.Second)) {
		t.Errorf("ActiveFrom mismatch: got %v, want %v", loaded.ActiveFrom(), original.ActiveFrom())
	}
	if !loaded.ActiveUntil().Round(time.Second).Equal(original.ActiveUntil().Round(time.Second)) {
		t.Errorf("ActiveUntil mismatch: got %v, want %v", loaded.ActiveUntil(), original.ActiveUntil())
	}
}

func TestPostgresRouteRepository_FindActiveRoutes(t *testing.T) {
	pool := setupTestPool(t)
	defer pool.Close()

	repo := repositories.NewPostgresRouteRepository(pool)
	now := time.Now().UTC()

	// Create test routes with different active periods
	activeRouteID := "test-route-active-" + time.Now().Format("20060102150405")
	inactiveRouteID := "test-route-inactive-" + time.Now().Format("20060102150405")
	expiredRouteID := "test-route-expired-" + time.Now().Format("20060102150405")

	cleanupRoutes(t, pool, activeRouteID, inactiveRouteID, expiredRouteID)
	defer cleanupRoutes(t, pool, activeRouteID, inactiveRouteID, expiredRouteID)

	// Create an active route (active now)
	activeRoute := createTestRoute(t, activeRouteID)
	if err := repo.Save(context.Background(), activeRoute); err != nil {
		t.Fatalf("Failed to save active route: %v", err)
	}

	// Create an expired route (ended yesterday)
	expiredStops := []valueobjects.Stop{
		{ID: types.StopID("exp-stop-1"), Name: "Exp Stop 1", Location: types.GeoLocation{Latitude: -1.29, Longitude: 36.82}},
		{ID: types.StopID("exp-stop-2"), Name: "Exp Stop 2", Location: types.GeoLocation{Latitude: -1.28, Longitude: 36.83}},
	}
	expiredRoute, _ := aggregates.NewRoute(
		types.RouteID(expiredRouteID),
		[]types.OperatorID{"op-1"},
		expiredStops,
		"Expired Route",
		enums.RouteDirectionBidirectional,
		enums.RouteTypeUrban,
		now.Add(-48*time.Hour), // Started 2 days ago
		now.Add(-24*time.Hour), // Ended yesterday
		now.Add(-48*time.Hour),
		&testEventRecorder{},
	)
	if err := repo.Save(context.Background(), expiredRoute); err != nil {
		t.Fatalf("Failed to save expired route: %v", err)
	}

	// Query active routes
	activeRoutes, err := repo.FindActiveRoutes(context.Background(), now)
	if err != nil {
		t.Fatalf("FindActiveRoutes failed: %v", err)
	}

	// Verify our active route is in the results
	found := false
	for _, r := range activeRoutes {
		if r.ID() == types.RouteID(activeRouteID) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Active route %s not found in results", activeRouteID)
	}

	// Verify expired route is NOT in results
	for _, r := range activeRoutes {
		if r.ID() == types.RouteID(expiredRouteID) {
			t.Errorf("Expired route %s should not be in active results", expiredRouteID)
		}
	}
}

func TestPostgresRouteRepository_OptimisticLocking(t *testing.T) {
	pool := setupTestPool(t)
	defer pool.Close()

	repo := repositories.NewPostgresRouteRepository(pool)
	routeID := "test-route-lock-" + time.Now().Format("20060102150405")

	cleanupRoutes(t, pool, routeID)
	defer cleanupRoutes(t, pool, routeID)

	// Create and save initial route
	route := createTestRoute(t, routeID)
	if err := repo.Save(context.Background(), route); err != nil {
		t.Fatalf("Initial save failed: %v", err)
	}

	// Load two copies
	copy1, err := repo.FindByID(context.Background(), types.RouteID(routeID))
	if err != nil {
		t.Fatalf("First load failed: %v", err)
	}

	copy2, err := repo.FindByID(context.Background(), types.RouteID(routeID))
	if err != nil {
		t.Fatalf("Second load failed: %v", err)
	}

	// Save first copy (should succeed)
	if err := repo.Save(context.Background(), copy1); err != nil {
		t.Fatalf("First save after load failed: %v", err)
	}

	// Save second copy (should fail due to stale version)
	err = repo.Save(context.Background(), copy2)
	if err == nil {
		t.Error("Expected version conflict error, got nil")
	}
	if err != repositories.ErrRouteVersionStale {
		t.Errorf("Expected ErrRouteVersionStale, got: %v", err)
	}
}

func TestPostgresRouteRepository_UpdateIncrementsVersion(t *testing.T) {
	pool := setupTestPool(t)
	defer pool.Close()

	repo := repositories.NewPostgresRouteRepository(pool)
	routeID := "test-route-version-" + time.Now().Format("20060102150405")

	cleanupRoutes(t, pool, routeID)
	defer cleanupRoutes(t, pool, routeID)

	// Create and save initial route
	route := createTestRoute(t, routeID)
	initialVersion := route.Version() // Should be 1 from NewRoute

	if err := repo.Save(context.Background(), route); err != nil {
		t.Fatalf("Initial save failed: %v", err)
	}

	// Load - version should be initialVersion + 1 (Save always increments)
	loaded, err := repo.FindByID(context.Background(), types.RouteID(routeID))
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	expectedAfterFirstSave := initialVersion + 1
	if loaded.Version() != expectedAfterFirstSave {
		t.Errorf("Version should be %d after first save, got %d", expectedAfterFirstSave, loaded.Version())
	}

	// Save again (simulating update)
	if err := repo.Save(context.Background(), loaded); err != nil {
		t.Fatalf("Second save failed: %v", err)
	}

	// Load again and check version incremented
	reloaded, err := repo.FindByID(context.Background(), types.RouteID(routeID))
	if err != nil {
		t.Fatalf("Second FindByID failed: %v", err)
	}

	expectedAfterSecondSave := expectedAfterFirstSave + 1
	if reloaded.Version() != expectedAfterSecondSave {
		t.Errorf("Version should be %d after second save, got %d", expectedAfterSecondSave, reloaded.Version())
	}
}
