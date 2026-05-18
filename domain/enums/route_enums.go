package enums

type RouteDirection int

const (
    RouteDirectionOutbound RouteDirection = iota
    RouteDirectionInbound
    RouteDirectionBidirectional
    RouteDirectionUnidirectional
)

// Direction represents the travel direction for a journey on a route
type Direction int

const (
    DirectionOutbound Direction = iota // Travel from first stop to last stop
    DirectionInbound                   // Travel from last stop to first stop
)

type RouteStatus int

const (
    RouteStatusActive RouteStatus = iota
    RouteStatusInactive
    RouteStatusSuspended
)

type RouteType int

const (
    RouteTypeUrban RouteType = iota
    RouteTypeHighway
    RouteTypeMixed
  
)


