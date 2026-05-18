package db_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/MathewM27/busTrack-alebus/infrastructure/db"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestDatabaseURL returns the database URL for testing.
// Falls back to a default local development URL if not set.
func getTestDatabaseURL() string {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://alebus:alebus@localhost:5432/alebus?sslmode=disable"
	}
	return url
}

func TestNewPool_Success(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPoolFromURL(ctx, getTestDatabaseURL())
	require.NoError(t, err)
	require.NotNil(t, pool)
	defer pool.Close()

	// Verify connection is alive
	err = pool.Ping(ctx)
	assert.NoError(t, err)

	// Verify stats are available
	stats := pool.Stats()
	assert.NotNil(t, stats)
}

func TestNewPool_InvalidURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewPoolFromURL(ctx, "invalid://url")
	assert.Error(t, err)
}

func TestNewPool_EmptyURL(t *testing.T) {
	ctx := context.Background()

	cfg := db.Config{
		DatabaseURL: "",
	}

	_, err := db.NewPool(ctx, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_URL is required")
}

func TestWithTransaction_Commit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPoolFromURL(ctx, getTestDatabaseURL())
	require.NoError(t, err)
	defer pool.Close()

	// Create a temporary table for testing
	_, err = pool.Pool().Exec(ctx, `
		CREATE TEMPORARY TABLE tx_test (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	// Execute transaction that should commit
	err = pool.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO tx_test (value) VALUES ($1)", "test_value")
		return err
	})
	require.NoError(t, err)

	// Verify the data was committed
	var value string
	err = pool.Pool().QueryRow(ctx, "SELECT value FROM tx_test WHERE value = $1", "test_value").Scan(&value)
	assert.NoError(t, err)
	assert.Equal(t, "test_value", value)
}

func TestWithTransaction_Rollback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPoolFromURL(ctx, getTestDatabaseURL())
	require.NoError(t, err)
	defer pool.Close()

	// Create a temporary table for testing
	_, err = pool.Pool().Exec(ctx, `
		CREATE TEMPORARY TABLE tx_rollback_test (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	testErr := errors.New("intentional error")

	// Execute transaction that should rollback
	err = pool.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO tx_rollback_test (value) VALUES ($1)", "should_not_exist")
		if err != nil {
			return err
		}
		return testErr // Force rollback
	})
	assert.ErrorIs(t, err, testErr)

	// Verify the data was NOT committed
	var count int
	err = pool.Pool().QueryRow(ctx, "SELECT COUNT(*) FROM tx_rollback_test").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestWithTransaction_Panic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPoolFromURL(ctx, getTestDatabaseURL())
	require.NoError(t, err)
	defer pool.Close()

	// Create a temporary table for testing
	_, err = pool.Pool().Exec(ctx, `
		CREATE TEMPORARY TABLE tx_panic_test (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	// Execute transaction that panics
	assert.Panics(t, func() {
		_ = pool.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
			_, _ = tx.Exec(ctx, "INSERT INTO tx_panic_test (value) VALUES ($1)", "panic_value")
			panic("intentional panic")
		})
	})

	// Verify the data was NOT committed (rollback on panic)
	var count int
	err = pool.Pool().QueryRow(ctx, "SELECT COUNT(*) FROM tx_panic_test").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestPool_Stats(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPoolFromURL(ctx, getTestDatabaseURL())
	require.NoError(t, err)
	defer pool.Close()

	stats := pool.Stats()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.TotalConns(), int32(0))
}
