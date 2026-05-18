package aggregates

import (
	"math"
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/errors"
	"github.com/MathewM27/busTrack-alebus/domain/events"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
)

func (j *Journey) InitializeTracking(
	recommendations []valueobjects.EnhancedBusRecommendation,
	duration types.Duration,
	requiredDirection types.Direction,
) error {
	if len(recommendations) == 0 {
		return errors.ErrNoRecommendations
	}
	if duration <= 0 {
		return errors.ErrInvalidJourneyDuration
	}
	// Validate duration is within reasonable bounds (5 minutes to 3 hours)
	if time.Duration(duration) < 5*time.Minute {
		return errors.ErrJourneyTooShort
	}
	if time.Duration(duration) > 3*time.Hour {
		return errors.ErrJourneyTooLong
	}
	j.recommendedBuses = recommendations
	j.activeBusID = recommendations[0].BusID
	j.estimatedDuration = duration
	j.expirationTime = j.createdAt.Add(time.Duration(duration) * 2)
	j.requiredDirection = requiredDirection

	// Validate status transition: Searching → Tracking
	if !j.status.CanTransitionTo(enums.JourneyStatusTracking) {
		return errors.ErrInvalidStatusTransition
	}
	j.status = enums.JourneyStatusTracking

	//Record JourneyStarted event
	if j.eventRecorder != nil {
		event := events.JourneyStartedEvent{
			EventIDValue:      "journey-started-" + string(j.journeyID),
			JourneyID:         j.journeyID,
			UserID:            j.userID,
			ActiveBusID:       j.activeBusID,
			Recommendations:   j.recommendedBuses,
			RequiredDirection: j.requiredDirection,
			OccurredAtTime:    time.Now(),
		}
		_ = j.eventRecorder.Record(event)
	}
	return nil
}

func (j *Journey) UpdateRecommendations(
	newRecommendations []valueobjects.EnhancedBusRecommendation,
) error {
	if j.IsExpired(time.Now()) {
		return errors.ErrJourneyExpired
	}
	if j.status == enums.JourneyStatusBoardingPrompt {
		return errors.ErrCannotUpdateRecommendationsWhileDuringBoarding
	}
	if !j.ShouldUpdateRecommendations(newRecommendations) {
		return nil
	}
	significantChange := j.hasSignificantChange(newRecommendations)
	oldBusID := j.activeBusID
	j.recommendedBuses = newRecommendations
	if len(newRecommendations) > 0 {
		j.activeBusID = newRecommendations[0].BusID
	}

	// If the active bus changed, reset proximity tracking.
	// Proximity is tied to the currently active bus; switching buses must restart
	// the proximity progression for the new bus.
	if oldBusID != j.activeBusID {
		j.lastProximityLevel = enums.ProximityLevelNone
		j.boardingWindowStartedAt = nil
	}

	// Record BusSwitched event if active bus changed
	if oldBusID != j.activeBusID && j.eventRecorder != nil {
		event := events.JourneyBusSwitchedEvent{
			EventIDValue:   "journey-bus-switched-" + string(j.journeyID),
			JourneyID:      j.journeyID,
			OldBusID:       oldBusID,
			NewBusID:       j.activeBusID,
			Reason:         enums.JourneySwitchReasonOvertaken, // or another reason as per logic
			OccurredAtTime: time.Now(),
		}
		_ = j.eventRecorder.Record(event)
	}

	// Record JourneyRecommendationsUpdated event
	if j.eventRecorder != nil {
		event := events.JourneyRecommendationsUpdatedEvent{
			EventIDValue:         "journey-recommendations-updated-" + string(j.journeyID),
			JourneyID:            j.journeyID,
			UserID:               j.userID,
			NewRecommendations:   j.recommendedBuses,
			SignificantChange:    significantChange,
			ActiveRecommendation: j.recommendedBuses[0],
			OccurredAtTime:       time.Now(),
		}
		_ = j.eventRecorder.Record(event)
	}
	return nil
}

func (j *Journey) ShouldUpdateRecommendations(newRecs []valueobjects.EnhancedBusRecommendation) bool {
	if j.status == enums.JourneyStatusBoardingPrompt {
		return false
	}
	if len(newRecs) == 0 {
		return false
	}
	// if active bus has changed
	if newRecs[0].BusID != j.activeBusID {
		return true
	}
	return j.hasSignificantChange(newRecs)
}

func (j *Journey) hasSignificantChange(newRecs []valueobjects.EnhancedBusRecommendation) bool {
	if len(newRecs) != len(j.recommendedBuses) {
		return true
	}
	for i, newRec := range newRecs {
		if i >= len(j.recommendedBuses) {
			return true
		}
		oldRec := j.recommendedBuses[i]

		//significant if estimated arrival change
		timeChange := math.Abs(float64(newRec.EstimatedArrival - oldRec.EstimatedArrival))
		if timeChange > float64(3*time.Minute) {
			return true
		}
		//significant if bus ranking change
		if newRec.BusID != oldRec.BusID {
			return true
		}
	}
	return false
}

func (j *Journey) SwitchBus(
	newBusID types.BusID,
	reason enums.JourneySwitchReason,
) error {
	if j.IsExpired(time.Now()) {
		return errors.ErrJourneyExpired
	}
	if j.status == enums.JourneyStatusBoardingPrompt {
		return errors.ErrCannotUpdateRecommendationsWhileDuringBoarding
	}
	found := false
	for _, rec := range j.recommendedBuses {
		if rec.BusID == newBusID {
			found = true
			break
		}
	}
	if !found {
		return errors.ErrNoRecommendations
	}
	if newBusID == j.activeBusID {
		return nil
	}
	oldBusID := j.activeBusID
	j.activeBusID = newBusID
	j.lastSwitchReason = reason
	// Reset decline count for new bus (per-bus tracking - fresh chance with each bus)
	j.declineCount = 0

	if j.eventRecorder != nil {
		event := events.JourneyBusSwitchedEvent{
			EventIDValue:   "journey-bus-switched-" + string(j.journeyID),
			JourneyID:      j.journeyID,
			OldBusID:       oldBusID,
			NewBusID:       j.activeBusID,
			Reason:         reason,
			OccurredAtTime: time.Now(),
		}
		_ = j.eventRecorder.Record(event)
	}
	return nil
}

// isValidProximityProgression checks if proximity level transition is allowed
// Ensures progression follows: None → Approaching → Nearby → Arrived (no skipping levels)
func isValidProximityProgression(from, to enums.ProximityLevel) bool {
	transitions := map[enums.ProximityLevel][]enums.ProximityLevel{
		enums.ProximityLevelNone:        {enums.ProximityLevelApproaching},
		enums.ProximityLevelApproaching: {enums.ProximityLevelNearby},
		enums.ProximityLevelNearby:      {enums.ProximityLevelArrived},
		enums.ProximityLevelArrived:     {}, // Terminal state - no transitions allowed
	}

	allowedTransitions := transitions[from]
	for _, validLevel := range allowedTransitions {
		if validLevel == to {
			return true
		}
	}
	return false
}

// NextProximityLevelForDistance determines the next valid proximity level transition
// based on the current distance between the active bus and the origin stop.
//
// It enforces the monotonic, stepwise progression:
// None → Approaching → Nearby → Arrived
//
// If the journey is not in Tracking status, or distance does not warrant a transition,
// it returns (0, false).
func (j *Journey) NextProximityLevelForDistance(distance types.Distance) (enums.ProximityLevel, bool) {
	if j.status != enums.JourneyStatusTracking {
		return enums.ProximityLevelNone, false
	}
	if j.lastProximityLevel >= enums.ProximityLevelArrived {
		return enums.ProximityLevelNone, false
	}

	meters := distance.Meters()
	var target enums.ProximityLevel
	switch {
	case meters <= 50:
		target = enums.ProximityLevelArrived
	case meters <= 100:
		target = enums.ProximityLevelNearby
	case meters <= 500:
		target = enums.ProximityLevelApproaching
	default:
		target = enums.ProximityLevelNone
	}

	if target <= j.lastProximityLevel {
		return enums.ProximityLevelNone, false
	}

	// Advance at most one step, even if the bus is already within a tighter threshold.
	next := j.lastProximityLevel + 1
	if next > target {
		return enums.ProximityLevelNone, false
	}
	if !isValidProximityProgression(j.lastProximityLevel, next) {
		return enums.ProximityLevelNone, false
	}
	return next, true
}

func (j *Journey) UpdateProximity(
	busID types.BusID,
	newLevel enums.ProximityLevel,
	distance types.Distance,
) error {
	if j.IsExpired(time.Now()) {
		return errors.ErrJourneyExpired
	}
	if busID != j.activeBusID {
		return errors.ErrProximityForWrongBus
	}
	if newLevel <= j.lastProximityLevel {
		return errors.ErrProximityMustIncrease
	}
	// Validate proximity progression - no skipping levels
	if !isValidProximityProgression(j.lastProximityLevel, newLevel) {
		return errors.ErrInvalidProximityTransition
	}
	previousLevel := j.lastProximityLevel
	j.lastProximityLevel = newLevel

	// If bus has arrived, update status
	if newLevel == enums.ProximityLevelArrived {
		now := time.Now()
		j.boardingWindowStartedAt = &now

		// Validate status transition: Tracking → BordingPrompt
		if !j.status.CanTransitionTo(enums.JourneyStatusBoardingPrompt) {
			return errors.ErrInvalidStatusTransition
		}
		j.status = enums.JourneyStatusBoardingPrompt
	}

	// Record event
	if j.eventRecorder != nil {
		event := events.BusProximityUpdated{
			EventIDValue:   "bus-proximity-updated-" + string(j.journeyID),
			JourneyID:      j.journeyID,
			UserID:         j.userID,
			BusID:          busID,
			Level:          newLevel,
			PreviousLevel:  previousLevel,
			Distance:       distance,
			OccurredAtTime: time.Now(),
		}
		_ = j.eventRecorder.Record(event)
	}
	return nil
}

func (j *Journey) CompleteJourney() error {
	if j.status != enums.JourneyStatusBoarded {
		return errors.ErrNotInBoardedState
	}

	// Validate status transition: Boarded → Completed
	if !j.status.CanTransitionTo(enums.JourneyStatusCompleted) {
		return errors.ErrInvalidStatusTransition
	}
	j.status = enums.JourneyStatusCompleted

	// Record event
	if j.eventRecorder != nil {
		event := events.JourneyCompleted{
			EventIDValue:   "journey-completed-" + string(j.journeyID),
			JourneyID:      j.journeyID,
			BusID:          j.activeBusID,
			OccurredAtTime: time.Now(),
		}
		_ = j.eventRecorder.Record(event)
	}
	return nil
}

func (j *Journey) ConfirmBoarded() error {
	if j.status != enums.JourneyStatusBoardingPrompt {
		return errors.ErrNotInBoardingState
	}
	if j.IsBoardingWindowExpired() {
		return errors.ErrBoardingWindowExpired
	}

	// Validate status transition: BordingPrompt → Boarded
	if !j.status.CanTransitionTo(enums.JourneyStatusBoarded) {
		return errors.ErrInvalidStatusTransition
	}
	now := time.Now()
	j.boardedAt = &now
	j.status = enums.JourneyStatusBoarded

	// Record event
	if j.eventRecorder != nil {
		event := events.JourneyBoardingConfirmed{
			EventIDValue:   "journey-boarded-" + string(j.journeyID),
			JourneyID:      j.journeyID,
			BusID:          j.activeBusID,
			BoardedAt:      now,
			OccurredAtTime: time.Now(),
		}
		_ = j.eventRecorder.Record(event)
	}
	return nil
}

func (j *Journey) DeclineBoarding() error {
	if j.IsExpired(time.Now()) {
		return errors.ErrJourneyExpired
	}
	if j.status != enums.JourneyStatusBoardingPrompt {
		return errors.ErrNotInBoardingState
	}
	if j.declineCount >= 5 {
		return errors.ErrMaxDeclinesReached
	}
	j.declineCount++

	// Validate status transition: BordingPrompt → Tracking
	if !j.status.CanTransitionTo(enums.JourneyStatusTracking) {
		return errors.ErrInvalidStatusTransition
	}
	j.status = enums.JourneyStatusTracking
	j.lastProximityLevel = enums.ProximityLevelNone

	// Record event
	if j.eventRecorder != nil {
		event := events.JourneyBoardingDeclined{
			EventIDValue:   "journey-boarding-declined-" + string(j.journeyID),
			JourneyID:      j.journeyID,
			BusID:          j.activeBusID,
			DeclineCount:   j.declineCount,
			OccurredAtTime: time.Now(),
		}
		_ = j.eventRecorder.Record(event)
	}
	return nil
}

func (j *Journey) CancelJourney() error {
	if j.status == enums.JourneyStatusCompleted {
		return errors.ErrCannotCancelCompletedJourney
	}

	// Validate status transition to Cancelled
	if !j.status.CanTransitionTo(enums.JourneyStatusCancelled) {
		return errors.ErrInvalidStatusTransition
	}
	j.status = enums.JourneyStatusCancelled

	// Record event
	if j.eventRecorder != nil {
		event := events.JourneyCancelled{
			EventIDValue:   "journey-cancelled-" + string(j.journeyID),
			JourneyID:      j.journeyID,
			OccurredAtTime: time.Now(),
		}
		_ = j.eventRecorder.Record(event)
	}
	return nil
}

func (j *Journey) ExpireJourney(expriedAt time.Time) error {
	// Validate status transition to Expired
	if !j.status.CanTransitionTo(enums.JourneyStatusExpired) {
		return errors.ErrInvalidStatusTransition
	}
	j.status = enums.JourneyStatusExpired

	if j.eventRecorder != nil {
		event := events.JourneyExpired{
			EventIDValue:   "journey-expired-" + string(j.journeyID),
			JourneyID:      j.journeyID,
			ExpiredAtTime:  expriedAt,
			OccurredAtTime: time.Now(),
		}
		_ = j.eventRecorder.Record(event)
	}
	return nil
}

func (j *Journey) IsExpired(currentTime time.Time) bool {
	return currentTime.After(j.expirationTime)
}

func (j *Journey) AutoCompleteIfBoardingWindowExpired() error {
	if j.status != enums.JourneyStatusBoardingPrompt {
		return nil
	}
	if !j.IsBoardingWindowExpired() {
		return nil
	}

	// Auto-transition to Boarded when window expires
	if !j.status.CanTransitionTo(enums.JourneyStatusBoarded) {
		return errors.ErrInvalidStatusTransition
	}
	now := time.Now()
	j.boardedAt = &now
	j.status = enums.JourneyStatusBoarded

	if j.eventRecorder != nil {
		event := events.JourneyAutoCompletedDueToTimeout{
			EventIDValue:   "journey-auto-completed-due-to-time-" + string(j.journeyID),
			JourneyID:      j.journeyID,
			BusID:          j.activeBusID,
			TimeoutMinutes: 10,
			OccurredAtTime: time.Now(),
		}
		_ = j.eventRecorder.Record(event)
	}
	return nil
}
