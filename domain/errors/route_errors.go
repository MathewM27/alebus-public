package errors

import "errors"

var (
	ErrRouteIdRequired	= errors.New("route id is required")
	ErrRouteNameRequired	= errors.New("route name is required")
	ErrOperatorIdRequired	= errors.New("at least one operator id is required")
	ErrInsufficientStops	= errors.New("a route must have at least two stops")
	ErrInvalidRouteDirection	= errors.New("invalid route direction")
	ErrInvalidRouteType	= errors.New("invalid route type")
	ErrInvalidActivePeriod	= errors.New("invalid active period for the route")
	ErrInvalidOriginStop    = errors.New("origin stop not found on route")
    ErrInvalidDestinationStop = errors.New("destination stop not found on route")
    ErrInvalidStopSequence  = errors.New("invalid stop sequence for direction")
)
