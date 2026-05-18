package aggregates

import (
	"context"
	"fmt"
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/errors"
	"github.com/MathewM27/busTrack-alebus/domain/events"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
)

func (r *Route) findStopIndex(stopId types.StopID) int {
	for i, stop := range r.stops {
		if stop.ID == stopId {
			return i
		}
	}
	return -1
}

// ApproximateDistanceBetweenStops estimates the route distance between two stops using GPS and a detour factor.
// Applies a stop density factor for more realistic approximation.
func (r *Route) ApproximateDistanceBetweenStops(fromStopIndex, toStopIndex int) types.Distance {
	if fromStopIndex == toStopIndex {
		return 0
	}
	// Validate indices
	if fromStopIndex < 0 || toStopIndex < 0 || fromStopIndex >= len(r.stops) || toStopIndex >= len(r.stops) {
		return 0
	}

	fromStop := r.stops[fromStopIndex]
	toStop := r.stops[toStopIndex]

	// Get straight-line (Haversine) distance
	straightLine := fromStop.Location.DistanceTo(toStop.Location)

	// Apply a detour factor (e.g., 1.3 for mixed, 1.4 for urban, 1.2 for highway)
	detourFactor := r.avgDetourRate
	if detourFactor == 0 {
		detourFactor = 1.3 // default detour factor
	}

	// Apply stop density factor (3% extra per intermediate stop)
	stopCount := abs(toStopIndex - fromStopIndex)
	stopDensity := 1.0 + (float64(stopCount) * 0.03)

	return types.Distance(float64(straightLine) * detourFactor * stopDensity)
}

// abs is a helper for absolute value of an int.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ApproximateDistanceToTerminal estimates the route distance from a given stop index to the terminal stop,
// based on the direction.
func (r *Route) ApproximateDistanceToTerminal(fromStopIndex int, direction enums.Direction) types.Distance {
	stops := r.stops
	if len(stops) == 0 {
		return 0
	}
	var terminalIndex int
	if direction == enums.DirectionOutbound {
		terminalIndex = len(stops) - 1
	} else {
		terminalIndex = 0
	}
	return r.ApproximateDistanceBetweenStops(fromStopIndex, terminalIndex)
}

func (r *Route) isValidDirection(originIndex, destIndex int, direction enums.Direction) bool {
	switch r.direction {
	case enums.RouteDirectionUnidirectional:
		// Only allow outbound (A→Z), which means origin must come before destination in route
		return direction == enums.DirectionOutbound && originIndex < destIndex
	case enums.RouteDirectionBidirectional:
		// For bidirectional routes, allow travel in either direction as long as both stops exist
		// Outbound: origin comes before destination in the route sequence
		// Inbound: origin comes after destination in the route sequence
		if direction == enums.DirectionOutbound {
			return originIndex < destIndex
		}
		if direction == enums.DirectionInbound {
			return originIndex > destIndex
		}
	}
	return false
}

func (r *Route) ValidateJourney(originStopID, destinationStopID types.StopID, direction enums.Direction) (int, int, error) {
	originIndex := r.findStopIndex(originStopID)
	if originIndex == -1 {
		return -1, -1, errors.ErrInvalidOriginStop
	}
	destinationIndex := r.findStopIndex(destinationStopID)
	if destinationIndex == -1 {
		return -1, -1, errors.ErrInvalidDestinationStop
	}
	if !r.isValidDirection(originIndex, destinationIndex, direction) {
		if r.direction == enums.RouteDirectionUnidirectional {
			return -1, -1, fmt.Errorf("route is unidirectional: only outbound journeys (A→Z) are allowed")
		}
		if r.direction == enums.RouteDirectionBidirectional {
			return -1, -1, fmt.Errorf("invalid journey direction for this route (must be outbound or inbound as per stops order)")
		}
		return -1, -1, errors.ErrInvalidStopSequence
	}
	return originIndex, destinationIndex, nil
}

func (r *Route) IsActive(currentTime time.Time) bool {
	return r.status == enums.RouteStatusActive &&
		currentTime.After(r.activeFrom) && currentTime.Before(r.activeUntil)
}

// CalculateDistanceBetweenStops calculates the distance between two stops using the provided distance calculator.
// The calculator (if provided) implements the fallback strategy at the infrastructure layer.
//
// NOTE: As of Dec 2024, no external distance calculator is wired in the application.
// When calculator is nil or returns an error, this method falls back to ApproximateDistanceBetweenStops
// which uses Haversine distance with detour and stop-density factors.
func (r *Route) CalculateDistanceBetweenStops(
	ctx context.Context,
	calculator types.RouteDistanceCalculator,
	fromStopIndex, toStopIndex int,
) types.Distance {
	if fromStopIndex == toStopIndex {
		return 0
	}
	if fromStopIndex < 0 || toStopIndex < 0 || fromStopIndex >= len(r.stops) || toStopIndex >= len(r.stops) {
		return 0
	}
	fromStop := r.stops[fromStopIndex]
	toStop := r.stops[toStopIndex]

	// Use the calculator port - orchestration happens in infrastructure
	if calculator != nil {
		dist, err := calculator.CalculateDistance(ctx, fromStop.Location, toStop.Location)
		if err == nil && dist > 0 {
			return dist
		}
	}

	// Fallback to domain's own approximation logic
	return r.ApproximateDistanceBetweenStops(fromStopIndex, toStopIndex)
}

func (r *Route) setRouteCharacteristics(routeType enums.RouteType) {
	switch routeType {
	case enums.RouteTypeUrban:
		r.avgDetourRate = 1.4
	case enums.RouteTypeHighway:
		r.avgDetourRate = 1.2
	case enums.RouteTypeMixed:
		r.avgDetourRate = 1.3
	default:
		r.avgDetourRate = 1.3
	}
}

func (r *Route) ChangeStatus(newStatus enums.RouteStatus, reason string) error {
	if r.status == newStatus {
		return nil // No change needed
	}
	oldStatus := r.status
	r.status = newStatus

	// Record event if eventRecorder is set
	if r.eventRecorder != nil {
		event := events.RouteStatusChanged{
			EventIDValue:   fmt.Sprintf("route-status-%d", time.Now().UnixNano()),
			RouteID:        r.routeID,
			OldStatus:      oldStatus,
			NewStatus:      newStatus,
			Reason:         reason,
			OccurredAtTime: time.Now(),
		}
		_ = r.eventRecorder.Record(event)
	}
	return nil
}

func (r *Route) UpdateStops(newStops []valueobjects.Stop, reason string) error {

	// ensure atlest 2 stops
	if len(newStops) < 2 {
		return errors.ErrInsufficientStops
	}
	oldStops := r.stops
	r.stops = newStops

	// Prepare old and new stop IDs for the event
	oldStopIDs := make([]types.StopID, len(oldStops))
	for i, s := range oldStops {
		oldStopIDs[i] = s.ID
	}
	newStopIDs := make([]types.StopID, len(newStops))
	for i, s := range newStops {
		newStopIDs[i] = s.ID
	}

	// Record event if eventRecorder is set
	if r.eventRecorder != nil {
		event := events.RouteStopsUpdated{
			EventIDValue:   fmt.Sprintf("route-stops-%d", time.Now().UnixNano()),
			RouteID:        r.routeID,
			OldStops:       oldStopIDs,
			NewStops:       newStopIDs,
			Reason:         reason,
			OccurredAtTime: time.Now(),
		}
		_ = r.eventRecorder.Record(event)
	}
	return nil
}

func (r *Route) ChangeRouteType(newType enums.RouteType, reason string) error {
	if r.routeType == newType {
		return nil // No change needed
	}
	// Enforce invariant: validate route type (same as constructor)
	if newType != enums.RouteTypeUrban && newType != enums.RouteTypeHighway && newType != enums.RouteTypeMixed {
		return errors.ErrInvalidRouteType
	}
	oldType := r.routeType
	r.routeType = newType
	r.setRouteCharacteristics(newType)

	// Record event if eventRecorder is set
	if r.eventRecorder != nil {
		event := events.RouteTypeChanged{
			EventIDValue:   fmt.Sprintf("route-type-%d", time.Now().UnixNano()),
			RouteID:        r.routeID,
			OldType:        oldType,
			NewType:        newType,
			Reason:         reason,
			OccurredAtTime: time.Now(),
		}
		_ = r.eventRecorder.Record(event)
	}
	return nil
}

func (r *Route) ChangeActivePeriod(newActiveFrom, newActiveUntil time.Time, reason string) error {
	if r.activeFrom.Equal(newActiveFrom) && r.activeUntil.Equal(newActiveUntil) {
		return nil // No change needed
	}
	// Enforce invariant: valid active period (same as constructor)
	if newActiveFrom.IsZero() || newActiveUntil.IsZero() || !newActiveUntil.After(newActiveFrom) {
		return errors.ErrInvalidActivePeriod
	}
	oldActiveFrom := r.activeFrom
	oldActiveUntil := r.activeUntil
	r.activeFrom = newActiveFrom
	r.activeUntil = newActiveUntil

	// Record event if eventRecorder is set
	if r.eventRecorder != nil {
		event := events.RouteActivePeriodChanged{
			EventIDValue:   fmt.Sprintf("route-activeperiod-%d", time.Now().UnixNano()),
			RouteID:        r.routeID,
			OldActiveFrom:  oldActiveFrom,
			OldActiveUntil: oldActiveUntil,
			NewActiveFrom:  newActiveFrom,
			NewActiveUntil: newActiveUntil,
			Reason:         reason,
			OccurredAtTime: time.Now(),
		}
		_ = r.eventRecorder.Record(event)
	}
	return nil
}

func (r *Route) ChangeName(newName string, reason string) error {
	if r.name == newName {
		return nil // No change needed
	}
	// Enforce invariant: name must be non-empty (same as constructor)
	if newName == "" {
		return errors.ErrRouteNameRequired
	}
	oldName := r.name
	r.name = newName

	// Record event if eventRecorder is set
	if r.eventRecorder != nil {
		event := events.RouteNameChanged{
			EventIDValue:   fmt.Sprintf("route-name-%d", time.Now().UnixNano()),
			RouteID:        r.routeID,
			OldName:        oldName,
			NewName:        newName,
			Reason:         reason,
			OccurredAtTime: time.Now(),
		}
		_ = r.eventRecorder.Record(event)
	}
	return nil
}

// AddStop inserts a new stop at the specified position (0 = start, len(stops) = end).
func (r *Route) AddStop(newStop valueobjects.Stop, position int, reason string) error {
	if position < 0 || position > len(r.stops) {
		return fmt.Errorf("invalid position for adding stop")
	}
	// Capture old stop IDs before change
	oldStopIDs := make([]types.StopID, len(r.stops))
	for i, s := range r.stops {
		oldStopIDs[i] = s.ID
	}
	// Insert the new stop
	r.stops = append(r.stops[:position], append([]valueobjects.Stop{newStop}, r.stops[position:]...)...)
	// Prepare new stop IDs after change
	newStopIDs := make([]types.StopID, len(r.stops))
	for i, s := range r.stops {
		newStopIDs[i] = s.ID
	}
	// Record event if eventRecorder is set
	if r.eventRecorder != nil {
		event := events.RouteStopsUpdated{
			EventIDValue:   fmt.Sprintf("route-stops-add-%d", time.Now().UnixNano()),
			RouteID:        r.routeID,
			OldStops:       oldStopIDs,
			NewStops:       newStopIDs,
			Reason:         fmt.Sprintf("Added stop %s: %s", newStop.ID, reason),
			OccurredAtTime: time.Now(),
		}
		_ = r.eventRecorder.Record(event)
	}
	return nil
}

// RemoveStop removes the stop with the given stopID.
func (r *Route) RemoveStop(stopID types.StopID, reason string) error {
	index := -1
	for i, s := range r.stops {
		if s.ID == stopID {
			index = i
			break
		}
	}
	if index == -1 {
		return fmt.Errorf("stop ID %s not found", stopID)
	}
	// Enforce invariant: route must have at least 2 stops
	if len(r.stops) <= 2 {
		return errors.ErrInsufficientStops
	}
	// Capture old stop IDs before change
	oldStopIDs := make([]types.StopID, len(r.stops))
	for i, s := range r.stops {
		oldStopIDs[i] = s.ID
	}
	// Remove the stop
	r.stops = append(r.stops[:index], r.stops[index+1:]...)
	// Prepare new stop IDs after change
	newStopIDs := make([]types.StopID, len(r.stops))
	for i, s := range r.stops {
		newStopIDs[i] = s.ID
	}
	// Record event if eventRecorder is set
	if r.eventRecorder != nil {
		event := events.RouteStopsUpdated{
			EventIDValue:   fmt.Sprintf("route-stops-remove-%d", time.Now().UnixNano()),
			RouteID:        r.routeID,
			OldStops:       oldStopIDs,
			NewStops:       newStopIDs,
			Reason:         fmt.Sprintf("Removed stop %s: %s", stopID, reason),
			OccurredAtTime: time.Now(),
		}
		_ = r.eventRecorder.Record(event)
	}
	return nil
}

func (r *Route) FindStopIndex(stopID types.StopID) int {
	return r.findStopIndex(stopID)
}

func (r *Route) HasStop(stopID types.StopID) bool {
	return r.findStopIndex(stopID) != -1
}

func (r *Route) CompareStops(a, b types.StopID) (int, error) {
	aIdx := r.findStopIndex(a)
	bIdx := r.findStopIndex(b)
	if aIdx == -1 || bIdx == -1 {
		return 0, errors.ErrInvalidStopSequence
	}
	return aIdx - bIdx, nil
}

// =============================================================================
// Cumulative Distance Methods (algorithm.md compliance)
// =============================================================================

// CumulativeDistanceAtStopIndex returns the pre-calculated cumulative distance
// from the route origin (stop 0) to the given stop index.
// This is the authoritative distance for the algorithm.
func (r *Route) CumulativeDistanceAtStopIndex(idx int) (float64, error) {
	if idx < 0 || idx >= len(r.stops) {
		return 0, errors.ErrInvalidStopIndex
	}
	return r.stops[idx].CumulativeDistanceMeters, nil
}

// DistanceBetweenStopIndices returns the route distance between two stop indices
// using the pre-calculated cumulative distances. This is exact, not approximate.
// Returns absolute distance (always positive).
func (r *Route) DistanceBetweenStopIndices(fromIdx, toIdx int) (float64, error) {
	if fromIdx < 0 || fromIdx >= len(r.stops) || toIdx < 0 || toIdx >= len(r.stops) {
		return 0, errors.ErrInvalidStopIndex
	}
	fromDist := r.stops[fromIdx].CumulativeDistanceMeters
	toDist := r.stops[toIdx].CumulativeDistanceMeters

	diff := toDist - fromDist
	if diff < 0 {
		diff = -diff
	}
	return diff, nil
}

// TotalRouteDistance returns the total route distance (cumulative distance at last stop).
func (r *Route) TotalRouteDistance() float64 {
	if len(r.stops) == 0 {
		return 0
	}
	return r.stops[len(r.stops)-1].CumulativeDistanceMeters
}

// StopAtIndex returns the stop at the given index, or error if out of bounds.
func (r *Route) StopAtIndex(idx int) (valueobjects.Stop, error) {
	if idx < 0 || idx >= len(r.stops) {
		return valueobjects.Stop{}, errors.ErrInvalidStopIndex
	}
	return r.stops[idx], nil
}

// CalculateCumulativeDistances computes cumulative distances for all stops
// using Haversine distance between consecutive stops.
//
// IMPORTANT: This method only calculates distances for stops that don't already
// have cumulative distances set (CumulativeDistanceMeters == 0 for non-first stops).
// This allows pre-computed road distances to be preserved when provided.
//
// For accurate route-following distances, provide real road distances when creating
// routes rather than relying on Haversine (which underestimates road distances).
func (r *Route) CalculateCumulativeDistances() {
	if len(r.stops) == 0 {
		return
	}

	// First stop is always at distance 0 from itself
	r.stops[0].CumulativeDistanceMeters = 0

	// Check if cumulative distances are already provided (last stop has distance > 0)
	// If so, skip calculation to preserve pre-computed road distances
	if len(r.stops) > 1 && r.stops[len(r.stops)-1].CumulativeDistanceMeters > 0 {
		// Distances already provided, don't overwrite with Haversine approximations
		return
	}

	// Calculate cumulative distance for each subsequent stop using Haversine
	for i := 1; i < len(r.stops); i++ {
		prevStop := r.stops[i-1]
		currStop := r.stops[i]

		// Haversine distance between consecutive stops
		segmentDistance := prevStop.Location.DistanceTo(currStop.Location).Meters()

		// Cumulative = previous cumulative + this segment
		r.stops[i].CumulativeDistanceMeters = prevStop.CumulativeDistanceMeters + segmentDistance
	}
}
