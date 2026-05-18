package aggregates

import (
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/errors"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
)

// RehydrateJourney rebuilds a Journey aggregate from a persisted snapshot.
// It intentionally avoids time.Now()-based creation checks so old journeys can be loaded.
// The version parameter must match the persisted version for optimistic locking to work.
func RehydrateJourney(
	journeyID types.JourneyID,
	userID types.UserID,
	originLocation types.GeoLocation,
	originStopID types.StopID,
	destinationStopID types.StopID,
	recommendedBuses []valueobjects.EnhancedBusRecommendation,
	activeBusID types.BusID,
	status enums.JourneyStatus,
	lastProximityLevel enums.ProximityLevel,
	declineCount int,
	requiredDirection types.Direction,
	estimatedDuration types.Duration,
	expirationTime time.Time,
	boardingWindowStartedAt *time.Time,
	boardedAt *time.Time,
	createdAt time.Time,
	version types.AggregateJourneyVersion,
	eventRecorder types.EventRecorder,
) (*Journey, error) {
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
	if eventRecorder == nil {
		return nil, errors.ErrEventRecorderRequired
	}

	j := &Journey{
		journeyID:               journeyID,
		userID:                  userID,
		originLocation:          originLocation,
		originStopID:            originStopID,
		destinationStopID:       destinationStopID,
		recommendedBuses:        recommendedBuses,
		activeBusID:             activeBusID,
		status:                  status,
		lastProximityLevel:      lastProximityLevel,
		declineCount:            declineCount,
		requiredDirection:       requiredDirection,
		estimatedDuration:       estimatedDuration,
		expirationTime:          expirationTime,
		createdAt:               createdAt,
		boardingWindowStartedAt: boardingWindowStartedAt,
		boardingTimeoutDuration: 3 * time.Minute,
		boardedAt:               boardedAt,
		version:                 version,
		eventRecorder:           eventRecorder,
	}

	return j, nil
}
