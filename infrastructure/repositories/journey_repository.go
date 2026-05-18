package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
	"github.com/MathewM27/busTrack-alebus/infrastructure/db"
)

// Infrastructure-level errors for Journey repository operations
var (
	ErrJourneyNotFound     = errors.New("journey not found")
	ErrJourneyVersionStale = errors.New("journey version conflict: stale version")
)

// PostgresJourneyRepository implements domain/repositories.JourneyRepository using Postgres
type PostgresJourneyRepository struct {
	pool *db.Pool
}

// NewPostgresJourneyRepository creates a new Postgres-backed JourneyRepository
func NewPostgresJourneyRepository(pool *db.Pool) *PostgresJourneyRepository {
	return &PostgresJourneyRepository{pool: pool}
}

// Save persists a Journey aggregate to Postgres.
// Uses UPSERT with optimistic locking via version check.
// Following error.md UPSERT template to avoid version conflict issues.
func (r *PostgresJourneyRepository) Save(ctx context.Context, journey *aggregates.Journey) error {
	// Marshal recommendations to JSONB
	recommendationsJSON, err := r.marshalRecommendations(journey.RecommendedBuses())
	if err != nil {
		return fmt.Errorf("failed to marshal recommendations: %w", err)
	}

	// Handle nullable active_bus_id
	var activeBusID *string
	if journey.ActiveBusID() != "" {
		s := string(journey.ActiveBusID())
		activeBusID = &s
	}

	// UPSERT with version check for optimistic locking
	const query = `
		INSERT INTO journeys (
			journey_id, user_id,
			origin_lat, origin_lon, origin_stop_id, destination_stop_id,
			recommended_buses, active_bus_id,
			status, last_switch_reason, last_proximity_level, decline_count, required_direction,
			estimated_duration_ns, expiration_time, boarding_window_started_at, boarded_at,
			version, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		ON CONFLICT (journey_id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			origin_lat = EXCLUDED.origin_lat,
			origin_lon = EXCLUDED.origin_lon,
			origin_stop_id = EXCLUDED.origin_stop_id,
			destination_stop_id = EXCLUDED.destination_stop_id,
			recommended_buses = EXCLUDED.recommended_buses,
			active_bus_id = EXCLUDED.active_bus_id,
			status = EXCLUDED.status,
			last_switch_reason = EXCLUDED.last_switch_reason,
			last_proximity_level = EXCLUDED.last_proximity_level,
			decline_count = EXCLUDED.decline_count,
			required_direction = EXCLUDED.required_direction,
			estimated_duration_ns = EXCLUDED.estimated_duration_ns,
			expiration_time = EXCLUDED.expiration_time,
			boarding_window_started_at = EXCLUDED.boarding_window_started_at,
			boarded_at = EXCLUDED.boarded_at,
			version = EXCLUDED.version
		WHERE journeys.version = $20
	`

	// Per error.md: Always increment version on save
	currentVersion := int(journey.Version())
	newVersion := currentVersion + 1

	origin := journey.OriginLocation()

	tag, err := r.pool.Exec(ctx, query,
		string(journey.JourneyID()),         // $1
		string(journey.UserID()),            // $2
		origin.Latitude,                     // $3
		origin.Longitude,                    // $4
		string(journey.OriginStopID()),      // $5
		string(journey.DestinationStopID()), // $6
		recommendationsJSON,                 // $7
		activeBusID,                         // $8 (nullable)
		int(journey.Status()),               // $9
		0,                                   // $10 last_switch_reason (not exposed via getter)
		int(journey.LastProximityLevel()),   // $11
		journey.DeclineCount(),              // $12
		int(journey.RequiredDirection()),    // $13
		int64(journey.EstimatedDuration()),  // $14 duration as nanoseconds
		journey.ExpirationTime(),            // $15
		journey.BoardingWindowStartedAt(),   // $16 (nullable)
		journey.BoardedAt(),                 // $17 (nullable)
		newVersion,                          // $18 version to persist
		journey.CreatedAt(),                 // $19
		currentVersion,                      // $20 WHERE version = (for UPDATE check)
	)
	if err != nil {
		return fmt.Errorf("failed to save journey: %w", err)
	}

	// Per error.md: If no rows affected, version conflict
	if tag.RowsAffected() == 0 {
		return ErrJourneyVersionStale
	}

	return nil
}

// FindByID loads a Journey aggregate by its ID.
// Uses RehydrateJourney() to reconstruct the aggregate without time-based checks.
func (r *PostgresJourneyRepository) FindByID(ctx context.Context, id types.JourneyID) (*aggregates.Journey, error) {
	const query = `
		SELECT 
			journey_id, user_id,
			origin_lat, origin_lon, origin_stop_id, destination_stop_id,
			recommended_buses, active_bus_id,
			status, last_switch_reason, last_proximity_level, decline_count, required_direction,
			estimated_duration_ns, expiration_time, boarding_window_started_at, boarded_at,
			version, created_at
		FROM journeys
		WHERE journey_id = $1
	`

	return r.scanJourney(ctx, query, string(id))
}

// FindActiveByUserID finds the active journey for a user.
// Active means status is Searching, Tracking, BoardingPrompt, or Boarded.
func (r *PostgresJourneyRepository) FindActiveByUserID(ctx context.Context, userID types.UserID) (*aggregates.Journey, error) {
	const query = `
		SELECT 
			journey_id, user_id,
			origin_lat, origin_lon, origin_stop_id, destination_stop_id,
			recommended_buses, active_bus_id,
			status, last_switch_reason, last_proximity_level, decline_count, required_direction,
			estimated_duration_ns, expiration_time, boarding_window_started_at, boarded_at,
			version, created_at
		FROM journeys
		WHERE user_id = $1 AND status IN ($2, $3, $4, $5)
		ORDER BY created_at DESC
		LIMIT 1
	`

	return r.scanJourney(ctx, query,
		string(userID),
		int(enums.JourneyStatusSearching),
		int(enums.JourneyStatusTracking),
		int(enums.JourneyStatusBoardingPrompt),
		int(enums.JourneyStatusBoarded),
	)
}

// CountActiveByUserID counts active journeys for a user.
func (r *PostgresJourneyRepository) CountActiveByUserID(ctx context.Context, userID types.UserID) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM journeys
		WHERE user_id = $1 AND status IN ($2, $3, $4, $5)
	`

	var count int
	err := r.pool.QueryRow(ctx, query,
		string(userID),
		int(enums.JourneyStatusSearching),
		int(enums.JourneyStatusTracking),
		int(enums.JourneyStatusBoardingPrompt),
		int(enums.JourneyStatusBoarded),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active journeys: %w", err)
	}

	return count, nil
}

// scanJourney scans a journey from a query result
func (r *PostgresJourneyRepository) scanJourney(ctx context.Context, query string, args ...any) (*aggregates.Journey, error) {
	var (
		journeyID               string
		userID                  string
		originLat               float64
		originLon               float64
		originStopID            string
		destinationStopID       string
		recommendedBusesJSON    []byte
		activeBusID             *string
		status                  int
		lastSwitchReason        int
		lastProximityLevel      int
		declineCount            int
		requiredDirection       int
		estimatedDurationNs     int64
		expirationTime          time.Time
		boardingWindowStartedAt *time.Time
		boardedAt               *time.Time
		version                 int
		createdAt               time.Time
	)

	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&journeyID,
		&userID,
		&originLat,
		&originLon,
		&originStopID,
		&destinationStopID,
		&recommendedBusesJSON,
		&activeBusID,
		&status,
		&lastSwitchReason,
		&lastProximityLevel,
		&declineCount,
		&requiredDirection,
		&estimatedDurationNs,
		&expirationTime,
		&boardingWindowStartedAt,
		&boardedAt,
		&version,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrJourneyNotFound
		}
		return nil, fmt.Errorf("failed to query journey: %w", err)
	}

	// Unmarshal recommendations
	recommendations, err := r.unmarshalRecommendations(recommendedBusesJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal recommendations: %w", err)
	}

	// Handle nullable active_bus_id
	var busID types.BusID
	if activeBusID != nil {
		busID = types.BusID(*activeBusID)
	}

	// Rehydrate aggregate (bypasses time-based creation checks)
	journey, err := aggregates.RehydrateJourney(
		types.JourneyID(journeyID),
		types.UserID(userID),
		types.GeoLocation{Latitude: originLat, Longitude: originLon},
		types.StopID(originStopID),
		types.StopID(destinationStopID),
		recommendations,
		busID,
		enums.JourneyStatus(status),
		enums.ProximityLevel(lastProximityLevel),
		declineCount,
		types.Direction(requiredDirection),
		types.Duration(estimatedDurationNs),
		expirationTime,
		boardingWindowStartedAt,
		boardedAt,
		createdAt,
		types.AggregateJourneyVersion(version), // Pass the persisted version for optimistic locking
		&noOpEventRecorder{},                   // Required but won't emit during rehydration
	)
	if err != nil {
		return nil, fmt.Errorf("failed to rehydrate journey: %w", err)
	}

	return journey, nil
}

// JSON structures for recommendations JSONB serialization
type recommendationJSON struct {
	BusID                string          `json:"bus_id"`
	OperatorID           string          `json:"operator_id"`
	ActualRouteDistance  float64         `json:"actual_route_distance"`
	EstimatedArrival     int64           `json:"estimated_arrival"`
	JourneyInfo          journeyInfoJSON `json:"journey_info"`
	Direction            int             `json:"direction"`
	RequiredDirection    int             `json:"required_direction"`
	IsWrongDirection     bool            `json:"is_wrong_direction"`
	DisplayText          string          `json:"display_text"`
	ConfidenceLevel      float64         `json:"confidence_level"`
	RankingScore         float64         `json:"ranking_score"`
	DistanceToOriginStop float64         `json:"distance_to_origin_stop"`
	Confidence           float64         `json:"confidence"`
	Rank                 int             `json:"rank"`
}

type journeyInfoJSON struct {
	Type             string        `json:"type"`
	TotalDistance    float64       `json:"total_distance"`
	EstimatedTime    int64         `json:"estimated_time"`
	RequiresTerminal bool          `json:"requires_terminal"`
	TerminalWaitTime int64         `json:"terminal_wait_time"`
	Breakdown        breakdownJSON `json:"breakdown"`
}

type breakdownJSON struct {
	ToTerminal   int64 `json:"to_terminal"`
	AtTerminal   int64 `json:"at_terminal"`
	FromTerminal int64 `json:"from_terminal"`
}

// marshalRecommendations converts domain recommendations to JSON
func (r *PostgresJourneyRepository) marshalRecommendations(recs []valueobjects.EnhancedBusRecommendation) ([]byte, error) {
	if len(recs) == 0 {
		return []byte("[]"), nil
	}

	jsonRecs := make([]recommendationJSON, len(recs))
	for i, rec := range recs {
		jsonRecs[i] = recommendationJSON{
			BusID:               string(rec.BusID),
			OperatorID:          string(rec.OperatorID),
			ActualRouteDistance: float64(rec.ActualRouteDistance),
			EstimatedArrival:    int64(rec.EstimatedArrival),
			JourneyInfo: journeyInfoJSON{
				Type:             rec.JourneyInfo.Type,
				TotalDistance:    float64(rec.JourneyInfo.TotalDistance),
				EstimatedTime:    int64(rec.JourneyInfo.EstimatedTime),
				RequiresTerminal: rec.JourneyInfo.RequiresTerminal,
				TerminalWaitTime: int64(rec.JourneyInfo.TerminalWaitTime),
				Breakdown: breakdownJSON{
					ToTerminal:   int64(rec.JourneyInfo.Breakdown.ToTerminal),
					AtTerminal:   int64(rec.JourneyInfo.Breakdown.AtTerminal),
					FromTerminal: int64(rec.JourneyInfo.Breakdown.FromTerminal),
				},
			},
			Direction:            int(rec.Direction),
			RequiredDirection:    int(rec.RequiredDirection),
			IsWrongDirection:     rec.IsWrongDirection,
			DisplayText:          rec.DisplayText,
			ConfidenceLevel:      rec.ConfidenceLevel,
			RankingScore:         rec.RankingScore,
			DistanceToOriginStop: float64(rec.DistanceToOriginStop),
			Confidence:           rec.Confidence,
			Rank:                 rec.Rank,
		}
	}

	return json.Marshal(jsonRecs)
}

// unmarshalRecommendations converts JSON to domain recommendations
func (r *PostgresJourneyRepository) unmarshalRecommendations(data []byte) ([]valueobjects.EnhancedBusRecommendation, error) {
	var jsonRecs []recommendationJSON
	if err := json.Unmarshal(data, &jsonRecs); err != nil {
		return nil, err
	}

	recs := make([]valueobjects.EnhancedBusRecommendation, len(jsonRecs))
	for i, jr := range jsonRecs {
		recs[i] = valueobjects.EnhancedBusRecommendation{
			BusID:               types.BusID(jr.BusID),
			OperatorID:          types.OperatorID(jr.OperatorID),
			ActualRouteDistance: types.Distance(jr.ActualRouteDistance),
			EstimatedArrival:    types.Duration(jr.EstimatedArrival),
			JourneyInfo: valueobjects.BusJourneyInfo{
				Type:             jr.JourneyInfo.Type,
				TotalDistance:    types.Distance(jr.JourneyInfo.TotalDistance),
				EstimatedTime:    types.Duration(jr.JourneyInfo.EstimatedTime),
				RequiresTerminal: jr.JourneyInfo.RequiresTerminal,
				TerminalWaitTime: types.Duration(jr.JourneyInfo.TerminalWaitTime),
				Breakdown: valueobjects.JourneyBreakdown{
					ToTerminal:   types.Duration(jr.JourneyInfo.Breakdown.ToTerminal),
					AtTerminal:   types.Duration(jr.JourneyInfo.Breakdown.AtTerminal),
					FromTerminal: types.Duration(jr.JourneyInfo.Breakdown.FromTerminal),
				},
			},
			Direction:            types.Direction(jr.Direction),
			RequiredDirection:    types.Direction(jr.RequiredDirection),
			IsWrongDirection:     jr.IsWrongDirection,
			DisplayText:          jr.DisplayText,
			ConfidenceLevel:      jr.ConfidenceLevel,
			RankingScore:         jr.RankingScore,
			DistanceToOriginStop: types.Distance(jr.DistanceToOriginStop),
			Confidence:           jr.Confidence,
			Rank:                 jr.Rank,
		}
	}

	return recs, nil
}

// Compile-time interface satisfaction check
var _ interface {
	Save(ctx context.Context, journey *aggregates.Journey) error
	FindByID(ctx context.Context, id types.JourneyID) (*aggregates.Journey, error)
	FindActiveByUserID(ctx context.Context, userID types.UserID) (*aggregates.Journey, error)
	CountActiveByUserID(ctx context.Context, userID types.UserID) (int, error)
} = (*PostgresJourneyRepository)(nil)
