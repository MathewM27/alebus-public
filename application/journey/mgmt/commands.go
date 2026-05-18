package journeymgmt

import (
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
)

type CreateJourneyCommand struct {
	JourneyID         types.JourneyID
	UserID            types.UserID
	OriginLocation    types.GeoLocation
	OriginStopID      types.StopID
	DestinationStopID types.StopID
	CreatedAt         time.Time
	EventRecorder     types.EventRecorder
}

type InitializeTrackingCommand struct {
	JourneyID         types.JourneyID
	Recommendations   []valueobjects.EnhancedBusRecommendation
	Duration          types.Duration
	RequiredDirection types.Direction
}

type UpdateRecommendationsCommand struct {
	JourneyID       types.JourneyID
	Recommendations []valueobjects.EnhancedBusRecommendation
}

type SwitchBusCommand struct {
	JourneyID types.JourneyID
	NewBusID  types.BusID
	Reason    enums.JourneySwitchReason
}

type UpdateProximityCommand struct {
	JourneyID types.JourneyID
	BusID     types.BusID
	Level     enums.ProximityLevel
	Distance  types.Distance
}

type ConfirmBoardingCommand struct {
	Journey types.JourneyID
}

type CompleteJourneyCommand struct {
	Journey types.JourneyID
}

type DeclineBoardingCommand struct {
	Journey types.JourneyID
}

type CancelJourneyCommand struct {
	Journey types.JourneyID
}

type ExpireJourneyCommand struct {
	Journey   types.JourneyID
	ExpiredAt time.Time
}
