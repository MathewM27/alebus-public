package mqtt

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// ─────────────────────────────────────────────────────────────────────────────
// Heartbeat — Redis Liveness Indicator
// ─────────────────────────────────────────────────────────────────────────────
//
// Per Phase 3 blueprint:
//   - Heartbeat key indicates active ingestion
//   - TTL-based expiry detects ingestor failure
//   - Downstream services can check liveness
//
// ─────────────────────────────────────────────────────────────────────────────

// HeartbeatConfig configures the heartbeat.
type HeartbeatConfig struct {
	// Key is the Redis key for the heartbeat.
	// Default: "ingestor:heartbeat"
	Key string

	// TTL is the time-to-live for the heartbeat key.
	// Should be longer than the update interval to avoid false expiry.
	// Default: 30s
	TTL time.Duration

	// UpdateInterval is how often to touch the heartbeat.
	// Should be shorter than TTL/2 to ensure continuous presence.
	// Default: 10s
	UpdateInterval time.Duration
}

// DefaultHeartbeatConfig returns sensible defaults.
func DefaultHeartbeatConfig() HeartbeatConfig {
	return HeartbeatConfig{
		Key:            "ingestor:heartbeat",
		TTL:            30 * time.Second,
		UpdateInterval: 10 * time.Second,
	}
}

// HeartbeatValue is the structure stored in the heartbeat key.
type HeartbeatValue struct {
	Timestamp      int64   `json:"timestamp"`        // Unix milliseconds
	ClientID       string  `json:"client_id"`        // MQTT client ID
	Processed      uint64  `json:"processed"`        // Total messages processed
	MessagesPerSec float64 `json:"messages_per_sec"` // Recent throughput
}

// Heartbeat manages the Redis heartbeat for liveness detection.
type Heartbeat struct {
	config   HeartbeatConfig
	client   redis.Cmdable
	clientID string

	// Stats provider
	statsProvider func() HeartbeatStats

	// Lifecycle
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started atomic.Bool
	stopped atomic.Bool

	// Internal state
	lastProcessed  uint64
	lastUpdateTime time.Time
}

// HeartbeatStats provides statistics for the heartbeat value.
type HeartbeatStats struct {
	Processed uint64
}

// NewHeartbeat creates a new heartbeat manager.
//
// Parameters:
//   - cfg: heartbeat configuration
//   - client: Redis client
//   - clientID: MQTT client ID for identification
//   - statsProvider: function that returns current stats (can be nil)
func NewHeartbeat(cfg HeartbeatConfig, client redis.Cmdable, clientID string, statsProvider func() HeartbeatStats) *Heartbeat {
	if cfg.Key == "" {
		cfg.Key = "ingestor:heartbeat"
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 30 * time.Second
	}
	if cfg.UpdateInterval <= 0 {
		cfg.UpdateInterval = 10 * time.Second
	}

	return &Heartbeat{
		config:        cfg,
		client:        client,
		clientID:      clientID,
		statsProvider: statsProvider,
	}
}

// SetStatsProvider sets or updates the stats provider function.
// This is useful when the stats source isn't available at construction time.
func (h *Heartbeat) SetStatsProvider(provider func() HeartbeatStats) {
	h.statsProvider = provider
}

// Start begins the heartbeat update loop.
//
// The heartbeat will be updated every UpdateInterval until Stop() is called
// or the context is cancelled.
func (h *Heartbeat) Start(ctx context.Context) {
	if h.started.Swap(true) {
		return // Already started
	}

	h.ctx, h.cancel = context.WithCancel(ctx)
	h.lastUpdateTime = time.Now()

	h.wg.Add(1)
	go h.loop()
}

// Stop stops the heartbeat update loop.
//
// The heartbeat key is NOT deleted on stop — it will expire naturally.
// This allows brief restarts without falsely indicating failure.
func (h *Heartbeat) Stop() {
	if h.stopped.Swap(true) {
		return // Already stopped
	}

	if h.cancel != nil {
		h.cancel()
	}
	h.wg.Wait()
}

// Touch updates the heartbeat key immediately.
//
// This can be called in addition to the automatic update loop for
// more frequent updates during high activity.
func (h *Heartbeat) Touch(ctx context.Context) error {
	now := time.Now()

	// Calculate throughput
	var processed uint64
	var throughput float64

	if h.statsProvider != nil {
		stats := h.statsProvider()
		processed = stats.Processed

		elapsed := now.Sub(h.lastUpdateTime).Seconds()
		if elapsed > 0 && h.lastProcessed > 0 {
			throughput = float64(processed-h.lastProcessed) / elapsed
		}
	}

	// Build value
	value := fmt.Sprintf(`{"timestamp":%d,"client_id":"%s","processed":%d,"messages_per_sec":%.2f}`,
		now.UnixMilli(),
		h.clientID,
		processed,
		throughput,
	)

	// Set with TTL
	err := h.client.Set(ctx, h.config.Key, value, h.config.TTL).Err()
	if err != nil {
		return fmt.Errorf("heartbeat touch failed: %w", err)
	}

	// Update tracking
	h.lastProcessed = processed
	h.lastUpdateTime = now

	return nil
}

// IsAlive checks if the heartbeat key exists and is not expired.
// This can be used by external services to check ingestor liveness.
func (h *Heartbeat) IsAlive(ctx context.Context) (bool, error) {
	result, err := h.client.Exists(ctx, h.config.Key).Result()
	if err != nil {
		return false, fmt.Errorf("heartbeat check failed: %w", err)
	}
	return result > 0, nil
}

// GetValue retrieves the current heartbeat value.
func (h *Heartbeat) GetValue(ctx context.Context) (*HeartbeatValue, error) {
	data, err := h.client.Get(ctx, h.config.Key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Key doesn't exist
		}
		return nil, fmt.Errorf("heartbeat get failed: %w", err)
	}

	// Parse JSON manually for simplicity
	var val HeartbeatValue
	// Simple JSON parsing (avoiding full unmarshal for performance)
	_, _ = fmt.Sscanf(string(data), `{"timestamp":%d,"client_id":"%s","processed":%d,"messages_per_sec":%f}`,
		&val.Timestamp, &val.ClientID, &val.Processed, &val.MessagesPerSec)

	return &val, nil
}

// TTL returns the remaining TTL of the heartbeat key.
func (h *Heartbeat) TTL(ctx context.Context) (time.Duration, error) {
	ttl, err := h.client.TTL(ctx, h.config.Key).Result()
	if err != nil {
		return 0, fmt.Errorf("heartbeat TTL check failed: %w", err)
	}
	return ttl, nil
}

// Key returns the heartbeat key name.
func (h *Heartbeat) Key() string {
	return h.config.Key
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal
// ─────────────────────────────────────────────────────────────────────────────

func (h *Heartbeat) loop() {
	defer h.wg.Done()

	// Initial touch
	_ = h.Touch(h.ctx)

	ticker := time.NewTicker(h.config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			if err := h.Touch(h.ctx); err != nil {
				// Best-effort: ignore and retry on next tick.
				_ = err
			}
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Null Heartbeat (for testing without Redis)
// ─────────────────────────────────────────────────────────────────────────────

// NullHeartbeat is a no-op implementation for testing.
type NullHeartbeat struct{}

// Start is a no-op.
func (NullHeartbeat) Start(ctx context.Context) {}

// Stop is a no-op.
func (NullHeartbeat) Stop() {}

// Touch is a no-op.
func (NullHeartbeat) Touch(ctx context.Context) error { return nil }

// IsAlive always returns true.
func (NullHeartbeat) IsAlive(ctx context.Context) (bool, error) { return true, nil }
