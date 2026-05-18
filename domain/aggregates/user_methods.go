package aggregates

import (
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/errors"
	"github.com/MathewM27/busTrack-alebus/domain/events"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
	"github.com/google/uuid"
)

func (u *User) CanStartNewJourney(currentJourneyCount int) bool {
	return currentJourneyCount < u.subscription.GetMaxConcurrentJourneys()
}

func (u *User) AddSavedLocation(location valueobjects.SavedLocation) error {
	const maxLocations = 10 // Business rule
	if len(u.savedLocs) >= maxLocations {
		return errors.ErrMaxSavedLocationsReached
	}
	u.savedLocs = append(u.savedLocs, location)
	return u.recordEvent(&events.UserSavedLocationAdded{
		EventIDValue:   uuid.NewString(),
		UserID:         u.userID,
		Location:       location,
		OccurredAtTime: time.Now(),
	})
}

func (u *User) RemoveSavedLocation(name string) error {
	for i, loc := range u.savedLocs {
		if loc.Name == name {
			u.savedLocs = append(u.savedLocs[:i], u.savedLocs[i+1:]...)
			return u.recordEvent(&events.UserSavedLocationRemoved{
				EventIDValue:   uuid.NewString(),
				UserID:         u.userID,
				LocationName:   name,
				OccurredAtTime: time.Now(),
			})
		}
	}
	return errors.ErrSavedLocationNotFound
}

func (u *User) UpdateSubscription(newSubscription valueobjects.Subscription) error {
	if !newSubscription.IsActive() {
		return errors.ErrInvalidSubscription
	}
	u.subscription = newSubscription
	return u.recordEvent(&events.UserSubscriptionUpdated{
		EventIDValue:   uuid.NewString(),
		UserID:         u.userID,
		NewPlan:        newSubscription.Plan,
		OccurredAtTime: time.Now(),
	})
}
func (u *User) HasSavedLocation(name string) bool {
	for _, loc := range u.savedLocs {
		if loc.Name == name {
			return true
		}
	}
	return false
}
func (u *User) UpdateSavedLocation(updated valueobjects.SavedLocation) error {
	for i, loc := range u.savedLocs {
		if loc.Name == updated.Name {
			u.savedLocs[i] = updated
			return u.recordEvent(&events.UserSavedLocationUpdated{
				EventIDValue:   uuid.NewString(),
				UserID:         u.userID,
				Location:       updated,
				OccurredAtTime: time.Now(),
			})
		}
	}
	return errors.ErrSavedLocationNotFound
}

func (u *User) ClearSavedLocations() error {
	u.savedLocs = []valueobjects.SavedLocation{}
	return u.recordEvent(&events.UserSavedLocationsCleared{
		EventIDValue:   uuid.NewString(),
		UserID:         u.userID,
		OccurredAtTime: time.Now(),
	})
}

func (u *User) ChangeEmail(newEmail valueobjects.Email) error {
	if u.email == newEmail {
		return errors.ErrEmailUnchanged
	}
	oldEmail := u.email
	u.email = newEmail
	return u.recordEvent(&events.UserEmailChanged{
		EventIDValue:   uuid.NewString(),
		UserID:         u.userID,
		OldEmail:       oldEmail.String(),
		NewEmail:       newEmail.String(),
		OccurredAtTime: time.Now(),
	})
}
