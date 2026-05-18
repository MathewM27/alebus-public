package errors

import "errors"

//Bus errors defined

//Bus error during initialization and creation of instance and update of bus position during change
var (
	ErrBusIdRequired = errors.New("bus id is required")

	ErrInvalidGeoLocation    = errors.New("invalid geo location")
	ErrInvalidDirection      = errors.New("invalid direction")
	ErrCreatedAtRequired     = errors.New("created at is required")
	ErrEventRecorderRequired = errors.New("event recorder is required")
)

// Bus errors during position update
var (
	ErrPositionTimestampInvalid    = errors.New("position timestamp is invalid")
	ErrInvalidStopIndex            = errors.New("invalid stop index")
	ErrInvalidSpeedKmh             = errors.New("invalid speed in km/h")
	ErrPositionTimestampRegression = errors.New("position timestamp cannot go backwards")
)

// Bus errors for ArrivalAtTerminal method

var (
	ErrAlreadyAtTerminal    = errors.New("bus is already at the terminal")
	ErrInvalidArrivalTime   = errors.New("invalid arrival time")
	ErrNotAtTerminal        = errors.New("bus is not at the terminal")
	ErrInvalidDepartureTime = errors.New("invalid departure time")
)

//ChangeStatus errors of bus
var (
	ErrAlreadyOffline = errors.New("bus is already offline")
)
