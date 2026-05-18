package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
	"github.com/MathewM27/busTrack-alebus/infrastructure/db"
)

// Infrastructure-level errors for User repository operations
var (
	ErrUserNotFound     = errors.New("user not found")
	ErrUserVersionStale = errors.New("user version conflict: stale version")
)

// PostgresUserRepository implements domain/repositories.UserRepository using Postgres
type PostgresUserRepository struct {
	pool *db.Pool
}

// NewPostgresUserRepository creates a new Postgres-backed UserRepository
func NewPostgresUserRepository(pool *db.Pool) *PostgresUserRepository {
	return &PostgresUserRepository{pool: pool}
}

// Save persists a User aggregate to Postgres.
// Uses UPSERT with optimistic locking via version check.
// Following error.md UPSERT template to avoid version conflict issues.
func (r *PostgresUserRepository) Save(ctx context.Context, user *aggregates.User) error {
	// Marshal saved locations to JSONB
	savedLocationsJSON, err := r.marshalSavedLocations(user.SavedLocations())
	if err != nil {
		return fmt.Errorf("failed to marshal saved locations: %w", err)
	}

	// UPSERT with version check for optimistic locking
	const query = `
		INSERT INTO users (
			user_id, email,
			subscription_status, subscription_plan, subscription_start_date, subscription_expiry_date,
			saved_locations,
			version, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id) DO UPDATE SET
			email = EXCLUDED.email,
			subscription_status = EXCLUDED.subscription_status,
			subscription_plan = EXCLUDED.subscription_plan,
			subscription_start_date = EXCLUDED.subscription_start_date,
			subscription_expiry_date = EXCLUDED.subscription_expiry_date,
			saved_locations = EXCLUDED.saved_locations,
			version = EXCLUDED.version
		WHERE users.version = $10
	`

	// Per error.md: Always increment version on save
	currentVersion := int(user.Version())
	newVersion := currentVersion + 1

	sub := user.Subscription()

	tag, err := r.pool.Exec(ctx, query,
		string(user.ID()),  // $1
		user.Email(),       // $2
		int(sub.Status),    // $3
		int(sub.Plan),      // $4
		sub.StartDate,      // $5
		sub.ExpiryDate,     // $6
		savedLocationsJSON, // $7
		newVersion,         // $8 version to persist
		user.CreatedAt(),   // $9
		currentVersion,     // $10 WHERE version = (for UPDATE check)
	)
	if err != nil {
		return fmt.Errorf("failed to save user: %w", err)
	}

	// Per error.md: If no rows affected, version conflict
	if tag.RowsAffected() == 0 {
		return ErrUserVersionStale
	}

	return nil
}

// FindByID loads a User aggregate by its ID.
// Uses RehydrateUser() to reconstruct the aggregate.
func (r *PostgresUserRepository) FindByID(ctx context.Context, id types.UserID) (*aggregates.User, error) {
	const query = `
		SELECT 
			user_id, email,
			subscription_status, subscription_plan, subscription_start_date, subscription_expiry_date,
			saved_locations,
			version, created_at
		FROM users
		WHERE user_id = $1
	`

	var (
		userID                 string
		email                  string
		subscriptionStatus     int
		subscriptionPlan       int
		subscriptionStartDate  time.Time
		subscriptionExpiryDate time.Time
		savedLocationsJSON     []byte
		version                int
		createdAt              time.Time
	)

	err := r.pool.QueryRow(ctx, query, string(id)).Scan(
		&userID,
		&email,
		&subscriptionStatus,
		&subscriptionPlan,
		&subscriptionStartDate,
		&subscriptionExpiryDate,
		&savedLocationsJSON,
		&version,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	// Unmarshal saved locations
	savedLocations, err := r.unmarshalSavedLocations(savedLocationsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal saved locations: %w", err)
	}

	// Build subscription value object
	subscription := valueobjects.Subscription{
		Status:     valueobjects.SubscriptionStatus(subscriptionStatus),
		Plan:       valueobjects.SubscriptionPlan(subscriptionPlan),
		StartDate:  subscriptionStartDate,
		ExpiryDate: subscriptionExpiryDate,
	}

	// Rehydrate aggregate
	user, err := aggregates.RehydrateUser(
		types.UserID(userID),
		email,
		subscription,
		savedLocations,
		createdAt,
		types.AggregateUserVersion(version),
		&noOpEventRecorder{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to rehydrate user: %w", err)
	}

	return user, nil
}

// JSON structure for saved locations JSONB serialization
type savedLocationJSON struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	StopID    *string `json:"stop_id,omitempty"`
}

// marshalSavedLocations converts domain saved locations to JSON
func (r *PostgresUserRepository) marshalSavedLocations(locs []valueobjects.SavedLocation) ([]byte, error) {
	if len(locs) == 0 {
		return []byte("[]"), nil
	}

	jsonLocs := make([]savedLocationJSON, len(locs))
	for i, loc := range locs {
		jsonLocs[i] = savedLocationJSON{
			Name:      loc.Name,
			Latitude:  loc.Location.Latitude,
			Longitude: loc.Location.Longitude,
		}
		if loc.StopID != nil {
			s := string(*loc.StopID)
			jsonLocs[i].StopID = &s
		}
	}

	return json.Marshal(jsonLocs)
}

// unmarshalSavedLocations converts JSON to domain saved locations
func (r *PostgresUserRepository) unmarshalSavedLocations(data []byte) ([]valueobjects.SavedLocation, error) {
	var jsonLocs []savedLocationJSON
	if err := json.Unmarshal(data, &jsonLocs); err != nil {
		return nil, err
	}

	locs := make([]valueobjects.SavedLocation, len(jsonLocs))
	for i, jl := range jsonLocs {
		locs[i] = valueobjects.SavedLocation{
			Name: jl.Name,
			Location: types.GeoLocation{
				Latitude:  jl.Latitude,
				Longitude: jl.Longitude,
			},
		}
		if jl.StopID != nil {
			stopID := types.StopID(*jl.StopID)
			locs[i].StopID = &stopID
		}
	}

	return locs, nil
}

// Compile-time interface satisfaction check
var _ interface {
	Save(ctx context.Context, user *aggregates.User) error
	FindByID(ctx context.Context, id types.UserID) (*aggregates.User, error)
} = (*PostgresUserRepository)(nil)
