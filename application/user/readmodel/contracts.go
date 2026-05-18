package userreadmodel

// Read-model contracts (DTOs + query interface) for Users.
// Must not depend on aggregates or call domain methods.

type UserDTO struct {
	UserID         string
	Email          string
	Plan           int
	Status         int
	StartDate      string
	ExpiryDate     string
	SavedLocations []SavedLocationDTO
	CreatedAt      string
}

type SavedLocationDTO struct {
	Name   string
	Lat    float64
	Lon    float64
	StopID *string
}

type GetUserRequest struct {
	UserID string
}

type UserReader interface {
	GetUser(GetUserRequest) (UserDTO, bool, error)
}
