package mqtt

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// Heartbeat Construction Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestNewHeartbeat(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	cfg := DefaultHeartbeatConfig()
	hb := NewHeartbeat(cfg, client, "test-client", nil)

	require.NotNil(t, hb)
	assert.Equal(t, "ingestor:heartbeat", hb.Key())
}

func TestNewHeartbeat_DefaultConfig(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	// Empty config should use defaults
	cfg := HeartbeatConfig{}
	hb := NewHeartbeat(cfg, client, "test-client", nil)

	assert.Equal(t, "ingestor:heartbeat", hb.config.Key)
	assert.Equal(t, 30*time.Second, hb.config.TTL)
	assert.Equal(t, 10*time.Second, hb.config.UpdateInterval)
}

// ─────────────────────────────────────────────────────────────────────────────
// Touch Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestHeartbeat_Touch(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	cfg := HeartbeatConfig{
		Key: "test:heartbeat",
		TTL: 30 * time.Second,
	}
	hb := NewHeartbeat(cfg, client, "test-client-123", nil)

	ctx := context.Background()
	err = hb.Touch(ctx)
	require.NoError(t, err)

	// Verify key exists
	exists, err := client.Exists(ctx, "test:heartbeat").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists)

	// Verify TTL is set
	ttl, err := client.TTL(ctx, "test:heartbeat").Result()
	require.NoError(t, err)
	assert.True(t, ttl > 0 && ttl <= 30*time.Second)

	// Verify value contains expected fields
	val, err := client.Get(ctx, "test:heartbeat").Result()
	require.NoError(t, err)
	assert.Contains(t, val, "timestamp")
	assert.Contains(t, val, "test-client-123")
}

func TestHeartbeat_Touch_WithStats(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	cfg := DefaultHeartbeatConfig()
	statsProvider := func() HeartbeatStats {
		return HeartbeatStats{Processed: 1000}
	}
	hb := NewHeartbeat(cfg, client, "test-client", statsProvider)

	ctx := context.Background()
	err = hb.Touch(ctx)
	require.NoError(t, err)

	// Verify processed count is in value
	val, err := client.Get(ctx, cfg.Key).Result()
	require.NoError(t, err)
	assert.Contains(t, val, "1000")
}

// ─────────────────────────────────────────────────────────────────────────────
// Expiry Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestHeartbeat_Expires(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	cfg := HeartbeatConfig{
		Key: "test:heartbeat:expire",
		TTL: 100 * time.Millisecond, // Short TTL for testing
	}
	hb := NewHeartbeat(cfg, client, "test-client", nil)

	ctx := context.Background()
	err = hb.Touch(ctx)
	require.NoError(t, err)

	// Key should exist immediately
	alive, err := hb.IsAlive(ctx)
	require.NoError(t, err)
	assert.True(t, alive)

	// Fast-forward time in miniredis
	mr.FastForward(200 * time.Millisecond)

	// Key should be expired
	alive, err = hb.IsAlive(ctx)
	require.NoError(t, err)
	assert.False(t, alive)
}

// ─────────────────────────────────────────────────────────────────────────────
// IsAlive Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestHeartbeat_IsAlive(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	cfg := DefaultHeartbeatConfig()
	hb := NewHeartbeat(cfg, client, "test-client", nil)

	ctx := context.Background()

	// Initially not alive
	alive, err := hb.IsAlive(ctx)
	require.NoError(t, err)
	assert.False(t, alive)

	// Touch to create
	err = hb.Touch(ctx)
	require.NoError(t, err)

	// Now alive
	alive, err = hb.IsAlive(ctx)
	require.NoError(t, err)
	assert.True(t, alive)
}

// ─────────────────────────────────────────────────────────────────────────────
// TTL Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestHeartbeat_TTL(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	cfg := HeartbeatConfig{
		Key: "test:heartbeat",
		TTL: 30 * time.Second,
	}
	hb := NewHeartbeat(cfg, client, "test-client", nil)

	ctx := context.Background()
	err = hb.Touch(ctx)
	require.NoError(t, err)

	ttl, err := hb.TTL(ctx)
	require.NoError(t, err)
	assert.True(t, ttl > 0)
	assert.True(t, ttl <= 30*time.Second)
}

// ─────────────────────────────────────────────────────────────────────────────
// Start/Stop Loop Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestHeartbeat_StartStop(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	cfg := HeartbeatConfig{
		Key:            "test:heartbeat",
		TTL:            5 * time.Second,
		UpdateInterval: 50 * time.Millisecond, // Fast for testing
	}
	hb := NewHeartbeat(cfg, client, "test-client", nil)

	ctx := context.Background()
	hb.Start(ctx)

	// Wait for at least one update
	time.Sleep(100 * time.Millisecond)

	// Key should exist
	alive, err := hb.IsAlive(ctx)
	require.NoError(t, err)
	assert.True(t, alive)

	hb.Stop()
}

func TestHeartbeat_DoubleStart(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	cfg := DefaultHeartbeatConfig()
	hb := NewHeartbeat(cfg, client, "test-client", nil)

	ctx := context.Background()
	hb.Start(ctx)
	hb.Start(ctx) // Should be no-op
	hb.Stop()
}

func TestHeartbeat_DoubleStop(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	cfg := DefaultHeartbeatConfig()
	hb := NewHeartbeat(cfg, client, "test-client", nil)

	ctx := context.Background()
	hb.Start(ctx)
	hb.Stop()
	hb.Stop() // Should be no-op
}

// ─────────────────────────────────────────────────────────────────────────────
// Multiple Updates Test
// ─────────────────────────────────────────────────────────────────────────────

func TestHeartbeat_MultipleUpdates(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	processedCount := uint64(0)
	cfg := HeartbeatConfig{
		Key:            "test:heartbeat",
		TTL:            5 * time.Second,
		UpdateInterval: 30 * time.Millisecond,
	}
	hb := NewHeartbeat(cfg, client, "test-client", func() HeartbeatStats {
		processedCount += 100 // Increment each call
		return HeartbeatStats{Processed: processedCount}
	})

	ctx := context.Background()
	hb.Start(ctx)

	// Wait for multiple updates
	time.Sleep(100 * time.Millisecond)

	hb.Stop()

	// Should have processed count > 0
	val, err := client.Get(ctx, cfg.Key).Result()
	require.NoError(t, err)
	assert.Contains(t, val, "processed")
}

// ─────────────────────────────────────────────────────────────────────────────
// Null Heartbeat Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestNullHeartbeat(t *testing.T) {
	hb := NullHeartbeat{}

	ctx := context.Background()

	// All methods should be no-ops
	hb.Start(ctx)
	hb.Stop()

	err := hb.Touch(ctx)
	assert.NoError(t, err)

	alive, err := hb.IsAlive(ctx)
	assert.NoError(t, err)
	assert.True(t, alive)
}

// ─────────────────────────────────────────────────────────────────────────────
// Default Config Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestDefaultHeartbeatConfig(t *testing.T) {
	cfg := DefaultHeartbeatConfig()

	assert.Equal(t, "ingestor:heartbeat", cfg.Key)
	assert.Equal(t, 30*time.Second, cfg.TTL)
	assert.Equal(t, 10*time.Second, cfg.UpdateInterval)
}
