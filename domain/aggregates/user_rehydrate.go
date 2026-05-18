package aggregates

import (
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
)

// RehydrateUser rebuilds a User aggregate from a persisted snapshot.
// It enforces the same invariants as NewUser and restores saved locations.
func RehydrateUser(
	userID types.UserID,
	email string,
	subscription valueobjects.Subscription,
	savedLocations []valueobjects.SavedLocation,
	createdAt time.Time,
	version types.AggregateUserVersion,
	eventRecorder types.EventRecorder,
) (*User, error) {
	u, err := NewUser(userID, email, subscription, createdAt, eventRecorder)
	if err != nil {
		return nil, err
	}

	u.savedLocs = savedLocations
	if version > 0 {
		u.version = version
	}

	return u, nil
}
