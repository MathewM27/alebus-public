package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/infrastructure/repositories"
)

// createTestBus creates a Bus aggregate for testing
func createTestBus(t *testing.T, busID string) *aggregates.Bus {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Microsecond)

	position := types.PositionSnapshot{
		Location: types.GeoLocation{
			Latitude:  -1.2921,
			Longitude: 36.8219,
		},
		Timestamp: now,
		Accuracy:  5.0,
		SpeedKmh:  45.5,
	}

	recorder := &testEventRecorder{}

	bus, err := aggregates.NewBus(
		types.BusID(busID),
		types.OperatorID("operator-1"),
		types.RouteID("route-1"),
		position,
		0, // initialStopIndex
		enums.DirectionOutbound,
		now,
		recorder,
	)
	if err != nil {
		t.Fatalf("Failed to create test bus: %v", err)
	}

	return bus
}

func TestPostgresBusRepository_Save_NewBus(t *testing.T) {
	pool := setupTestPool(t)
	defer pool.Close()

	repo := repositories.NewPostgresBusRepository(pool)
	busID := "test-bus-save-new-" + time.Now().Format("20060102150405")

	// Cleanup before and after
	ctx := context.Background()
	_, _ = pool.Exec(ctx, "DELETE FROM buses WHERE bus_id = $1", busID)
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM buses WHERE bus_id = $1", busID)
	}()

	bus := createTestBus(t, busID)

	// Save the bus
	err := repo.Save(context.Background(), bus)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify by loading
	loaded, err := repo.FindByID(context.Background(), types.BusID(busID))
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	if loaded.ID() != bus.ID() {
		t.Errorf("ID mismatch: got %v, want %v", loaded.ID(), bus.ID())
	}
	if loaded.OperatorID() != bus.OperatorID() {
		t.Errorf("OperatorID mismatch: got %v, want %v", loaded.OperatorID(), bus.OperatorID())
	}
	if loaded.RouteID() != bus.RouteID() {
		t.Errorf("RouteID mismatch: got %v, want %v", loaded.RouteID(), bus.RouteID())
	}
}

func TestPostgresBusRepository_FindByID_NotFound(t *testing.T) {
	pool := setupTestPool(t)
	defer pool.Close()

	repo := repositories.NewPostgresBusRepository(pool)

	_, err := repo.FindByID(context.Background(), types.BusID("non-existent-bus"))
	if err == nil {
		t.Error("Expected error for non-existent bus, got nil")
	}
	if err != repositories.ErrBusNotFound {
		t.Errorf("Expected ErrBusNotFound, got: %v", err)
	}
}

func TestPostgresBusRepository_RoundTrip_AllFields(t *testing.T) {
	pool := setupTestPool(t)
	defer pool.Close()

	repo := repositories.NewPostgresBusRepository(pool)
	busID := "test-bus-roundtrip-" + time.Now().Format("20060102150405")

	ctx := context.Background()
	_, _ = pool.Exec(ctx, "DELETE FROM buses WHERE bus_id = $1", busID)
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM buses WHERE bus_id = $1", busID)
	}()

	original := createTestBus(t, busID)
	originalVersion := original.Version()

	// Save (version will be incremented per error.md)
	if err := repo.Save(context.Background(), original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load
	loaded, err := repo.FindByID(context.Background(), types.BusID(busID))
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	// Verify all fields
	if loaded.ID() != original.ID() {
		t.Errorf("ID mismatch: got %v, want %v", loaded.ID(), original.ID())
	}
	if loaded.OperatorID() != original.OperatorID() {
		t.Errorf("OperatorID mismatch: got %v, want %v", loaded.OperatorID(), original.OperatorID())
	}
	if loaded.RouteID() != original.RouteID() {
		t.Errorf("RouteID mismatch: got %v, want %v", loaded.RouteID(), original.RouteID())
	}
	if loaded.Direction() != original.Direction() {
		t.Errorf("Direction mismatch: got %v, want %v", loaded.Direction(), original.Direction())
	}
	if loaded.Status() != original.Status() {
		t.Errorf("Status mismatch: got %v, want %v", loaded.Status(), original.Status())
	}
	if loaded.StopIndex() != original.StopIndex() {
		t.Errorf("StopIndex mismatch: got %v, want %v", loaded.StopIndex(), original.StopIndex())
	}
	if loaded.IsAtTerminal() != original.IsAtTerminal() {
		t.Errorf("IsAtTerminal mismatch: got %v, want %v", loaded.IsAtTerminal(), original.IsAtTerminal())
	}

	// Version should be incremented after save (per error.md)
	expectedVersion := originalVersion + 1
	if loaded.Version() != expectedVersion {
		t.Errorf("Version mismatch: got %v, want %v", loaded.Version(), expectedVersion)
	}

	// Verify position snapshot
	loadedPos := loaded.Position()
	originalPos := original.Position()
	if loadedPos.Location.Latitude != originalPos.Location.Latitude {
		t.Errorf("Position.Lat mismatch: got %v, want %v", loadedPos.Location.Latitude, originalPos.Location.Latitude)
	}
	if loadedPos.Location.Longitude != originalPos.Location.Longitude {
		t.Errorf("Position.Lon mismatch: got %v, want %v", loadedPos.Location.Longitude, originalPos.Location.Longitude)
	}
	if loadedPos.Accuracy != originalPos.Accuracy {
		t.Errorf("Position.Accuracy mismatch: got %v, want %v", loadedPos.Accuracy, originalPos.Accuracy)
	}
	if loadedPos.SpeedKmh != originalPos.SpeedKmh {
		t.Errorf("Position.SpeedKmh mismatch: got %v, want %v", loadedPos.SpeedKmh, originalPos.SpeedKmh)
	}

	// Verify current speed (derived from position)
	if loaded.CurrentSpeed() != original.CurrentSpeed() {
		t.Errorf("CurrentSpeed mismatch: got %v, want %v", loaded.CurrentSpeed(), original.CurrentSpeed())
	}
}

func TestPostgresBusRepository_OptimisticLocking(t *testing.T) {
	pool := setupTestPool(t)
	defer pool.Close()

	repo := repositories.NewPostgresBusRepository(pool)
	busID := "test-bus-lock-" + time.Now().Format("20060102150405")

	ctx := context.Background()
	_, _ = pool.Exec(ctx, "DELETE FROM buses WHERE bus_id = $1", busID)
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM buses WHERE bus_id = $1", busID)
	}()

	// Create and save initial bus
	bus := createTestBus(t, busID)
	if err := repo.Save(context.Background(), bus); err != nil {
		t.Fatalf("Initial save failed: %v", err)
	}

	// Load two copies
	copy1, err := repo.FindByID(context.Background(), types.BusID(busID))
	if err != nil {
		t.Fatalf("First load failed: %v", err)
	}

	copy2, err := repo.FindByID(context.Background(), types.BusID(busID))
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
	if err != repositories.ErrBusVersionStale {
		t.Errorf("Expected ErrBusVersionStale, got: %v", err)
	}
}

func TestPostgresBusRepository_UpdateIncrementsVersion(t *testing.T) {
	pool := setupTestPool(t)
	defer pool.Close()

	repo := repositories.NewPostgresBusRepository(pool)
	busID := "test-bus-version-" + time.Now().Format("20060102150405")

	ctx := context.Background()
	_, _ = pool.Exec(ctx, "DELETE FROM buses WHERE bus_id = $1", busID)
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM buses WHERE bus_id = $1", busID)
	}()

	// Create and save initial bus
	bus := createTestBus(t, busID)
	initialVersion := bus.Version() // Should be 1 from NewBus

	if err := repo.Save(context.Background(), bus); err != nil {
		t.Fatalf("Initial save failed: %v", err)
	}

	// Load - version should be initialVersion + 1 (per error.md)
	loaded, err := repo.FindByID(context.Background(), types.BusID(busID))
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	expectedAfterFirstSave := initialVersion + 1
	if loaded.Version() != expectedAfterFirstSave {
		t.Errorf("Version should be %d after first save, got %d", expectedAfterFirstSave, loaded.Version())
	}

	// Save again
	if err := repo.Save(context.Background(), loaded); err != nil {
		t.Fatalf("Second save failed: %v", err)
	}

	// Load again and check version incremented
	reloaded, err := repo.FindByID(context.Background(), types.BusID(busID))
	if err != nil {
		t.Fatalf("Second FindByID failed: %v", err)
	}

	expectedAfterSecondSave := expectedAfterFirstSave + 1
	if reloaded.Version() != expectedAfterSecondSave {
		t.Errorf("Version should be %d after second save, got %d", expectedAfterSecondSave, reloaded.Version())
	}
}

func TestPostgresBusRepository_TerminalArrivalTime_Nullable(t *testing.T) {
	pool := setupTestPool(t)
	defer pool.Close()

	repo := repositories.NewPostgresBusRepository(pool)
	busID := "test-bus-terminal-" + time.Now().Format("20060102150405")

	ctx := context.Background()
	_, _ = pool.Exec(ctx, "DELETE FROM buses WHERE bus_id = $1", busID)
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM buses WHERE bus_id = $1", busID)
	}()

	// Create bus (terminal arrival time should be nil initially)
	bus := createTestBus(t, busID)

	if err := repo.Save(context.Background(), bus); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load and verify terminal arrival time is nil
	loaded, err := repo.FindByID(context.Background(), types.BusID(busID))
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	if loaded.TerminalArrivalTime() != nil {
		t.Errorf("TerminalArrivalTime should be nil, got %v", loaded.TerminalArrivalTime())
	}

	if loaded.IsAtTerminal() != false {
		t.Errorf("IsAtTerminal should be false, got %v", loaded.IsAtTerminal())
	}
}

func TestPostgresBusRepository_PositionSnapshot_Persistence(t *testing.T) {
	pool := setupTestPool(t)
	defer pool.Close()

	repo := repositories.NewPostgresBusRepository(pool)
	busID := "test-bus-position-" + time.Now().Format("20060102150405")

	ctx := context.Background()
	_, _ = pool.Exec(ctx, "DELETE FROM buses WHERE bus_id = $1", busID)
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM buses WHERE bus_id = $1", busID)
	}()

	// Create bus with specific position values
	now := time.Now().UTC().Truncate(time.Microsecond)
	position := types.PositionSnapshot{
		Location: types.GeoLocation{
			Latitude:  -1.2921,
			Longitude: 36.8219,
		},
		Timestamp: now,
		Accuracy:  10.5,
		SpeedKmh:  60.0,
	}

	recorder := &testEventRecorder{}
	bus, err := aggregates.NewBus(
		types.BusID(busID),
		types.OperatorID("op-pos-test"),
		types.RouteID("route-pos-test"),
		position,
		0, // initialStopIndex
		enums.DirectionInbound,
		now,
		recorder,
	)
	if err != nil {
		t.Fatalf("Failed to create bus: %v", err)
	}

	if err := repo.Save(context.Background(), bus); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load and verify all position fields
	loaded, err := repo.FindByID(context.Background(), types.BusID(busID))
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	loadedPos := loaded.Position()

	if loadedPos.Location.Latitude != position.Location.Latitude {
		t.Errorf("Latitude mismatch: got %v, want %v", loadedPos.Location.Latitude, position.Location.Latitude)
	}
	if loadedPos.Location.Longitude != position.Location.Longitude {
		t.Errorf("Longitude mismatch: got %v, want %v", loadedPos.Location.Longitude, position.Location.Longitude)
	}
	if loadedPos.Accuracy != position.Accuracy {
		t.Errorf("Accuracy mismatch: got %v, want %v", loadedPos.Accuracy, position.Accuracy)
	}
	if loadedPos.SpeedKmh != position.SpeedKmh {
		t.Errorf("SpeedKmh mismatch: got %v, want %v", loadedPos.SpeedKmh, position.SpeedKmh)
	}
	// Verify timestamp with some tolerance
	if !loadedPos.Timestamp.Round(time.Second).Equal(position.Timestamp.Round(time.Second)) {
		t.Errorf("Timestamp mismatch: got %v, want %v", loadedPos.Timestamp, position.Timestamp)
	}

	// Verify direction persisted correctly
	if loaded.Direction() != enums.DirectionInbound {
		t.Errorf("Direction mismatch: got %v, want %v", loaded.Direction(), enums.DirectionInbound)
	}
}
