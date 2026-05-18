package journeytrackingreadmodel

import (
	"context"

	"github.com/MathewM27/busTrack-alebus/application/journey/dto"
)

// ============================================================================
// DTOs for Journey Tracking UI
// ============================================================================

// BusRecommendationDTO is aliased to the canonical DTO to prevent drift.
type BusRecommendationDTO = dto.BusRecommendationDTO

// JourneyTrackingDTO - Full journey state for tracking UI
type JourneyTrackingDTO struct {
	// Identity
	JourneyID string `json:"journeyId"`
	UserID    string `json:"userId"`

	// Status
	Status     int    `json:"status"`
	StatusName string `json:"statusName"`

	// Origin
	OriginLat    float64 `json:"originLat"`
	OriginLon    float64 `json:"originLon"`
	OriginStopID string  `json:"originStopId"`

	// Destination
	DestinationStopID string `json:"destinationStopId"`

	// Direction intelligence
	RequiredDirection int `json:"requiredDirection"`

	// Active tracking
	ActiveBusID    string `json:"activeBusId"`
	ProximityLevel int    `json:"proximityLevel"`
	ProximityName  string `json:"proximityName"`

	// Recommendations
	Recommendations []BusRecommendationDTO `json:"recommendations"`

	// Boarding state
	BoardingWindowStart *string `json:"boardingWindowStart,omitempty"`
	BoardedAt           *string `json:"boardedAt,omitempty"`
	DeclineCount        int     `json:"declineCount"`

	// Timing
	EstimatedDuration int64  `json:"estimatedDuration"` // milliseconds
	ExpiresAt         string `json:"expiresAt"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

// JourneyHistoryDTO - Completed journey summary for history list
type JourneyHistoryDTO struct {
	JourneyID         string `json:"journeyId"`
	Status            int    `json:"status"`
	StatusName        string `json:"statusName"`
	OriginStopID      string `json:"originStopId"`
	DestinationStopID string `json:"destinationStopId"`
	BusID             string `json:"busId"`
	BoardedAt         string `json:"boardedAt,omitempty"`
	CompletedAt       string `json:"completedAt,omitempty"`
	CreatedAt         string `json:"createdAt"`
}

// ============================================================================
// Query Inputs
// ============================================================================

type GetJourneyRequest struct {
	JourneyID string
}

type GetActiveJourneyRequest struct {
	UserID string
}

type ListJourneyHistoryRequest struct {
	UserID string
	Limit  int
}

// ============================================================================
// Query Interface (Port)
// ============================================================================

type JourneyTrackingReader interface {
	// GetJourney retrieves a journey by ID
	GetJourney(ctx context.Context, req GetJourneyRequest) (JourneyTrackingDTO, bool, error)

	// GetActiveJourney retrieves the user's active (non-terminal) journey
	GetActiveJourney(ctx context.Context, req GetActiveJourneyRequest) (JourneyTrackingDTO, bool, error)

	// ListJourneyHistory retrieves completed/cancelled journeys for history
	ListJourneyHistory(ctx context.Context, req ListJourneyHistoryRequest) ([]JourneyHistoryDTO, error)
}
