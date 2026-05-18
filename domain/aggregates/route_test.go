package aggregates

import (
	"testing"
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/errors"
	"github.com/MathewM27/busTrack-alebus/domain/testhelpers"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
	"github.com/stretchr/testify/assert"
)

// --- Constructor Tests ---
func TestNewRoute(t *testing.T) {
	now := time.Now()
	later := now.Add(365 * 24 * time.Hour)
	recorder := &testhelpers.MockEventRecorder{}

	validStops := []valueobjects.Stop{
		{ID: "stop-1", Name: "First Stop", Location: types.GeoLocation{Latitude: 10, Longitude: 20}},
		{ID: "stop-2", Name: "Second Stop", Location: types.GeoLocation{Latitude: 11, Longitude: 21}},
	}

	tests := []struct {
		name        string
		routeID     types.RouteID
		operatorID  []types.OperatorID
		stops       []valueobjects.Stop
		routeName   string
		direction   enums.RouteDirection
		routeType   enums.RouteType
		activeFrom  time.Time
		activeUntil time.Time
		expectError error
	}{
		{
			name:        "valid route",
			routeID:     "route-1",
			operatorID:  []types.OperatorID{"op-1"},
			stops:       validStops,
			routeName:   "Test Route",
			direction:   enums.RouteDirectionBidirectional,
			routeType:   enums.RouteTypeUrban,
			activeFrom:  now,
			activeUntil: later,
			expectError: nil,
		},
		{
			name:        "missing route ID",
			routeID:     "",
			operatorID:  []types.OperatorID{"op-1"},
			stops:       validStops,
			routeName:   "Test Route",
			direction:   enums.RouteDirectionBidirectional,
			routeType:   enums.RouteTypeUrban,
			activeFrom:  now,
			activeUntil: later,
			expectError: errors.ErrRouteIdRequired,
		},
		{
			name:        "no operators",
			routeID:     "route-1",
			operatorID:  []types.OperatorID{},
			stops:       validStops,
			routeName:   "Test Route",
			direction:   enums.RouteDirectionBidirectional,
			routeType:   enums.RouteTypeUrban,
			activeFrom:  now,
			activeUntil: later,
			expectError: errors.ErrOperatorIdRequired,
		},
		{
			name:        "empty name",
			routeID:     "route-1",
			operatorID:  []types.OperatorID{"op-1"},
			stops:       validStops,
			routeName:   "",
			direction:   enums.RouteDirectionBidirectional,
			routeType:   enums.RouteTypeUrban,
			activeFrom:  now,
			activeUntil: later,
			expectError: errors.ErrRouteNameRequired,
		},
		{
			name:        "insufficient stops (only 1)",
			routeID:     "route-1",
			operatorID:  []types.OperatorID{"op-1"},
			stops:       []valueobjects.Stop{{ID: "stop-1", Name: "First Stop", Location: types.GeoLocation{Latitude: 10, Longitude: 20}}},
			routeName:   "Test Route",
			direction:   enums.RouteDirectionBidirectional,
			routeType:   enums.RouteTypeUrban,
			activeFrom:  now,
			activeUntil: later,
			expectError: errors.ErrInsufficientStops,
		},
		{
			name:        "invalid direction",
			routeID:     "route-1",
			operatorID:  []types.OperatorID{"op-1"},
			stops:       validStops,
			routeName:   "Test Route",
			direction:   99,
			routeType:   enums.RouteTypeUrban,
			activeFrom:  now,
			activeUntil: later,
			expectError: errors.ErrInvalidRouteDirection,
		},
		{
			name:        "invalid route type",
			routeID:     "route-1",
			operatorID:  []types.OperatorID{"op-1"},
			stops:       validStops,
			routeName:   "Test Route",
			direction:   enums.RouteDirectionBidirectional,
			routeType:   99,
			activeFrom:  now,
			activeUntil: later,
			expectError: errors.ErrInvalidRouteType,
		},
		{
			name:        "invalid active period (zero from)",
			routeID:     "route-1",
			operatorID:  []types.OperatorID{"op-1"},
			stops:       validStops,
			routeName:   "Test Route",
			direction:   enums.RouteDirectionBidirectional,
			routeType:   enums.RouteTypeUrban,
			activeFrom:  time.Time{},
			activeUntil: later,
			expectError: errors.ErrInvalidActivePeriod,
		},
		{
			name:        "invalid active period (until before from)",
			routeID:     "route-1",
			operatorID:  []types.OperatorID{"op-1"},
			stops:       validStops,
			routeName:   "Test Route",
			direction:   enums.RouteDirectionBidirectional,
			routeType:   enums.RouteTypeUrban,
			activeFrom:  later,
			activeUntil: now,
			expectError: errors.ErrInvalidActivePeriod,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			route, err := NewRoute(
				tc.routeID,
				tc.operatorID,
				tc.stops,
				tc.routeName,
				tc.direction,
				tc.routeType,
				tc.activeFrom,
				tc.activeUntil,
				now,
				recorder,
			)

			if tc.expectError != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tc.expectError)
				assert.Nil(t, route)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, route)
				assert.Equal(t, tc.routeID, route.ID())
				assert.Equal(t, tc.routeName, route.Name())
				assert.Equal(t, tc.direction, route.Direction())
				assert.Equal(t, tc.routeType, route.RouteType())
			}
		})
	}
}

// --- Invariant Enforcement Tests (Critical!) ---

func TestRoute_RemoveStop_EnforcesMinimumStops(t *testing.T) {
	now := time.Now()
	later := now.Add(365 * 24 * time.Hour)
	recorder := &testhelpers.MockEventRecorder{}

	stops := []valueobjects.Stop{
		{ID: "stop-1", Name: "Stop 1", Location: types.GeoLocation{Latitude: 10, Longitude: 20}},
		{ID: "stop-2", Name: "Stop 2", Location: types.GeoLocation{Latitude: 11, Longitude: 21}},
		{ID: "stop-3", Name: "Stop 3", Location: types.GeoLocation{Latitude: 12, Longitude: 22}},
	}

	route, _ := NewRoute("route-1", []types.OperatorID{"op-1"}, stops, "Test Route",
		enums.RouteDirectionBidirectional, enums.RouteTypeUrban, now, later, now, recorder)

	// Can remove when more than 2 stops
	err := route.RemoveStop("stop-3", "test removal")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(route.Stops()))

	// Cannot remove when only 2 stops remain
	err = route.RemoveStop("stop-2", "should fail")
	assert.Error(t, err)
	assert.ErrorIs(t, err, errors.ErrInsufficientStops)
	assert.Equal(t, 2, len(route.Stops()), "stops should not be removed")
}

func TestRoute_ChangeActivePeriod_EnforcesInvariants(t *testing.T) {
	now := time.Now()
	later := now.Add(365 * 24 * time.Hour)
	recorder := &testhelpers.MockEventRecorder{}

	stops := []valueobjects.Stop{
		{ID: "stop-1", Name: "Stop 1", Location: types.GeoLocation{Latitude: 10, Longitude: 20}},
		{ID: "stop-2", Name: "Stop 2", Location: types.GeoLocation{Latitude: 11, Longitude: 21}},
	}

	route, _ := NewRoute("route-1", []types.OperatorID{"op-1"}, stops, "Test Route",
		enums.RouteDirectionBidirectional, enums.RouteTypeUrban, now, later, now, recorder)

	tests := []struct {
		name        string
		newFrom     time.Time
		newUntil    time.Time
		expectError error
	}{
		{
			name:        "valid period change",
			newFrom:     now.Add(1 * time.Hour),
			newUntil:    later.Add(1 * time.Hour),
			expectError: nil,
		},
		{
			name:        "zero from time",
			newFrom:     time.Time{},
			newUntil:    later,
			expectError: errors.ErrInvalidActivePeriod,
		},
		{
			name:        "zero until time",
			newFrom:     now,
			newUntil:    time.Time{},
			expectError: errors.ErrInvalidActivePeriod,
		},
		{
			name:        "until before from",
			newFrom:     later,
			newUntil:    now,
			expectError: errors.ErrInvalidActivePeriod,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := route.ChangeActivePeriod(tc.newFrom, tc.newUntil, "test")
			if tc.expectError != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tc.expectError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRoute_ChangeRouteType_EnforcesInvariants(t *testing.T) {
	now := time.Now()
	later := now.Add(365 * 24 * time.Hour)
	recorder := &testhelpers.MockEventRecorder{}

	stops := []valueobjects.Stop{
		{ID: "stop-1", Name: "Stop 1", Location: types.GeoLocation{Latitude: 10, Longitude: 20}},
		{ID: "stop-2", Name: "Stop 2", Location: types.GeoLocation{Latitude: 11, Longitude: 21}},
	}

	route, _ := NewRoute("route-1", []types.OperatorID{"op-1"}, stops, "Test Route",
		enums.RouteDirectionBidirectional, enums.RouteTypeUrban, now, later, now, recorder)

	tests := []struct {
		name        string
		newType     enums.RouteType
		expectError error
	}{
		{
			name:        "valid change to highway",
			newType:     enums.RouteTypeHighway,
			expectError: nil,
		},
		{
			name:        "valid change to mixed",
			newType:     enums.RouteTypeMixed,
			expectError: nil,
		},
		{
			name:        "invalid type",
			newType:     99,
			expectError: errors.ErrInvalidRouteType,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := route.ChangeRouteType(tc.newType, "test")
			if tc.expectError != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tc.expectError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.newType, route.RouteType())
			}
		})
	}
}

func TestRoute_ChangeName_EnforcesInvariants(t *testing.T) {
	now := time.Now()
	later := now.Add(365 * 24 * time.Hour)
	recorder := &testhelpers.MockEventRecorder{}

	stops := []valueobjects.Stop{
		{ID: "stop-1", Name: "Stop 1", Location: types.GeoLocation{Latitude: 10, Longitude: 20}},
		{ID: "stop-2", Name: "Stop 2", Location: types.GeoLocation{Latitude: 11, Longitude: 21}},
	}

	route, _ := NewRoute("route-1", []types.OperatorID{"op-1"}, stops, "Test Route",
		enums.RouteDirectionBidirectional, enums.RouteTypeUrban, now, later, now, recorder)

	// Valid change
	err := route.ChangeName("New Route Name", "test")
	assert.NoError(t, err)
	assert.Equal(t, "New Route Name", route.Name())

	// Invalid: empty name
	err = route.ChangeName("", "should fail")
	assert.Error(t, err)
	assert.ErrorIs(t, err, errors.ErrRouteNameRequired)
	assert.Equal(t, "New Route Name", route.Name(), "name should not change")
}

// --- Business Logic Tests ---

func TestRoute_ValidateJourney(t *testing.T) {
	now := time.Now()
	later := now.Add(365 * 24 * time.Hour)
	recorder := &testhelpers.MockEventRecorder{}

	stops := []valueobjects.Stop{
		{ID: "A", Name: "Stop A", Location: types.GeoLocation{Latitude: 10, Longitude: 20}},
		{ID: "B", Name: "Stop B", Location: types.GeoLocation{Latitude: 11, Longitude: 21}},
		{ID: "C", Name: "Stop C", Location: types.GeoLocation{Latitude: 12, Longitude: 22}},
		{ID: "D", Name: "Stop D", Location: types.GeoLocation{Latitude: 13, Longitude: 23}},
	}

	t.Run("bidirectional route", func(t *testing.T) {
		route, _ := NewRoute("route-1", []types.OperatorID{"op-1"}, stops, "Bidirectional Route",
			enums.RouteDirectionBidirectional, enums.RouteTypeUrban, now, later, now, recorder)

		tests := []struct {
			name        string
			origin      types.StopID
			destination types.StopID
			direction   enums.Direction
			expectError bool
		}{
			{
				name:        "outbound A→D valid",
				origin:      "A",
				destination: "D",
				direction:   enums.DirectionOutbound,
				expectError: false,
			},
			{
				name:        "inbound D→A valid",
				origin:      "D",
				destination: "A",
				direction:   enums.DirectionInbound,
				expectError: false,
			},
			{
				name:        "outbound D→A invalid (wrong direction)",
				origin:      "D",
				destination: "A",
				direction:   enums.DirectionOutbound,
				expectError: true,
			},
			{
				name:        "invalid origin stop",
				origin:      "X",
				destination: "D",
				direction:   enums.DirectionOutbound,
				expectError: true,
			},
			{
				name:        "same stop",
				origin:      "B",
				destination: "B",
				direction:   enums.DirectionOutbound,
				expectError: true,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				originIdx, destIdx, err := route.ValidateJourney(tc.origin, tc.destination, tc.direction)
				if tc.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.GreaterOrEqual(t, originIdx, 0)
					assert.GreaterOrEqual(t, destIdx, 0)
				}
			})
		}
	})

	t.Run("unidirectional route", func(t *testing.T) {
		route, _ := NewRoute("route-1", []types.OperatorID{"op-1"}, stops, "Unidirectional Route",
			enums.RouteDirectionUnidirectional, enums.RouteTypeUrban, now, later, now, recorder)

		tests := []struct {
			name        string
			origin      types.StopID
			destination types.StopID
			direction   enums.Direction
			expectError bool
		}{
			{
				name:        "outbound A→D valid",
				origin:      "A",
				destination: "D",
				direction:   enums.DirectionOutbound,
				expectError: false,
			},
			{
				name:        "inbound not allowed",
				origin:      "D",
				destination: "A",
				direction:   enums.DirectionInbound,
				expectError: true,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				_, _, err := route.ValidateJourney(tc.origin, tc.destination, tc.direction)
				if tc.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestRoute_IsActive(t *testing.T) {
	now := time.Now()
	activeFrom := now.Add(-1 * time.Hour)
	activeUntil := now.Add(1 * time.Hour)
	recorder := &testhelpers.MockEventRecorder{}

	stops := []valueobjects.Stop{
		{ID: "stop-1", Name: "Stop 1", Location: types.GeoLocation{Latitude: 10, Longitude: 20}},
		{ID: "stop-2", Name: "Stop 2", Location: types.GeoLocation{Latitude: 11, Longitude: 21}},
	}

	route, _ := NewRoute("route-1", []types.OperatorID{"op-1"}, stops, "Test Route",
		enums.RouteDirectionBidirectional, enums.RouteTypeUrban, activeFrom, activeUntil, now, recorder)

	tests := []struct {
		name       string
		checkTime  time.Time
		wantActive bool
	}{
		{
			name:       "within active period",
			checkTime:  now,
			wantActive: true,
		},
		{
			name:       "before active period",
			checkTime:  activeFrom.Add(-1 * time.Minute),
			wantActive: false,
		},
		{
			name:       "after active period",
			checkTime:  activeUntil.Add(1 * time.Minute),
			wantActive: false,
		},
		{
			name:       "at exact start (exclusive)",
			checkTime:  activeFrom,
			wantActive: false,
		},
		{
			name:       "at exact end (exclusive)",
			checkTime:  activeUntil,
			wantActive: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isActive := route.IsActive(tc.checkTime)
			assert.Equal(t, tc.wantActive, isActive)
		})
	}
}

func TestRoute_ApproximateDistanceBetweenStops(t *testing.T) {
	now := time.Now()
	later := now.Add(365 * 24 * time.Hour)
	recorder := &testhelpers.MockEventRecorder{}

	stops := []valueobjects.Stop{
		{ID: "stop-0", Name: "Stop 0", Location: types.GeoLocation{Latitude: 0, Longitude: 0}},
		{ID: "stop-1", Name: "Stop 1", Location: types.GeoLocation{Latitude: 1, Longitude: 1}},
		{ID: "stop-2", Name: "Stop 2", Location: types.GeoLocation{Latitude: 2, Longitude: 2}},
	}

	route, _ := NewRoute("route-1", []types.OperatorID{"op-1"}, stops, "Test Route",
		enums.RouteDirectionBidirectional, enums.RouteTypeUrban, now, later, now, recorder)

	tests := []struct {
		name      string
		fromIdx   int
		toIdx     int
		wantZero  bool
		wantError bool
	}{
		{
			name:     "valid indices",
			fromIdx:  0,
			toIdx:    2,
			wantZero: false,
		},
		{
			name:     "same index",
			fromIdx:  1,
			toIdx:    1,
			wantZero: true,
		},
		{
			name:     "invalid negative index",
			fromIdx:  -1,
			toIdx:    2,
			wantZero: true,
		},
		{
			name:     "invalid out of range",
			fromIdx:  0,
			toIdx:    10,
			wantZero: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			distance := route.ApproximateDistanceBetweenStops(tc.fromIdx, tc.toIdx)
			if tc.wantZero {
				assert.Equal(t, types.Distance(0), distance)
			} else {
				assert.Greater(t, float64(distance), 0.0)
			}
		})
	}
}
