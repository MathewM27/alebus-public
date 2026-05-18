// Package db provides database connection and transaction management for Postgres.
// This is infrastructure code - it must NOT contain business logic.
package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds database connection configuration.
type Config struct {
	DatabaseURL     string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// DefaultConfig returns a Config with sensible defaults.
// DatabaseURL is read from DATABASE_URL environment variable.
func DefaultConfig() Config {
	return Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		MaxConns:        10,
		MinConns:        2,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
	}
}

// Pool wraps pgxpool.Pool to provide a clean interface.
type Pool struct {
	pool *pgxpool.Pool
}

// NewPool creates a new connection pool with the given configuration.
func NewPool(ctx context.Context, cfg Config) (*Pool, error) {
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Pool{pool: pool}, nil
}

// NewPoolFromURL creates a new connection pool from a database URL string.
// This is a convenience function for simple use cases.
func NewPoolFromURL(ctx context.Context, databaseURL string) (*Pool, error) {
	cfg := DefaultConfig()
	cfg.DatabaseURL = databaseURL
	return NewPool(ctx, cfg)
}

// Close closes all connections in the pool.
func (p *Pool) Close() {
	if p.pool != nil {
		p.pool.Close()
	}
}

// Ping verifies the database connection is alive.
func (p *Pool) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

// Pool returns the underlying pgxpool.Pool for advanced use cases.
// Prefer using the provided methods when possible.
func (p *Pool) Pool() *pgxpool.Pool {
	return p.pool
}

// Stats returns connection pool statistics.
func (p *Pool) Stats() *pgxpool.Stat {
	return p.pool.Stat()
}

// Exec executes a SQL statement and returns the command tag.
// Implements the Querier interface.
func (p *Pool) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return p.pool.Exec(ctx, sql, arguments...)
}

// Query executes a SQL query and returns the resulting rows.
// Implements the Querier interface.
func (p *Pool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return p.pool.Query(ctx, sql, args...)
}

// QueryRow executes a SQL query that is expected to return at most one row.
// Implements the Querier interface.
func (p *Pool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return p.pool.QueryRow(ctx, sql, args...)
}
