package aggregates

import (
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/errors"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
)

type Journey struct {
	//Identity
	journeyID types.JourneyID
	userID    types.UserID

	//Business intent
	originLocation    types.GeoLocation
	destinationStopID types.StopID
	originStopID      types.StopID

	// Business state
	recommendedBuses   []valueobjects.EnhancedBusRecommendation
	activeBusID        types.BusID
	status             enums.JourneyStatus
	lastSwitchReason   enums.JourneySwitchReason
	lastProximityLevel enums.ProximityLevel
	// Decline counts for recommendations
	declineCount int

	//Direction intelligence
	requiredDirection types.Direction
	eventRecorder     types.EventRecorder

	//Business rules
	estimatedDuration types.Duration
	expirationTime    time.Time

	//Aggregate management
	version   types.AggregateJourneyVersion
	createdAt time.Time

	boardingWindowStartedAt *time.Time
	boardingTimeoutDuration time.Duration
	boardedAt               *time.Time
}

func NewJourney(
	journeyID types.JourneyID,
	userID types.UserID,
	originLocation types.GeoLocation,
	originStopID types.StopID,
	destinationStopID types.StopID,
	createdAt time.Time,
	eventRecorder types.EventRecorder,
) (*Journey, error) {
	// Validate geo location bounds
	if originLocation.Latitude < -90 || originLocation.Latitude > 90 {
		return nil, errors.ErrInvalidGeoLocation
	}
	if originLocation.Longitude < -180 || originLocation.Longitude > 180 {
		return nil, errors.ErrInvalidGeoLocation
	}
	if originLocation.Latitude == 0 && originLocation.Longitude == 0 {
		return nil, errors.ErrInvalidGeoLocation
	}

	if originStopID == "" || destinationStopID == "" {
		return nil, errors.ErrStopIdRequired
	}
	if originStopID == destinationStopID {
		return nil, errors.ErrOriginCannotBeDestination
	}
	if createdAt.IsZero() {
		return nil, errors.ErrCreatedAtRequired
	}
	// Validate createdAt is not in the future (clock skew protection)
	if createdAt.After(time.Now()) {
		return nil, errors.ErrCreatedAtInFuture
	}
	// Validate createdAt is not older than 24 hours (stale journey prevention)
	maxAge := time.Now().Add(-24 * time.Hour)
	if createdAt.Before(maxAge) {
		return nil, errors.ErrCreatedAtTooOld
	}
	if eventRecorder == nil {
		return nil, errors.ErrEventRecorderRequired
	}
	return &Journey{
		journeyID:               journeyID,
		userID:                  userID,
		originLocation:          originLocation,
		originStopID:            originStopID,
		destinationStopID:       destinationStopID,
		createdAt:               createdAt,
		eventRecorder:           eventRecorder,
		status:                  enums.JourneyStatusSearching,
		expirationTime:          createdAt.Add(30 * time.Minute),
		version:                 1,
		boardingTimeoutDuration: 3 * time.Minute,
	}, nil
}

// Getters
func (j *Journey) JourneyID() types.JourneyID        { return j.journeyID }
func (j *Journey) UserID() types.UserID              { return j.userID }
func (j *Journey) OriginLocation() types.GeoLocation { return j.originLocation }
func (j *Journey) OriginStopID() types.StopID        { return j.originStopID }
func (j *Journey) DestinationStopID() types.StopID   { return j.destinationStopID }
func (j *Journey) RecommendedBuses() []valueobjects.EnhancedBusRecommendation {
	return j.recommendedBuses
}
func (j *Journey) ActiveBusID() types.BusID                 { return j.activeBusID }
func (j *Journey) Status() enums.JourneyStatus              { return j.status }
func (j *Journey) RequiredDirection() types.Direction       { return j.requiredDirection }
func (j *Journey) EstimatedDuration() types.Duration        { return j.estimatedDuration }
func (j *Journey) ExpirationTime() time.Time                { return j.expirationTime }
func (j *Journey) CreatedAt() time.Time                     { return j.createdAt }
func (j *Journey) Version() types.AggregateJourneyVersion   { return j.version }
func (j *Journey) DeclineCount() int                        { return j.declineCount }
func (j *Journey) LastProximityLevel() enums.ProximityLevel { return j.lastProximityLevel }

func (j *Journey) BoardingWindowStartedAt() *time.Time {
	return j.boardingWindowStartedAt
}

func (j *Journey) BoardedAt() *time.Time {
	return j.boardedAt
}

func (j *Journey) IsBoardingWindowExpired() bool {
	if j.boardingWindowStartedAt == nil {
		return false
	}
	return time.Since(*j.boardingWindowStartedAt) > j.boardingTimeoutDuration
}
