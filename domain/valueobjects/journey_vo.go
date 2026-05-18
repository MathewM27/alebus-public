package valueobjects

import(
	"github.com/MathewM27/busTrack-alebus/domain/types"
)



type EnhancedBusRecommendation struct {
	BusID types.BusID
	OperatorID types.OperatorID

	//Distance information (GPS approximation)
	ActualRouteDistance types.Distance // in meters
	EstimatedArrival types.Duration // in seconds
	JourneyInfo BusJourneyInfo

	//Direction information
	Direction types.Direction
	RequiredDirection types.Direction
	IsWrongDirection bool

	//User Experience
	DisplayText string
	ConfidenceLevel float64
	RankingScore float64

	//Legacy fields for compatibility
	DistanceToOriginStop types.Distance // in meters
	Confidence float64
	Rank int
}

type BusJourneyInfo struct {
	Type string
	TotalDistance types.Distance // in meters
	EstimatedTime types.Duration // in seconds
	RequiresTerminal bool
	TerminalWaitTime types.Duration // in seconds
	Breakdown JourneyBreakdown
}


type JourneyBreakdown struct {
	ToTerminal types.Duration
	AtTerminal types.Duration
	FromTerminal types.Duration
}

type ProximityConfig struct {
	InitialRadius float64
	MaxRadius float64
	ExpandStep float64
}