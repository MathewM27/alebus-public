package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	journeytrackingreadmodel "github.com/MathewM27/busTrack-alebus/application/journey/trackingreadmodel"
	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/infrastructure/db"
)

// PostgresJourneyTrackingReadModel implements journeytrackingreadmodel.JourneyTrackingReader.
//
// CQRS rule: this is read-only. It does not load aggregates, does not call domain methods,
// and does not mutate state.
//
// Source of truth: Postgres `journeys` table (recommended_buses JSONB is treated as persisted projection).
// Recommendation freshness is handled by command-side workflows (e.g. /journeys/refresh).
//
// NOTE: journeys currently has no updated_at; we use created_at as a best-effort UpdatedAt.
// If you need a true UpdatedAt, add an updated_at column and set it on save.
//
// IMPORTANT: This is intentionally UI-shaped (DTO) but still a query/read-model.
// It must remain side-effect free.
//
// Timeouts: uses request context if provided and applies a short query timeout.

type PostgresJourneyTrackingReadModel struct {
	pool *db.Pool
}

func NewPostgresJourneyTrackingReadModel(pool *db.Pool) *PostgresJourneyTrackingReadModel {
	return &PostgresJourneyTrackingReadModel{pool: pool}
}

type journeyRecommendationJSON struct {
	BusID                string  `json:"bus_id"`
	OperatorID           string  `json:"operator_id"`
	EstimatedArrival     int64   `json:"estimated_arrival"` // nanoseconds (types.Duration persisted as int64)
	Direction            int     `json:"direction"`
	IsWrongDirection     bool    `json:"is_wrong_direction"`
	ConfidenceLevel      float64 `json:"confidence_level"`
	DisplayText          string  `json:"display_text"`
	DistanceToOriginStop float64 `json:"distance_to_origin_stop"`
}

func (r *PostgresJourneyTrackingReadModel) GetJourney(
	ctx context.Context,
	req journeytrackingreadmodel.GetJourneyRequest,
) (journeytrackingreadmodel.JourneyTrackingDTO, bool, error) {
	ctx, cancel := context.WithTimeout(safeContext(ctx), 3*time.Second)
	defer cancel()

	if req.JourneyID == "" {
		return journeytrackingreadmodel.JourneyTrackingDTO{}, false, fmt.Errorf("journeyId required")
	}

	const query = `
		SELECT
			journey_id, user_id,
			status,
			origin_lat, origin_lon, origin_stop_id,
			destination_stop_id,
			required_direction,
			active_bus_id,
			last_proximity_level,
			decline_count,
			recommended_buses,
			boarding_window_started_at,
			boarded_at,
			estimated_duration_ns,
			expiration_time,
			created_at
		FROM journeys
		WHERE journey_id = $1
	`

	var dto journeytrackingreadmodel.JourneyTrackingDTO
	var (
		status                  int
		requiredDirection       int
		activeBusID             *string
		proximityLevel          int
		declineCount            int
		recommendedBusesJSON    []byte
		boardingWindowStartedAt *time.Time
		boardedAt               *time.Time
		estimatedDurationNs     int64
		expirationTime          time.Time
		createdAt               time.Time
	)

	err := r.pool.QueryRow(ctx, query, req.JourneyID).Scan(
		&dto.JourneyID,
		&dto.UserID,
		&status,
		&dto.OriginLat,
		&dto.OriginLon,
		&dto.OriginStopID,
		&dto.DestinationStopID,
		&requiredDirection,
		&activeBusID,
		&proximityLevel,
		&declineCount,
		&recommendedBusesJSON,
		&boardingWindowStartedAt,
		&boardedAt,
		&estimatedDurationNs,
		&expirationTime,
		&createdAt,
	)
	if err != nil {
		return journeytrackingreadmodel.JourneyTrackingDTO{}, false, nil
	}

	dto.Status = status
	dto.StatusName = journeyStatusName(status)
	dto.RequiredDirection = requiredDirection
	if activeBusID != nil {
		dto.ActiveBusID = *activeBusID
	}
	dto.ProximityLevel = proximityLevel
	dto.ProximityName = proximityName(proximityLevel)
	dto.DeclineCount = declineCount

	recs, err := decodeRecommendations(recommendedBusesJSON)
	if err != nil {
		return journeytrackingreadmodel.JourneyTrackingDTO{}, false, fmt.Errorf("failed to decode recommendations: %w", err)
	}
	dto.Recommendations = recs

	if boardingWindowStartedAt != nil {
		s := boardingWindowStartedAt.UTC().Format(time.RFC3339Nano)
		dto.BoardingWindowStart = &s
	}
	if boardedAt != nil {
		s := boardedAt.UTC().Format(time.RFC3339Nano)
		dto.BoardedAt = &s
	}

	// Convert ns -> ms for UI DTO
	dto.EstimatedDuration = estimatedDurationNs / 1_000_000

	dto.ExpiresAt = expirationTime.UTC().Format(time.RFC3339Nano)
	dto.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	dto.UpdatedAt = dto.CreatedAt

	return dto, true, nil
}

func (r *PostgresJourneyTrackingReadModel) GetActiveJourney(
	ctx context.Context,
	req journeytrackingreadmodel.GetActiveJourneyRequest,
) (journeytrackingreadmodel.JourneyTrackingDTO, bool, error) {
	ctx, cancel := context.WithTimeout(safeContext(ctx), 3*time.Second)
	defer cancel()

	if req.UserID == "" {
		return journeytrackingreadmodel.JourneyTrackingDTO{}, false, fmt.Errorf("userId required")
	}

	const query = `
		SELECT
			journey_id
		FROM journeys
		WHERE user_id = $1 AND status IN ($2, $3, $4, $5)
		ORDER BY created_at DESC
		LIMIT 1
	`

	var journeyID string
	err := r.pool.QueryRow(
		ctx,
		query,
		req.UserID,
		int(enums.JourneyStatusSearching),
		int(enums.JourneyStatusTracking),
		int(enums.JourneyStatusBoardingPrompt),
		int(enums.JourneyStatusBoarded),
	).Scan(&journeyID)
	if err != nil {
		return journeytrackingreadmodel.JourneyTrackingDTO{}, false, nil
	}

	return r.GetJourney(ctx, journeytrackingreadmodel.GetJourneyRequest{JourneyID: journeyID})
}

func (r *PostgresJourneyTrackingReadModel) ListJourneyHistory(
	ctx context.Context,
	req journeytrackingreadmodel.ListJourneyHistoryRequest,
) ([]journeytrackingreadmodel.JourneyHistoryDTO, error) {
	ctx, cancel := context.WithTimeout(safeContext(ctx), 3*time.Second)
	defer cancel()

	if req.UserID == "" {
		return nil, fmt.Errorf("userId required")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	// Best-effort history: journeys table does not currently persist completed_at.
	// We return terminal journeys ordered by created_at.
	const query = `
		SELECT
			journey_id,
			status,
			origin_stop_id,
			destination_stop_id,
			active_bus_id,
			boarded_at,
			created_at
		FROM journeys
		WHERE user_id = $1 AND status IN ($2, $3, $4)
		ORDER BY created_at DESC
		LIMIT $5
	`

	rows, err := r.pool.Query(
		ctx,
		query,
		req.UserID,
		int(enums.JourneyStatusCompleted),
		int(enums.JourneyStatusCancelled),
		int(enums.JourneyStatusExpired),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query journey history: %w", err)
	}
	defer rows.Close()

	out := make([]journeytrackingreadmodel.JourneyHistoryDTO, 0)
	for rows.Next() {
		var (
			dto       journeytrackingreadmodel.JourneyHistoryDTO
			status    int
			activeBus *string
			boardedAt *time.Time
			createdAt time.Time
		)
		if err := rows.Scan(
			&dto.JourneyID,
			&status,
			&dto.OriginStopID,
			&dto.DestinationStopID,
			&activeBus,
			&boardedAt,
			&createdAt,
		); err != nil {
			continue
		}

		dto.Status = status
		dto.StatusName = journeyStatusName(status)
		if activeBus != nil {
			dto.BusID = *activeBus
		}
		if boardedAt != nil {
			dto.BoardedAt = boardedAt.UTC().Format(time.RFC3339Nano)
		}
		dto.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)

		out = append(out, dto)
	}

	return out, nil
}

func decodeRecommendations(data []byte) ([]journeytrackingreadmodel.BusRecommendationDTO, error) {
	if len(data) == 0 {
		return []journeytrackingreadmodel.BusRecommendationDTO{}, nil
	}

	var raw []journeyRecommendationJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	out := make([]journeytrackingreadmodel.BusRecommendationDTO, 0, len(raw))
	for _, r := range raw {
		out = append(out, journeytrackingreadmodel.BusRecommendationDTO{
			BusID:            r.BusID,
			OperatorID:       r.OperatorID,
			EstimatedArrival: r.EstimatedArrival / 1_000_000, // ns -> ms
			DistanceMeters:   r.DistanceToOriginStop,
			Direction:        r.Direction,
			IsWrongDirection: r.IsWrongDirection,
			ConfidenceLevel:  r.ConfidenceLevel,
			DisplayText:      r.DisplayText,
		})
	}

	return out, nil
}

func journeyStatusName(status int) string {
	switch enums.JourneyStatus(status) {
	case enums.JourneyStatusSearching:
		return "Searching"
	case enums.JourneyStatusTracking:
		return "Tracking"
	case enums.JourneyStatusBoardingPrompt:
		return "BoardingPrompt"
	case enums.JourneyStatusBoarded:
		return "Boarded"
	case enums.JourneyStatusCompleted:
		return "Completed"
	case enums.JourneyStatusCancelled:
		return "Cancelled"
	case enums.JourneyStatusExpired:
		return "Expired"
	default:
		return "Unknown"
	}
}

func proximityName(level int) string {
	switch enums.ProximityLevel(level) {
	case enums.ProximityLevelNone:
		return "None"
	case enums.ProximityLevelApproaching:
		return "Approaching"
	case enums.ProximityLevelNearby:
		return "Nearby"
	case enums.ProximityLevelArrived:
		return "Arrived"
	default:
		return "Unknown"
	}
}

var _ journeytrackingreadmodel.JourneyTrackingReader = (*PostgresJourneyTrackingReadModel)(nil)
