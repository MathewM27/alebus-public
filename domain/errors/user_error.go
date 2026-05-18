package errors

import "errors"

//USER errors defined
var (
	ErrUserIdRequired        = errors.New("user id is required")
	ErrEmailRequired         = errors.New("email is required")
	ErrInactiveSubscription  = errors.New("subscription is inactive")
	ErrInvalidCreationTime   = errors.New("invalid creation time")
	ErrSavedLocationRequired = errors.New("saved location is required")
	ErrInvalidSavedLocation  = errors.New("invalid saved location")
)

// User error for methods in Bus aggregate

var (
	ErrMaxSavedLocationsReached     = errors.New("maximum number of saved locations reached")
	ErrSavedLocationNotFound        = errors.New("saved location not found")
	ErrInvalidSubscription          = errors.New("invalid subscription")
	ErrEmailUnchanged               = errors.New("email is unchanged")
	ErrMaxConcurrentJourneysReached = errors.New("maximum number of concurrent journeys reached")
)
