package wire

import (
	"context"
	"fmt"

	"github.com/MathewM27/busTrack-alebus/infrastructure/db"
	"github.com/MathewM27/busTrack-alebus/infrastructure/repositories"
)

// PostgresInfra holds all Postgres-backed infrastructure components.
// This struct owns the lifecycle of the connection pool.
type PostgresInfra struct {
	Pool *db.Pool

	// Domain repositories
	RouteRepo   *repositories.PostgresRouteRepository
	JourneyRepo *repositories.PostgresJourneyRepository
	UserRepo    *repositories.PostgresUserRepository
	BusRepo     *repositories.PostgresBusRepository

	// Read models
	BusReadModel             *repositories.PostgresBusReadModel
	JourneyTrackingReadModel *repositories.PostgresJourneyTrackingReadModel
	StopGeoRepo              *repositories.PostgresStopGeoRepository
}

// NewPostgresInfra creates all Postgres-backed infrastructure.
// Returns an error if the database connection fails.
func NewPostgresInfra(ctx context.Context, cfg Config) (*PostgresInfra, error) {
	dbCfg := db.Config{
		DatabaseURL:     cfg.DatabaseURL,
		MaxConns:        10,
		MinConns:        2,
		MaxConnLifetime: cfg.DBTimeout * 6, // Connection lifetime
		MaxConnIdleTime: cfg.DBTimeout * 3, // Idle timeout
	}

	pool, err := db.NewPool(ctx, dbCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create database pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresInfra{
		Pool: pool,

		// Domain repositories
		RouteRepo:   repositories.NewPostgresRouteRepository(pool),
		JourneyRepo: repositories.NewPostgresJourneyRepository(pool),
		UserRepo:    repositories.NewPostgresUserRepository(pool),
		BusRepo:     repositories.NewPostgresBusRepository(pool),

		// Read models
		BusReadModel:             repositories.NewPostgresBusReadModel(pool),
		JourneyTrackingReadModel: repositories.NewPostgresJourneyTrackingReadModel(pool),
		StopGeoRepo:              repositories.NewPostgresStopGeoRepository(pool),
	}, nil
}

// Close releases all database resources.
func (p *PostgresInfra) Close() {
	if p.Pool != nil {
		p.Pool.Close()
	}
}
