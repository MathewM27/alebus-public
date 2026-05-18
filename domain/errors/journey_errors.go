package errors

import "errors"

const (
	// Duration validation
	MinJourneyDuration = "5 minutes"
	MaxJourneyDuration = "3 hours"
)

var (
	ErrStopIdRequired                                 = errors.New("stop id is required")
	ErrNoRecommendations                              = errors.New("no bus recommendations provided")
	ErrInvalidJourneyDuration                         = errors.New("journey duration must be positive")
	ErrJourneyTooShort                                = errors.New("journey duration must be at least 5 minutes")
	ErrJourneyTooLong                                 = errors.New("journey duration cannot exceed 3 hours")
	ErrCannotUpdateRecommendationsWhileDuringBoarding = errors.New("cannot update recommendations while journey is in boarding prompt status")
	ErrProximityForWrongBus                           = errors.New("proximity update is for a bus that is not currently active in the journey")
	ErrProximityMustIncrease                          = errors.New("proximity level must increase")
	ErrNotInBoardingState                             = errors.New("journey is not in boarding prompt state")
	ErrNotInBoardedState                              = errors.New("journey is not in boarded state")
	ErrCannotCancelCompletedJourney                   = errors.New("cannot cancel a completed journey")
	ErrMaxDeclinesReached                             = errors.New("user has declined boarding too many times")
	ErrBoardingWindowExpired                          = errors.New("boarding window has expired (10 minutes)")
	ErrOriginCannotBeDestination                      = errors.New("origin stop cannot be the same as destination stop")
	ErrInvalidProximityTransition                     = errors.New("proximity level transition is invalid - must progress sequentially")
	ErrInvalidStatusTransition                        = errors.New("invalid journey status transition")
	ErrJourneyExpired                                 = errors.New("journey has expired and cannot be modified")
	ErrCreatedAtInFuture                              = errors.New("journey creation timestamp cannot be in the future")
	ErrCreatedAtTooOld                                = errors.New("journey creation timestamp cannot be older than 24 hours")
)
