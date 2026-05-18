package events

import (
    "time"
    "github.com/MathewM27/busTrack-alebus/domain/types"
    "github.com/MathewM27/busTrack-alebus/domain/valueobjects"
)

type UserSavedLocationAdded struct {
    EventIDValue string
    UserID       types.UserID
    Location     valueobjects.SavedLocation
    OccurredAtTime   time.Time
}

func (e *UserSavedLocationAdded) EventID() string {
    return e.EventIDValue
}
func (e *UserSavedLocationAdded) EventType() string {
    return "UserSavedLocationAdded"
}
func (e *UserSavedLocationAdded) OccurredAt() time.Time {
    return e.OccurredAtTime
}

type UserSavedLocationRemoved struct {
	EventIDValue string
	UserID       types.UserID
	LocationName  string
	OccurredAtTime   time.Time
}
func (e *UserSavedLocationRemoved) EventID() string {
	return e.EventIDValue
}
func (e *UserSavedLocationRemoved) EventType() string {
	return "UserSavedLocationRemoved"
}
func (e *UserSavedLocationRemoved) OccurredAt() time.Time {
	return e.OccurredAtTime
}

type UserSubscriptionUpdated struct {
	EventIDValue string
	UserID       types.UserID
	NewPlan     valueobjects.SubscriptionPlan
	OccurredAtTime   time.Time
}
func (e *UserSubscriptionUpdated) EventID() string {
	return e.EventIDValue
}
func (e *UserSubscriptionUpdated) EventType() string {
	return "UserSubscriptionUpdated"
}
func (e *UserSubscriptionUpdated) OccurredAt() time.Time {
	return e.OccurredAtTime
}

type UserSavedLocationUpdated struct {
	EventIDValue string
	UserID       types.UserID
	Location     valueobjects.SavedLocation
	OccurredAtTime   time.Time
}

func (e *UserSavedLocationUpdated) EventID() string {
	return e.EventIDValue
}

func (e *UserSavedLocationUpdated) EventType() string {
	return "UserSavedLocationUpdated"
}
func (e *UserSavedLocationUpdated) OccurredAt() time.Time {
	return e.OccurredAtTime
}


type UserSavedLocationsCleared struct {
	EventIDValue string
	UserID       types.UserID
	OccurredAtTime   time.Time
}

func (e *UserSavedLocationsCleared) EventID() string {
	return e.EventIDValue
}

func (e *UserSavedLocationsCleared) EventType() string {
	return "UserSavedLocationsCleared"
}

func (e *UserSavedLocationsCleared) OccurredAt() time.Time {
	return e.OccurredAtTime
}

type UserEmailChanged struct {
	EventIDValue string
	UserID       types.UserID
	OldEmail     string
	NewEmail     string
	OccurredAtTime   time.Time
}

func (e *UserEmailChanged) EventID() string {
	return e.EventIDValue
}

func (e *UserEmailChanged) EventType() string {
	return "UserEmailChanged"
}

func (e *UserEmailChanged) OccurredAt() time.Time {
	return e.OccurredAtTime
}