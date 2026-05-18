package usermgmt

import (
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
)

type CreateUserCommand struct {
	UserID        types.UserID
	Email         string
	Subscription  valueobjects.Subscription
	CreatedAt     time.Time
	EventRecorder types.EventRecorder
}

type ChangeEmailCommand struct {
	UserID   types.UserID
	NewEmail string
}

type UpdateSubscriptionCommand struct {
	UserID          types.UserID
	NewSubscription valueobjects.Subscription
}

type AddSavedLocationCommand struct {
	UserID types.UserID
	Name   string
	Lat    float64
	Lon    float64
	StopID *types.StopID
}

type RemoveSavedLocationCommand struct {
	UserID types.UserID
	Name   string
}

type UpdateSavedLocationCommand struct {
	UserID types.UserID
	Name   string
	Lat    float64
	Lon    float64
	StopID *types.StopID
}

type ClearSavedLocationsCommand struct {
	UserID types.UserID
}
