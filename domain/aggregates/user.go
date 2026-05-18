package aggregates

import (
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/errors"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
)

type User struct {
	userID        types.UserID
	email         valueobjects.Email
	subscription  valueobjects.Subscription
	savedLocs     []valueobjects.SavedLocation
	createdAt     time.Time
	version       types.AggregateUserVersion
	eventRecorder types.EventRecorder
}

func NewUser(
	userID types.UserID,
	email string,
	subscription valueobjects.Subscription,
	createdAt time.Time,
	eventRecorder types.EventRecorder,
) (*User, error) {
	if userID == "" {
		return nil, errors.ErrUserIdRequired
	}
	if email == "" {
		return nil, errors.ErrEmailRequired
	}
	emailVO, err := valueobjects.NewEmail(email)
	if err != nil {
		return nil, err
	}
	if !subscription.IsActive() {
		return nil, errors.ErrInactiveSubscription
	}
	if createdAt.IsZero() {
		return nil, errors.ErrInvalidCreationTime
	}
	return &User{
		userID:        userID,
		email:         emailVO,
		subscription:  subscription,
		savedLocs:     []valueobjects.SavedLocation{},
		createdAt:     createdAt,
		version:       1,
		eventRecorder: eventRecorder,
	}, nil
}

func (u *User) ID() types.UserID {
	return u.userID
}
func (u *User) Email() string {
    return u.email.String()
}
func (u *User) Subscription() valueobjects.Subscription {
	return u.subscription
}
func (u *User) SavedLocations() []valueobjects.SavedLocation {
	return u.savedLocs
}
func (u *User) CreatedAt() time.Time {
	return u.createdAt
}
func (u *User) Version() types.AggregateUserVersion {
	return u.version
}

func (u *User) recordEvent(event types.DomainEvent) error {
	if u.eventRecorder != nil {
		return u.eventRecorder.Record(event)
	}
	return errors.ErrEventRecorderRequired
}
