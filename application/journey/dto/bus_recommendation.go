package dto

// BusRecommendationDTO is the canonical DTO shape for bus recommendations.
//
// It is intentionally shared across multiple application-layer boundaries
// (automation responses and tracking read models) to prevent contract drift.
type BusRecommendationDTO struct {
	BusID            string  `json:"busId"`
	OperatorID       string  `json:"operatorId"`
	EstimatedArrival int64   `json:"estimatedArrival"` // milliseconds
	DistanceMeters   float64 `json:"distanceMeters"`
	Direction        int     `json:"direction"`
	IsWrongDirection bool    `json:"isWrongDirection"`
	ConfidenceLevel  float64 `json:"confidenceLevel"`
	DisplayText      string  `json:"displayText"`
}
