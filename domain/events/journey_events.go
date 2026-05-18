package events

import (
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"time"
	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
)



type JourneyStartedEvent struct {
	EventIDValue      string
	JourneyID         types.JourneyID
	UserID            types.UserID
	ActiveBusID       types.BusID
	Recommendations   []valueobjects.EnhancedBusRecommendation
	RequiredDirection types.Direction
	OccurredAtTime    time.Time
}

func (e JourneyStartedEvent) EventID() string {
	return e.EventIDValue
}
func (e JourneyStartedEvent) OccurredAt() time.Time {
	return e.OccurredAtTime
}
func (e JourneyStartedEvent) EventType() string {
	return "JourneyStarted"
}

type JourneyRecommendationsUpdatedEvent struct {
	EventIDValue         string
	JourneyID            types.JourneyID
	UserID               types.UserID
	NewRecommendations   []valueobjects.EnhancedBusRecommendation
	SignificantChange    bool
	ActiveRecommendation valueobjects.EnhancedBusRecommendation
	OccurredAtTime       time.Time
}

func (e JourneyRecommendationsUpdatedEvent) EventID() string {
	return e.EventIDValue
}
func (e JourneyRecommendationsUpdatedEvent) OccurredAt() time.Time {
	return e.OccurredAtTime
}
func (e JourneyRecommendationsUpdatedEvent) EventType() string {
	return "JourneyRecommendationsUpdated"
}

type JourneyBusSwitchedEvent struct {
	EventIDValue   string
	JourneyID      types.JourneyID
	UserID         types.UserID
	OldBusID       types.BusID
	NewBusID       types.BusID
	Reason         enums.JourneySwitchReason
	OccurredAtTime time.Time
}

func (e JourneyBusSwitchedEvent) EventID() string {
	return e.EventIDValue
}
func (e JourneyBusSwitchedEvent) OccurredAt() time.Time {
	return e.OccurredAtTime
}
func (e JourneyBusSwitchedEvent) EventType() string {
	return "JourneyBusSwitched"
}

type BusProximityUpdated struct {
	EventIDValue   string
	JourneyID      types.JourneyID
	UserID         types.UserID
	BusID          types.BusID
	Level          enums.ProximityLevel
	PreviousLevel  enums.ProximityLevel
	Distance       types.Distance
	OccurredAtTime time.Time
}

func (e BusProximityUpdated) EventID() string {
	return e.EventIDValue
}
func (e BusProximityUpdated) OccurredAt() time.Time {
	return e.OccurredAtTime
}
func (e BusProximityUpdated) EventType() string {
	return "BusProximityUpdated"
}

type JourneyCompleted struct {
	EventIDValue   string
	JourneyID      types.JourneyID
	BusID          types.BusID
	OccurredAtTime time.Time
}

func (e JourneyCompleted) EventID() string {
	return e.EventIDValue
}
func (e JourneyCompleted) OccurredAt() time.Time {
	return e.OccurredAtTime
}
func (e JourneyCompleted) EventType() string {
	return "JourneyCompleted"
}

type JourneyBoardingConfirmed struct {
	EventIDValue   string
	JourneyID      types.JourneyID
	BusID          types.BusID
	BoardedAt      time.Time
	OccurredAtTime time.Time
}

func (e JourneyBoardingConfirmed) EventID() string {
	return e.EventIDValue
}
func (e JourneyBoardingConfirmed) OccurredAt() time.Time {
	return e.OccurredAtTime
}
func (e JourneyBoardingConfirmed) EventType() string {
	return "JourneyBoardingConfirmed"
}

type JourneyBoardingDeclined struct {
	EventIDValue   string
	JourneyID      types.JourneyID
	BusID          types.BusID
	DeclineCount   int
	OccurredAtTime time.Time
}

func (e JourneyBoardingDeclined) EventID() string {
	return e.EventIDValue
}

func (e JourneyBoardingDeclined) OccurredAt() time.Time {
	return e.OccurredAtTime
}

func (e JourneyBoardingDeclined) EventType() string {
	return "JourneyBoardingDeclined"
}

type JourneyCancelled struct {
	EventIDValue   string
	JourneyID      types.JourneyID
	DeclineCount   int
	OccurredAtTime time.Time
}

func (e JourneyCancelled) EventID() string {
	return e.EventIDValue
}
func (e JourneyCancelled) OccurredAt() time.Time {
	return e.OccurredAtTime
}
func (e JourneyCancelled) EventType() string {
	return "JourneyCancelled"
}

type JourneyExpired struct {
	EventIDValue   string
	JourneyID      types.JourneyID
	ExpiredAtTime  time.Time
	OccurredAtTime time.Time
}

func (e JourneyExpired) EventID() string {
	return e.EventIDValue
}
func (e JourneyExpired) OccurredAt() time.Time {
	return e.OccurredAtTime
}
func (e JourneyExpired) EventType() string {
	return "JourneyExpired"
}

type JourneyAutoCompletedDueToTimeout struct {
	EventIDValue   string
	JourneyID      types.JourneyID
	BusID          types.BusID
	TimeoutMinutes int
	OccurredAtTime time.Time
}

func (e JourneyAutoCompletedDueToTimeout) EventID() string {
	return e.EventIDValue
}
func (e JourneyAutoCompletedDueToTimeout) OccurredAt() time.Time {
	return e.OccurredAtTime
}
func (e JourneyAutoCompletedDueToTimeout) EventType() string {
	return "JourneyAutoCompletedDueToTimeout"
}
