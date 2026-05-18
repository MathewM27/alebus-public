package redis

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// ─────────────────────────────────────────────────────────────────────────────
// Sentinel Client - Phase 3: High Availability
// ─────────────────────────────────────────────────────────────────────────────
//
// The SentinelClient provides Redis connectivity through Sentinel for automatic
// failover. It handles:
//   - Master discovery through Sentinel
//   - Automatic failover when master goes down
//   - Connection pooling with reconnection
//
// Usage:
//   cfg := SentinelConfig{
//       MasterName:    "mymaster",
//       SentinelAddrs: []string{"localhost:26379", "localhost:26380", "localhost:26381"},
//   }
//   client, err := NewSentinelClient(ctx, cfg)

// SentinelConfig holds configuration for Sentinel-based Redis client.
type SentinelConfig struct {
	// MasterName is the name of the master set in Sentinel config.
	MasterName string

	// SentinelAddrs is the list of Sentinel addresses (host:port).
	SentinelAddrs []string

	// Password is the Redis password (if any).
	Password string

	// SentinelPassword is the Sentinel password (if different from Redis).
	SentinelPassword string

	// DB is the Redis database number.
	DB int

	// PoolSize is the maximum number of connections in the pool.
	PoolSize int

	// MinIdleConns is the minimum number of idle connections.
	MinIdleConns int

	// DialTimeout is the timeout for establishing connections.
	DialTimeout time.Duration

	// ReadTimeout is the timeout for read operations.
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for write operations.
	WriteTimeout time.Duration
}

// DefaultSentinelConfig returns a SentinelConfig with sensible defaults.
func DefaultSentinelConfig() SentinelConfig {
	masterName := os.Getenv("REDIS_MASTER_NAME")
	if masterName == "" {
		masterName = "mymaster"
	}

	sentinelAddrs := os.Getenv("REDIS_SENTINEL_ADDRS")
	addrs := []string{"localhost:26379", "localhost:26380", "localhost:26381"}
	if sentinelAddrs != "" {
		addrs = strings.Split(sentinelAddrs, ",")
	}

	return SentinelConfig{
		MasterName:       masterName,
		SentinelAddrs:    addrs,
		Password:         os.Getenv("REDIS_PASSWORD"),
		SentinelPassword: os.Getenv("REDIS_SENTINEL_PASSWORD"),
		DB:               0,
		PoolSize:         10,
		MinIdleConns:     2,
		DialTimeout:      5 * time.Second,
		ReadTimeout:      3 * time.Second,
		WriteTimeout:     3 * time.Second,
	}
}

// SentinelClient wraps a Sentinel-backed Redis client.
type SentinelClient struct {
	client     *redis.Client
	config     SentinelConfig
	failovers  int64 // Count of observed failovers
	lastMaster string
}

// NewSentinelClient creates a new Sentinel-backed Redis client.
func NewSentinelClient(ctx context.Context, cfg SentinelConfig) (*SentinelClient, error) {
	opts := &redis.FailoverOptions{
		MasterName:       cfg.MasterName,
		SentinelAddrs:    cfg.SentinelAddrs,
		Password:         cfg.Password,
		SentinelPassword: cfg.SentinelPassword,
		DB:               cfg.DB,
		PoolSize:         cfg.PoolSize,
		MinIdleConns:     cfg.MinIdleConns,
		DialTimeout:      cfg.DialTimeout,
		ReadTimeout:      cfg.ReadTimeout,
		WriteTimeout:     cfg.WriteTimeout,
	}

	client := redis.NewFailoverClient(opts)

	// Verify connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis via Sentinel: %w", err)
	}

	sc := &SentinelClient{
		client: client,
		config: cfg,
	}

	// Get initial master address
	sc.updateMasterAddress(ctx)

	return sc, nil
}

// updateMasterAddress queries Sentinel for current master.
func (c *SentinelClient) updateMasterAddress(ctx context.Context) {
	// Create a temporary Sentinel client
	sentinel := redis.NewSentinelClient(&redis.Options{
		Addr: c.config.SentinelAddrs[0],
	})
	defer sentinel.Close()

	addr, err := sentinel.GetMasterAddrByName(ctx, c.config.MasterName).Result()
	if err == nil && len(addr) == 2 {
		newMaster := addr[0] + ":" + addr[1]
		if c.lastMaster != "" && c.lastMaster != newMaster {
			c.failovers++
		}
		c.lastMaster = newMaster
	}
}

// Close closes the Redis client connection.
func (c *SentinelClient) Close() error {
	return c.client.Close()
}

// Ping verifies the Redis connection is alive.
func (c *SentinelClient) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Underlying returns the underlying go-redis client.
func (c *SentinelClient) Underlying() *redis.Client {
	return c.client
}

// FailoverCount returns the number of observed failovers.
func (c *SentinelClient) FailoverCount() int64 {
	return c.failovers
}

// CurrentMaster returns the current master address.
func (c *SentinelClient) CurrentMaster() string {
	return c.lastMaster
}

// RefreshMaster re-queries Sentinel for the current master.
func (c *SentinelClient) RefreshMaster(ctx context.Context) {
	c.updateMasterAddress(ctx)
}

// ─────────────────────────────────────────────────────────────────────────────
// Universal Client Factory
// ─────────────────────────────────────────────────────────────────────────────

// ClientMode indicates the Redis client mode.
type ClientMode int

const (
	// ClientModeStandalone uses a single Redis server.
	ClientModeStandalone ClientMode = iota

	// ClientModeSentinel uses Redis Sentinel for HA.
	ClientModeSentinel
)

// UniversalConfig can configure either standalone or Sentinel mode.
type UniversalConfig struct {
	Mode ClientMode

	// Standalone config
	Addr     string
	Password string
	DB       int

	// Sentinel config
	MasterName       string
	SentinelAddrs    []string
	SentinelPassword string

	// Shared settings
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DefaultUniversalConfig returns config based on environment.
// If REDIS_SENTINEL_ADDRS is set, uses Sentinel mode.
func DefaultUniversalConfig() UniversalConfig {
	sentinelAddrs := os.Getenv("REDIS_SENTINEL_ADDRS")

	cfg := UniversalConfig{
		Password:     os.Getenv("REDIS_PASSWORD"),
		PoolSize:     10,
		MinIdleConns: 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	if sentinelAddrs != "" {
		cfg.Mode = ClientModeSentinel
		cfg.MasterName = os.Getenv("REDIS_MASTER_NAME")
		if cfg.MasterName == "" {
			cfg.MasterName = "mymaster"
		}
		cfg.SentinelAddrs = strings.Split(sentinelAddrs, ",")
		cfg.SentinelPassword = os.Getenv("REDIS_SENTINEL_PASSWORD")
	} else {
		cfg.Mode = ClientModeStandalone
		cfg.Addr = os.Getenv("REDIS_URL")
		if cfg.Addr == "" {
			cfg.Addr = "127.0.0.1:6379"
		}
	}

	return cfg
}

// UniversalClient wraps either standalone or Sentinel client.
type UniversalClient struct {
	client redis.UniversalClient
	mode   ClientMode
}

// NewUniversalClient creates a client based on the config mode.
func NewUniversalClient(ctx context.Context, cfg UniversalConfig) (*UniversalClient, error) {
	var client redis.UniversalClient

	switch cfg.Mode {
	case ClientModeSentinel:
		client = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:       cfg.MasterName,
			SentinelAddrs:    cfg.SentinelAddrs,
			Password:         cfg.Password,
			SentinelPassword: cfg.SentinelPassword,
			DB:               cfg.DB,
			PoolSize:         cfg.PoolSize,
			MinIdleConns:     cfg.MinIdleConns,
			DialTimeout:      cfg.DialTimeout,
			ReadTimeout:      cfg.ReadTimeout,
			WriteTimeout:     cfg.WriteTimeout,
		})
	default:
		client = redis.NewClient(&redis.Options{
			Addr:         cfg.Addr,
			Password:     cfg.Password,
			DB:           cfg.DB,
			PoolSize:     cfg.PoolSize,
			MinIdleConns: cfg.MinIdleConns,
			DialTimeout:  cfg.DialTimeout,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
		})
	}

	// Verify connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	return &UniversalClient{
		client: client,
		mode:   cfg.Mode,
	}, nil
}

// Close closes the client connection.
func (c *UniversalClient) Close() error {
	return c.client.Close()
}

// Ping verifies the connection.
func (c *UniversalClient) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Mode returns the client mode.
func (c *UniversalClient) Mode() ClientMode {
	return c.mode
}

// Client returns the underlying universal client.
func (c *UniversalClient) Client() redis.UniversalClient {
	return c.client
}

// AsClient returns a *Client wrapper if in standalone mode.
// Returns nil if in Sentinel mode (use Client() instead).
func (c *UniversalClient) AsClient() *Client {
	if c.mode == ClientModeStandalone {
		if client, ok := c.client.(*redis.Client); ok {
			return &Client{client: client}
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Failover Listener
// ─────────────────────────────────────────────────────────────────────────────

// FailoverEvent represents a Sentinel failover event.
type FailoverEvent struct {
	OldMaster string
	NewMaster string
	Timestamp time.Time
}

// FailoverListener listens for Sentinel failover events.
type FailoverListener struct {
	config  SentinelConfig
	events  chan FailoverEvent
	stopCh  chan struct{}
	running bool
}

// NewFailoverListener creates a new failover listener.
func NewFailoverListener(cfg SentinelConfig) *FailoverListener {
	return &FailoverListener{
		config: cfg,
		events: make(chan FailoverEvent, 10),
		stopCh: make(chan struct{}),
	}
}

// Start begins listening for failover events.
func (l *FailoverListener) Start(ctx context.Context) error {
	if l.running {
		return nil
	}
	l.running = true

	go l.listen(ctx)
	return nil
}

// Stop stops listening for failover events.
func (l *FailoverListener) Stop() {
	if l.running {
		close(l.stopCh)
		l.running = false
	}
}

// Events returns the channel of failover events.
func (l *FailoverListener) Events() <-chan FailoverEvent {
	return l.events
}

func (l *FailoverListener) listen(ctx context.Context) {
	// Connect to first available Sentinel
	var sentinel *redis.SentinelClient
	for _, addr := range l.config.SentinelAddrs {
		opts := &redis.Options{Addr: addr}
		if l.config.SentinelPassword != "" {
			opts.Password = l.config.SentinelPassword
		}
		sentinel = redis.NewSentinelClient(opts)
		if err := sentinel.Ping(ctx).Err(); err == nil {
			break
		}
		sentinel.Close()
		sentinel = nil
	}

	if sentinel == nil {
		return
	}
	defer sentinel.Close()

	// Subscribe to failover channel
	pubsub := sentinel.Subscribe(ctx, "+switch-master")
	defer pubsub.Close()

	for {
		select {
		case <-l.stopCh:
			return
		case <-ctx.Done():
			return
		case msg := <-pubsub.Channel():
			// Parse failover message
			// Format: <master-name> <old-ip> <old-port> <new-ip> <new-port>
			parts := strings.Split(msg.Payload, " ")
			if len(parts) >= 5 && parts[0] == l.config.MasterName {
				event := FailoverEvent{
					OldMaster: parts[1] + ":" + parts[2],
					NewMaster: parts[3] + ":" + parts[4],
					Timestamp: time.Now(),
				}
				select {
				case l.events <- event:
				default:
					// Channel full, drop event
				}
			}
		}
	}
}
