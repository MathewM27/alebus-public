package enums

type JourneyStatus int

const (
	JourneyStatusSearching JourneyStatus = iota
	JourneyStatusTracking
	JourneyStatusBoardingPrompt
	JourneyStatusBoarded
	JourneyStatusCompleted
	JourneyStatusCancelled
	JourneyStatusExpired
)

type JourneySwitchReason int

const (
	JourneySwitchReasonUnknown JourneySwitchReason = iota
	JourneySwitchReasonUser
	JourneySwitchReasonOvertaken
	JourneySwitchReasonLocation
	JourneySwitchReasonOffline
	JourneySwitchReasonTerminalDelay
	JourneySwitchReasonUserDeclined
)

type ProximityLevel int

const (
	ProximityLevelNone        ProximityLevel = iota
	ProximityLevelApproaching                //500m
	ProximityLevelNearby                     //<=100m
	ProximityLevelArrived                    //50m
)

// ValidStatusTransitions defines all allowed journey state transitions
// Maps current status to allowed next statuses
var ValidStatusTransitions = map[JourneyStatus][]JourneyStatus{
	JourneyStatusSearching: {JourneyStatusTracking},
	JourneyStatusTracking: {
		JourneyStatusBoardingPrompt,
		JourneyStatusTracking, // Update recommendations without changing status
		JourneyStatusCancelled,
		JourneyStatusExpired,
	},
	JourneyStatusBoardingPrompt: {
		JourneyStatusBoarded,  // User boarded the bus
		JourneyStatusTracking, // User declined - go back to tracking
		JourneyStatusCancelled,
		JourneyStatusExpired, // Boarding window timeout
	},
	JourneyStatusBoarded: {
		JourneyStatusCompleted, // Journey completed successfully
		JourneyStatusCancelled, // User got off before destination
		JourneyStatusExpired,   // Unexpected expiration while boarded
	},
	JourneyStatusCompleted: {}, // Terminal state - no transitions allowed
	JourneyStatusCancelled: {}, // Terminal state - no transitions allowed
	JourneyStatusExpired:   {}, // Terminal state - no transitions allowed
}

// CanTransitionTo checks if transitioning from current status to target status is allowed
func (s JourneyStatus) CanTransitionTo(target JourneyStatus) bool {
    switch s {
    case JourneyStatusSearching:
        return target == JourneyStatusTracking ||
            target == JourneyStatusCancelled ||
            target == JourneyStatusExpired
    case JourneyStatusTracking:
        return target == JourneyStatusBoardingPrompt ||
            target == JourneyStatusCancelled ||
            target == JourneyStatusExpired
    case JourneyStatusBoardingPrompt:
        return target == JourneyStatusBoarded ||
            target == JourneyStatusTracking ||
            target == JourneyStatusCancelled ||
            target == JourneyStatusExpired
    case JourneyStatusBoarded:
        return target == JourneyStatusCompleted ||
            target == JourneyStatusCancelled ||
            target == JourneyStatusExpired
    case JourneyStatusCompleted, JourneyStatusCancelled, JourneyStatusExpired:
        return false
    default:
        return false
    }
}
