package ports

import (
	"testing"
	"time"
)

func TestLiveBusUpdate_Validate(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	validUpdate := func() LiveBusUpdate {
		return LiveBusUpdate{
			BusID:        "bus-123",
			RouteID:      "route-456",
			Direction:    0,
			StopIndex:    5,
			StopID:       "STOP-005",
			IsAtTerminal: false,
			Lat:          1.3521,
			Lon:          103.8198,
			SpeedKmh:     30.5,
			Status:       "active",
			Timestamp:    now.Add(-1 * time.Second),
		}
	}

	t.Run("Valid update passes", func(t *testing.T) {
		u := validUpdate()
		if err := u.Validate(now); err != nil {
			t.Errorf("expected valid update, got error: %v", err)
		}
	})

	t.Run("Empty BusID fails", func(t *testing.T) {
		u := validUpdate()
		u.BusID = ""
		err := u.Validate(now)
		if err == nil {
			t.Fatal("expected error for empty BusID")
		}
		if ve, ok := err.(*ValidationError); !ok || ve.Field != "BusID" {
			t.Errorf("expected BusID validation error, got: %v", err)
		}
	})

	t.Run("Empty RouteID fails", func(t *testing.T) {
		u := validUpdate()
		u.RouteID = ""
		err := u.Validate(now)
		if err == nil {
			t.Fatal("expected error for empty RouteID")
		}
		if ve, ok := err.(*ValidationError); !ok || ve.Field != "RouteID" {
			t.Errorf("expected RouteID validation error, got: %v", err)
		}
	})

	t.Run("Invalid Direction fails", func(t *testing.T) {
		u := validUpdate()
		u.Direction = 2
		err := u.Validate(now)
		if err == nil {
			t.Fatal("expected error for invalid Direction")
		}
		if ve, ok := err.(*ValidationError); !ok || ve.Field != "Direction" {
			t.Errorf("expected Direction validation error, got: %v", err)
		}
	})

	t.Run("Negative StopIndex fails", func(t *testing.T) {
		u := validUpdate()
		u.StopIndex = -1
		err := u.Validate(now)
		if err == nil {
			t.Fatal("expected error for negative StopIndex")
		}
		if ve, ok := err.(*ValidationError); !ok || ve.Field != "StopIndex" {
			t.Errorf("expected StopIndex validation error, got: %v", err)
		}
	})

	t.Run("Lat out of range fails", func(t *testing.T) {
		u := validUpdate()
		u.Lat = 91.0
		err := u.Validate(now)
		if err == nil {
			t.Fatal("expected error for out-of-range Lat")
		}
		if ve, ok := err.(*ValidationError); !ok || ve.Field != "Lat" {
			t.Errorf("expected Lat validation error, got: %v", err)
		}
	})

	t.Run("Lon out of range fails", func(t *testing.T) {
		u := validUpdate()
		u.Lon = -181.0
		err := u.Validate(now)
		if err == nil {
			t.Fatal("expected error for out-of-range Lon")
		}
		if ve, ok := err.(*ValidationError); !ok || ve.Field != "Lon" {
			t.Errorf("expected Lon validation error, got: %v", err)
		}
	})

	t.Run("Negative SpeedKmh fails", func(t *testing.T) {
		u := validUpdate()
		u.SpeedKmh = -5.0
		err := u.Validate(now)
		if err == nil {
			t.Fatal("expected error for negative SpeedKmh")
		}
		if ve, ok := err.(*ValidationError); !ok || ve.Field != "SpeedKmh" {
			t.Errorf("expected SpeedKmh validation error, got: %v", err)
		}
	})

	t.Run("Invalid Status fails", func(t *testing.T) {
		u := validUpdate()
		u.Status = "unknown"
		err := u.Validate(now)
		if err == nil {
			t.Fatal("expected error for invalid Status")
		}
		if ve, ok := err.(*ValidationError); !ok || ve.Field != "Status" {
			t.Errorf("expected Status validation error, got: %v", err)
		}
	})

	t.Run("Zero Timestamp fails", func(t *testing.T) {
		u := validUpdate()
		u.Timestamp = time.Time{}
		err := u.Validate(now)
		if err == nil {
			t.Fatal("expected error for zero Timestamp")
		}
		if ve, ok := err.(*ValidationError); !ok || ve.Field != "Timestamp" {
			t.Errorf("expected Timestamp validation error, got: %v", err)
		}
	})

	t.Run("Future Timestamp fails", func(t *testing.T) {
		u := validUpdate()
		u.Timestamp = now.Add(1 * time.Minute) // 1 minute in future
		err := u.Validate(now)
		if err == nil {
			t.Fatal("expected error for future Timestamp")
		}
		if ve, ok := err.(*ValidationError); !ok || ve.Field != "Timestamp" {
			t.Errorf("expected Timestamp validation error, got: %v", err)
		}
	})

	t.Run("Valid Status values", func(t *testing.T) {
		statuses := []string{"active", "inactive", "delayed"}
		for _, status := range statuses {
			u := validUpdate()
			u.Status = status
			if err := u.Validate(now); err != nil {
				t.Errorf("expected valid for status '%s', got error: %v", status, err)
			}
		}
	})

	t.Run("Direction 1 is valid", func(t *testing.T) {
		u := validUpdate()
		u.Direction = 1
		if err := u.Validate(now); err != nil {
			t.Errorf("expected valid for Direction=1, got error: %v", err)
		}
	})

	t.Run("Boundary Lat values", func(t *testing.T) {
		// Valid boundaries
		for _, lat := range []float64{-90.0, 0, 90.0} {
			u := validUpdate()
			u.Lat = lat
			if err := u.Validate(now); err != nil {
				t.Errorf("expected valid for Lat=%f, got error: %v", lat, err)
			}
		}
	})

	t.Run("Boundary Lon values", func(t *testing.T) {
		// Valid boundaries
		for _, lon := range []float64{-180.0, 0, 180.0} {
			u := validUpdate()
			u.Lon = lon
			if err := u.Validate(now); err != nil {
				t.Errorf("expected valid for Lon=%f, got error: %v", lon, err)
			}
		}
	})
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{Field: "TestField", Reason: "test reason"}
	expected := "invalid TestField: test reason"
	if err.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, err.Error())
	}
}
