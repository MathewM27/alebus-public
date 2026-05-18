package journeyreadmodel

import "time"

// FindJourneyRecommendationsRequest is a read-model query input.
// Keep it primitive/DTO-oriented.
type FindJourneyRecommendationsRequest struct {
    OriginLat     float64   `json:"originLat"`
    OriginLon     float64   `json:"originLon"`
    DestLat       float64   `json:"destLat"`
    DestLon       float64   `json:"destLon"`
    RadiusMeters  float64   `json:"radiusMeters"`
    RequestedAt   time.Time `json:"-"`
}

type NearbyStopDTO struct {
    StopID         string  `json:"stopId"`
    Name           string  `json:"name"`
    DistanceMeters float64 `json:"distanceMeters"`
}

type StopRefDTO struct {
    StopID         string  `json:"stopId"`
    Name           string  `json:"name"`
    DistanceMeters float64 `json:"distanceMeters"`
    Index          int     `json:"index"`
}

type AlternativeDTO struct {
    BoardingStopID   string  `json:"boardingStopId"`
    AlightingStopID  string  `json:"alightingStopId"`
    BoardWalkMeters  float64 `json:"boardWalkMeters"`
    AlightWalkMeters float64 `json:"alightWalkMeters"`
}

type RouteRecommendationDTO struct {
    RouteID       string           `json:"routeId"`
    RouteName     string           `json:"routeName"`
    Direction     int              `json:"direction"` // enums.Direction as int for UI
    BestBoarding  StopRefDTO       `json:"bestBoarding"`
    BestAlighting StopRefDTO       `json:"bestAlighting"`
    Alternatives  []AlternativeDTO `json:"alternatives,omitempty"`
}

type JourneyRecommendationsResponse struct {
    RequestedAt            string                  `json:"requestedAt"`
    NearbyOriginStops      []NearbyStopDTO         `json:"nearbyOriginStops"`
    NearbyDestinationStops []NearbyStopDTO         `json:"nearbyDestinationStops"`
    RouteRecommendations   []RouteRecommendationDTO `json:"routeRecommendations"`
}

type JourneyLegDTO struct {
    RouteID   string     `json:"routeId"`
    RouteName string     `json:"routeName"`
    Direction int        `json:"direction"` // enums.Direction as int for UI
    Boarding  StopRefDTO `json:"boarding"`
    Alighting StopRefDTO `json:"alighting"`
}

type TwoLegJourneyRecommendationDTO struct {
    TransferStop StopRefDTO    `json:"transferStop"`
    Leg1         JourneyLegDTO `json:"leg1"`
    Leg2         JourneyLegDTO `json:"leg2"`
}

type TwoLegJourneyRecommendationsResponse struct {
    RequestedAt            string                          `json:"requestedAt"`
    NearbyOriginStops      []NearbyStopDTO                 `json:"nearbyOriginStops"`
    NearbyDestinationStops []NearbyStopDTO                 `json:"nearbyDestinationStops"`
    Found                  bool                            `json:"found"`
    Message                string                          `json:"message"`
    Recommendation         *TwoLegJourneyRecommendationDTO  `json:"recommendation,omitempty"`
}

// Recommender is the query/read-model contract.
// Implementations can be in-memory (UI_test) or Redis/Postgres (infrastructure) later.
type Recommender interface {
    Recommend(req FindJourneyRecommendationsRequest) (JourneyRecommendationsResponse, error)
    RecommendTwoLeg(req FindJourneyRecommendationsRequest) (TwoLegJourneyRecommendationsResponse, error)
}